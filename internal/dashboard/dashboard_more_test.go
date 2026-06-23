package dashboard

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// readyModel returns a sized model with the sample ledger loaded.
func readyModel(t *testing.T) Model {
	t.Helper()
	m := New(nil, nil)
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{projects: sampleLedger()})
	return m
}

func TestInitReturnsCommand(t *testing.T) {
	if New(nil, nil).Init() == nil {
		t.Error("Init() returned nil command")
	}
}

func TestWindowSizeMakesReady(t *testing.T) {
	m, _ := step(New(nil, nil), tea.WindowSizeMsg{Width: 100, Height: 30})
	if !m.ready {
		t.Fatal("model not ready after WindowSizeMsg")
	}
	if m.width != 100 || m.height != 30 {
		t.Errorf("size = %dx%d, want 100x30", m.width, m.height)
	}
	if m.vp.Height != m.bodyHeight() {
		t.Errorf("vp height = %d, want %d", m.vp.Height, m.bodyHeight())
	}
}

func TestLedgerMsgPopulatesRows(t *testing.T) {
	m := New(nil, nil)
	if m.status != "loading…" {
		t.Fatalf("initial status = %q", m.status)
	}
	m, _ = step(m, ledgerMsg{projects: sampleLedger()})
	if len(m.rows) != 4 {
		t.Errorf("rows = %d, want 4", len(m.rows))
	}
	if m.status != "" {
		t.Errorf("status not cleared after load: %q", m.status)
	}
}

func TestLedgerMsgError(t *testing.T) {
	m, _ := step(New(nil, nil), ledgerMsg{err: errors.New("boom")})
	if !m.statusErr || !strings.Contains(m.status, "boom") {
		t.Errorf("status=%q err=%v", m.status, m.statusErr)
	}
}

func TestDiffMsgEntersDiffMode(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, diffMsg{title: "alpha/feat-1", content: "diff body"})
	if m.mode != modeDiff {
		t.Fatalf("mode = %v, want diff", m.mode)
	}
	if m.diffTitle != "alpha/feat-1" {
		t.Errorf("diffTitle = %q", m.diffTitle)
	}

	// Empty diff still enters diff mode with a placeholder.
	m2 := readyModel(t)
	m2, _ = step(m2, diffMsg{title: "x", content: ""})
	if m2.mode != modeDiff {
		t.Error("empty diff should still enter diff mode")
	}
	if !strings.Contains(m2.vp.View(), "no changes") {
		t.Errorf("placeholder missing: %q", m2.vp.View())
	}
}

func TestDiffMsgError(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, diffMsg{err: errors.New("nope")})
	if m.mode != modeLedger || !m.statusErr {
		t.Errorf("mode=%v statusErr=%v, want ledger+err", m.mode, m.statusErr)
	}
}

func TestActionMsg(t *testing.T) {
	m, _ := step(New(nil, nil), actionMsg{err: errors.New("x")})
	if !m.statusErr || !strings.Contains(m.status, "failed") {
		t.Errorf("error action: status=%q", m.status)
	}
	m, _ = step(New(nil, nil), actionMsg{msg: "done"})
	if m.statusErr || m.status != "done" {
		t.Errorf("ok action: status=%q", m.status)
	}
	_, cmd := step(New(nil, nil), actionMsg{msg: "done", refresh: true})
	if cmd == nil {
		t.Error("refresh action should return a refresh command")
	}
}

func TestTickRefreshesOnlyOnLedger(t *testing.T) {
	m := readyModel(t)
	_, cmd := step(m, tickMsg(time.Now()))
	if cmd == nil {
		t.Error("tick on ledger should return a command")
	}
	// In diff mode the tick keeps ticking but does not refresh the ledger.
	md, _ := step(m, diffMsg{title: "t", content: "c"})
	mdAfter, cmd := step(md, tickMsg(time.Now()))
	if cmd == nil {
		t.Error("tick should always reschedule")
	}
	if mdAfter.mode != modeDiff {
		t.Error("tick should not change mode")
	}
}

func TestCtrlCQuits(t *testing.T) {
	_, cmd := step(New(nil, nil), tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl-c returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl-c command did not produce QuitMsg")
	}
}

func TestLedgerNavigation(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0

	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("down → %d, want 1", m.cursor)
	}
	m, _ = step(m, runeKey("j"))
	if m.cursor != 2 {
		t.Errorf("j → %d, want 2", m.cursor)
	}
	m, _ = step(m, runeKey("k"))
	if m.cursor != 1 {
		t.Errorf("k → %d, want 1", m.cursor)
	}
	m, _ = step(m, runeKey("G"))
	if m.cursor != len(m.rows)-1 {
		t.Errorf("G → %d, want %d", m.cursor, len(m.rows)-1)
	}
	m, _ = step(m, runeKey("g"))
	if m.cursor != 0 {
		t.Errorf("g → %d, want 0", m.cursor)
	}
}

func TestLedgerQuit(t *testing.T) {
	m := readyModel(t)
	_, cmd := step(m, runeKey("q"))
	if cmd == nil || func() bool { _, ok := cmd().(tea.QuitMsg); return !ok }() {
		t.Error("q on ledger should quit")
	}
}

