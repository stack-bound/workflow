package cli

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/dashboard"
)

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"dash", "ui"},
		Short:   "Open the TUI ledger: projects → worktrees, diffs, merge, cleanup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDashboard()
		},
	}
}

func runDashboard() error {
	m, g, err := manager()
	if err != nil {
		return err
	}
	return dashboard.Run(m, g)
}

// stdoutIsTTY reports whether stdout is an interactive terminal, so bare `wf`
// can open the dashboard interactively but still print a plain list when piped.
func stdoutIsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
