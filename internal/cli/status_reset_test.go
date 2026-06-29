package cli

import (
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
)

// `wf status reset` reverts a borrowed window wf decorated but never cleaned up
// (the SIGKILL/never-reused orphan) — restoring its name and clearing the
// markers — without anyone having to fire SessionEnd.
func TestStatusResetSweepsBorrowed(t *testing.T) {
	isolateConfig(t)
	socket := startIsolatedTmux(t)

	winID := rawTmux(t, socket, "new-window", "-d", "-P", "-F", "#{window_id}", "-n", "myeditor")
	rawTmux(t, socket, "set-window-option", "-t", winID, "automatic-rename", "off")
	pane := winOption(t, socket, winID, "#{pane_id}")
	t.Setenv("TMUX_PANE", pane)

	// Decorate it (leaving it "orphaned"), then sweep.
	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_name}"); got != "myeditor" {
		t.Fatalf("precondition: window not adopted, @wf_prev_name=%q", got)
	}

	out, err := execWF(t, "status", "reset")
	if err != nil {
		t.Fatalf("status reset: %v", err)
	}
	if !strings.Contains(out, "Reset 1 borrowed tab") {
		t.Errorf("status reset output = %q", out)
	}
	if got := winOption(t, socket, winID, "#{window_name}"); got != "myeditor" {
		t.Errorf("window not reverted by reset: name = %q", got)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_name}"); got != "" {
		t.Errorf("markers not cleared by reset: %q", got)
	}
}

// status.scope=wf leaves a truly-unrelated tab untouched: no decoration, no
// adoption markers.
func TestSetStatusScopeWFLeavesBorrowedAlone(t *testing.T) {
	isolateConfig(t)
	if err := config.SaveGlobal(&config.Global{Status: &config.StatusConfig{Scope: "wf"}}); err != nil {
		t.Fatal(err)
	}
	socket := startIsolatedTmux(t)

	winID := rawTmux(t, socket, "new-window", "-d", "-P", "-F", "#{window_id}", "-n", "myeditor")
	rawTmux(t, socket, "set-window-option", "-t", winID, "automatic-rename", "off")
	pane := winOption(t, socket, winID, "#{pane_id}")
	t.Setenv("TMUX_PANE", pane)
	t.Chdir(t.TempDir()) // cwd resolves to no worktree/project

	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	if got := winOption(t, socket, winID, "#{window_name}"); got != "myeditor" {
		t.Errorf("scope=wf decorated an unrelated tab: name = %q", got)
	}
	if got := winOption(t, socket, winID, "#{@wf_prev_name}"); got != "" {
		t.Errorf("scope=wf adopted an unrelated tab: @wf_prev_name = %q", got)
	}
}
