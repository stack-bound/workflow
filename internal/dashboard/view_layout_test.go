package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

// projectLedger builds a one-project ledger whose base (main) checkout is on a
// known branch (clean), for exercising the project-row renderer.
func projectLedger() []workspace.ProjectView {
	return []workspace.ProjectView{{
		Project: registry.Project{Name: "workflow", Path: "/srv/workflow"},
		Main:    workspace.MainCheckout{Path: "/srv/workflow", Branch: "development"},
		Workspaces: []workspace.View{
			wsView("workflow", "feature/layout", true),
			wsView("workflow", "feature/docs", false),
		},
	}}
}

// TestMainState pins the base-checkout circle/word: dirty roots read "● dirty",
// clean roots "○ clean".
func TestMainState(t *testing.T) {
	if mark, word, _ := mainState(workspace.MainCheckout{Dirty: true}); mark != "●" || word != "dirty" {
		t.Errorf("dirty main state = %q/%q, want ●/dirty", mark, word)
	}
	if mark, word, _ := mainState(workspace.MainCheckout{Dirty: false}); mark != "○" || word != "clean" {
		t.Errorf("clean main state = %q/%q, want ○/clean", mark, word)
	}
}

// TestMainSummary covers the base-checkout line in both render paths: the branch
// and state show through, and a broken root degrades to "! — <message>".
func TestMainSummary(t *testing.T) {
	clean := workspace.MainCheckout{Branch: "development"}
	for _, got := range []string{mainStyled(clean), mainLine(clean)} {
		if !strings.Contains(got, "development") || !strings.Contains(got, "clean") {
			t.Errorf("clean line = %q, want branch+clean", got)
		}
	}
	dirty := workspace.MainCheckout{Branch: "main", Dirty: true}
	if got := mainStyled(dirty); !strings.Contains(got, "main") || !strings.Contains(got, "dirty") {
		t.Errorf("dirty line = %q, want branch+dirty", got)
	}
	missing := workspace.MainCheckout{Err: errors.New("not a repo")}
	for _, got := range []string{mainStyled(missing), mainLine(missing)} {
		if !strings.Contains(got, "not a repo") || !strings.Contains(got, "—") {
			t.Errorf("broken-root line = %q, want the error message and —", got)
		}
	}

	// A multi-line git stderr dump is collapsed to its first line so it can never
	// wrap the base row and break the table alignment.
	noisy := workspace.MainCheckout{Err: errors.New("fatal: not a git repository\nStopping at filesystem boundary")}
	for _, got := range []string{mainStyled(noisy), mainLine(noisy)} {
		if strings.Contains(got, "Stopping at filesystem boundary") || strings.Contains(got, "\n") {
			t.Errorf("multi-line error not collapsed: %q", got)
		}
	}
}

// TestProjectRowShowsBaseBranch asserts the project header carries the project
// name and path (no status columns), the base (main) row carries the branch its
// root checkout is on and its state, and selection adds the cursor.
func TestProjectRowShowsBaseBranch(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: projectLedger()})

	header := m.renderRow(0, m.rows[0]) // project header
	for _, want := range []string{"workflow", "/srv/workflow"} {
		if !strings.Contains(header, want) {
			t.Errorf("project header missing %q:\n%s", want, header)
		}
	}
	if strings.Contains(header, "development") || strings.Contains(header, "clean") {
		t.Errorf("project header should carry no base status columns:\n%s", header)
	}

	base := m.renderRow(1, m.rows[1]) // base (main) row
	for _, want := range []string{"development", "clean"} {
		if !strings.Contains(base, want) {
			t.Errorf("base row missing %q:\n%s", want, base)
		}
	}

	m.cursor = 0
	if sel := m.renderRow(0, m.rows[0]); !strings.HasPrefix(sel, "❯ ") {
		t.Errorf("selected project header should start with the cursor: %q", sel)
	}
}

