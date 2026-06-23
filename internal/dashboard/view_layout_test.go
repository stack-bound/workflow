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

// TestMainSummary covers the base-checkout summary in both render paths: the
// branch and state show through, and a missing branch degrades to "(no branch)".
func TestMainSummary(t *testing.T) {
	clean := workspace.MainCheckout{Branch: "development"}
	for _, got := range []string{mainSummaryStyled(clean), mainSummaryPlain(clean)} {
		if !strings.Contains(got, "development") || !strings.Contains(got, "clean") {
			t.Errorf("clean summary = %q, want branch+clean", got)
		}
	}
	dirty := workspace.MainCheckout{Branch: "main", Dirty: true}
	if got := mainSummaryStyled(dirty); !strings.Contains(got, "main") || !strings.Contains(got, "dirty") {
		t.Errorf("dirty summary = %q, want branch+dirty", got)
	}
	missing := workspace.MainCheckout{Err: errors.New("not a repo")}
	for _, got := range []string{mainSummaryStyled(missing), mainSummaryPlain(missing)} {
		if !strings.Contains(got, "no branch") {
			t.Errorf("missing-branch summary = %q, want (no branch)", got)
		}
	}
}

// TestProjectRowShowsBaseBranch asserts the project row renders the project
// name, the branch its root checkout is on, its state, and the project path —
// and that selection adds the cursor.
func TestProjectRowShowsBaseBranch(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: projectLedger()})

	row := m.renderRow(0, m.rows[0]) // project (base) row
	for _, want := range []string{"workflow", "development", "clean", "/srv/workflow"} {
		if !strings.Contains(row, want) {
			t.Errorf("project row missing %q:\n%s", want, row)
		}
	}

	m.cursor = 0
	if sel := m.renderRow(0, m.rows[0]); !strings.HasPrefix(sel, "❯ ") {
		t.Errorf("selected project row should start with the cursor: %q", sel)
	}
}

// TestWorktreeRowShowsPath asserts a worktree row displays its directory to the
// right of the status columns.
func TestWorktreeRowShowsPath(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 140, Height: 24})
	pv := []workspace.ProjectView{{
		Project:    registry.Project{Name: "workflow", Path: "/srv/workflow"},
		Main:       workspace.MainCheckout{Path: "/srv/workflow", Branch: "development"},
		Workspaces: []workspace.View{wsViewPath("workflow", "feature/x", "/srv/workflow_worktrees/feature-x")},
	}}
	m, _ = step(m, ledgerMsg{projects: pv})
	if row := m.renderRow(1, m.rows[1]); !strings.Contains(row, "feature-x") {
		t.Errorf("worktree row should show its directory path:\n%s", row)
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
	m.setRows(projectLedger()) // [project, feature/layout, feature/docs]
	if m.isLastWorkspace(1) {
		t.Error("first worktree (followed by another) should not be last")
	}
	if !m.isLastWorkspace(2) {
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

// TestProjectRowOpenIndicator asserts the project (base) row shows the tmux
// open marker once its root path has a window open.
func TestProjectRowOpenIndicator(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 120, Height: 24})
	m, _ = step(m, ledgerMsg{projects: projectLedger()})

	if got := m.renderRow(0, m.rows[0]); strings.Contains(got, "▣") {
		t.Errorf("closed base checkout should not show ▣:\n%s", got)
	}
	m.openPaths = map[string]bool{"/srv/workflow": true}
	if got := m.renderRow(0, m.rows[0]); !strings.Contains(got, "▣") {
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
