package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

func TestViewLoadingBeforeReady(t *testing.T) {
	if got := New(nil, nil).View(); got != "loading…" {
		t.Errorf("pre-ready View = %q, want loading…", got)
	}
}

func TestViewLedgerRendersRows(t *testing.T) {
	m := readyModel(t)
	out := m.View()
	for _, want := range []string{"WorkFlow — dashboard", "alpha", "feat-1", "beta"} {
		if !strings.Contains(out, want) {
			t.Errorf("ledger view missing %q:\n%s", want, out)
		}
	}
	// Footer help line for the ledger.
	if !strings.Contains(out, "enter diff") {
		t.Errorf("ledger footer missing help:\n%s", out)
	}
}

func TestViewLedgerEmpty(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{projects: nil})
	if !strings.Contains(m.View(), "No projects registered") {
		t.Errorf("empty ledger view:\n%s", m.View())
	}
}

func TestViewDiff(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, diffMsg{title: "alpha/feat-1", content: "+added\n-removed"})
	out := m.View()
	if !strings.Contains(out, "diff — alpha/feat-1") {
		t.Errorf("diff view missing title:\n%s", out)
	}
	if !strings.Contains(out, "scroll") {
		t.Errorf("diff view missing help:\n%s", out)
	}
}

func TestFooterInputAndConfirm(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, runeKey("a")) // enter input mode
	if !strings.Contains(m.View(), "enter create") {
		t.Errorf("input footer help missing:\n%s", m.View())
	}

	m2 := readyModel(t)
	m2.cursor = 1
	m2, _ = step(m2, runeKey("x")) // confirm rm
	out := m2.View()
	if !strings.Contains(out, "y/n") || !strings.Contains(out, "feat-1") {
		t.Errorf("confirm footer missing prompt:\n%s", out)
	}
}

func TestConfirmPrompt(t *testing.T) {
	mkView := func(dirty bool, ahead, added, deleted int, statErr error) *workspace.View {
		v := wsView("alpha", "feat-1", dirty)
		v.Worktree.Base = "master"
		v.Stat.Ahead, v.Stat.Added, v.Stat.Deleted = ahead, added, deleted
		v.StatErr = statErr
		return &v
	}

	t.Run("merge is a plain caution", func(t *testing.T) {
		got := confirmFor("merge", mkView(true, 3, 9, 1, nil)).prompt()
		if !strings.Contains(got, "Merge alpha/feat-1?") || !strings.Contains(got, "[y/n]") {
			t.Errorf("merge prompt = %q", got)
		}
	})

	t.Run("clean and merged branch is safe", func(t *testing.T) {
		got := confirmFor("rm", mkView(false, 0, 0, 0, nil)).prompt()
		if !strings.Contains(got, "Safe to remove") || !strings.Contains(got, "vs master") {
			t.Errorf("safe prompt = %q", got)
		}
		if strings.Contains(got, "discards") {
			t.Errorf("safe prompt should not warn of discarded work: %q", got)
		}
	})

	t.Run("uncommitted changes warn", func(t *testing.T) {
		got := confirmFor("rm", mkView(true, 0, 0, 0, nil)).prompt()
		if !strings.Contains(got, "discards uncommitted changes") || !strings.Contains(got, "work will be lost") {
			t.Errorf("dirty prompt = %q", got)
		}
	})

	t.Run("unmerged commits warn with counts", func(t *testing.T) {
		got := confirmFor("rm", mkView(false, 2, 9, 1, nil)).prompt()
		if !strings.Contains(got, "2 unmerged commits (+9 -1 vs master)") {
			t.Errorf("ahead prompt = %q", got)
		}
	})

	t.Run("single unmerged commit is singular", func(t *testing.T) {
		got := confirmFor("rm", mkView(false, 1, 4, 0, nil)).prompt()
		if !strings.Contains(got, "1 unmerged commit (") {
			t.Errorf("singular prompt = %q", got)
		}
	})

	t.Run("dirty and ahead are joined", func(t *testing.T) {
		got := confirmFor("rm", mkView(true, 1, 4, 0, nil)).removeRisk()
		if !strings.Contains(got, "uncommitted changes and 1 unmerged commit") {
			t.Errorf("combined risk = %q", got)
		}
	})

	t.Run("unknown status stays cautious", func(t *testing.T) {
		got := confirmFor("rm", mkView(false, 0, 0, 0, errors.New("boom"))).prompt()
		if !strings.Contains(got, "couldn't be verified") || strings.Contains(got, "Safe to remove") {
			t.Errorf("statErr prompt = %q", got)
		}
	})
}

