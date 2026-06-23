package dashboard

import (
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

func wsView(project, branch string, dirty bool) workspace.View {
	v := workspace.View{Worktree: registry.Worktree{Project: project, Branch: branch, Base: "main"}}
	v.Stat.Dirty = dirty
	return v
}

func sampleLedger() []workspace.ProjectView {
	return []workspace.ProjectView{
		{
			Project:    registry.Project{Name: "alpha", Path: "/a"},
			Workspaces: []workspace.View{wsView("alpha", "feat-1", true), wsView("alpha", "feat-2", false)},
		},
		{
			Project:    registry.Project{Name: "beta", Path: "/b"},
			Workspaces: nil, // project with no workspaces still gets a header row
		},
	}
}

func TestSetRowsFlattensTree(t *testing.T) {
	m := New(nil, nil)
	m.setRows(sampleLedger())

	// Each project block is header → base (main) → its worktrees.
	wantKinds := []rowKind{rowProject, rowMain, rowWorkspace, rowWorkspace, rowProject, rowMain}
	if len(m.rows) != len(wantKinds) {
		t.Fatalf("got %d rows, want %d", len(m.rows), len(wantKinds))
	}
	for i, k := range wantKinds {
		if m.rows[i].kind != k {
			t.Errorf("row %d kind = %v, want %v", i, m.rows[i].kind, k)
		}
	}
	if m.rows[0].wsCount != 2 {
		t.Errorf("alpha header wsCount = %d, want 2", m.rows[0].wsCount)
	}
	if m.rows[4].project != "beta" {
		t.Errorf("second project header = %q, want beta", m.rows[4].project)
	}
}

func TestSetRowsPreservesSelection(t *testing.T) {
	m := New(nil, nil)
	m.setRows(sampleLedger())
	m.cursor = 3 // alpha/feat-2 (after header@0, main@1, feat-1@2)
	if _, ok := m.currentWorkspace(); !ok {
		t.Fatal("expected a workspace under the cursor")
	}

	// A refresh that reorders projects should keep the cursor on feat-2.
	reordered := []workspace.ProjectView{
		sampleLedger()[1], // beta first now
		sampleLedger()[0],
	}
	m.setRows(reordered)

	r, ok := m.currentWorkspace()
	if !ok {
		t.Fatal("cursor no longer on a workspace after refresh")
	}
	if r.view.Worktree.Branch != "feat-2" {
		t.Errorf("selection moved to %q, want feat-2", r.view.Worktree.Branch)
	}
}

func TestCursorClamp(t *testing.T) {
	m := New(nil, nil)
	m.setRows(sampleLedger())

	m.cursor = 0
	m.moveCursor(-5)
	if m.cursor != 0 {
		t.Errorf("moveCursor below 0 = %d, want 0", m.cursor)
	}
	m.moveCursor(100)
	if m.cursor != len(m.rows)-1 {
		t.Errorf("moveCursor past end = %d, want %d", m.cursor, len(m.rows)-1)
	}
}

func TestCurrentProject(t *testing.T) {
	m := New(nil, nil)
	m.setRows(sampleLedger())
	m.cursor = 2 // alpha/feat-1
	if got := m.currentProject(); got != "alpha" {
		t.Errorf("currentProject on workspace = %q, want alpha", got)
	}
	m.cursor = 4 // beta header (empty project)
	if got := m.currentProject(); got != "beta" {
		t.Errorf("currentProject on header = %q, want beta", got)
	}
	if _, ok := m.currentWorkspace(); ok {
		t.Error("project header should not be a workspace")
	}
}

func TestColorizeDiffPreservesText(t *testing.T) {
	in := strings.Join([]string{
		"diff --git a/x b/x",
		"@@ -1 +1 @@",
		"-one",
		"+two",
		" ctx",
	}, "\n")
	out := colorizeDiff(in)

	inLines := strings.Split(in, "\n")
	outLines := strings.Split(out, "\n")
	if len(inLines) != len(outLines) {
		t.Fatalf("line count changed: %d -> %d", len(inLines), len(outLines))
	}
	// Styling may add ANSI, but the original text must remain visible.
	for i := range inLines {
		if !strings.Contains(outLines[i], inLines[i]) {
			t.Errorf("line %d %q not preserved in %q", i, inLines[i], outLines[i])
		}
	}
}