// TestWorktreeRowOmitsPath asserts a worktree row no longer carries its
// directory path — that lives on the project header now, to keep rows tidy.
func TestWorktreeRowOmitsPath(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 140, Height: 24})
	pv := []workspace.ProjectView{{
		Project:    registry.Project{Name: "workflow", Path: "/srv/workflow"},
		Main:       workspace.MainCheckout{Path: "/srv/workflow", Branch: "development"},
		Workspaces: []workspace.View{wsViewPath("workflow", "feature/x", "/srv/workflow_worktrees/feature-x")},
	}}
	m, _ = step(m, ledgerMsg{projects: pv})
	if row := m.renderRow(2, m.rows[2]); strings.Contains(row, "feature-x") {
		t.Errorf("worktree row should not show its directory path:\n%s", row)
	}
}

// TestConnectorFor pins the tree connectors: the final child gets └─, earlier
// children ├─.
func TestConnectorFor(t *testing.T) {
	if got := connectorFor(false); got != "├─ " {
		t.Errorf("non-last connector = %q, want ├─ ", got)
	}
	if got := connectorFor(true); got != "└─ " {
		t.Errorf("last connector = %q, want └─ ", got)
	}
}

// TestIsLastWorkspace checks the last-child detection that drives the connector:
// in a project the final worktree is "last", earlier ones are not.
func TestIsLastWorkspace(t *testing.T) {
	m := New(nil, nil)
	m.setRows(projectLedger()) // [project, main, feature/layout, feature/docs]
	if m.isLastWorkspace(2) {
		t.Error("first worktree (followed by another) should not be last")
	}
	if !m.isLastWorkspace(3) {
		t.Error("final worktree (end of rows) should be last")
	}
}

// TestHeaderBarCounts asserts the header bar summarises the ledger.
func TestHeaderBarCounts(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: projectLedger()})
	if bar := m.headerBar(); !strings.Contains(bar, "1 projects") || !strings.Contains(bar, "2 worktrees") {
		t.Errorf("header bar = %q, want 1 projects · 2 worktrees", bar)
	}
}

// TestMainRowSelectedAndBroken covers the base row's selected highlight and a
// broken-root degradation: a missing/non-repo root shows an em dash and the
// error message in place of a branch and state, with the cursor when selected.
func TestMainRowSelectedAndBroken(t *testing.T) {
	pv := []workspace.ProjectView{{
		Project: registry.Project{Name: "tmp", Path: "/tmp/tmp.XXXX"},
		Main:    workspace.MainCheckout{Path: "/tmp/tmp.XXXX", Err: errors.New("not a git repo")},
	}}
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: pv})

	m.cursor = 1 // the base (main) row
	got := m.renderRow(1, m.rows[1])
	if !strings.HasPrefix(got, "❯ ") {
		t.Errorf("selected base row should start with the cursor: %q", got)
	}
	for _, want := range []string{"—", "not a git repo"} {
		if !strings.Contains(got, want) {
			t.Errorf("broken base row missing %q:\n%s", want, got)
		}
	}
}

// TestMainRowOpenIndicator asserts the base (main) row shows the tmux open
// marker once its root path has a window open.
func TestMainRowOpenIndicator(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: projectLedger()})

	if got := m.renderRow(1, m.rows[1]); strings.Contains(got, "▣") {
		t.Errorf("closed base checkout should not show ▣:\n%s", got)
	}
	m.openPaths = map[string]bool{"/srv/workflow": true}
	if got := m.renderRow(1, m.rows[1]); !strings.Contains(got, "▣") {
		t.Errorf("open base checkout should show ▣:\n%s", got)
	}
}

// TestOpenMainWindowEmptyPath covers the guard: launching a base window with no
// known root surfaces an error rather than shelling out to tmux.
func TestOpenMainWindowEmptyPath(t *testing.T) {
	msg := New(nil, nil).openMainWindowCmd("proj", "", "branch")()
	am, ok := msg.(actionMsg)
	if !ok || am.err == nil {
		t.Fatalf("empty-path base window = %#v, want an error actionMsg", msg)
	}
}
