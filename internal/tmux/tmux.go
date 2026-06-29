// Package tmux is WorkFlow's thin tmux client: detect whether we are inside a
// session, list/create/select/kill windows, and answer the live "which
// workspaces have a window open?" query. It shells out to the tmux CLI.
//
// WorkFlow is a guest, never an owner. Every operation here targets the user's
// *current* session — it never bootstraps a captive session, never remaps keys,
// and never touches another server. Windows are tracked by a per-window user
// option (@wf_workspace = the worktree path), so the binding is derived live
// from tmux rather than persisted, matching the "persist facts, derive the
// rest" rule. That tag is also how `resurrect` rebinds windows after a restart.
package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// workspaceOption is the per-window user option holding a worktree path. It is
// how a window is rediscovered (open/close/resurrect, the "window open?"
// indicator) without persisting ephemeral window ids.
const workspaceOption = "@wf_workspace"

// Inside reports whether the current process is running inside a tmux session.
func Inside() bool {
	return os.Getenv("TMUX") != ""
}

// Available reports whether tmux integration can be used: we are inside a tmux
// session and the tmux binary is on PATH. Every window operation assumes this;
// callers gate on it so the no-tmux paths stay unchanged.
func Available() bool {
	if !Inside() {
		return false
	}
	_, err := exec.LookPath("tmux")
	return err == nil
}

// Window is a tmux window in the current session.
type Window struct {
	ID        string // tmux window id, e.g. "@3"
	Index     int    // window index within the session
	Name      string // window name (cosmetic; matching uses Workspace)
	Workspace string // worktree path we tagged it with, or "" if not ours
	Active    bool   // currently the session's active window
}

// serverFlags are prepended to every tmux invocation. Empty in normal use
// (commands then target the user's current server via $TMUX). Tests set it to
// "-L <socket>" so the window lifecycle can be exercised against an isolated
// server, never the user's default one.
var serverFlags []string

func run(args ...string) (string, error) {
	full := append(append([]string{}, serverFlags...), args...)
	cmd := exec.Command("tmux", full...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tmux %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// listFormat is the field layout parseWindows expects. window_name is last so
// an (unlikely) tab inside a name cannot shift the earlier fields.
const listFormat = "#{window_id}\t#{window_index}\t#{window_active}\t#{" + workspaceOption + "}\t#{window_name}"

// Windows lists the windows in the current session.
func Windows() ([]Window, error) {
	out, err := run("list-windows", "-F", listFormat)
	if err != nil {
		return nil, err
	}
	return parseWindows(out), nil
}

// parseWindows turns list-windows output into Window values. It is pure so the
// parsing is unit-testable without a tmux server.
func parseWindows(out string) []Window {
	if out == "" {
		return nil
	}
	var wins []Window
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 5)
		if len(f) < 5 {
			continue
		}
		idx, _ := strconv.Atoi(f[1])
		wins = append(wins, Window{
			ID:        f[0],
			Index:     idx,
			Active:    f[2] == "1",
			Workspace: f[3],
			Name:      f[4],
		})
	}
	return wins
}

// FindByWorkspace returns the window tagged for the given worktree path, or nil
// when none is open.
func FindByWorkspace(path string) (*Window, error) {
	wins, err := Windows()
	if err != nil {
		return nil, err
	}
	for i := range wins {
		if wins[i].Workspace == path {
			return &wins[i], nil
		}
	}
	return nil, nil
}

// OpenWorkspaces returns the set of worktree paths that currently have a window
// open in the session — the live "window open?" fact the dashboard and sidebar
// derive each refresh.
func OpenWorkspaces() (map[string]bool, error) {
	wins, err := Windows()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(wins))
	for _, w := range wins {
		if w.Workspace != "" {
			set[w.Workspace] = true
		}
	}
	return set, nil
}

// NewWindow creates a detached window at path, names it, and tags it with the
// worktree path so it can be rediscovered. It returns the new window id. The
// window is created without stealing focus (-d); callers select it explicitly.
func NewWindow(path, name string) (string, error) {
	id, err := run("new-window", "-d", "-P", "-F", "#{window_id}", "-c", path, "-n", name)
	if err != nil {
		return "", err
	}
	// Tag the window so open/close/resurrect can find it again. A failure here
	// leaves an untracked window, so surface it.
	if _, err := run("set-window-option", "-t", id, workspaceOption, path); err != nil {
		return id, fmt.Errorf("created window but tagging it failed: %w", err)
	}
	// Keep the branch label in the status bar rather than letting tmux rename the
	// window after the running program. Best-effort and scoped to our window;
	// matching never relies on the name.
	_, _ = run("set-window-option", "-t", id, "automatic-rename", "off")
	return id, nil
}

// SelectWindow makes the given window the active one in its session.
func SelectWindow(id string) error {
	_, err := run("select-window", "-t", id)
	return err
}

