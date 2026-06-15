package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	done := wsView("p", "calm", false)
	if l := workspaceLine(&done); !strings.Contains(l, "○") || !strings.Contains(l, "done") {
		t.Errorf("done line = %q", l)
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
