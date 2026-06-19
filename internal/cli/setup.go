package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/ide"
	"gopkg.in/yaml.v3"
)

// resolveConfigEditor picks the command to open a single config file: $VISUAL or
// $EDITOR when set, else a detected terminal editor (preferred for one file),
// else any detected editor, else a plain "vi".
func resolveConfigEditor(g *config.Global) []string {
	if v := os.Getenv("VISUAL"); v != "" {
		return strings.Fields(v)
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return strings.Fields(v)
	}
	ides := ide.Detect(g)
	for _, i := range ides {
		if !i.GUI {
			return i.Exec
		}
	}
	if len(ides) > 0 {
		return ides[0].Exec
	}
	return []string{"vi"}
}

func newInitCmd() *cobra.Command {
	var force bool
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write an example .workFlow.yaml in the current repo and register it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			root := cwd
			if r, err := git.RepoRoot(cwd); err == nil {
				root = r
			}
			path := config.RepoConfigPath(root)
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", path)
			}

			// Pick the base branch instead of guessing "main": detect the repo's
			// default and, when several real branches could be it, ask. A wrong
			// base here surfaces later as "fatal: invalid reference" on `wf add`.
			base, candidates := detectInitBase(root)
			if git.IsRepo(root) && !assumeYes && stdinIsTTY() && len(candidates) > 1 {
				chosen, err := promptBranch(cmd.OutOrStdout(), cmd.InOrStdin(), candidates, base)
				if err != nil {
					return err
				}
				base = chosen
			}

			if err := os.WriteFile(path, []byte(config.ExampleRepoYAML(base)), 0o644); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Wrote %s (base branch: %s)\n", path, base)

			// Offer to register the repo so it shows up in the dashboard. Only a
			// git repo can be a project; a bare `init` in a non-repo just writes
			// the template.
			if !git.IsRepo(root) {
				return nil
			}
			proj, outcome, err := promptRegister(cmd, root, assumeYes)
			if err != nil {
				return err
			}
			switch outcome {
			case registeredNow:
				_, _ = fmt.Fprintf(out, "Registered project %q at %s\n", proj.Name, proj.Path)
			case alreadyRegistered:
				_, _ = fmt.Fprintf(out, "Already registered as %q\n", proj.Name)
			case registerDeclined:
				_, _ = fmt.Fprintln(out, "Not registered. Register later with: wf project add")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .workFlow.yaml")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "register the repo without prompting")
	return cmd
}

// detectInitBase chooses the base branch to suggest for the repo at root and
// the ordered candidate branches to offer. git's tracked default (origin/HEAD)
// wins when present; otherwise development is preferred over main/master, since
// that is this team's usual default. Only branches that actually exist are
// offered, with the current branch appended so non-standard defaults (e.g.
// trunk) still appear. def always falls back to a usable value, never "".
func detectInitBase(root string) (def string, candidates []string) {
	locals, _ := git.LocalBranches(root)
	exists := func(b string) bool {
		for _, l := range locals {
			if l == b {
				return true
			}
		}
		return false
	}

	if b, err := git.OriginHEAD(root); err == nil && b != "" {
		def = b
	}

	seen := map[string]bool{}
	add := func(b string) {
		if b == "" || seen[b] {
			return
		}
		seen[b] = true
		candidates = append(candidates, b)
	}
	// git's default goes first even if no local branch tracks it yet; the rest
	// are only offered when they really exist.
	if def != "" {
		add(def)
	}
	for _, b := range []string{"development", "main", "master"} {
		if exists(b) {
			add(b)
		}
	}
	if cur, err := git.CurrentBranch(root); err == nil && exists(cur) {
		add(cur)
	}

	if def == "" {
		switch {
		case len(candidates) > 0:
			def = candidates[0]
		default:
			if cur, err := git.CurrentBranch(root); err == nil && cur != "" {
				def, candidates = cur, []string{cur}
			} else {
				def = "main"
			}
		}
	}
	return def, candidates
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage global WorkFlow config",
	}
	cmd.AddCommand(newConfigPathCmd(), newConfigShowCmd(), newConfigEditCmd())
	return cmd
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the global config file path",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := config.GlobalPath()
			if err != nil {
				return err
			}
			fmt.Println(p)
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the effective global config",
		RunE: func(_ *cobra.Command, _ []string) error {
			g, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			data, err := yaml.Marshal(g)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			fmt.Printf("# config editor: %s\n", strings.Join(resolveConfigEditor(g), " "))
			return nil
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the global config in your editor",
		RunE: func(_ *cobra.Command, _ []string) error {
			g, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			p, err := config.GlobalPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(p); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return err
				}
				if err := config.SaveGlobal(g); err != nil {
					return err
				}
			}
			argv := resolveConfigEditor(g)
			c := exec.Command(argv[0], append(argv[1:], p)...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}
}
