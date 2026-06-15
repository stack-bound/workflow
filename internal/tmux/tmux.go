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
// ($TMUX_PANE), used as a fallback when a workspace's window is not tagged.
func CurrentWindowID() (string, error) {
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		return run("display-message", "-p", "-t", pane, "#{window_id}")
	}
	return run("display-message", "-p", "#{window_id}")
}
