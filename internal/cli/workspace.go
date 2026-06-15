package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/stack-bound/workflow/internal/launcher"
	"github.com/stack-bound/workflow/internal/workspace"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var opts workspace.AddOptions
	cmd := &cobra.Command{
		Use:   "add <branch>",
		Short: "Create a branch + worktree workspace (runs setup)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Branch = args[0]
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Add(opts)
			if err != nil {
				return err
			}
			fmt.Printf("Created workspace %s/%s (base %s)\n", wt.Project, wt.Branch, wt.Base)
			fmt.Printf("  %s\n", wt.Path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "project to create the workspace in (default: infer from cwd)")
	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "base branch (default: repo/global config or detected default)")
	cmd.Flags().BoolVar(&opts.NoSetup, "no-setup", false, "skip setup commands and file copy/symlink")
	return cmd
}

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces with live status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	return cmd
}

func runList(cmd *cobra.Command, asJSON bool) error {
	m, _, err := manager()
	if err != nil {
		return err
	}
	views, err := m.List()
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(toJSON(views))
	}

	if len(views) == 0 {
		fmt.Println("No workspaces yet. Create one with: wf add <branch>")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tBRANCH\tSTATE\tBASE\tA/B\tCHANGES\tPATH")
	for _, v := range views {
		state := "done"
		if v.Active() {
			state = "active"
		}
		ab := fmt.Sprintf("+%d/-%d", v.Stat.Ahead, v.Stat.Behind)
		changes := fmt.Sprintf("+%d -%d", v.Stat.Added, v.Stat.Deleted)
		if v.Stat.Dirty {
			changes += " *"
		}
		if v.StatErr != nil {
			state = "?"
			ab = "-"
			changes = v.StatErr.Error()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			v.Worktree.Project, v.Worktree.Branch, state, v.Worktree.Base, ab, changes, v.Worktree.Path)
	}
	return tw.Flush()
}

// jsonView is the stable JSON shape emitted by `list --json`.
type jsonView struct {
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Base    string `json:"base"`
	Path    string `json:"path"`
	Active  bool   `json:"active"`
	Dirty   bool   `json:"dirty"`
	Ahead   int    `json:"ahead"`
	Behind  int    `json:"behind"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

func toJSON(views []workspace.View) []jsonView {
	out := make([]jsonView, 0, len(views))
	for _, v := range views {
		jv := jsonView{
			Project: v.Worktree.Project,
			Branch:  v.Worktree.Branch,
			Base:    v.Worktree.Base,
			Path:    v.Worktree.Path,
			Active:  v.Active(),
			Dirty:   v.Stat.Dirty,
			Ahead:   v.Stat.Ahead,
			Behind:  v.Stat.Behind,
			Added:   v.Stat.Added,
			Deleted: v.Stat.Deleted,
		}
		if v.StatErr != nil {
			jv.Error = v.StatErr.Error()
		}
		out = append(out, jv)
	}
	return out
}

func newPathCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "path <branch>",
		Short: "Print a workspace's filesystem path (for shell cd integration)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			path, err := m.Path(args[0], project)
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}

func newOpenCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "open <branch>",
		Short: "Open a workspace in your editor (universal launcher)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, g, err := manager()
			if err != nil {
				return err
			}
			path, err := m.Path(args[0], project)
			if err != nil {
				return err
			}
			return launcher.NewUniversal(g).OpenInEditor(path)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}

func newCopyCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "copy <branch>",
		Short: "Copy a workspace's path to the clipboard",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, g, err := manager()
			if err != nil {
				return err
			}
			path, err := m.Path(args[0], project)
			if err != nil {
				return err
			}
			if err := launcher.NewUniversal(g).CopyPath(path); err != nil {
				return err
			}
			fmt.Printf("Copied: %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}

func newRmCmd() *cobra.Command {
	var project string
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <branch>",
		Short: "Remove a workspace: worktree + branch + registration (no merge)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Remove(args[0], project, force)
			if err != nil {
				return err
			}
			fmt.Printf("Removed workspace %s/%s\n", wt.Project, wt.Branch)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	cmd.Flags().BoolVar(&force, "force", false, "remove even with uncommitted changes or an unmerged branch")
	return cmd
}

func newMergeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "merge <branch>",
		Short: "Merge into base, then remove the worktree, branch, and registration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Merge(args[0], project)
			if err != nil {
				return err
			}
			fmt.Printf("Merged %s into %s and cleaned up the workspace\n", wt.Branch, wt.Base)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}
