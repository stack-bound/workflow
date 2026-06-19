package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/stack-bound/workflow/internal/ide"
	"github.com/stack-bound/workflow/internal/launcher"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/tmux"
	"github.com/stack-bound/workflow/internal/workspace"
)

// closeWindowBestEffort closes a workspace's tmux window after merge/rm. It is
// best-effort: a tmux hiccup must never mask a successful merge or removal.
func closeWindowBestEffort(path string) {
	if !tmux.Available() {
		return
	}
	_, _ = launcher.NewTmux().Close(path)
}

func newAddCmd() *cobra.Command {
	var opts workspace.AddOptions
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "add <branch>",
		Short: "Create a branch + worktree workspace (runs setup)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Branch = args[0]
			m, g, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Add(opts)
			// If the cwd is a git repo that just isn't registered yet, offer to
			// register it and retry — but only when the project wasn't named
			// explicitly via --project.
			var unreg *workspace.ErrUnregisteredRepo
			if errors.As(err, &unreg) && opts.Project == "" {
				proj, outcome, rerr := promptRegister(cmd, unreg.Root, assumeYes)
				if rerr != nil {
					return rerr
				}
				if outcome == registerDeclined {
					if !assumeYes && !stdinIsTTY() {
						return fmt.Errorf("%s is not registered; run `wf project add` or pass --yes", unreg.Root)
					}
					return fmt.Errorf("aborted: %s not registered", unreg.Root)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Registered project %q at %s\n", proj.Name, proj.Path)
				wt, err = m.Add(opts)
			}
			if err != nil {
				return err
			}
			fmt.Printf("Created workspace %s/%s (base %s)\n", wt.Project, wt.Branch, wt.Base)
			fmt.Printf("  %s\n", wt.Path)
			// In tmux, give the workspace a real window straight away (detached,
			// so the setup output above stays put). Best-effort: a tmux failure
			// must not undo a created workspace.
			if tmux.Available() {
				if made, werr := launcher.NewTmux().EnsureWindow(wt.Path, launcher.IdleName(g, wt.Branch)); werr != nil {
					fmt.Printf("  (tmux window not created: %v)\n", werr)
				} else if made {
					fmt.Printf("  tmux window %q ready — jump with: wf open %s\n", wt.Branch, wt.Branch)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "project to create the workspace in (default: infer from cwd)")
	cmd.Flags().StringVarP(&opts.Base, "base", "b", "", "base branch (default: repo/global config or detected default)")
	cmd.Flags().BoolVar(&opts.NoSetup, "no-setup", false, "skip setup commands and file copy/symlink")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "register the current repo without prompting if it isn't yet")
	return cmd
}

func newListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces with live status",
		RunE: func(cmd *cobra.Command, _ []string) error {
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

	out := cmd.OutOrStdout()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(toJSON(views))
	}

	if len(views) == 0 {
		_, _ = fmt.Fprintln(out, "No workspaces yet. Create one with: wf add <branch>")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PROJECT\tBRANCH\tSTATE\tBASE\tA/B\tCHANGES\tPATH")
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
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
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
		RunE: func(_ *cobra.Command, args []string) error {
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
		Short: "Open a workspace: jump to its tmux window, or launch your editor",
		Long: "Open a workspace. Inside tmux this jumps to the workspace's window " +
			"(creating it if needed); otherwise it launches the workspace's default " +
			"editor. Use `wf edit` to choose an editor interactively.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, g, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Resolve(args[0], project)
			if err != nil {
				return err
			}
			if tmux.Available() {
				return launcher.NewTmux().Open(wt.Path, launcher.IdleName(g, args[0]))
			}
			// No tmux: launch the resolved default editor (repo default, else
			// global default, else the first detected editor).
			ides := ide.Detect(g)
			defaultID, _, _ := m.ProjectIDEPrefs(wt.Project)
			if defaultID == "" {
				defaultID = g.DefaultIDE
			}
			i, ok := ide.Find(ides, defaultID)
			if !ok {
				if len(ides) == 0 {
					return fmt.Errorf("no editor detected; add one under `ides:` in config (see: wf edit --list)")
				}
				i = ides[0]
			}
			return launchIDE(cmd, i, wt.Path)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}

func newCloseCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "close <branch>",
		Short: "Close a workspace's tmux window (keeps the worktree and branch)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !tmux.Available() {
				return fmt.Errorf("close needs a tmux session (no $TMUX detected)")
			}
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Resolve(args[0], project)
			if err != nil {
				return err
			}
			closed, err := launcher.NewTmux().Close(wt.Path)
			if err != nil {
				return err
			}
			// The window (and its agent) are gone; reflect idle so the dashboard
			// and sidebar drop the working/waiting marker. Best-effort.
			_ = status.Write(wt.Project, wt.Branch, wt.Path, status.Idle)
			out := cmd.OutOrStdout()
			if closed {
				_, _ = fmt.Fprintf(out, "Closed the window for %s\n", args[0])
			} else {
				_, _ = fmt.Fprintf(out, "No open window for %s\n", args[0])
			}
			return nil
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
		RunE: func(_ *cobra.Command, args []string) error {
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
		RunE: func(_ *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Remove(args[0], project, force)
			if err != nil {
				return err
			}
			closeWindowBestEffort(wt.Path)
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
		RunE: func(_ *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			wt, err := m.Merge(args[0], project)
			if err != nil {
				return err
			}
			closeWindowBestEffort(wt.Path)
			fmt.Printf("Merged %s into %s and cleaned up the workspace\n", wt.Branch, wt.Base)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "scope to a project when the branch is ambiguous")
	return cmd
}
