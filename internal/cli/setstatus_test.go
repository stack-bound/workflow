package cli

import (
	"encoding/json"
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

// The hook passes the agent's cwd on stdin as JSON; set-status must prefer it
// over the process working directory, so it resolves the right workspace even
// when invoked from elsewhere.
func TestSetStatusCwdFromStdin(t *testing.T) {
	cfg := isolateConfig(t)
	wt := addWorktreeForStatus(t, cfg)
	// The process cwd is a throwaway dir that resolves to nothing; only the stdin
	// cwd points at the worktree.
	t.Chdir(t.TempDir())

	payload, _ := json.Marshal(map[string]string{"cwd": wt.Path})
	if _, err := execWFIn(t, string(payload), "set-status", "ready"); err != nil {
		t.Fatalf("set-status ready: %v", err)
	}
	st, ok, err := status.ReadFor(wt.Project, wt.Branch, wt.Path)
	if err != nil || !ok {
		t.Fatalf("ReadFor ok=%v err=%v", ok, err)
	}
	if st.State != status.Ready {
		t.Errorf("state = %q, want ready (resolved from the stdin cwd)", st.State)
	}
}

// An agent at the project ROOT (a registered project, but not itself a worktree)
// writes a base status file keyed by the root path, so the dashboard base row
// lights up.
func TestSetStatusWritesBaseFile(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	_ = cfg
	t.Chdir(repo)

	if _, err := execWF(t, "set-status", "working"); err != nil {
		t.Fatalf("set-status working: %v", err)
	}
	st, ok, err := status.ReadBase("proj", repo)
	if err != nil || !ok {
		t.Fatalf("ReadBase ok=%v err=%v", ok, err)
	}
	if st.State != status.Working {
		t.Errorf("base state = %q, want working", st.State)
	}
	// No per-worktree file was written for the base (it has no branch).
	if st.Branch != "" {
		t.Errorf("base status branch = %q, want empty", st.Branch)
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
