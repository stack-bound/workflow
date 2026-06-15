package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/launcher"
	"github.com/stack-bound/workflow/internal/tmux"
)

// newResurrectCmd recreates tmux windows for tracked workspaces after a tmux or
// computer restart. The registry is the source of truth; windows are derived,
// so this reconciles live tmux against it, creating a window for any tracked
// worktree that does not have one. Matching is by the worktree-path tag, so a
// workspace that already has a window open is left untouched.
func newResurrectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "resurrect",
		Aliases: []string{"restore"},
		Short:   "Recreate tmux windows for tracked workspaces (after a tmux/computer restart)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !tmux.Available() {
				return fmt.Errorf("resurrect needs a tmux session (no $TMUX detected)")
			}
			m, g, err := manager()
			if err != nil {
				return err
			}
			views, err := m.List()
			if err != nil {
				return err
			}

			lt := launcher.NewTmux()
			out := cmd.OutOrStdout()
			created, open, missing := 0, 0, 0
			for _, v := range views {
				path := v.Worktree.Path
				if _, statErr := os.Stat(path); statErr != nil {
					missing++ // worktree gone from disk; nothing to bind
					continue
				}
				made, err := lt.EnsureWindow(path, launcher.IdleName(g, v.Worktree.Branch))
				if err != nil {
					_, _ = fmt.Fprintf(out, "  %s/%s: %v\n", v.Worktree.Project, v.Worktree.Branch, err)
					continue
				}
				if made {
					created++
					_, _ = fmt.Fprintf(out, "  rebound %s/%s\n", v.Worktree.Project, v.Worktree.Branch)
				} else {
					open++
				}
			}
			_, _ = fmt.Fprintf(out, "Resurrect: %d created, %d already open", created, open)
			if missing > 0 {
				_, _ = fmt.Fprintf(out, ", %d missing on disk", missing)
			}
			_, _ = fmt.Fprintln(out)
			return nil
		},
	}
}
