package cli

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/tmux"
)

// newSetStatusCmd is the target of the Claude Code lifecycle hooks installed by
// `wf hooks install`. It records the calling agent's status — figured out from
// the cwd — to a status file, and (inside tmux) decorates the agent's current
// tmux window with the state glyph and color.
//
// It is designed to be invisible and indestructible: it runs after *every* tool
// call, so it NEVER errors (always exits 0) and silently no-ops when there is
// nothing to do. Every file/tmux operation is best-effort.
func newSetStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-status <state>",
		Short: "Record agent status for the current workspace (working|waiting|ready|done)",
		Long: "Record the current agent's status, shown live in the tmux tab, the " +
			"dashboard, and the sidebar. Intended to be driven by Claude Code hooks " +
			"(see `wf hooks install`); the workspace is inferred from the hook's cwd " +
			"(read from stdin JSON, falling back to the process working directory).",
		Args:   cobra.ExactArgs(1),
		Hidden: true, // a hook target, not a day-to-day command
		RunE: func(cmd *cobra.Command, args []string) error {
			setStatus(args[0], cwdFromStdin(cmd.InOrStdin()))
			return nil // never fail: this runs on every tool call
		},
	}
}

// cwdFromStdin reads the agent's cwd from a Claude Code hook's stdin JSON
// payload ({"cwd": "…", …}). It returns "" when there is no piped payload — and
// deliberately does NOT block on a terminal stdin, so a human running
// `wf set-status` by hand falls through to os.Getwd rather than hanging.
func cwdFromStdin(r io.Reader) string {
	if f, ok := r.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return "" // a terminal — no hook payload to read
		}
	}
	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return ""
	}
	var payload struct {
		Cwd string `json:"cwd"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload.Cwd
}

// setStatus performs the work for `wf set-status`. It is split out for direct
// testing and swallows every error by design. cwd is the agent's working
// directory (from the hook payload); empty means fall back to os.Getwd.
func setStatus(stateArg, cwd string) {
	st := status.Normalize(stateArg)

	if cwd == "" {
		c, err := os.Getwd()
		if err != nil {
			return
		}
		cwd = c
	}
	rp, err := config.RegistryPath()
	if err != nil {
		return
	}
	store, err := registry.Load(rp)
	if err != nil {
		return
	}

	g, _ := config.LoadGlobal() // nil is fine: StatusLook handles it
	look := g.StatusLook()

	// Resolve cwd: innermost worktree → project base → unrelated. The status file
	// (the dashboard's durable source) is keyed accordingly; an unrelated tab gets
	// no file and no dashboard presence.
	wt := status.ResolveByCwd(store, cwd)
	var proj *registry.Project
	if wt != nil {
		_ = status.Write(wt.Project, wt.Branch, wt.Path, st)
	} else if proj = status.ResolveProjectByCwd(store, cwd); proj != nil {
		_ = status.WriteBase(proj.Name, proj.Path, st)
	}

	// scope=wf leaves truly-unrelated tabs untouched (no decoration); scope=all
	// (default) decorates the current window for any agent, anywhere.
	if look.Scope == config.ScopeWF && wt == nil && proj == nil {
		return
	}

	if tmux.Available() {
		decorateCurrentWindow(st, look)
	}
}

// decorateCurrentWindow updates the agent's current tmux window for the state.
// Owned windows (wf opened them — they carry @wf_workspace) are decorated
// permanently; borrowed windows are adopted-and-reverted so wf stays a polite
// guest.
func decorateCurrentWindow(st status.State, look config.ResolvedStatus) {
	id, err := tmux.CurrentWindowID()
	if err != nil || id == "" {
		return
	}
	if tmux.WindowOwned(id) {
		decorateOwned(id, st, look)
		return
	}
	decorateBorrowed(id, st, look)
}

// decorateOwned recolors a wf-owned window (worktree or project base): rebuild
// "<glyph> <label>" from the existing name (strip-and-prepend, no git call) and
// recolor per color_mode. On done the state is idle, so it keeps the idle glyph
// and reverts the tab color — wf owns the window permanently, never reverting
// the name.
func decorateOwned(id string, st status.State, look config.ResolvedStatus) {
	cur, err := tmux.GetWindowName(id)
	if err != nil {
		return
	}
	applyGlyph(id, tmux.StripGlyph(cur, glyphsOf(look)), st, look)
}

// decorateBorrowed adopts a window wf does not own. On the first touch it
// snapshots the true original (name + automatic-rename) so it can be reverted,
// then decorates "<glyph> <existing-name>". On done it fully reverts — name and
// automatic-rename restored, markers and tab style cleared — leaving the tab
// exactly as found.
func decorateBorrowed(id string, st status.State, look config.ResolvedStatus) {
	if st == status.Idle { // done / SessionEnd
		if prevName, autoSnap, adopted := tmux.AdoptedSnapshot(id); adopted {
			tmux.RevertWindow(id, prevName, autoSnap)
		}
		return
	}
	tmux.AdoptWindow(id) // idempotent snapshot + pin automatic-rename off
	cur, err := tmux.GetWindowName(id)
	if err != nil {
		return
	}
	applyGlyph(id, tmux.StripGlyph(cur, glyphsOf(look)), st, look)
}

// applyGlyph renames the window to "<glyph> <label>" for the state and applies
// the whole-tab color (tab mode only). Best-effort.
func applyGlyph(id, label string, st status.State, look config.ResolvedStatus) {
	l := look.Look[string(st)]
	_ = tmux.RenameWindow(id, tmux.WindowName(l.Glyph, label, look.ColorMode, l.Color))
	if ops := tmux.TabStyleOps(look.ColorMode, l.Color); ops != nil {
		_ = tmux.ApplyWindowStyle(id, ops)
	}
}

// glyphsOf returns every resolved state glyph, the set StripGlyph removes a
// leading occurrence of when rebuilding a window name.
func glyphsOf(look config.ResolvedStatus) []string {
	gs := make([]string, 0, len(look.Look))
	for _, l := range look.Look {
		gs = append(gs, l.Glyph)
	}
	return gs
}
