package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/registry"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Manage registered projects (git repos)",
		Aliases: []string{"projects", "proj"},
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectRenameCmd(), newProjectRmCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Register a git repo as a project (default: current directory)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			added, err := registerProject(target, name)
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
		RunE: func(_ *cobra.Command, _ []string) error {
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
			_, _ = fmt.Fprintln(tw, "NAME\tWORKSPACES\tPATH")
			for _, p := range s.Projects {
				_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\n", p.Name, len(s.WorktreesForProject(p.Name)), p.Path)
			}
			return tw.Flush()
		},
	}
}

func newProjectRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rename <old> <new>",
		Aliases: []string{"mv"},
		Short:   "Rename a registered project (retargets its worktrees)",
		Args:    cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			rp, err := config.RegistryPath()
			if err != nil {
				return err
			}
			err = registry.WithLock(rp, func(s *registry.Store) error {
				return s.RenameProject(args[0], args[1])
			})
			if err != nil {
				return err
			}
			fmt.Printf("Renamed project %q to %q\n", args[0], args[1])
			return nil
		},
	}
	return cmd
}

func newProjectRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Unregister a project (does not touch the repo on disk)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
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

// registerProject registers the git repo containing path as a project, picking
// a unique default name when name is empty. It is the shared core of
// `wf project add` and the interactive auto-register prompt.
func registerProject(path, name string) (*registry.Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !git.IsRepo(abs) {
		return nil, fmt.Errorf("%s is not a git repository", abs)
	}
	root, err := git.RepoRoot(abs)
	if err != nil {
		return nil, err
	}
	rp, err := config.RegistryPath()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return &added, nil
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
