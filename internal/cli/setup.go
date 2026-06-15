package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write an example .workFlow.yaml in the current repo",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Printf("Wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .workFlow.yaml")
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
