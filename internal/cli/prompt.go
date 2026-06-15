package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// stdinIsTTY reports whether stdin is an interactive terminal. It is a var so
// tests can simulate (or suppress) an interactive prompt.
var stdinIsTTY = func() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// promptYesNo writes question to w and reads a yes/no answer from r. Empty
// input returns def; only y/yes (or n/no) override it, anything else is "no".
func promptYesNo(w io.Writer, r io.Reader, question string, def bool) (bool, error) {
	hint := " [Y/n] "
	if !def {
		hint = " [y/N] "
	}
	if _, err := fmt.Fprint(w, question+hint); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// promptBranch asks the user to choose a base branch. candidates are shown as a
// numbered list with def pre-selected (def should appear in candidates). Empty
// input returns def; an in-range number selects that candidate; any other text
// is returned verbatim as a branch name so a not-yet-created base can be named.
// EOF (a closed stdin) returns def.
func promptBranch(w io.Writer, r io.Reader, candidates []string, def string) (string, error) {
	if _, err := fmt.Fprintln(w, "Default base branch for new worktrees:"); err != nil {
		return "", err
	}
	for i, c := range candidates {
		marker := ""
		if c == def {
			marker = "  (default)"
		}
		if _, err := fmt.Fprintf(w, "  %d) %s%s\n", i+1, c, marker); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprintf(w, "Enter a number or branch name [%s]: ", def); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return def, nil
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n >= 1 && n <= len(candidates) {
			return candidates[n-1], nil
		}
		// Out-of-range number: keep the default rather than inventing a branch
		// literally named e.g. "9".
		return def, nil
	}
	return choice, nil
}

// registerOutcome reports what promptRegister did.
type registerOutcome int

const (
	registeredNow     registerOutcome = iota // newly registered this call
	alreadyRegistered                        // the repo was already a project
	registerDeclined                         // declined, or no interactive terminal
)

// promptRegister offers to register the git repo at root as a project. On an
// interactive terminal it asks first, naming the project and its path so a
// wrong-folder run is obvious; with assumeYes it registers without asking. When
// stdin is not a terminal and assumeYes is unset it leaves the repo unregistered
// (registerDeclined) rather than silently mutating global state.
func promptRegister(cmd *cobra.Command, root string, assumeYes bool) (*registry.Project, registerOutcome, error) {
	rp, err := config.RegistryPath()
	if err != nil {
		return nil, registerDeclined, err
	}
	store, err := registry.Load(rp)
	if err != nil {
		return nil, registerDeclined, err
	}
	if existing := store.ProjectByPath(root); existing != nil {
		return existing, alreadyRegistered, nil
	}

	name := uniqueProjectName(store, filepath.Base(root))
	if !assumeYes {
		if !stdinIsTTY() {
			return nil, registerDeclined, nil
		}
		ok, err := promptYesNo(cmd.OutOrStdout(), cmd.InOrStdin(),
			fmt.Sprintf("Register project %q (%s)?", name, root), true)
		if err != nil {
			return nil, registerDeclined, err
		}
		if !ok {
			return nil, registerDeclined, nil
		}
	}

	proj, err := registerProject(root, name)
	if err != nil {
		return nil, registerDeclined, err
	}
	return proj, registeredNow, nil
}
