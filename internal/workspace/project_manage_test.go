package workspace

import (
	"path/filepath"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// seedRegistry writes a project (with optional worktrees) to a fresh registry
// and returns a Manager over it.
func seedManager(t *testing.T, project string, worktrees ...registry.Worktree) (*Manager, string) {
	t.Helper()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		if err := s.AddProject(registry.Project{Name: project, Path: "/p/" + project}); err != nil {
			return err
		}
		for _, w := range worktrees {
			if err := s.AddWorktree(w); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return New(regPath, &config.Global{}), regPath
}

func TestManagerRenameProject(t *testing.T) {
	m, regPath := seedManager(t, "old",
		registry.Worktree{Project: "old", Path: "/wt/a", Branch: "a", Base: "main"},
	)

	if err := m.RenameProject("old", "fresh"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	s, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if s.FindProject("fresh") == nil {
		t.Error("project not renamed in the registry")
	}
	if got := s.WorktreesForProject("fresh"); len(got) != 1 {
		t.Errorf("worktrees not retargeted: %d under new name, want 1", len(got))
	}

	// Renaming onto an unknown source surfaces the registry error.
	if err := m.RenameProject("ghost", "x"); err == nil {
		t.Error("expected error renaming an unknown project")
	}
}

func TestManagerRemoveProject(t *testing.T) {
	m, regPath := seedManager(t, "proj",
		registry.Worktree{Project: "proj", Path: "/wt/a", Branch: "a", Base: "main"},
	)

	// Without force the engine refuses while the project still owns worktrees.
	if err := m.RemoveProject("proj", false); err == nil {
		t.Error("expected refusal removing a project with worktrees and no force")
	}

	// With force the project and its worktree registrations are dropped.
	if err := m.RemoveProject("proj", true); err != nil {
		t.Fatalf("RemoveProject(force): %v", err)
	}
	s, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if s.FindProject("proj") != nil {
		t.Error("project still registered after force remove")
	}
	if len(s.Worktrees) != 0 {
		t.Errorf("worktrees not dropped: %+v", s.Worktrees)
	}
}
