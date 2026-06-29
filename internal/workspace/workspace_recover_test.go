package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/git"
)

// TestRemoveHealsWhenDirAlreadyGone covers a removal that previously deleted the
// worktree files but failed before unregistering: a retry must converge to a
// clean state rather than erroring on the missing directory.
func TestRemoveHealsWhenDirAlreadyGone(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Simulate the files already gone (e.g. removed out-of-band) while the
	// branch, git metadata, and registry entry all remain.
	if err := os.RemoveAll(wt.Path); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Remove("feat", proj, false); err != nil {
		t.Fatalf("Remove after dir vanished: %v", err)
	}
	if git.BranchExists(repo, "feat") {
		t.Error("branch still exists after Remove")
	}
	views, _ := m.List()
	if len(views) != 0 {
		t.Errorf("registry not cleared after Remove: %+v", views)
	}
}

// TestRemoveHealsOrphanedDir covers a worktree whose .git pointer is gone — the
// shape a half-finished `git worktree remove` leaves — so git no longer treats
// it as a working tree. Remove must fall back to deleting the directory and
// finish the cleanup.
func TestRemoveHealsOrphanedDir(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Corrupt the worktree: drop the .git pointer so git can't remove it cleanly.
	if err := os.Remove(filepath.Join(wt.Path, ".git")); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Remove("feat", proj, true); err != nil {
		t.Fatalf("Remove orphaned worktree: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Error("orphaned worktree dir not deleted")
	}
	if git.BranchExists(repo, "feat") {
		t.Error("branch still exists after Remove")
	}
	views, _ := m.List()
	if len(views) != 0 {
		t.Errorf("registry not cleared after Remove: %+v", views)
	}
}

// TestRemovePermissionDeniedKeepsRegistration covers the root-owned-files case:
// when the directory genuinely can't be deleted, Remove must surface an
// actionable error pointing at `wf forget` and must NOT silently drop the
// registration.
func TestRemovePermissionDeniedKeepsRegistration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Make a child whose contents can't be unlinked (no write on its parent),
	// mirroring root-owned files left by a Docker bind mount.
	locked := filepath.Join(wt.Path, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) // let t.TempDir cleanup succeed
	// Orphan it so removal takes the direct os.RemoveAll path that will fail.
	if err := os.Remove(filepath.Join(wt.Path, ".git")); err != nil {
		t.Fatal(err)
	}

	_, err = m.Remove("feat", proj, true)
	if err == nil {
		t.Fatal("expected Remove to fail on an undeletable worktree")
	}
	for _, want := range []string{"sudo rm -rf", "wf forget"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q guidance: %v", want, err)
		}
	}
	// The registration must survive so the workspace isn't silently lost.
	views, _ := m.List()
	if len(views) != 1 {
		t.Errorf("registration dropped despite failed removal: %+v", views)
	}
}

// TestForgetKeepsFilesAndBranch covers the escape hatch: forgetting a workspace
// clears wf's registry and status but leaves the directory and branch in place.
func TestForgetKeepsFilesAndBranch(t *testing.T) {
	repo := newRepo(t)
	m, proj := register(t, repo)
	wt, err := m.Add(AddOptions{Branch: "feat", Project: proj, NoSetup: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := m.Forget("feat", proj)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if got.Path != wt.Path {
		t.Errorf("Forget returned %q, want %q", got.Path, wt.Path)
	}
	if _, err := os.Stat(wt.Path); err != nil {
		t.Errorf("Forget deleted the worktree dir: %v", err)
	}
	if !git.BranchExists(repo, "feat") {
		t.Error("Forget deleted the branch; it should be left intact")
	}
	views, _ := m.List()
	if len(views) != 0 {
		t.Errorf("registry not cleared after Forget: %+v", views)
	}
}
