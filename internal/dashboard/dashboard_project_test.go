package dashboard

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

// --- opening the project menu ---

func TestEnterOnProjectRowOpensMenu(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0 // alpha header
	m, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeMenu {
		t.Fatalf("enter on project row → mode=%v, want modeMenu", m.mode)
	}
	if m.menuProject != "alpha" {
		t.Errorf("menuProject = %q, want alpha", m.menuProject)
	}
	if cmd != nil {
		t.Error("opening the menu should not dispatch a command")
	}
}

// "d" (the diff alias) on a project header has nothing to show and must not open
// the menu — only enter does.
func TestDiffAliasOnProjectRowDoesNotOpenMenu(t *testing.T) {
	m := readyModel(t)
	m.cursor = 4 // beta header
	m, _ = step(m, runeKey("d"))
	if m.mode != modeLedger {
		t.Errorf("d on project row → mode=%v, want ledger", m.mode)
	}
	if !m.statusErr {
		t.Error("d on a project row should hint to select a workspace")
	}
}

// --- menu navigation ---

func TestMenuNavigationAndCancel(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // open menu, cursor on Rename
	if m.menu.cursor != 0 {
		t.Fatalf("menu opens on cursor %d, want 0", m.menu.cursor)
	}

	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.menu.cursor != 1 {
		t.Errorf("down → %d, want 1", m.menu.cursor)
	}
	// Down past the end clamps.
	m, _ = step(m, runeKey("j"))
	if m.menu.cursor != len(m.menu.items)-1 {
		t.Errorf("j past end → %d, want %d", m.menu.cursor, len(m.menu.items)-1)
	}
	m, _ = step(m, runeKey("k"))
	if m.menu.cursor != 0 {
		t.Errorf("k → %d, want 0", m.menu.cursor)
	}
	m, _ = step(m, runeKey("G"))
	if m.menu.cursor != len(m.menu.items)-1 {
		t.Errorf("G → %d, want last", m.menu.cursor)
	}
	m, _ = step(m, runeKey("g"))
	if m.menu.cursor != 0 {
		t.Errorf("g → %d, want 0", m.menu.cursor)
	}

	// esc dismisses back to the ledger.
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeLedger {
		t.Errorf("esc → mode=%v, want ledger", m.mode)
	}
}

// --- rename flow ---

func TestMenuRenameFlow(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})    // open menu
	m, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter}) // select Rename (cursor 0)
	if m.mode != modeRename {
		t.Fatalf("select Rename → mode=%v, want modeRename", m.mode)
	}
	if m.input.Value() != "alpha" {
		t.Errorf("rename input prefill = %q, want alpha", m.input.Value())
	}
	if cmd == nil {
		t.Error("entering rename should start the cursor blink")
	}

	// Submitting a new name returns to the ledger with a rename command.
	m.input.SetValue("gamma")
	m, cmd = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeLedger || cmd == nil {
		t.Errorf("rename submit → mode=%v cmd=%v", m.mode, cmd)
	}
	if !strings.Contains(m.status, "renaming alpha") {
		t.Errorf("status = %q", m.status)
	}
}

func TestRenameEmptyUnchangedAndCancel(t *testing.T) {
	// Empty name cancels with an error.
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // Rename
	m.input.SetValue("   ")
	m, cmd := step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || !m.statusErr || !strings.Contains(m.status, "empty name") {
		t.Errorf("empty rename → cmd=%v status=%q", cmd, m.status)
	}

	// Unchanged name is a no-op success (no command).
	m2 := readyModel(t)
	m2.cursor = 0
	m2, _ = step(m2, tea.KeyMsg{Type: tea.KeyEnter})
	m2, _ = step(m2, tea.KeyMsg{Type: tea.KeyEnter})
	m2.input.SetValue("alpha")
	m2, cmd2 := step(m2, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 != nil || m2.statusErr || !strings.Contains(m2.status, "unchanged") {
		t.Errorf("unchanged rename → cmd=%v status=%q", cmd2, m2.status)
	}

	// esc out of the rename input.
	m3 := readyModel(t)
	m3.cursor = 0
	m3, _ = step(m3, tea.KeyMsg{Type: tea.KeyEnter})
	m3, _ = step(m3, tea.KeyMsg{Type: tea.KeyEnter})
	m3, _ = step(m3, tea.KeyMsg{Type: tea.KeyEsc})
	if m3.mode != modeLedger || !strings.Contains(m3.status, "cancelled") {
		t.Errorf("rename esc → mode=%v status=%q", m3.mode, m3.status)
	}
}

// Typing into the rename input updates its value (the input.Update path).
func TestRenameInputAcceptsTyping(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // Rename, input = "alpha"
	m, _ = step(m, runeKey("X"))
	if !strings.HasSuffix(m.input.Value(), "X") {
		t.Errorf("typed key not appended: %q", m.input.Value())
	}
}

// --- delete flow ---

func TestMenuDeleteFlow(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0                                   // alpha (has 2 workspaces)
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // open menu
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown})  // move to Delete
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // select Delete
	if m.mode != modeConfirm {
		t.Fatalf("select Delete → mode=%v, want modeConfirm", m.mode)
	}
	if m.confirm.action != "deleteProject" || m.confirm.project != "alpha" {
		t.Errorf("confirm = %+v", m.confirm)
	}
	if m.confirm.wsCount != 2 {
		t.Errorf("wsCount = %d, want 2", m.confirm.wsCount)
	}

	// Confirming dispatches the delete command.
	m, cmd := step(m, runeKey("y"))
	if cmd == nil || m.mode != modeLedger {
		t.Errorf("delete confirm y → cmd=%v mode=%v", cmd, m.mode)
	}
	if !strings.Contains(m.status, "deleting project alpha") {
		t.Errorf("status = %q", m.status)
	}
}

