package sidebar

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func step(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func sample() []entry {
	return []entry{
		{winID: "@1", project: "alpha", branch: "feat-1", path: "/wt/a1", active: true},
		{winID: "@2", project: "beta", branch: "feat-2", path: "/wt/b2"},
	}
}

func loaded() Model {
	m, _ := step(New(""), tea.WindowSizeMsg{Width: 30, Height: 20})
	m, _ = step(m, entriesMsg{entries: sample()})
	return m
}

func TestInitReturnsCommand(t *testing.T) {
	if New("").Init() == nil {
		t.Error("Init() returned nil")
	}
}

func TestWindowSizeSetsDimensions(t *testing.T) {
	m, _ := step(New(""), tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.width != 40 || m.height != 10 {
		t.Errorf("size = %dx%d, want 40x10", m.width, m.height)
	}
}

func TestEntriesMsgPopulatesAndClamps(t *testing.T) {
	m := loaded()
	if len(m.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(m.entries))
	}
	m.cursor = 5
	// A shorter refresh clamps the cursor back into range.
	m, _ = step(m, entriesMsg{entries: sample()[:1]})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want clamped to 0", m.cursor)
	}
	// Emptying the list clamps to 0 (not -1).
	m, _ = step(m, entriesMsg{entries: nil})
	if m.cursor != 0 {
		t.Errorf("cursor on empty = %d, want 0", m.cursor)
	}
}

func TestEntriesMsgError(t *testing.T) {
	m, _ := step(loaded(), entriesMsg{err: errBoom{}})
	if m.errMsg == "" {
		t.Error("error not recorded")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestNavigationBounds(t *testing.T) {
	m := loaded()
	m.cursor = 0
	m, _ = step(m, runeKey("k")) // already at top
	if m.cursor != 0 {
		t.Errorf("k at top = %d, want 0", m.cursor)
	}
	m, _ = step(m, runeKey("j"))
	if m.cursor != 1 {
		t.Errorf("j = %d, want 1", m.cursor)
	}
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown}) // already at bottom
	if m.cursor != 1 {
		t.Errorf("down at bottom = %d, want 1", m.cursor)
	}
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("up = %d, want 0", m.cursor)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, msg := range []tea.Msg{runeKey("q"), tea.KeyMsg{Type: tea.KeyCtrlC}} {
		_, cmd := step(loaded(), msg)
		if cmd == nil {
			t.Fatalf("%v returned no command", msg)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v did not quit", msg)
		}
	}
}

func TestRefreshKeyAndEnterReturnCommands(t *testing.T) {
	m := loaded()
	if _, cmd := step(m, runeKey("r")); cmd == nil {
		t.Error("r returned no command")
	}
	m.cursor = 0
	if _, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Error("enter on an entry returned no command")
	}
}

func TestEnterWithNoEntries(t *testing.T) {
	m, _ := step(New(""), tea.WindowSizeMsg{Width: 30, Height: 20})
	if _, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("enter with no entries should not return a command")
	}
}

func TestTickReschedules(t *testing.T) {
	if _, cmd := step(loaded(), tickMsg(time.Now())); cmd == nil {
		t.Error("tick should return a command")
	}
}

func TestViewStates(t *testing.T) {
	// Populated.
	out := loaded().View()
	for _, want := range []string{"now", "alpha/feat-1", "beta/feat-2", "jump"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}

	// Empty.
	empty, _ := step(New(""), tea.WindowSizeMsg{Width: 30, Height: 20})
	if !strings.Contains(empty.View(), "no open workspace windows") {
		t.Errorf("empty view:\n%s", empty.View())
	}

	// Error.
	errM, _ := step(loaded(), entriesMsg{err: errBoom{}})
	if !strings.Contains(errM.View(), "boom") {
		t.Errorf("error view:\n%s", errM.View())
	}
}

func TestRenderEntrySelection(t *testing.T) {
	m := loaded()
	if got := m.renderEntry(0, m.entries[0]); !strings.HasPrefix(got, "❯ ") {
		t.Errorf("selected entry = %q, want ❯ prefix", got)
	}
	if got := m.renderEntry(1, m.entries[1]); strings.HasPrefix(got, "❯ ") {
		t.Errorf("unselected entry = %q, should not have ❯ prefix", got)
	}
}
