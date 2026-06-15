package cli

import (
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
)

func addWorktreeForStatus(t *testing.T, cfg string) registry.Worktree {
	t.Helper()
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatalf("add: %v", err)
	}
	store, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	wts := store.FindWorktrees("feat", "proj")
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	return wts[0]
}

func TestSetStatusWritesFile(t *testing.T) {
	cfg := isolateConfig(t)
	wt := addWorktreeForStatus(t, cfg)
	t.Chdir(wt.Path)

	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	st, ok, err := status.ReadFor(wt.Project, wt.Branch, wt.Path)
	if err != nil || !ok {
		t.Fatalf("ReadFor ok=%v err=%v", ok, err)
	}
	if st.State != status.Working {
		t.Errorf("state = %q, want working", st.State)
	}

	// An unknown state normalizes to idle and still exits 0.
	if _, err := execWF(t, "set-status", "bogus"); err != nil {
		t.Fatalf("set-status bogus should not error: %v", err)
	}
	if st, _, _ := status.ReadFor(wt.Project, wt.Branch, wt.Path); st.State != status.Idle {
		t.Errorf("state after bogus = %q, want idle", st.State)
	}
}

func TestSetStatusNoopOutsideWorktree(t *testing.T) {
	cfg := isolateConfig(t)
	wt := addWorktreeForStatus(t, cfg)
	// cwd is a throwaway dir not inside any registered worktree.
	t.Chdir(t.TempDir())
	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status outside a worktree should exit 0: %v", err)
	}
	// Nothing was recorded for the registered worktree.
	if _, ok, _ := status.ReadFor(wt.Project, wt.Branch, wt.Path); ok {
		t.Errorf("status written despite cwd being outside any worktree")
	}
}
