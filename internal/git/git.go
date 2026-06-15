// Package git shells out to the git CLI for the small set of operations
// WorkFlow needs: worktree management, branch detection, and the live stats
// the dashboard derives on every refresh.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Stat is the live git status derived for a worktree.
type Stat struct {
	Branch  string // current branch name
	Dirty   bool   // uncommitted changes (including untracked)
	Added   int    // inserted lines vs base
	Deleted int    // deleted lines vs base
	Ahead   int    // commits on this branch not on base
	Behind  int    // commits on base not on this branch
}

// run executes git in dir and returns trimmed stdout.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// runIO runs git inheriting stdio so the user sees progress (e.g. merges).
// Callers target a repo via an explicit "-C <repo>" rather than a working dir.
func runIO(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// IsRepo reports whether dir is inside a git working tree.
func IsRepo(dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// RepoRoot returns the top-level directory of the working tree containing dir.
func RepoRoot(dir string) (string, error) {
	return run(dir, "rev-parse", "--show-toplevel")
}

// BranchExists reports whether a local branch exists in repo.
func BranchExists(repo, branch string) bool {
	_, err := run(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// DefaultBranch detects the repo's default branch (origin/HEAD, then main,
// then master), falling back to the currently checked-out branch.
func DefaultBranch(repo string) (string, error) {
	if out, err := run(repo, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(out, "origin/"), nil
	}
	for _, cand := range []string{"main", "master"} {
		if BranchExists(repo, cand) {
			return cand, nil
		}
	}
	return CurrentBranch(repo)
}

// CurrentBranch returns the branch checked out in dir.
func CurrentBranch(dir string) (string, error) {
	return run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// WorktreeAdd creates a worktree at path. When newBranch is true a new branch
// is created from base; otherwise the existing branch is checked out.
func WorktreeAdd(repo, path, branch, base string, newBranch bool) error {
	args := []string{"-C", repo, "worktree", "add"}
	if newBranch {
		args = append(args, "-b", branch, path, base)
	} else {
		args = append(args, path, branch)
	}
	return runIO(args...)
}

// WorktreeRemove removes the worktree at path.
func WorktreeRemove(repo, path string, force bool) error {
	args := []string{"-C", repo, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return runIO(args...)
}

// Merge checks out base in repo and merges branch into it (no fast-forward so
// the workspace's history is preserved as a unit). A default merge message is
// supplied so the merge is non-interactive — important for the dashboard,
// which cannot host the merge-message editor mid-flow.
func Merge(repo, base, branch string) error {
	if err := runIO("-C", repo, "checkout", base); err != nil {
		return err
	}
	msg := fmt.Sprintf("Merge branch '%s' into %s", branch, base)
	return runIO("-C", repo, "merge", "--no-ff", "-m", msg, branch)
}

// DeleteBranch deletes a local branch. force uses -D (unmerged-safe).
func DeleteBranch(repo, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return runIO("-C", repo, "branch", flag, branch)
}

// Diff returns the cumulative diff of a worktree against base: everything on
// the branch plus uncommitted tracked edits. Untracked files are listed
// separately at the end since `git diff` does not include them.
func Diff(dir, base string) (string, error) {
	diff, err := run(dir, "diff", base)
	if err != nil {
		return "", err
	}
	untracked, err := run(dir, "ls-files", "--others", "--exclude-standard")
	if err == nil && untracked != "" {
		var b strings.Builder
		b.WriteString(diff)
		if diff != "" {
			b.WriteString("\n")
		}
		b.WriteString("# Untracked files (not in the diff):\n")
		for _, f := range strings.Split(untracked, "\n") {
			if f != "" {
				b.WriteString("#   " + f + "\n")
			}
		}
		return b.String(), nil
	}
	return diff, nil
}

// Stats derives the live status of a worktree against base.
func Stats(dir, base string) (Stat, error) {
	st := Stat{}

	branch, err := CurrentBranch(dir)
	if err != nil {
		return st, err
	}
	st.Branch = branch

	// Dirty: any porcelain output (includes untracked files).
	if out, err := run(dir, "status", "--porcelain"); err == nil {
		st.Dirty = out != ""
	}

	// Ahead/behind vs base. left = base-only (behind), right = HEAD-only (ahead).
	if out, err := run(dir, "rev-list", "--left-right", "--count", base+"...HEAD"); err == nil {
		fields := strings.Fields(out)
		if len(fields) == 2 {
			st.Behind, _ = strconv.Atoi(fields[0])
			st.Ahead, _ = strconv.Atoi(fields[1])
		}
	}

	// +/- lines vs base, covering committed and unstaged tracked changes.
	if out, err := run(dir, "diff", "--numstat", base); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			// "-" marks binary files; skip the line counts for those.
			if a, err := strconv.Atoi(fields[0]); err == nil {
				st.Added += a
			}
			if d, err := strconv.Atoi(fields[1]); err == nil {
				st.Deleted += d
			}
		}
	}

	return st, nil
}
