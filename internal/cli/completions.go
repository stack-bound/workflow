package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCompletionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completions [bash|zsh|fish|powershell]",
		Short: "Generate or install shell completion scripts",
		Long: `Generate a shell completion script (to stdout) or install it for your shell.

Print a script:
  wf completions zsh > _wf

Install it into the right place for your shell (auto-detected from $SHELL when
no shell is given):
  wf completions install
  wf completions install bash`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return genCompletion(cmd.Root(), args[0], os.Stdout)
		},
	}
	cmd.AddCommand(newCompletionsInstallCmd())
	return cmd
}

func newCompletionsInstallCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:       "install [bash|zsh|fish]",
		Short:     "Install a completion script into your shell's completion directory",
		ValidArgs: []string{"bash", "zsh", "fish"},
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := ""
			if len(args) == 1 {
				shell = args[0]
			} else {
				shell = detectShell()
				if shell == "" {
					return fmt.Errorf("could not detect your shell from $SHELL; pass one of: bash, zsh, fish")
				}
			}

			dest, post, err := completionInstallPath(shell)
			if err != nil {
				return err
			}
			if _, err := os.Stat(dest); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", dest)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			var buf bytes.Buffer
			if err := genCompletion(cmd.Root(), shell, &buf); err != nil {
				return err
			}
			if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("write completion script: %w", err)
			}
			fmt.Printf("Installed %s completions: %s\n", shell, dest)
			if post != "" {
				fmt.Println(post)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing completion file")
	return cmd
}

// genCompletion writes the completion script for shell to w.
func genCompletion(root *cobra.Command, shell string, w io.Writer) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(w, true)
	case "zsh":
		return root.GenZshCompletion(w)
	case "fish":
		return root.GenFishCompletion(w, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(w)
	default:
		return fmt.Errorf("unsupported shell %q (want bash, zsh, fish, or powershell)", shell)
	}
}

// completionInstallPath returns where to install a completion script for shell,
// plus a post-install hint. Installation targets the user's XDG directories.
func completionInstallPath(shell string) (dest, post string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	switch shell {
	case "bash":
		dest = filepath.Join(dataHome, "bash-completion", "completions", "wf")
		post = "bash-completion auto-loads this directory. Start a new shell to pick it up."
	case "zsh":
		dir := filepath.Join(dataHome, "zsh", "site-functions")
		dest = filepath.Join(dir, "_wf")
		post = fmt.Sprintf("Ensure this directory is on your fpath before compinit in ~/.zshrc:\n"+
			"  fpath=(%s $fpath)\n"+
			"  autoload -U compinit && compinit\n"+
			"Then start a new shell.", dir)
	case "fish":
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		dest = filepath.Join(configHome, "fish", "completions", "wf.fish")
		post = "fish auto-loads this directory. Start a new shell to pick it up."
	default:
		return "", "", fmt.Errorf("install supports bash, zsh, and fish; use 'wf completions powershell' to print the PowerShell script")
	}
	return dest, post, nil
}

// detectShell guesses the shell family from $SHELL.
func detectShell() string {
	base := filepath.Base(os.Getenv("SHELL"))
	switch {
	case strings.Contains(base, "bash"):
		return "bash"
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "fish"):
		return "fish"
	default:
		return ""
	}
}
