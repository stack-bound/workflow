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
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDashboard()
		},
	}
}

func runDashboard() error {
	m, g, err := manager()
	if err != nil {
		return err
	}
	// Startup chores run concurrently once the dashboard opens (see
	// dashboard.StartupTask), so none of them can hold up the ledger appearing.
	// Each surfaces a one-line notice in the status line if it has something to
	// report.
	return dashboard.Run(m, g, startupTasks()...)
}

// startupTasks lists the best-effort jobs the dashboard runs on open, off the
// launch path. Each returns a one-line notice (or "" for nothing to report).
// Register additional independent launch-time work here.
//
// Today the only task keeps an already-opted-in user's Claude Code hooks current
// (e.g. add SessionEnd / flip Stop→ready after an upgrade) — best-effort and
// silent unless something actually changed.
func startupTasks() []dashboard.StartupTask {
	return []dashboard.StartupTask{
		autoUpdateHooks,
	}
}

// stdoutIsTTY reports whether stdout is an interactive terminal, so bare `wf`
// can open the dashboard interactively but still print a plain list when piped.
func stdoutIsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
