package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/tmux"
)

// newStatusCmd groups the manual agent-status maintenance commands. The live
// status itself is driven by the Claude Code hooks (`wf hooks install` →
// `wf set-status`); this is the escape hatch for the rare orphan.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Inspect and maintain agent status decorations",
		Long: "Maintenance for the live agent-status decorations driven by the Claude " +
			"Code hooks. The only subcommand today is `reset`, which reverts any " +
			"borrowed tmux tab wf decorated but never cleaned up.",
	}
	cmd.AddCommand(newStatusResetCmd())
	return cmd
}

// newStatusResetCmd sweeps every borrowed window still carrying wf's decoration
// markers and reverts it to its original name and automatic-rename. It is the
// key-press escape hatch for the residual "agent's session died, the tab was
// never reused, and SessionEnd never fired" case — there is no polling janitor
// by design. Owned (worktree/base) windows are untouched: wf decorates those
// permanently and they self-heal through the normal lifecycle.
func newStatusResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Revert any borrowed tmux tabs wf decorated but left behind",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !tmux.Available() {
				return fmt.Errorf("status reset needs a tmux session (no $TMUX detected)")
			}
			adopted, err := tmux.AdoptedWindows()
			if err != nil {
				return fmt.Errorf("list windows: %w", err)
			}
			for _, w := range adopted {
				tmux.RevertWindow(w.ID, w.PrevName, w.AutoSnap)
			}
			n := len(adopted)
			noun := "tabs"
			if n == 1 {
				noun = "tab"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset %d borrowed %s\n", n, noun)
			return nil
		},
	}
}
