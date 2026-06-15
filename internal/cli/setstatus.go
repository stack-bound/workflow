package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/tmux"
)

// newSetStatusCmd is the target of the Claude Code lifecycle hooks installed by
// `wf hooks install`. It records the calling workspace's agent status — figured
// out from the cwd — to a status file, and (inside tmux) updates that window's
// tab icon and color.
//
// It is designed to be invisible and indestructible: it runs after *every* tool
// call, so it NEVER errors (always exits 0) and silently no-ops when run outside
// a registered worktree. Every file/tmux operation is best-effort.
func newSetStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-status <state>",
		Short: "Record agent status for the current workspace (working|waiting|done)",
		Long: "Record the current workspace's agent status, shown live in the tmux " +
			"tab, the dashboard, and the sidebar. Intended to be driven by Claude " +
			"Code hooks (see `wf hooks install`); the workspace is inferred from the " +
			"working directory. A no-op outside a registered worktree.",
		Args:   cobra.ExactArgs(1),
		Hidden: true, // a hook target, not a day-to-day command
		RunE: func(_ *cobra.Command, args []string) error {
			setStatus(args[0])
			return nil // never fail: this runs on every tool call
		},
	}
}

// setStatus performs the work for `wf set-status`. It is split out for direct
// testing and swallows every error by design.
func setStatus(stateArg string) {
	st := status.Normalize(stateArg)

	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	rp, err := config.RegistryPath()
	if err != nil {
		return
	}
	store, err := registry.Load(rp)
	if err != nil {
		return
	}
	wt := status.ResolveByCwd(store, cwd)
	if wt == nil {
		return // not inside a registered worktree — nothing to record
	}

	_ = status.Write(wt.Project, wt.Branch, wt.Path, st)

	if tmux.Available() {
		if g, err := config.LoadGlobal(); err == nil {
			updateTmuxIcon(*wt, st, g.StatusLook())
		}
	}
}

// updateTmuxIcon rebuilds the workspace window's name with the state glyph and,
// in tab color mode, recolors the whole tab (reverting on idle). Best-effort.
func updateTmuxIcon(wt registry.Worktree, st status.State, look config.ResolvedStatus) {
	id := windowIDFor(wt.Path)
	if id == "" {
		return
	}
	l := look.Look[string(st)]
	_ = tmux.RenameWindow(id, tmux.WindowName(l.Glyph, wt.Branch, look.ColorMode, l.Color))
	if ops := tmux.TabStyleOps(look.ColorMode, l.Color); ops != nil {
		_ = tmux.ApplyWindowStyle(id, ops)
	}
}

// windowIDFor finds the tmux window bound to a worktree (by the @wf_workspace
// tag), falling back to the caller's current window when none is tagged (e.g. an
// agent started by hand in an untagged window).
func windowIDFor(path string) string {
	if w, err := tmux.FindByWorkspace(path); err == nil && w != nil {
		return w.ID
	}
	if id, err := tmux.CurrentWindowID(); err == nil {
		return id
	}
	return ""
}
