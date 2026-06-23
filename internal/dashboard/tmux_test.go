package dashboard

import (
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

// wsViewPath is like wsView but with a worktree path, for open-window tests.
func wsViewPath(project, branch, path string) workspace.View {
	return workspace.View{Worktree: registry.Worktree{Project: project, Branch: branch, Base: "main", Path: path}}
}

func TestTerminalKeyWithoutTmux(t *testing.T) {
	m := readyModel(t)
	m.inTmux = false
	m.cursor = 2 // a workspace
	m, cmd := step(m, runeKey("t"))
	if cmd != nil {
		t.Error("t without tmux should not return a command")
	}
	if !m.statusErr || !strings.Contains(m.status, "tmux not detected") {
		t.Errorf("t without tmux → status=%q err=%v", m.status, m.statusErr)
	}
}

func TestTerminalKeyInTmuxReturnsCommand(t *testing.T) {
	m := readyModel(t)
	m.inTmux = true
	m.cursor = 2 // alpha/feat-1
	_, cmd := step(m, runeKey("t"))
	if cmd == nil {
		t.Error("t in tmux on a workspace should return a jump command")
	}

	// On the base (main) row, t opens a window on the base checkout at the root.
	m.cursor = 1
	_, cmd = step(m, runeKey("t"))
	if cmd == nil {
		t.Error("t on the base row should return a base-window command")
	}
}

func TestRenderRowWindowOpenMarker(t *testing.T) {
	m := readyModel(t)
	open := wsViewPath("alpha", "live", "/wt/live")
	r := row{kind: rowWorkspace, project: "alpha", view: &open}

	// No marker when the path is not in the open set.
	if got := m.renderRow(9, r); strings.Contains(got, "▣") {
		t.Errorf("unopened workspace should not show ▣: %q", got)
	}
	// Marker appears once the window is open.
	m.openPaths = map[string]bool{"/wt/live": true}
	if got := m.renderRow(9, r); !strings.Contains(got, "▣") {
		t.Errorf("open workspace should show ▣: %q", got)
	}
}

func TestFooterHelpAdaptsToTmux(t *testing.T) {
	m := readyModel(t)

	// "t term" (jump to the tmux window) only shows when running inside tmux; the
	// edit/config keys are always present.
	m.inTmux = false
	if f := m.footer(); !strings.Contains(f, "e edit") || !strings.Contains(f, "o config") || strings.Contains(f, "t term") {
		t.Errorf("no-tmux footer = %q", f)
	}

	m.inTmux = true
	if f := m.footer(); !strings.Contains(f, "t term") || !strings.Contains(f, "e edit") {
		t.Errorf("tmux footer = %q", f)
	}
}

func TestLedgerMsgStoresOpenPaths(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, ledgerMsg{projects: sampleLedger(), openPaths: map[string]bool{"/wt/x": true}})
	if !m.openPaths["/wt/x"] {
		t.Errorf("openPaths not stored: %v", m.openPaths)
	}
}
