package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/sidebar"
	"github.com/stack-bound/workflow/internal/tmux"
)

// newSidebarCmd opens the live "now" strip: the workspace windows currently
// open in tmux. It is meant to be run in a split pane for an always-on glance.
func newSidebarCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sidebar",
		Short: "Live strip of the workspace windows open right now (tmux)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !tmux.Available() {
				return fmt.Errorf("sidebar needs a tmux session (no $TMUX detected)")
			}
			g, err := config.LoadGlobal()
			if err != nil {
				return err
			}
			rp, err := config.RegistryPath()
			if err != nil {
				return err
			}
			return sidebar.Run(rp, g.StatusLook())
		},
	}
}
