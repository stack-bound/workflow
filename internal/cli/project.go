package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/mattnelsonuk/workflow/internal/config"
	"github.com/mattnelsonuk/workflow/internal/git"
	"github.com/mattnelsonuk/workflow/internal/registry"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Manage registered projects (git repos)",
		Aliases: []string{"projects", "proj"},
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectRmCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Register a git repo as a project (default: current directory)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			abs, err := filepath.Abs(target)
			if err != nil {
				return err
			}
			if !git.IsRepo(abs) {
				return fmt.Errorf("%s is not a git repository", abs)
			}
			root, err := git.RepoRoot(abs)
			if err != nil {
				return err
			}

			rp, err := config.RegistryPath()
			if err != nil {
				return err
			}
			var added registry.Project
			err = registry.WithLock(rp, func(s *registry.Store) error {
				if existing := s.ProjectByPath(root); existing != nil {
					return fmt.Errorf("already registered as %q", existing.Name)
				}
				pname := name
				if pname == "" {
					pname = uniqueProjectName(s, filepath.Base(root))
				}
				added = registry.Project{
					Name:    pname,
					Path:    root,
					AddedAt: time.Now().UTC().Format(time.RFC3339),
				}
				return s.AddProject(added)
			})
			if err != nil {
				return err
			}
			fmt.Printf("Registered project %q at %s\n", added.Name, added.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: repo directory name)")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List registered projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, err := config.RegistryPath()
			if err != nil {
				return err
			}
			s, err := registry.Load(rp)
			if err != nil {
				return err
			}
			if len(s.Projects) == 0 {
				fmt.Println("No projects registered. Add one with: wf project add")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tWORKSPACES\tPATH")
			for _, p := range s.Projects {
				fmt.Fprintf(tw, "%s\t%d\t%s\n", p.Name, len(s.WorktreesForProject(p.Name)), p.Path)
			}
			return tw.Flush()
		},
	}
}

func newProjectRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Unregister a project (does not touch the repo on disk)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rp, err := config.RegistryPath()
			if err != nil {
				return err
			}
			err = registry.WithLock(rp, func(s *registry.Store) error {
				return s.RemoveProject(args[0], force)
			})
			if err != nil {
				return err
			}
			fmt.Printf("Unregistered project %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "remove even if it still has worktrees (drops them from the registry)")
	return cmd
}

// uniqueProjectName returns base, or base-2/base-3/… if already taken.
func uniqueProjectName(s *registry.Store, base string) string {
	if s.FindProject(base) == nil {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if s.FindProject(cand) == nil {
			return cand
		}
	}
}
