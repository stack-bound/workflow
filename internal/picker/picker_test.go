package picker

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stack-bound/workflow/internal/ide"
)

func step(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func sample() []ide.IDE {
	return []ide.IDE{
		{ID: "code", Name: "VS Code", GUI: true, Exec: []string{"code"}},
		{ID: "goland", Name: "GoLand", GUI: true, Exec: []string{"goland"}},
		{ID: "vim", Name: "Vim", Exec: []string{"vim"}},
	}
}

func TestDefaultOrderedFirstAndPreselected(t *testing.T) {
	m := New(sample(), "goland")
	if m.ides[0].ID != "goland" {
		t.Errorf("default not first: %s", m.ides[0].ID)
	}
	cur, ok := m.current()
	if !ok || cur.ID != "goland" {
		t.Errorf("default not pre-selected: %+v ok=%v", cur, ok)
	}
}

func TestNavigateAndLaunch(t *testing.T) {
	m := New(sample(), "") // no default: catalog order
	m = step(m, key("j"))  // -> goland
	m = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done() {
		t.Fatal("enter should finish the picker")
	}
	r := m.Result()
	if r.Action != Launch || r.IDE.ID != "goland" {
		t.Errorf("result = %+v, want Launch goland", r)
	}
}

func TestSetDefaultKey(t *testing.T) {
	m := New(sample(), "")
	m = step(m, key("d"))
	r := m.Result()
	if r.Action != SetDefault || r.IDE.ID != "code" {
		t.Errorf("result = %+v, want SetDefault code", r)
	}
}

func TestSetDefaultAutolaunchKey(t *testing.T) {
	m := New(sample(), "")
	m = step(m, key("j")) // -> goland
	m = step(m, key("a"))
	r := m.Result()
	if r.Action != SetDefaultAutolaunch || r.IDE.ID != "goland" {
		t.Errorf("result = %+v, want SetDefaultAutolaunch goland", r)
	}
}

func TestCancel(t *testing.T) {
	m := New(sample(), "")
	m = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Done() || m.Result().Action != Cancel {
		t.Errorf("esc should cancel: done=%v result=%+v", m.Done(), m.Result())
	}
}

func TestNavigationClamps(t *testing.T) {
	m := New(sample(), "")
	for i := 0; i < 10; i++ {
		m = step(m, key("k")) // up past the top
	}
	if m.cursor != 0 {
		t.Errorf("cursor underflowed to %d", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m = step(m, key("j")) // down past the bottom
	}
	if m.cursor != len(m.ides)-1 {
		t.Errorf("cursor overflowed to %d (len=%d)", m.cursor, len(m.ides))
	}
}

func TestEmptyPickerEnterIsNoop(t *testing.T) {
	m := New(nil, "")
	m = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Done() {
		t.Error("enter with no editors should not finish")
	}
	// But cancel still works.
	m = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Done() || m.Result().Action != Cancel {
		t.Error("esc should cancel an empty picker")
	}
}

func TestViewAndBoxRender(t *testing.T) {
	m := New(sample(), "goland")
	m = step(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(m.Box(), "GoLand") {
		t.Error("box should list editor names")
	}
	if v := m.View(); v == "" {
		t.Error("View should render")
	}
	// Empty-state box renders too.
	if !strings.Contains(New(nil, "").Box(), "No editors detected") {
		t.Error("empty box should explain there are no editors")
	}
}