// KillWindow kills the window with the given id.
func KillWindow(id string) error {
	_, err := run("kill-window", "-t", id)
	return err
}

// WindowName builds a window name with the status glyph prefixed:
// "<glyph> <branch>". So the status icon sits inside the tab, in a fixed slot
// just after tmux's index, and the layout never shifts (an empty glyph yields
// just the branch). In color_mode "glyph" with a color set, only the glyph is
// wrapped in an inline tmux style ("#[fg=colourN]<glyph>#[default] <branch>");
// note inline styles inside a window name are tmux-version-sensitive, which is
// why "tab" is the default mode. It is pure so it is unit-testable.
func WindowName(glyph, branch, mode, color string) string {
	if glyph == "" {
		return branch
	}
	if mode == "glyph" && color != "" {
		return fmt.Sprintf("#[fg=colour%s]%s#[default] %s", color, glyph, branch)
	}
	return glyph + " " + branch
}

// StyleOp is one per-window tmux option to apply (or unset). It is the unit
// TabStyleOps emits so the whole-tab coloring logic stays pure/testable.
type StyleOp struct {
	Option string // e.g. "window-status-current-style"
	Value  string // style value when setting
	Unset  bool   // when true, remove the per-window override (inherit again)
}

// TabStyleOps returns the per-window option operations that color the WHOLE tab
// for color_mode "tab": both the current and non-current window-status styles
// get a foreground color. When color is empty (idle) the ops UNSET the
// per-window override so the tab inherits the user's own theme again rather than
// a hardcoded default. Returns nil for any other mode (no tab styling). Pure.
func TabStyleOps(mode, color string) []StyleOp {
	if mode != "tab" {
		return nil
	}
	opts := []string{"window-status-style", "window-status-current-style"}
	ops := make([]StyleOp, 0, len(opts))
	for _, o := range opts {
		if color == "" {
			ops = append(ops, StyleOp{Option: o, Unset: true})
		} else {
			ops = append(ops, StyleOp{Option: o, Value: "fg=colour" + color})
		}
	}
	return ops
}

// RenameWindow sets a window's name.
func RenameWindow(id, name string) error {
	_, err := run("rename-window", "-t", id, name)
	return err
}