func TestDeleteProjectPrompt(t *testing.T) {
	withWS := confirm{action: "deleteProject", project: "alpha", wsCount: 2}
	if out := withWS.message(); !strings.Contains(out, "2 workspaces") || !strings.Contains(out, "untouched") {
		t.Errorf("message with workspaces = %q", out)
	}
	if withWS.accent() != cRed { // dropping registrations is destructive → red
		t.Errorf("delete-with-workspaces accent = %v, want red", withWS.accent())
	}
	if !strings.Contains(withWS.title(), "alpha") {
		t.Errorf("title = %q, want it to name the project", withWS.title())
	}

	one := confirm{action: "deleteProject", project: "alpha", wsCount: 1}
	if out := one.message(); !strings.Contains(out, "1 workspace") {
		t.Errorf("message with one workspace = %q", out)
	}

	empty := confirm{action: "deleteProject", project: "beta", wsCount: 0}
	if out := empty.message(); !strings.Contains(out, "untouched") || strings.Contains(out, "drops") {
		t.Errorf("message with no workspaces = %q", out)
	}
	if empty.accent() != cGreen { // nothing dropped → safe → green
		t.Errorf("delete-empty accent = %v, want green", empty.accent())
	}
}

// --- view ---

func TestRenamePopupRendersCard(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // open menu
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // Rename → modeRename
	out := m.View()
	for _, want := range []string{"Rename project", "alpha", "name:", "enter rename", "WorkFlow"} {
		if !strings.Contains(out, want) {
			t.Errorf("rename popup missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "╭") { // a bordered card, not a bottom line
		t.Errorf("rename popup is not a bordered box:\n%s", out)
	}
}

func TestDeleteConfirmPopupRendersCard(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // open menu
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyDown})  // to Delete
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter}) // Delete → modeConfirm
	out := m.View()
	for _, want := range []string{"Delete project", "alpha", "Unregister", "y confirm", "╭"} {
		if !strings.Contains(out, want) {
			t.Errorf("delete confirm popup missing %q:\n%s", want, out)
		}
	}
}

func TestAddWorkspacePopupRendersCard(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, runeKey("a")) // add → modeInput
	out := m.View()
	for _, want := range []string{"New workspace", "alpha", "branch:", "enter create", "╭"} {
		if !strings.Contains(out, want) {
			t.Errorf("add popup missing %q:\n%s", want, out)
		}
	}
}

func TestViewMenuRendersOverLedger(t *testing.T) {
	m := readyModel(t)
	m.cursor = 0
	m, _ = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	out := m.View()
	for _, want := range []string{"Rename", "Delete", "alpha", "WorkFlow"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu overlay missing %q in:\n%s", want, out)
		}
	}
}

// --- menu unit logic ---

func TestMenuMoveAndCurrent(t *testing.T) {
	mn := newProjectMenu("alpha")
	if it, ok := mn.current(); !ok || it.id != "rename" {
		t.Errorf("current at 0 = %+v ok=%v", it, ok)
	}
	mn.move(-5)
	if mn.cursor != 0 {
		t.Errorf("move below 0 → %d", mn.cursor)
	}
	mn.move(99)
	if mn.cursor != len(mn.items)-1 {
		t.Errorf("move past end → %d", mn.cursor)
	}

	empty := menu{}
	if _, ok := empty.current(); ok {
		t.Error("empty menu should have no current item")
	}
}

// --- engine-backed commands ---

// modelWithProjects returns a dashboard model over a real manager whose registry
// holds the named project plus the given worktrees.
func modelWithProjects(t *testing.T, project string, wts ...registry.Worktree) Model {
	t.Helper()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		if err := s.AddProject(registry.Project{Name: project, Path: "/p/" + project}); err != nil {
			return err
		}
		for _, w := range wts {
			if err := s.AddWorktree(w); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return New(workspace.New(regPath, &config.Global{}), &config.Global{})
}

func TestRenameProjectCmdRenamesRegistry(t *testing.T) {
	m := modelWithProjects(t, "alpha")
	msg, ok := m.renameProjectCmd("alpha", "gamma")().(actionMsg)
	if !ok || msg.err != nil || !msg.refresh {
		t.Fatalf("rename cmd msg = %+v ok=%v", msg, ok)
	}
	if !strings.Contains(msg.msg, "renamed alpha → gamma") {
		t.Errorf("msg = %q", msg.msg)
	}
	if root, err := m.mgr.ProjectRoot("gamma"); err != nil || root != "/p/alpha" {
		t.Errorf("project not renamed in registry: root=%q err=%v", root, err)
	}
}

func TestDeleteProjectCmd(t *testing.T) {
	wt := registry.Worktree{Project: "alpha", Path: "/wt/a", Branch: "a", Base: "main"}
	m := modelWithProjects(t, "alpha", wt)

	// Without force, the engine refuses (the project still owns a worktree).
	if msg, ok := m.deleteProjectCmd("alpha", false)().(actionMsg); !ok || msg.err == nil {
		t.Errorf("unforced delete of populated project should error: %+v", msg)
	}

	// With force, the project is unregistered.
	msg, ok := m.deleteProjectCmd("alpha", true)().(actionMsg)
	if !ok || msg.err != nil || !msg.refresh {
		t.Fatalf("forced delete msg = %+v ok=%v", msg, ok)
	}
	if _, err := m.mgr.ProjectRoot("alpha"); err == nil {
		t.Error("project still registered after forced delete")
	}
}
