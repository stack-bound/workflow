package registry

import (
	"path/filepath"
	"testing"
)

func TestRemoveWorktree(t *testing.T) {
	s := &Store{}
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/x", Branch: "x"})
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/y", Branch: "y"})

	if !s.RemoveWorktree("/wt/x") {
		t.Error("RemoveWorktree existing = false, want true")
	}
	if s.RemoveWorktree("/wt/missing") {
		t.Error("RemoveWorktree missing = true, want false")
	}
	if len(s.Worktrees) != 1 || s.Worktrees[0].Path != "/wt/y" {
		t.Errorf("remaining worktrees = %+v", s.Worktrees)
	}
}

func TestAddWorktreeDuplicatePath(t *testing.T) {
	s := &Store{}
	if err := s.AddWorktree(Worktree{Project: "a", Path: "/wt/x", Branch: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWorktree(Worktree{Project: "a", Path: "/wt/x", Branch: "other"}); err == nil {
		t.Error("expected duplicate-path error")
	}
}

func TestFindAndProjectByPath(t *testing.T) {
	s := &Store{}
	_ = s.AddProject(Project{Name: "demo", Path: "/p/demo"})

	if s.FindProject("demo") == nil {
		t.Error("FindProject(demo) = nil")
	}
	if s.FindProject("nope") != nil {
		t.Error("FindProject(nope) != nil")
	}
	if p := s.ProjectByPath("/p/demo"); p == nil || p.Name != "demo" {
		t.Errorf("ProjectByPath = %+v", p)
	}
	if s.ProjectByPath("/p/other") != nil {
		t.Error("ProjectByPath(other) != nil")
	}
}

func TestWorktreeByPath(t *testing.T) {
	s := &Store{}
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/feat", Branch: "feat"})
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/feat-sub", Branch: "sub"})

	if w := s.WorktreeByPath("/wt/feat"); w == nil || w.Branch != "feat" {
		t.Errorf("exact match = %+v", w)
	}
	// A path inside the worktree resolves to it.
	if w := s.WorktreeByPath("/wt/feat/internal/cli"); w == nil || w.Branch != "feat" {
		t.Errorf("ancestor match = %+v", w)
	}
	// "/wt/feat-sub" must not be swallowed by the "/wt/feat" prefix.
	if w := s.WorktreeByPath("/wt/feat-sub/x"); w == nil || w.Branch != "sub" {
		t.Errorf("prefix should respect path boundary, got %+v", w)
	}
	if w := s.WorktreeByPath("/somewhere/else"); w != nil {
		t.Errorf("unrelated path = %+v, want nil", w)
	}
}

func TestWorktreesForProject(t *testing.T) {
	s := &Store{}
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/a1", Branch: "a1"})
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/a2", Branch: "a2"})
	_ = s.AddWorktree(Worktree{Project: "b", Path: "/wt/b1", Branch: "b1"})

	if got := s.WorktreesForProject("a"); len(got) != 2 {
		t.Errorf("project a worktrees = %d, want 2", len(got))
	}
	if got := s.WorktreesForProject("none"); len(got) != 0 {
		t.Errorf("project none worktrees = %d, want 0", len(got))
	}
}

func TestRemoveProjectUnknown(t *testing.T) {
	s := &Store{}
	if err := s.RemoveProject("ghost", false); err == nil {
		t.Error("expected error removing unknown project")
	}
}

func TestRenameProjectRetargetsWorktrees(t *testing.T) {
	s := &Store{}
	_ = s.AddProject(Project{Name: "old", Path: "/p/old"})
	_ = s.AddWorktree(Worktree{Project: "old", Path: "/wt/a", Branch: "a"})
	_ = s.AddWorktree(Worktree{Project: "old", Path: "/wt/b", Branch: "b"})
	_ = s.AddProject(Project{Name: "other", Path: "/p/other"})
	_ = s.AddWorktree(Worktree{Project: "other", Path: "/wt/c", Branch: "c"})

	if err := s.RenameProject("old", "new"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	if s.FindProject("old") != nil {
		t.Error("old name still present after rename")
	}
	if p := s.FindProject("new"); p == nil || p.Path != "/p/old" {
		t.Errorf("renamed project = %+v, want path /p/old", p)
	}
	if got := s.WorktreesForProject("new"); len(got) != 2 {
		t.Errorf("renamed project worktrees = %d, want 2", len(got))
	}
	// An unrelated project's worktrees are left alone.
	if got := s.WorktreesForProject("other"); len(got) != 1 {
		t.Errorf("other project worktrees = %d, want 1", len(got))
	}
}

func TestRenameProjectErrors(t *testing.T) {
	s := &Store{}
	_ = s.AddProject(Project{Name: "a", Path: "/p/a"})
	_ = s.AddProject(Project{Name: "b", Path: "/p/b"})

	if err := s.RenameProject("ghost", "x"); err == nil {
		t.Error("expected error renaming unknown project")
	}
	if err := s.RenameProject("a", "b"); err == nil {
		t.Error("expected error renaming to a taken name")
	}
	if err := s.RenameProject("a", ""); err == nil {
		t.Error("expected error renaming to an empty name")
	}
	// Renaming to the same name is a no-op success.
	if err := s.RenameProject("a", "a"); err != nil {
		t.Errorf("rename to same name should be a no-op, got %v", err)
	}
	if s.FindProject("a") == nil {
		t.Error("project a vanished after a no-op rename")
	}
}

func TestSaveLoadVersionUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "registry.json")
	// Save into a not-yet-existing nested dir to exercise MkdirAll.
	if err := Save(path, &Store{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version {
		t.Errorf("version = %d, want %d", got.Version, Version)
	}
}
