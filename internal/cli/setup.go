package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"gopkg.in/yaml.v3"
)

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
			if err := os.WriteFile(path, []byte(config.ExampleRepoYAML()), 0o644); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Wrote %s\n", path)

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
			fmt.Printf("# resolved editor: %s\n", g.ResolveEditor())
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
			editor := g.ResolveEditor()
			c := exec.Command("sh", "-c", fmt.Sprintf("%s %q", editor, p))
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	}
}
