package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stack-bound/workflow/internal/ide"
)

func sampleIDEs() []ide.IDE {
	return []ide.IDE{
		{ID: "code", Name: "VS Code", GUI: true, Exec: []string{"code"}},
		{ID: "goland", Name: "GoLand", GUI: true, Exec: []string{"goland"}},
		{ID: "vim", Name: "Vim", Exec: []string{"vim"}},
	}
}

// e / o on a workspace row dispatch the (async) edit command and stay on the
// ledger until the resulting editMsg arrives.
func TestEditKeysDispatchCommand(t *testing.T) {
	for _, k := range []string{"e", "o"} {
		m := readyModel(t)
		m.cursor = 1 // alpha/feat-1
		m2, cmd := step(m, runeKey(k))
		if cmd == nil {
			t.Errorf("%q should dispatch a command", k)
		}
		if m2.mode != modeLedger {
			t.Errorf("%q should not change mode synchronously, got %v", k, m2.mode)
		}
	}
}

// e / o on a project (base) row dispatch the edit command for the base checkout
// at the project root, staying on the ledger until the editMsg arrives.
func TestEditKeyOnProjectRowOpensBase(t *testing.T) {
	for _, k := range []string{"e", "o"} {
		m := readyModel(t)
		m.cursor = 0 // alpha (base) row
		m2, cmd := step(m, runeKey(k))
		if cmd == nil {
			t.Errorf("%q on a project row should dispatch a base-edit command", k)
		}
		if m2.mode != modeLedger {
			t.Errorf("%q should not change mode synchronously, got %v", k, m2.mode)
		}
	}
}

func TestHandleEditMsgOpensPicker(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{
		project: "alpha", branch: "feat-1", path: "/wt/feat-1",
		ides: sampleIDEs(), defaultID: "goland", forcePicker: true,
	})
	if m.mode != modePicker {
		t.Fatalf("mode = %v, want modePicker", m.mode)
	}
	if m.pickerProject != "alpha" || m.pickerPath != "/wt/feat-1" {
		t.Errorf("picker target not stored: %+v", m)
	}
}

// With autolaunch on and the default editor detected, editMsg launches it
// directly instead of opening the picker.
func TestHandleEditMsgAutolaunch(t *testing.T) {
	m := readyModel(t)
	m, cmd := step(m, editMsg{
		project: "alpha", branch: "feat-1", path: "/wt/feat-1",
		ides: sampleIDEs(), defaultID: "goland", autolaunch: true, forcePicker: false,
	})
	if m.mode != modeLedger {
		t.Errorf("autolaunch should not open the picker, mode = %v", m.mode)
	}
	if cmd == nil {
		t.Error("autolaunch should return a launch command")
	}
}

// Autolaunch with a default that isn't installed falls back to the picker.
func TestHandleEditMsgAutolaunchMissingDefault(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{
		project: "alpha", branch: "feat-1", path: "/wt/feat-1",
		ides: sampleIDEs(), defaultID: "rider", autolaunch: true,
	})
	if m.mode != modePicker {
		t.Errorf("missing autolaunch default should open the picker, mode = %v", m.mode)
	}
}

func TestHandleEditMsgError(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{err: errors.New("no such workspace")})
	if m.mode != modeLedger || !m.statusErr {
		t.Errorf("edit error should surface on the ledger: mode=%v err=%v", m.mode, m.statusErr)
	}
}

// Choosing an editor in the picker returns to the ledger and dispatches a
// launch command.
func TestPickerLaunchReturnsToLedger(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{project: "alpha", branch: "feat-1", path: "/wt/feat-1", ides: sampleIDEs()})
	if m.mode != modePicker {
		t.Fatalf("setup: mode = %v", m.mode)
	}
	m, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeLedger {
		t.Errorf("after choosing, mode = %v, want ledger", m.mode)
	}
	if cmd == nil {
		t.Error("choosing an editor should dispatch a launch command")
	}
}

func TestPickerSetDefaultReturnsCommand(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{project: "alpha", branch: "feat-1", path: "/wt/feat-1", ides: sampleIDEs(), forcePicker: true})
	m, cmd := step(m, runeKey("d")) // set default
	if m.mode != modeLedger || cmd == nil {
		t.Errorf("set-default should return to ledger with a persist command: mode=%v cmd=%v", m.mode, cmd)
	}
}

func TestPickerCancel(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{project: "alpha", branch: "feat-1", ides: sampleIDEs()})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeLedger {
		t.Errorf("esc should close the picker, mode = %v", m.mode)
	}
	if !strings.Contains(m.status, "cancel") {
		t.Errorf("status = %q, want a cancel note", m.status)
	}
}

func TestViewPickerRendersOverLedger(t *testing.T) {
	m := readyModel(t)
	m, _ = step(m, editMsg{project: "alpha", branch: "feat-1", ides: sampleIDEs(), defaultID: "goland"})
	out := m.View()
	if !strings.Contains(out, "GoLand") {
		t.Error("picker overlay should show editor names over the ledger")
	}
	// The ledger underneath is still partly visible (title stays at the top).
	if !strings.Contains(out, "WorkFlow") {
		t.Error("ledger title should remain visible behind the popup")
	}
}
