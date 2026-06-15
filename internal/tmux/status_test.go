package tmux

import (
	"strings"
	"testing"
)

func TestWindowName(t *testing.T) {
	if got := WindowName("", "feature/x", "tab", "11"); got != "feature/x" {
		t.Errorf("empty glyph = %q, want branch only", got)
	}
	if got := WindowName("R", "feature/x", "tab", "11"); got != "R feature/x" {
		t.Errorf("tab mode = %q, want \"R feature/x\"", got)
	}
	// glyph-mode wraps only the glyph in an inline style.
	got := WindowName("R", "feature/x", "glyph", "11")
	if got != "#[fg=colour11]R#[default] feature/x" {
		t.Errorf("glyph mode = %q", got)
	}
	// glyph mode with no color falls back to a plain prefix.
	if got := WindowName("R", "feature/x", "glyph", ""); got != "R feature/x" {
		t.Errorf("glyph mode no color = %q, want plain prefix", got)
	}
}

func TestTabStyleOps(t *testing.T) {
	if got := TabStyleOps("glyph", "11"); got != nil {
		t.Errorf("non-tab mode should yield nil ops, got %+v", got)
	}
	if got := TabStyleOps("none", "11"); got != nil {
		t.Errorf("none mode should yield nil ops, got %+v", got)
	}

	set := TabStyleOps("tab", "11")
	if len(set) != 2 {
		t.Fatalf("tab+color ops = %d, want 2", len(set))
	}
	for _, op := range set {
		if op.Unset || op.Value != "fg=colour11" {
			t.Errorf("set op wrong: %+v", op)
		}
		if !strings.HasPrefix(op.Option, "window-status") {
			t.Errorf("unexpected option %q", op.Option)
		}
	}

	revert := TabStyleOps("tab", "")
	if len(revert) != 2 {
		t.Fatalf("tab revert ops = %d, want 2", len(revert))
	}
	for _, op := range revert {
		if !op.Unset {
			t.Errorf("idle (empty color) should unset, got %+v", op)
		}
	}
}

func TestRenameAndStyleIntegration(t *testing.T) {
	isolatedServer(t)

	dir := t.TempDir()
	id, err := NewWindow(dir, "feat-x")
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}

	if err := RenameWindow(id, "R feat-x"); err != nil {
		t.Fatalf("RenameWindow: %v", err)
	}
	w, err := FindByWorkspace(dir)
	if err != nil || w == nil {
		t.Fatalf("FindByWorkspace: w=%v err=%v", w, err)
	}
	if w.Name != "R feat-x" {
		t.Errorf("window name = %q, want %q", w.Name, "R feat-x")
	}

	// Applying and reverting the whole-tab style must not error (the options are
	// valid per-window options across tmux versions).
	if err := ApplyWindowStyle(id, TabStyleOps("tab", "11")); err != nil {
		t.Errorf("apply style: %v", err)
	}
	if err := ApplyWindowStyle(id, TabStyleOps("tab", "")); err != nil {
		t.Errorf("revert style: %v", err)
	}

	// A Claude Code hook always runs inside a pane, so $TMUX_PANE is set;
	// CurrentWindowID resolves that pane's window. Simulate it with the new
	// window's pane id.
	pane, err := run("display-message", "-p", "-t", id, "#{pane_id}")
	if err != nil || pane == "" {
		t.Fatalf("resolve pane id: %q err=%v", pane, err)
	}
	t.Setenv("TMUX_PANE", pane)
	if got, err := CurrentWindowID(); err != nil || got != id {
		t.Errorf("CurrentWindowID = %q err=%v, want %q", got, err, id)
	}

	// With no $TMUX_PANE, it falls back to the session's current window (no
	// target). On a detached test server this may be empty; we just exercise the
	// branch without erroring.
	t.Setenv("TMUX_PANE", "")
	if _, err := CurrentWindowID(); err != nil {
		t.Errorf("CurrentWindowID fallback errored: %v", err)
	}
}
