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

func TestStripGlyph(t *testing.T) {
	glyphs := []string{"🤖", "🔔", "🌿", ""} // working, ready, idle, plus an empty
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no glyph", "feature/x", "feature/x"},
		{"working prefix", "🤖 feature/x", "feature/x"},
		{"ready prefix", "🔔 feature/x", "feature/x"},
		{"only a glyph", "🌿", ""},
		{"empty glyph in set is skipped", "plain name", "plain name"},
		{"glyph-mode styled prefix unwrapped", "#[fg=colour11]🤖#[default] feature/x", "feature/x"},
	}
	for _, c := range cases {
		if got := StripGlyph(c.in, glyphs); got != c.want {
			t.Errorf("%s: StripGlyph(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}

	// ascii edge: strip requires the trailing space the decoration adds, so a bare
	// "*scratch*" is left untouched, while a name shaped like a decorated one
	// ("* scratch") is mis-stripped (documented, accepted — the true original
	// lives in @wf_prev_name, so revert is unaffected).
	if got := StripGlyph("*scratch*", []string{"*"}); got != "*scratch*" {
		t.Errorf("bare *scratch* should be untouched: got %q", got)
	}
	if got := StripGlyph("* scratch", []string{"*"}); got != "scratch" {
		t.Errorf("ascii mis-strip: got %q, want %q", got, "scratch")
	}
}

// AutoRenameSnapshot/RestoreAutoRename must capture inheritance faithfully: an
// inherited setting snapshots as "inherit" and restores by unsetting the
// per-window override, while an explicit on/off round-trips as itself.
func TestAutoRenameSnapshotRestore(t *testing.T) {
	isolatedServer(t)
	id, err := NewWindow(t.TempDir(), "feat")
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	// NewWindow pins automatic-rename off, so it is set at the window level.
	if snap, _ := AutoRenameSnapshot(id); snap != "off" {
		t.Errorf("snapshot after NewWindow = %q, want off", snap)
	}
	// Unset → the window inherits again, which must snapshot as "inherit".
	if err := RestoreAutoRename(id, "inherit"); err != nil {
		t.Fatalf("RestoreAutoRename inherit: %v", err)
	}
	if snap, _ := AutoRenameSnapshot(id); snap != "inherit" {
		t.Errorf("snapshot after restore-inherit = %q, want inherit", snap)
	}
	// Explicit on round-trips.
	if err := RestoreAutoRename(id, "on"); err != nil {
		t.Fatalf("RestoreAutoRename on: %v", err)
	}
	if snap, _ := AutoRenameSnapshot(id); snap != "on" {
		t.Errorf("snapshot after restore-on = %q, want on", snap)
	}
}

// AdoptWindow → AdoptedSnapshot → RevertWindow is the borrowed-tab lifecycle.
// Adoption snapshots the true original once (idempotent), and revert restores it
// and clears the markers, so the window returns to exactly its prior state.
func TestAdoptAndRevertWindow(t *testing.T) {
	isolatedServer(t)
	// A borrowed window: created without the @wf_workspace tag, named, autorename
	// pinned off for a deterministic name.
	id, err := run("new-window", "-d", "-P", "-F", "#{window_id}", "-n", "myeditor")
	if err != nil {
		t.Fatalf("new-window: %v", err)
	}
	if _, err := run("set-window-option", "-t", id, "automatic-rename", "off"); err != nil {
		t.Fatalf("pin autorename: %v", err)
	}

	if owned := WindowOwned(id); owned {
		t.Error("a window with no @wf_workspace must read as borrowed")
	}

	AdoptWindow(id)
	prevName, autoSnap, adopted := AdoptedSnapshot(id)
	if !adopted || prevName != "myeditor" || autoSnap != "off" {
		t.Fatalf("AdoptedSnapshot = (%q, %q, %v), want (myeditor, off, true)", prevName, autoSnap, adopted)
	}

	// Idempotent: a second adopt after a rename keeps the true original.
	if err := RenameWindow(id, "🤖 myeditor"); err != nil {
		t.Fatal(err)
	}
	AdoptWindow(id)
	if pn, _, _ := AdoptedSnapshot(id); pn != "myeditor" {
		t.Errorf("re-adopt overwrote the original: %q", pn)
	}

	// It shows up in the server-wide sweep list.
	adoptedWins, err := AdoptedWindows()
	if err != nil {
		t.Fatalf("AdoptedWindows: %v", err)
	}
	var found bool
	for _, w := range adoptedWins {
		if w.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("adopted window %q not in AdoptedWindows: %+v", id, adoptedWins)
	}

	// Revert restores the name and clears the markers.
	RevertWindow(id, prevName, autoSnap)
	if name, _ := GetWindowName(id); name != "myeditor" {
		t.Errorf("name after revert = %q, want myeditor", name)
	}
	if _, _, stillAdopted := AdoptedSnapshot(id); stillAdopted {
		t.Error("markers not cleared after revert")
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
