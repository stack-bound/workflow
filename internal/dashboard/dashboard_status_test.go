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
	if row := m.renderRow(2, m.rows[2]); !strings.Contains(row, working) {
		t.Errorf("working row missing glyph %q: %q", working, row)
	}

	// Idle/absent agent → no glyph in the row (the dashboard highlights only
	// active agents). Assert on the row, not View(): the legend always shows it.
	m2 := New(nil, nil)
	m2, _ = step(m2, tea.WindowSizeMsg{Width: 80, Height: 24})
	m2, _ = step(m2, ledgerMsg{projects: oneWorkspaceLedger("/wt/feat")}) // no statuses
	if row := m2.renderRow(2, m2.rows[2]); strings.Contains(row, working) {
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

// The base/root checkout now carries agent status: a base status file (keyed by
// the project-root path) lights up the agent cell on the base row, and readStatuses
// surfaces it under that path.
func TestMainRowShowsAgentCell(t *testing.T) {
	ready := (&config.Global{}).StatusLook().Look["ready"].Glyph
	projects := []workspace.ProjectView{{
		Project: registry.Project{Name: "p", Path: "/repo/p"},
		Main:    workspace.MainCheckout{Branch: "main"},
		Workspaces: []workspace.View{
			{Worktree: registry.Worktree{Project: "p", Branch: "feat", Path: "/wt/feat", Base: "main"}},
		},
	}}

	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{
		projects: projects,
		statuses: map[string]status.State{"/repo/p": status.Ready},
	})
	// Row 1 is the base/main row (row 0 is the project header).
	if row := m.renderRow(1, m.rows[1]); !strings.Contains(row, ready) {
		t.Errorf("base row missing ready glyph %q: %q", ready, row)
	}

	// With no base status the cell stays blank (no stray glyph).
	m2 := New(nil, nil)
	m2, _ = step(m2, tea.WindowSizeMsg{Width: 80, Height: 24})
	m2, _ = step(m2, ledgerMsg{projects: projects})
	if row := m2.renderRow(1, m2.rows[1]); strings.Contains(row, ready) {
		t.Errorf("idle base row should not show a glyph: %q", row)
	}
}

func TestReadStatusesIncludesBase(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := status.WriteBase("p", "/repo/p", status.Ready); err != nil {
		t.Fatal(err)
	}
	projects := []workspace.ProjectView{{Project: registry.Project{Name: "p", Path: "/repo/p"}}}
	got := readStatuses(projects, 30*time.Minute)
	if got["/repo/p"] != status.Ready {
		t.Errorf("base status = %q, want ready", got["/repo/p"])
	}
}

func TestLegendIncludesReady(t *testing.T) {
	ready := (&config.Global{}).StatusLook().Look["ready"].Glyph
	if got := New(nil, nil).ledgerLegend(); !strings.Contains(got, ready) {
		t.Errorf("legend missing ready glyph %q: %q", ready, got)
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