func TestEnterOpensDiffOnWorkspace(t *testing.T) {
	m := readyModel(t)
	m.cursor = 1 // alpha/feat-1
	m2, cmd := step(m, runeKey("d"))
	if cmd == nil {
		t.Fatal("d on workspace returned no diff command")
	}
	if m2.diffBranch != "feat-1" || m2.diffProject != "alpha" {
		t.Errorf("diff target = %s/%s", m2.diffProject, m2.diffBranch)
	}

	// On a project (base) row, enter opens the base checkout's diff.
	m.cursor = 0
	m4, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on a project row should produce a base-diff command")
	}
	if m4.diffProject != "alpha" {
		t.Errorf("base diff target project = %q, want alpha", m4.diffProject)
	}
}

func TestAddInputFlow(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0 // alpha header
	m, cmd := step(m, runeKey("a"))
	if m.mode != modeInput || cmd == nil {
		t.Fatalf("a → mode=%v cmd=%v", m.mode, cmd)
	}
	if m.addProject != "alpha" {
		t.Errorf("addProject = %q, want alpha", m.addProject)
	}

	// Submit a branch name.
	m.input.SetValue("new-feat")
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeLedger || cmd == nil {
		t.Errorf("submit → mode=%v cmd=%v", m.mode, cmd)
	}
	if !strings.Contains(m.status, "creating new-feat") {
		t.Errorf("status = %q", m.status)
	}
}

func TestAddInputEmptyAndCancel(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, runeKey("a"))
	m.input.SetValue("")
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.statusErr || !strings.Contains(m.status, "empty branch") {
		t.Errorf("empty submit status = %q", m.status)
	}

	m2 := readyModel(t)
	m2.cursor = 0
	m2, _ = step(m2, runeKey("a"))
	m2, _ = step(m2, tea.KeyMsg{Type: tea.KeyEsc})
	if m2.mode != modeLedger || !strings.Contains(m2.status, "cancelled") {
		t.Errorf("esc → mode=%v status=%q", m2.mode, m2.status)
	}
}

func TestAddWithoutProject(t *testing.T) {
	m := New(nil, nil) // no rows
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, runeKey("a"))
	if !m.statusErr || !strings.Contains(m.status, "no project") {
		t.Errorf("a with no project → status=%q", m.status)
	}
}

func TestMergeAndRemoveConfirm(t *testing.T) {
	for _, tc := range []struct {
		key, action string
	}{{"m", "merge"}, {"x", "rm"}} {
		m := readyModel(t)
		m.cursor = 1 // alpha/feat-1
		m, _ = step(m, runeKey(tc.key))
		if m.mode != modeConfirm {
			t.Fatalf("%s → mode=%v, want confirm", tc.key, m.mode)
		}
		if m.confirm.action != tc.action || m.confirm.branch != "feat-1" {
			t.Errorf("%s → confirm=%+v", tc.key, m.confirm)
		}

		// Confirm with y returns a command and returns to the ledger.
		m, cmd := step(m, runeKey("y"))
		if cmd == nil || m.mode != modeLedger {
			t.Errorf("%s confirm y → cmd=%v mode=%v", tc.key, cmd, m.mode)
		}
	}
}

func TestConfirmCancel(t *testing.T) {
	m := readyModel(t)
	m.cursor = 1
	m, _ = step(m, runeKey("m"))
	m, _ = step(m, runeKey("n"))
	if m.mode != modeLedger || !strings.Contains(m.status, "cancelled") {
		t.Errorf("confirm n → mode=%v status=%q", m.mode, m.status)
	}
}

func TestRefreshKey(t *testing.T) {
	m := readyModel(t)
	m, cmd := step(m, runeKey("r"))
	if cmd == nil || !strings.Contains(m.status, "refreshing") {
		t.Errorf("r → cmd=%v status=%q", cmd, m.status)
	}
}

func TestCopyKey(t *testing.T) {
	m := readyModel(t)
	m.cursor = 1
	_, cmd := step(m, runeKey("c"))
	if cmd == nil {
		t.Error("c on workspace returned no command")
	}
}

func TestOpenKeyWithRealManager(t *testing.T) {
	regPath := filepath.Join(t.TempDir(), "reg.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		if err := s.AddProject(registry.Project{Name: "alpha", Path: "/a"}); err != nil {
			return err
		}
		return s.AddWorktree(registry.Worktree{Project: "alpha", Path: "/wt/feat-1", Branch: "feat-1", Base: "main"})
	}); err != nil {
		t.Fatal(err)
	}
	mgr := workspace.New(regPath, &config.Global{})
	m := New(mgr, &config.Global{})
	m, _ = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(m, ledgerMsg{projects: sampleLedger()})
	m.cursor = 1 // alpha/feat-1
	_, cmd := step(m, runeKey("o"))
	if cmd == nil {
		t.Error("o on workspace returned no command")
	}
}

func TestDiffModeKeys(t *testing.T) {
	m := readyModel(t)
	m.diffProject, m.diffBranch = "alpha", "feat-1"
	m, _ = step(m, diffMsg{title: "alpha/feat-1", content: "line1\nline2"})
	if m.mode != modeDiff {
		t.Fatal("not in diff mode")
	}

	// r reloads (returns a command) and stays in diff mode.
	mr, cmd := step(m, runeKey("r"))
	if cmd == nil || mr.mode != modeDiff {
		t.Errorf("diff r → cmd=%v mode=%v", cmd, mr.mode)
	}

	// q returns to the ledger.
	mq, _ := step(m, runeKey("q"))
	if mq.mode != modeLedger {
		t.Errorf("diff q → mode=%v, want ledger", mq.mode)
	}
}