// ApplyWindowStyle applies (or unsets) per-window style options on a window.
func ApplyWindowStyle(id string, ops []StyleOp) error {
	for _, op := range ops {
		var err error
		if op.Unset {
			_, err = run("set-window-option", "-u", "-t", id, op.Option)
		} else {
			_, err = run("set-window-option", "-t", id, op.Option, op.Value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// CurrentWindowID returns the window id containing the caller's pane
// ($TMUX_PANE) — the window an agent's hook fired in, which is the window
// `set-status` decorates. Falls back to the session's active window when no
// pane is in the environment.
func CurrentWindowID() (string, error) {
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		return run("display-message", "-p", "-t", pane, "#{window_id}")
	}
	return run("display-message", "-p", "#{window_id}")
}

// prevNameOption and prevAutoRenameOption are the per-window markers wf snapshots
// onto a *borrowed* window (one wf did not open, so it has no @wf_workspace) the
// first time it decorates it, so the window can be reverted to exactly its prior
// state when the agent's session ends. prevAutoRenameOption is also the
// "have we adopted this window?" sentinel — it is always non-empty once set
// (AutoRenameSnapshot never returns "").
const (
	prevNameOption       = "@wf_prev_name"
	prevAutoRenameOption = "@wf_prev_autorename"
)

// GetWindowOption reads a per-window user option (e.g. @wf_workspace) via a
// format query, returning "" when it is unset (tmux renders an unset @option as
// the empty string).
func GetWindowOption(id, option string) (string, error) {
	return run("display-message", "-p", "-t", id, "#{"+option+"}")
}

// GetWindowName returns a window's current name.
func GetWindowName(id string) (string, error) {
	return run("display-message", "-p", "-t", id, "#{window_name}")
}

// WindowOwned reports whether the window is one wf opened — i.e. it carries the
// @wf_workspace tag (a worktree or a project base). An untagged window is
// "borrowed" (an agent started by hand in some directory).
func WindowOwned(id string) bool {
	v, err := GetWindowOption(id, workspaceOption)
	return err == nil && v != ""
}

// StripGlyph removes a leading status glyph (one of glyphs) and its trailing
// space from a window name, so the name can be rebuilt with a new glyph without
// stacking ("🤖 feat" → "feat" → "🔔 feat"). It also unwraps a glyph-mode inline
// style prefix ("#[fg=colourN]<glyph>#[default] rest" → "rest"). At most one
// glyph is stripped; a name with no known leading glyph is returned unchanged.
// Pure.
//
// Note (ascii preset): strip requires the trailing space the decoration adds,
// so a bare "*scratch*" is safe — but a borrowed name shaped like a decorated
// one ("* scratch") is mis-stripped in the label. The true original is preserved
// separately in @wf_prev_name, so revert is unaffected.
func StripGlyph(name string, glyphs []string) string {
	if strings.HasPrefix(name, "#[") {
		const reset = "#[default] "
		if i := strings.Index(name, reset); i >= 0 {
			return name[i+len(reset):]
		}
	}
	for _, g := range glyphs {
		if g == "" {
			continue
		}
		if name == g {
			return ""
		}
		if strings.HasPrefix(name, g+" ") {
			return name[len(g)+1:]
		}
	}
	return name
}

// AutoRenameSnapshot reads a window's automatic-rename in a restorable form:
// "on"/"off" when it is set at the window level, or "inherit" when the window
// has no per-window override (so RestoreAutoRename can unset it again). It never
// returns "", so a stored snapshot doubles as an "already adopted?" sentinel.
func AutoRenameSnapshot(id string) (string, error) {
	// show-window-options prints "automatic-rename off" when set at the window
	// level, and nothing when the window inherits the value.
	out, err := run("show-window-options", "-t", id, "automatic-rename")
	if err != nil {
		return "", err
	}
	if f := strings.Fields(out); len(f) >= 2 {
		return f[len(f)-1], nil
	}
	return "inherit", nil
}

// RestoreAutoRename reverts automatic-rename from an AutoRenameSnapshot value:
// "inherit" (or "") unsets the per-window override so the window inherits again;
// "on"/"off" set it explicitly.
func RestoreAutoRename(id, snap string) error {
	if snap == "inherit" || snap == "" {
		_, err := run("set-window-option", "-u", "-t", id, "automatic-rename")
		return err
	}
	_, err := run("set-window-option", "-t", id, "automatic-rename", snap)
	return err
}

// AdoptWindow snapshots a borrowed window's original name and automatic-rename
// (once) and pins automatic-rename off so wf's decoration is not immediately
// overwritten by tmux's process-name tracking. Idempotent: a window already
// adopted keeps its first (true) snapshot, so a re-used stale tab still reverts
// to the real original. Best-effort.
func AdoptWindow(id string) {
	if snap, _ := GetWindowOption(id, prevAutoRenameOption); snap != "" {
		return // already adopted
	}
	name, _ := GetWindowName(id)
	auto, err := AutoRenameSnapshot(id)
	if err != nil || auto == "" {
		auto = "inherit"
	}
	_, _ = run("set-window-option", "-t", id, prevNameOption, name)
	_, _ = run("set-window-option", "-t", id, prevAutoRenameOption, auto)
	_, _ = run("set-window-option", "-t", id, "automatic-rename", "off")
}

// AdoptedSnapshot reads the revert markers wf stored on a borrowed window.
// adopted is true when the window currently carries wf's decoration, so it
// should be reverted on done / by `status reset`.
func AdoptedSnapshot(id string) (prevName, autoSnap string, adopted bool) {
	autoSnap, _ = GetWindowOption(id, prevAutoRenameOption)
	if autoSnap == "" {
		return "", "", false
	}
	prevName, _ = GetWindowOption(id, prevNameOption)
	return prevName, autoSnap, true
}

// RevertWindow restores a borrowed window wf decorated to its pre-decoration
// state: its original name and automatic-rename, with wf's markers and any
// whole-tab style override cleared. Best-effort; harmless on a window that was
// never decorated.
func RevertWindow(id, prevName, autoSnap string) {
	// Drop any whole-tab colour wf layered on (unset → inherit the theme again).
	_ = ApplyWindowStyle(id, []StyleOp{
		{Option: "window-status-style", Unset: true},
		{Option: "window-status-current-style", Unset: true},
	})
	_ = RenameWindow(id, prevName)
	_ = RestoreAutoRename(id, autoSnap)
	_, _ = run("set-window-option", "-u", "-t", id, prevNameOption)
	_, _ = run("set-window-option", "-u", "-t", id, prevAutoRenameOption)
}

// AdoptedWindow is a borrowed window wf decorated, with the markers needed to
// revert it. `status reset` sweeps these across the whole server.
type AdoptedWindow struct {
	ID       string
	PrevName string
	AutoSnap string
}

// AdoptedWindows lists every window on the server (all sessions) that currently
// carries wf's borrowed-decoration markers — the orphans `status reset` reverts.
func AdoptedWindows() ([]AdoptedWindow, error) {
	const f = "#{window_id}\t#{" + prevAutoRenameOption + "}\t#{" + prevNameOption + "}"
	out, err := run("list-windows", "-a", "-F", f)
	if err != nil {
		return nil, err
	}
	var ws []AdoptedWindow
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 || parts[1] == "" {
			continue // not adopted
		}
		ws = append(ws, AdoptedWindow{ID: parts[0], AutoSnap: parts[1], PrevName: parts[2]})
	}
	return ws, nil
}
