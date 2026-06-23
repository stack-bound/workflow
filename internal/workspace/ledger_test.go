package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// gitCmd runs git in dir and fails the test on error.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// newRepo creates a git repo on `main` with one committed file.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "t@t")
	gitCmd(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")
	return dir
}

func TestLedgerAndDiff(t *testing.T) {
	repo := newRepo(t)

	regPath := filepath.Join(t.TempDir(), "registry.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		return s.AddProject(registry.Project{Name: "proj", Path: repo, AddedAt: time.Now().UTC().Format(time.RFC3339)})
	}); err != nil {
		t.Fatal(err)
	}

	m := New(regPath, &config.Global{})
	if _, err := m.Add(AddOptions{Branch: "feat", Project: "proj", NoSetup: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Fresh, clean workspace: present, base main, done (inactive).
	led, err := m.Ledger()
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}
	if len(led) != 1 || led[0].Project.Name != "proj" {
		t.Fatalf("expected 1 project 'proj', got %+v", led)
	}
	if len(led[0].Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(led[0].Workspaces))
	}
	ws := led[0].Workspaces[0]
	if ws.Worktree.Base != "main" {
		t.Errorf("base = %q, want main", ws.Worktree.Base)
	}
	if ws.StatErr != nil {
		t.Fatalf("stat error: %v", ws.StatErr)
	}
	if ws.Active() {
		t.Errorf("brand-new clean workspace should be inactive (done)")
	}

	// Edit a tracked file → dirty/active, and the diff reflects the change.
	wtPath := ws.Worktree.Path
	if err := os.WriteFile(filepath.Join(wtPath, "file.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	led, _ = m.Ledger()
	ws = led[0].Workspaces[0]
	if !ws.Stat.Dirty || !ws.Active() {
		t.Errorf("after edit: Dirty=%v Active=%v, want both true", ws.Stat.Dirty, ws.Active())
	}

	diff, err := m.Diff("feat", "proj")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(diff, "+two") || !strings.Contains(diff, "-one") {
		t.Errorf("diff missing expected change:\n%s", diff)
	}
}

// TestLedgerMainCheckout covers the project's base-checkout row: the branch the
// root is on, its clean→dirty transition, and MainDiff reflecting uncommitted
// work in the root.
func TestLedgerMainCheckout(t *testing.T) {
	repo := newRepo(t)

	regPath := filepath.Join(t.TempDir(), "registry.json")
	if err := registry.WithLock(regPath, func(s *registry.Store) error {
		return s.AddProject(registry.Project{Name: "proj", Path: repo, AddedAt: time.Now().UTC().Format(time.RFC3339)})
	}); err != nil {
		t.Fatal(err)
	}
	m := New(regPath, &config.Global{})

	led, err := m.Ledger()
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}
	main := led[0].Main
	if main.Err != nil {
		t.Fatalf("main checkout error: %v", main.Err)
	}
	if main.Branch != "main" {
		t.Errorf("main branch = %q, want main", main.Branch)
	}
	if main.Path != repo {
		t.Errorf("main path = %q, want %q", main.Path, repo)
	}
	if main.Dirty {
		t.Errorf("freshly committed root should be clean")
	}

	// Dirty the root checkout → Main.Dirty flips and MainDiff shows the change.
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	led, _ = m.Ledger()
	if !led[0].Main.Dirty {
		t.Errorf("root with uncommitted edit should be dirty")
	}
	diff, err := m.MainDiff("proj")
	if err != nil {
		t.Fatalf("MainDiff: %v", err)
	}
	if !strings.Contains(diff, "+changed") || !strings.Contains(diff, "-one") {
		t.Errorf("MainDiff missing expected change:\n%s", diff)
	}
}

// TestMainCheckoutMissingPath asserts a project whose root is gone degrades to a
// populated Err rather than failing the whole ledger.
func TestMainCheckoutMissingPath(t *testing.T) {
	mc := mainCheckoutFor(filepath.Join(t.TempDir(), "does-not-exist"))
	if mc.Err == nil {
		t.Fatal("missing path should populate Err")
	}
	if mc.Branch != "" {
		t.Errorf("missing path should have no branch, got %q", mc.Branch)
	}
}

// TestMainCheckoutNotARepo asserts a project root that exists but is not a git
// repository degrades to a concise, single-line error (not a raw git stderr
// dump) so the dashboard's base row stays clean.
func TestMainCheckoutNotARepo(t *testing.T) {
	mc := mainCheckoutFor(t.TempDir()) // an empty dir, no .git
	if mc.Err == nil {
		t.Fatal("a non-repo root should populate Err")
	}
	if got := mc.Err.Error(); got != "not a git repository" {
		t.Errorf("non-repo error = %q, want %q", got, "not a git repository")
	}
	if mc.Branch != "" {
		t.Errorf("non-repo root should have no branch, got %q", mc.Branch)
	}
}
