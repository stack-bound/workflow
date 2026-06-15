package registry

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")

	s := &Store{}
	if err := s.AddProject(Project{Name: "demo", Path: "/repo/demo"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddWorktree(Worktree{Project: "demo", Path: "/wt/feat", Branch: "feat", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version {
		t.Errorf("version = %d, want %d", got.Version, Version)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "demo" {
		t.Errorf("projects = %+v", got.Projects)
	}
	if len(got.Worktrees) != 1 || got.Worktrees[0].Branch != "feat" {
		t.Errorf("worktrees = %+v", got.Worktrees)
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 0 || len(got.Worktrees) != 0 {
		t.Errorf("expected empty store, got %+v", got)
	}
}

func TestAddProjectDuplicates(t *testing.T) {
	s := &Store{}
	if err := s.AddProject(Project{Name: "a", Path: "/p/a"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProject(Project{Name: "a", Path: "/p/other"}); err == nil {
		t.Error("expected duplicate-name error")
	}
	if err := s.AddProject(Project{Name: "b", Path: "/p/a"}); err == nil {
		t.Error("expected duplicate-path error")
	}
}

func TestRemoveProjectGuardsWorktrees(t *testing.T) {
	s := &Store{}
	_ = s.AddProject(Project{Name: "a", Path: "/p/a"})
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/x", Branch: "x"})

	if err := s.RemoveProject("a", false); err == nil {
		t.Error("expected error removing project with worktrees")
	}
	if err := s.RemoveProject("a", true); err != nil {
		t.Fatalf("force remove failed: %v", err)
	}
	if len(s.Projects) != 0 || len(s.Worktrees) != 0 {
		t.Errorf("force remove should drop worktrees too: %+v", s)
	}
}

func TestFindWorktreesScoping(t *testing.T) {
	s := &Store{}
	_ = s.AddWorktree(Worktree{Project: "a", Path: "/wt/a-feat", Branch: "feat"})
	_ = s.AddWorktree(Worktree{Project: "b", Path: "/wt/b-feat", Branch: "feat"})

	if got := s.FindWorktrees("feat", ""); len(got) != 2 {
		t.Errorf("unscoped = %d, want 2", len(got))
	}
	if got := s.FindWorktrees("feat", "a"); len(got) != 1 || got[0].Project != "a" {
		t.Errorf("scoped = %+v", got)
	}
	if got := s.FindWorktrees("missing", ""); len(got) != 0 {
		t.Errorf("missing = %+v", got)
	}
}

func TestWithLockPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	err := WithLock(path, func(s *Store) error {
		return s.AddProject(Project{Name: "x", Path: "/p/x"})
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Load(path)
	if got.FindProject("x") == nil {
		t.Error("WithLock did not persist the project")
	}
}
