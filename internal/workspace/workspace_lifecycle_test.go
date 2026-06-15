package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/registry"
)

// register creates a registry with a single project pointing at repo and
// returns a Manager plus the project name.
func register(t *testing.T, repo string) (*Manager, string) {
	t.Helper()
	regPath := filepath.Join(t.TempDir(), "registry.json")
	root, err := git.RepoRoot(repo)
	if err != nil {
		root = repo
	}
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		return s.AddProject(registry.Project{Name: "proj", Path: root, AddedAt: time.Now().UTC().Format(time.RFC3339)})
	}); err != nil {
		t.Fatal(err)
	}
	return New(regPath, &config.Global{}), "proj"
}

func TestAddRunsSetupCopyAndSymlink(t *testing.T) {
	repo := newRepo(t)
	// Seed files to copy/symlink and a repo config that drives all three ops.
	if err := os.WriteFile(filepath.Join(repo, "env.example"), []byte("EXAMPLE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "secret"), []byte("token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := "setup:\n  - touch SETUP_RAN\ncopy:\n  - env.example\nsymlink:\n  - secret\n"
	if err := os.WriteFile(config.RepoConfigPath(repo), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if b, err := os.ReadFile(filepath.Join(wt.Path, "env.example")); err != nil || string(b) != "EXAMPLE=1\n" {
		t.Errorf("copied file wrong: %q err=%v", b, err)
	}
	if fi, err := os.Lstat(filepath.Join(wt.Path, "secret")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink not created: mode=%v err=%v", fi.Mode(), err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "SETUP_RAN")); err != nil {
		t.Errorf("setup command did not run: %v", err)
	}
}

func TestAddSkipsSetupWhenNoSetup(t *testing.T) {
	repo := newRepo(t)
	cfg := "setup:\n  - touch SETUP_RAN\n"
	if err := os.WriteFile(config.RepoConfigPath(repo), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "SETUP_RAN")); !os.IsNotExist(err) {
		t.Error("setup ran despite NoSetup")
	}
}

func TestAddErrors(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)

	if _, err := m.Add(AddOptions{Branch: "", Project: proj}); err == nil {
		t.Error("expected error for empty branch")
	}
	if _, err := m.Add(AddOptions{Branch: "feat", Project: "ghost"}); err == nil {
		t.Error("expected error for unknown project")
	}
	if _, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if _, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true}); err == nil {
		t.Error("expected error re-adding existing worktree path")
	}
}

func TestPathAndListAndResolve(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatal(err)
	}

	got, err := m.Path("feat", proj)
	if err != nil || got != wt.Path {
		t.Errorf("Path = %q err=%v, want %q", got, err, wt.Path)
	}
	views, err := m.List()
	if err != nil || len(views) != 1 || views[0].Worktree.Branch != "feat" {
		t.Errorf("List = %+v err=%v", views, err)
	}
	r, err := m.Resolve("feat", proj)
	if err != nil || r.Branch != "feat" {
		t.Errorf("Resolve = %+v err=%v", r, err)
	}
	if _, err := m.Path("ghost", proj); err == nil {
		t.Error("expected error resolving unknown branch")
	}
	if _, err := m.Path("", proj); err == nil {
		t.Error("expected error for empty ref")
	}
}

func TestRemoveLifecycle(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.Remove("feat", proj, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("worktree dir still present after Remove")
	}
	if git.BranchExists(repo, "feat") {
		t.Error("branch still exists after Remove")
	}
	views, _ := m.List()
	if len(views) != 0 {
		t.Errorf("registry still has worktrees: %+v", views)
	}
	if _, err := m.Remove("feat", proj, false); err == nil {
		t.Error("expected error removing an already-removed workspace")
	}
}