func TestWorkspaceLine(t *testing.T) {
	clean := wsView("p", "calm", false)
	if l := workspaceLine(&clean); !strings.Contains(l, "○") || !strings.Contains(l, "clean") {
		t.Errorf("clean line = %q", l)
	}
	// "done" implied finished work; the clean state must not use it.
	if l := workspaceLine(&clean); strings.Contains(l, "done") {
		t.Errorf("clean line should not say done: %q", l)
	}

	active := wsView("p", "busy", true)
	if l := workspaceLine(&active); !strings.Contains(l, "●") || !strings.Contains(l, "active") {
		t.Errorf("active line = %q", l)
	}

	bad := workspace.View{Worktree: registry.Worktree{Branch: "broken", Base: "main"}, StatErr: errors.New("missing")}
	if l := workspaceLine(&bad); !strings.Contains(l, "!") || !strings.Contains(l, "missing") {
		t.Errorf("error line = %q", l)
	}
}

// TestWorkspaceStyled covers the accent-coloured row: branch and state text
// survive styling, the dirty * shows for uncommitted work, and a stat error
// degrades to the "! branch error" form.
func TestWorkspaceStyled(t *testing.T) {
	dirty := wsView("p", "busy", true)
	dirty.Stat.Added, dirty.Stat.Deleted = 12, 3
	l := workspaceStyled(&dirty)
	for _, want := range []string{"busy", "active", "+12", "-3", "*"} {
		if !strings.Contains(l, want) {
			t.Errorf("active styled line missing %q: %q", want, l)
		}
	}

	clean := wsView("p", "calm", false)
	l = workspaceStyled(&clean)
	if !strings.Contains(l, "calm") || !strings.Contains(l, "clean") {
		t.Errorf("clean styled line = %q", l)
	}
	if strings.Contains(l, "*") {
		t.Errorf("clean styled line should have no dirty marker: %q", l)
	}

	bad := workspace.View{Worktree: registry.Worktree{Branch: "broken", Base: "main"}, StatErr: errors.New("missing")}
	if l := workspaceStyled(&bad); !strings.Contains(l, "!") || !strings.Contains(l, "missing") {
		t.Errorf("error styled line = %q", l)
	}
}

// TestBehindAheadOrder pins the column to behind-first (↓) then ahead (↑), each
// arrow bound to its own count, matching the "behind ahead" heading.
func TestBehindAheadOrder(t *testing.T) {
	v := wsView("p", "lagging", false)
	v.Stat.Behind, v.Stat.Ahead = 3, 0
	if got := behindAhead(&v); got != "↓3|↑0" {
		t.Errorf("behindAhead = %q, want %q", got, "↓3|↑0")
	}
	v.Stat.Behind, v.Stat.Ahead = 1, 5
	if got := behindAhead(&v); got != "↓1|↑5" {
		t.Errorf("behindAhead = %q, want %q", got, "↓1|↑5")
	}
}

// TestLedgerHeaderAndLegend asserts the new column headings and the glyph key
// render into the ledger view so the numbers and symbols are self-explaining.
func TestLedgerHeaderAndLegend(t *testing.T) {
	out := readyModel(t).View()
	for _, want := range []string{
		"branch", "state", "behind|ahead", "diff", "base", // column headings
		"● active", "○ clean", "▣ tmux open", // glyph key
		"↓behind|↑ahead", "+added", "-removed", "* uncommitted", // column key
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ledger view missing %q:\n%s", want, out)
		}
	}
}

// TestLedgerHeaderAlignsWithRow checks the heading's "branch" label sits at the
// same display column as the branch name in a rendered row below it, so the
// table reads straight down. Byte offsets differ (the status mark is multibyte),
// so the comparison is on visible width.
func TestLedgerHeaderAlignsWithRow(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0                     // project selected → the workspace rows render unselected (accent-styled)
	row := m.renderRow(1, m.rows[1]) // alpha/feat-1
	h := ledgerHeader()

	hCol := lipgloss.Width(h[:strings.Index(h, "branch")])
	rCol := lipgloss.Width(row[:strings.Index(row, "feat-1")])
	if hCol != rCol {
		t.Errorf("branch heading at col %d, branch name at col %d:\nhdr: %q\nrow: %q", hCol, rCol, h, row)
	}
}

func TestRenderRowSelectionAndKinds(t *testing.T) {
	m := readyModel(t)
	// Project header row (index 0) selected.
	m.cursor = 0
	if got := m.renderRow(0, m.rows[0]); !strings.Contains(got, "alpha") || !strings.HasPrefix(got, "❯ ") {
		t.Errorf("selected project row = %q", got)
	}
	// Workspace row not selected.
	if got := m.renderRow(1, m.rows[1]); !strings.Contains(got, "feat-1") || strings.HasPrefix(got, "❯ ") {
		t.Errorf("unselected workspace row = %q", got)
	}
}

func TestBodyHeightClamp(t *testing.T) {
	m := New(nil, nil)
	m.height = 0
	if m.bodyHeight() != 1 {
		t.Errorf("bodyHeight(0) = %d, want 1", m.bodyHeight())
	}
	m.height = 24
	if m.bodyHeight() != 20 {
		t.Errorf("bodyHeight(24) = %d, want 20", m.bodyHeight())
	}
}
