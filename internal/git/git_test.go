package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitCmd runs git in dir with a hermetic environment and fails on error.
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
	writeFile(t, filepath.Join(dir, "file.txt"), "one\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsRepo(t *testing.T) {
	repo := newRepo(t)
	if !IsRepo(repo) {
		t.Error("IsRepo(repo) = false, want true")
	}
	if IsRepo(t.TempDir()) {
		t.Error("IsRepo(non-repo) = true, want false")
	}
}

func TestRepoRootAndCurrentBranch(t *testing.T) {
	repo := newRepo(t)
	root, err := RepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, _ := filepath.EvalSymlinks(repo)
	gotRoot, _ := filepath.EvalSymlinks(root)
	if gotRoot != wantRoot {
		t.Errorf("RepoRoot = %q, want %q", gotRoot, wantRoot)
	}
	br, err := CurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	if br != "main" {
		t.Errorf("CurrentBranch = %q, want main", br)
	}
}

func TestBranchExistsAndDefaultBranch(t *testing.T) {
	repo := newRepo(t)
	if !BranchExists(repo, "main") {
		t.Error("BranchExists(main) = false")
	}
	if BranchExists(repo, "ghost") {
		t.Error("BranchExists(ghost) = true")
	}
	def, err := DefaultBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	if def != "main" {
		t.Errorf("DefaultBranch = %q, want main", def)
	}
}

func TestWorktreeAddAndRemove(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "feat")
	if err := WorktreeAdd(repo, wt, "feat", "main", true); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if !IsRepo(wt) {
		t.Fatal("worktree dir is not a repo")
	}
	if br, _ := CurrentBranch(wt); br != "feat" {
		t.Errorf("worktree branch = %q, want feat", br)
	}
	if !BranchExists(repo, "feat") {
		t.Error("feat branch not created")
	}
	if err := WorktreeRemove(repo, wt, true); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still present after remove")
	}
}

func TestStatsAndDiff(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "feat")
	if err := WorktreeAdd(repo, wt, "feat", "main", true); err != nil {
		t.Fatal(err)
	}

	// Clean, even-with-base worktree.
	st, err := Stats(wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "feat" || st.Dirty || st.Ahead != 0 || st.Behind != 0 {
		t.Errorf("fresh stats unexpected: %+v", st)
	}

	// Edit a tracked file (uncommitted) and add an untracked file.
	writeFile(t, filepath.Join(wt, "file.txt"), "two\n")
	writeFile(t, filepath.Join(wt, "new.txt"), "brand new\n")

	st, err = Stats(wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Dirty {
		t.Error("expected dirty after edit")
	}
	if st.Added < 1 || st.Deleted < 1 {
		t.Errorf("expected +/- line counts, got +%d -%d", st.Added, st.Deleted)
	}

	diff, err := Diff(wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+two") || !strings.Contains(diff, "-one") {
		t.Errorf("diff missing tracked change:\n%s", diff)
	}
	if !strings.Contains(diff, "Untracked files") || !strings.Contains(diff, "new.txt") {
		t.Errorf("diff missing untracked section:\n%s", diff)
	}
}

func TestMergeAndDeleteBranch(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "feat")
	if err := WorktreeAdd(repo, wt, "feat", "main", true); err != nil {
		t.Fatal(err)
	}
	// Commit a change on feat so there is something to merge.
	writeFile(t, filepath.Join(wt, "feature.txt"), "feature\n")
	gitCmd(t, wt, "add", ".")
	gitCmd(t, wt, "commit", "-m", "add feature")

	if err := Merge(repo, "main", "feat"); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Errorf("merge did not bring feature.txt into main: %v", err)
	}

	// Branch is checked out in the worktree; remove it before deleting.
	if err := WorktreeRemove(repo, wt, false); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBranch(repo, "feat", false); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if BranchExists(repo, "feat") {
		t.Error("feat branch still exists after delete")
	}
}

func TestRunReportsStderr(t *testing.T) {
	repo := newRepo(t)
	_, err := run(repo, "not-a-real-subcommand")
	if err == nil {
		t.Fatal("expected error from bogus git subcommand")
	}
	if !strings.Contains(err.Error(), "git not-a-real-subcommand") {
		t.Errorf("error missing command context: %v", err)
	}
}

func TestStatsErrorOnNonRepo(t *testing.T) {
	if _, err := Stats(t.TempDir(), "main"); err == nil {
		t.Error("expected error deriving stats outside a repo")
	}
}