func TestMergeLifecycle(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	// Commit a change on the branch so there is something to merge.
	if err := os.WriteFile(filepath.Join(wt.Path, "feature.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, wt.Path, "add", ".")
	gitCmd(t, wt.Path, "commit", "-m", "feature")

	if _, err := m.Merge("feat", proj); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Errorf("merge did not land on base: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("worktree not cleaned up after merge")
	}
	if git.BranchExists(repo, "feat") {
		t.Error("branch not deleted after merge")
	}
}

func TestMergeRefusesDirty(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	// Uncommitted edit makes the workspace dirty.
	if err := os.WriteFile(filepath.Join(wt.Path, "file.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = m.Merge("feat", proj)
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("Merge dirty = %v, want uncommitted-changes error", err)
	}
}

func TestResolveBasePrecedence(t *testing.T) {
	repo := newRepo(t)
	m := New("", &config.Global{DefaultBase: "globbase"})

	if got := m.resolveBase("flagbase", &config.Repo{Base: "repobase"}, repo); got != "flagbase" {
		t.Errorf("flag should win: %q", got)
	}
	if got := m.resolveBase("", &config.Repo{Base: "repobase"}, repo); got != "repobase" {
		t.Errorf("repo cfg should win over global: %q", got)
	}
	if got := m.resolveBase("", &config.Repo{}, repo); got != "globbase" {
		t.Errorf("global should win over detection: %q", got)
	}

	m2 := New("", &config.Global{})
	if got := m2.resolveBase("", &config.Repo{}, repo); got != "main" {
		t.Errorf("detected default = %q, want main", got)
	}
}

func TestWorktreePath(t *testing.T) {
	m := New("", &config.Global{})
	proj := &registry.Project{Name: "repo", Path: "/parent/repo"}

	got, err := m.worktreePath(proj, &config.Repo{}, "feat/x")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/parent", "repo_worktrees", "feat-x"); got != want {
		t.Errorf("sibling default = %q, want %q", got, want)
	}

	got, _ = m.worktreePath(proj, &config.Repo{WorktreeDir: "wt"}, "feat")
	if want := filepath.Join("/parent/repo", "wt", "feat"); got != want {
		t.Errorf("relative repo dir = %q, want %q", got, want)
	}

	got, _ = m.worktreePath(proj, &config.Repo{WorktreeDir: "/abs/wt"}, "feat")
	if want := filepath.Join("/abs/wt", "feat"); got != want {
		t.Errorf("absolute repo dir = %q, want %q", got, want)
	}

	mg := New("", &config.Global{WorktreeDir: "/global/wt"})
	got, _ = mg.worktreePath(proj, &config.Repo{}, "feat")
	if want := filepath.Join("/global/wt", "feat"); got != want {
		t.Errorf("global dir = %q, want %q", got, want)
	}

	if _, err := m.worktreePath(proj, &config.Repo{}, "///"); err == nil {
		t.Error("expected error for branch that slugs to empty")
	}
}

func TestResolveProjectByCwd(t *testing.T) {
	repo := newRepo(t)
	m, _ := register(t, repo)
	store, err := registry.Load(m.registryPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(repo)
	p, err := m.resolveProject(store, "")
	if err != nil {
		t.Fatalf("resolveProject by cwd: %v", err)
	}
	if p.Name != "proj" {
		t.Errorf("inferred project = %q, want proj", p.Name)
	}

	if _, err := m.resolveProject(store, "ghost"); err == nil {
		t.Error("expected error for unknown project flag")
	}
}

func TestResolveProjectUnknownCwd(t *testing.T) {
	m, _ := register(t, newRepo(t))
	store, _ := registry.Load(m.registryPath)

	t.Chdir(t.TempDir()) // not inside any registered project
	if _, err := m.resolveProject(store, ""); err == nil {
		t.Error("expected error when cwd is not in a registered project")
	}
}

func TestResolveWorktreeAmbiguous(t *testing.T) {
	store := &registry.Store{}
	_ = store.AddProject(registry.Project{Name: "a", Path: "/p/a"})
	_ = store.AddProject(registry.Project{Name: "b", Path: "/p/b"})
	_ = store.AddWorktree(registry.Worktree{Project: "a", Path: "/wt/a", Branch: "feat"})
	_ = store.AddWorktree(registry.Worktree{Project: "b", Path: "/wt/b", Branch: "feat"})

	m := New("", &config.Global{})
	if _, err := m.resolveWorktree(store, "feat", ""); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got %v", err)
	}
	if w, err := m.resolveWorktree(store, "feat", "a"); err != nil || w.Project != "a" {
		t.Errorf("scoped resolve = %+v err=%v", w, err)
	}
}
