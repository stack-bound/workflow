package cli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// rawTmux runs a tmux command against the isolated test server and returns its
// trimmed stdout (fatal on error).
func rawTmux(t *testing.T, socket string, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", append([]string{"-L", socket}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// winOption reads a per-window option/format on the isolated server.
func winOption(t *testing.T, socket, winID, fmtStr string) string {
	t.Helper()
	return rawTmux(t, socket, "display-message", "-p", "-t", winID, fmtStr)
}

// With a tmux server, set-status decorates the agent's CURRENT window (the pane
// the hook fired in), found via $TMUX_PANE — not a registry lookup. An owned
// window (one wf opened, carrying @wf_workspace) is decorated permanently and
// keeps the idle glyph on done.
func TestSetStatusDecoratesOwnedCurrentWindow(t *testing.T) {
	cfg := isolateConfig(t)
	socket := startIsolatedTmux(t)

	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatalf("add: %v", err)
	}
	store, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	wt := store.FindWorktrees("feat", "proj")[0]

	// The wf-created worktree window is tagged @wf_workspace. Point $TMUX_PANE at
	// its pane so set-status decorates it as the "current" window.
	winID := ownedWindowID(t, socket, wt.Path)
	pane := winOption(t, socket, winID, "#{pane_id}")
	t.Setenv("TMUX_PANE", pane)
	t.Chdir(wt.Path)

	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	workLook := (&config.Global{}).StatusLook().Look["working"]
	if got := winOption(t, socket, winID, "#{window_name}"); !strings.Contains(got, workLook.Glyph) {
		t.Errorf("owned window did not get the working glyph %q: %q", workLook.Glyph, got)
	}
	// The glyph carries its colour inline so the tab icon matches the dashboard
	// glyph (default color_mode "tab" still tints the whole tab on top).
	wantInline := "#[fg=colour" + workLook.Color + "]" + workLook.Glyph
	if got := winOption(t, socket, winID, "#{window_name}"); !strings.Contains(got, wantInline) {
		t.Errorf("owned window icon not inline-coloured, want %q: %q", wantInline, got)
	}

	// done keeps the idle glyph (wf owns the window; it never reverts).
	if _, err := execWF(t, "set-status", "done"); err != nil {
		t.Fatalf("set-status done: %v", err)
	}
	idle := (&config.Global{}).StatusLook().Look["idle"].Glyph
	if got := winOption(t, socket, winID, "#{window_name}"); !strings.Contains(got, idle+" feat") {
		t.Errorf("owned window did not keep the idle glyph %q: %q", idle, got)
	}
}

// ownedWindowID finds the window tagged with the given worktree path.
func ownedWindowID(t *testing.T, socket, path string) string {
	t.Helper()
	out := rawTmux(t, socket, "list-windows", "-F", "#{window_id}\t#{@wf_workspace}")
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) == 2 && f[1] == path {
			return f[0]
		}
	}
	t.Fatalf("no window tagged %q in:\n%s", path, out)
	return ""
}

// A borrowed window (one wf did NOT open — no @wf_workspace) is adopted on first
// decoration and fully reverted on done: original name and automatic-rename
// restored, markers cleared. wf stays a polite guest.
func TestSetStatusAdoptsAndRevertsBorrowedWindow(t *testing.T) {
	isolateConfig(t)
	socket := startIsolatedTmux(t)

	// A bare window wf never opened. Pin automatic-rename off so its name stays
	// deterministic for the assertions; the snapshot/restore of that value has its
	// own focused tmux-package test.
	winID := rawTmux(t, socket, "new-window", "-d", "-P", "-F", "#{window_id}", "-n", "myeditor")
	rawTmux(t, socket, "set-window-option", "-t", winID, "automatic-rename", "off")
	pane := winOption(t, socket, winID, "#{pane_id}")
	t.Setenv("TMUX_PANE", pane)

	// Working decorates "<glyph> myeditor" and snapshots the true original.
	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	working := (&config.Global{}).StatusLook().Look["working"].Glyph
	if got := winOption(t, socket, winID, "#{window_name}"); !strings.Contains(got, working) || !strings.Contains(got, "myeditor") {
		t.Errorf("borrowed window not decorated: %q", got)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_name}"); got != "myeditor" {
		t.Errorf("@wf_prev_name = %q, want the original \"myeditor\"", got)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_autorename}"); got == "" {
		t.Error("@wf_prev_autorename not snapshotted")
	}

	// done fully reverts: name back to the original, markers cleared.
	if _, err := execWF(t, "set-status", "done"); err != nil {
		t.Fatalf("set-status done: %v", err)
	}
	if got := winOption(t, socket, winID, "#{window_name}"); got != "myeditor" {
		t.Errorf("borrowed window not reverted: name = %q, want \"myeditor\"", got)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_name}"); got != "" {
		t.Errorf("@wf_prev_name not cleared after revert: %q", got)
	}
}
