package dashboard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/workspace"
)

func oneWorkspaceLedger(path string) []workspace.ProjectView {
	return []workspace.ProjectView{{
		Project: registry.Project{Name: "p"},
		Workspaces: []workspace.View{
			{Worktree: registry.Worktree{Project: "p", Branch: "feat", Path: path, Base: "main"}},
		},
	}}
}

func TestLedgerMsgStoresStatuses(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{
		projects: oneWorkspaceLedger("/wt/feat"),
		statuses: map[string]status.State{"/wt/feat": status.Working},
	})
	if m.statuses["/wt/feat"] != status.Working {
		t.Fatalf("statuses not stored: %+v", m.statuses)
	}
}

func TestRowShowsWorkingGlyphAndHidesIdle(t *testing.T) {
	working := (&config.Global{}).StatusLook().Look["working"].Glyph

	// Working agent → the glyph renders in its row.
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{
		projects: oneWorkspaceLedger("/wt/feat"),
		statuses: map[string]status.State{"/wt/feat": status.Working},
	})
	if row := m.renderRow(1, m.rows[1]); !strings.Contains(row, working) {
		t.Errorf("working row missing glyph %q: %q", working, row)
	}

	// Idle/absent agent → no glyph in the row (the dashboard highlights only
	// active agents). Assert on the row, not View(): the legend always shows it.
	m2 := New(nil, nil)
	m2, _ = step(m2, tea.WindowSizeMsg{Width: 80, Height: 24})
	m2, _ = step(m2, ledgerMsg{projects: oneWorkspaceLedger("/wt/feat")}) // no statuses
	if row := m2.renderRow(1, m2.rows[1]); strings.Contains(row, working) {
		t.Errorf("idle row should not show the working glyph: %q", row)
	}
}

func TestStatusChangedMsgTriggersRefresh(t *testing.T) {
	m := readyModel(t)
	_, cmd := step(m, statusChangedMsg{})
	if cmd == nil {
		t.Error("statusChangedMsg should return a refresh command")
	}
}

func TestWatcherReadyMsgStoresWatcher(t *testing.T) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer func() { _ = w.Close() }()

	m := readyModel(t)
	m, cmd := step(m, watcherReadyMsg{w: w})
	if m.watcher != w {
		t.Error("watcher not stored on the model")
	}
	if cmd == nil {
		t.Error("expected a listen command after the watcher is ready")
	}
}

func TestWatchAndListenStatus(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// watchStatusCmd creates the watcher on the status dir and hands it back.
	msg := watchStatusCmd()()
	ready, ok := msg.(watcherReadyMsg)
	if !ok || ready.w == nil {
		t.Fatalf("watchStatusCmd msg = %#v, want watcherReadyMsg with a watcher", msg)
	}
	defer func() { _ = ready.w.Close() }()

	m := New(nil, nil)
	m.watcher = ready.w

	// A write into the watched dir wakes the listener with a statusChangedMsg.
	done := make(chan tea.Msg, 1)
	go func() { done <- m.listenStatusCmd()() }()
	if err := status.Write("p", "feat", "/wt/feat", status.Working); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if _, ok := got.(statusChangedMsg); !ok {
			t.Errorf("listener msg = %#v, want statusChangedMsg", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not fire on a status-file change")
	}

	// A nil watcher yields no listen command.
	if (Model{}).listenStatusCmd() != nil {
		t.Error("listenStatusCmd with no watcher should return nil")
	}
}

func TestReadStatuses(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := status.Write("p", "feat", "/wt/feat", status.Working); err != nil {
		t.Fatal(err)
	}
	projects := []workspace.ProjectView{{
		Project: registry.Project{Name: "p"},
		Workspaces: []workspace.View{
			{Worktree: registry.Worktree{Project: "p", Branch: "feat", Path: "/wt/feat"}},
			{Worktree: registry.Worktree{Project: "p", Branch: "none", Path: "/wt/none"}},
		},
	}}
	got := readStatuses(projects, 5*time.Minute)
	if got["/wt/feat"] != status.Working {
		t.Errorf("feat = %q, want working", got["/wt/feat"])
	}
	if _, ok := got["/wt/none"]; ok {
		t.Error("a workspace with no status file should be absent from the map")
	}
}
