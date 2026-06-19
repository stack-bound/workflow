package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// setup registers a project plus one worktree and returns the manager, the
// project root, and the worktree path.
func setupIDE(t *testing.T) (m *Manager, root, wtPath string) {
	t.Helper()
	root = t.TempDir()
	wtPath = filepath.Join(t.TempDir(), "wt-feat")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	regPath := filepath.Join(t.TempDir(), "reg.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		if err := s.AddProject(registry.Project{Name: "proj", Path: root}); err != nil {
			return err
		}
		return s.AddWorktree(registry.Worktree{Project: "proj", Path: wtPath, Branch: "feat", Base: "main"})
	}); err != nil {
		t.Fatal(err)
	}
	return New(regPath, &config.Global{}), root, wtPath
}

func TestProjectRootAndPrefsRoundTrip(t *testing.T) {
	m, root, _ := setupIDE(t)

	if got, err := m.ProjectRoot("proj"); err != nil || got != root {
		t.Fatalf("ProjectRoot = %q err=%v, want %q", got, err, root)
	}
	if _, err := m.ProjectRoot("nope"); err == nil {
		t.Error("ProjectRoot on unknown project should error")
	}

	def, auto, err := m.ProjectIDEPrefs("proj")
	if err != nil || def != "" || auto {
		t.Fatalf("initial prefs = %q %v (err %v), want empty/false", def, auto, err)
	}

	if err := m.SetProjectDefaultIDE("proj", "goland", true); err != nil {
		t.Fatal(err)
	}
	def, auto, err = m.ProjectIDEPrefs("proj")
	if err != nil || def != "goland" || !auto {
		t.Fatalf("after set: %q %v (err %v), want goland/true", def, auto, err)
	}
	// The pref lives in the project root's .workFlow.yaml.
	if _, err := os.Stat(config.RepoConfigPath(root)); err != nil {
		t.Errorf("expected .workFlow.yaml at project root: %v", err)
	}
}

func TestProjectForDirFindsWorktree(t *testing.T) {
	m, _, wtPath := setupIDE(t)

	if p, err := m.ProjectForDir(wtPath); err != nil || p != "proj" {
		t.Errorf("ProjectForDir(worktree) = %q err=%v, want proj", p, err)
	}
	if p, err := m.ProjectForDir(filepath.Join(wtPath, "internal", "cli")); err != nil || p != "proj" {
		t.Errorf("ProjectForDir(nested) = %q err=%v, want proj", p, err)
	}
}

func TestProjectForDirMainRepoFallback(t *testing.T) {
	m, root, _ := setupIDE(t)
	// Standing in the project root (not a worktree) still resolves the project.
	t.Chdir(root)
	if p, err := m.ProjectForDir(root); err != nil || p != "proj" {
		t.Errorf("ProjectForDir(project root) = %q err=%v, want proj", p, err)
	}
}
