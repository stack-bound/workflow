package cli

import (
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// With a tmux server, set-status updates the workspace window's tab icon. Drives
// updateTmuxIcon/windowIDFor end to end against an isolated server.
func TestSetStatusUpdatesTmuxTab(t *testing.T) {
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
	t.Chdir(wt.Path)

	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	working := (&config.Global{}).StatusLook().Look["working"].Glyph
	if got := tmuxWindows(t, socket); !strings.Contains(got, working) {
		t.Errorf("tab did not get the working glyph %q:\n%s", working, got)
	}

	// Back to idle reverts the tab to the branch glyph.
	if _, err := execWF(t, "set-status", "done"); err != nil {
		t.Fatalf("set-status done: %v", err)
	}
	idle := (&config.Global{}).StatusLook().Look["idle"].Glyph
	if got := tmuxWindows(t, socket); !strings.Contains(got, idle+" feat") {
		t.Errorf("tab did not revert to idle glyph %q:\n%s", idle, got)
	}
}
