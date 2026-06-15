package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/workspace"
)

// --- harness ---

// isolateConfig points the global config + registry at a private dir.
func isolateConfig(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	return cfg
}

func regPathFor(cfg string) string {
	return filepath.Join(cfg, "workFlow", "registry.json")
}

// execWF runs the root command with args, capturing cmd output.
func execWF(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

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

func gitRepo(t *testing.T) string {
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

func listJSON(t *testing.T) []map[string]any {
	t.Helper()
	out, err := execWF(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var views []map[string]any
	if err := json.Unmarshal([]byte(out), &views); err != nil {
		t.Fatalf("list --json parse: %v\n%s", err, out)
	}
	return views
}

// --- end-to-end ---

func TestCLIWorkspaceLifecycle(t *testing.T) {
	isolateConfig(t)
	repo := gitRepo(t)

	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if got := listJSON(t); len(got) != 0 {
		t.Fatalf("expected no workspaces initially, got %d", len(got))
	}

	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatalf("add: %v", err)
	}
	views := listJSON(t)
	if len(views) != 1 || views[0]["branch"] != "feat" {
		t.Fatalf("list after add = %+v", views)
	}

	if _, err := execWF(t, "path", "feat", "--project", "proj"); err != nil {
		t.Errorf("path: %v", err)
	}
	if _, err := execWF(t, "project", "ls"); err != nil {
		t.Errorf("project ls: %v", err)
	}

	if _, err := execWF(t, "rm", "feat", "--project", "proj", "--force"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if got := listJSON(t); len(got) != 0 {
		t.Errorf("workspace not removed: %+v", got)
	}
}

func TestCLIMerge(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatal(err)
	}

	store, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	wts := store.FindWorktrees("feat", "proj")
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	wt := wts[0].Path
	if err := os.WriteFile(filepath.Join(wt, "feature.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, wt, "add", ".")
	gitCmd(t, wt, "commit", "-m", "feature")

	if _, err := execWF(t, "merge", "feat", "--project", "proj"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Errorf("merge did not land on base: %v", err)
	}
}

func TestCLIErrors(t *testing.T) {
	isolateConfig(t)
	if _, err := execWF(t, "project", "add", t.TempDir()); err == nil {
		t.Error("expected error registering a non-git dir")
	}
	if _, err := execWF(t, "project", "rm", "ghost"); err == nil {
		t.Error("expected error removing unknown project")
	}
	if _, err := execWF(t, "add"); err == nil {
		t.Error("expected arg error for add with no branch")
	}
}

func TestCLIVersion(t *testing.T) {
	out, err := execWF(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wf version") {
		t.Errorf("version output = %q", out)
	}
}

func TestCLIInit(t *testing.T) {
	isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)

	if _, err := execWF(t, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	root, err := git.RepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".workFlow.yaml")); err != nil {
		t.Errorf("init did not write config: %v", err)
	}
	if _, err := execWF(t, "init"); err == nil {
		t.Error("expected error: config already exists")
	}
	if _, err := execWF(t, "init", "--force"); err != nil {
		t.Errorf("init --force: %v", err)
	}
}

func TestCLIConfigCommands(t *testing.T) {
	isolateConfig(t)
	if _, err := execWF(t, "config", "path"); err != nil {
		t.Errorf("config path: %v", err)
	}
	if _, err := execWF(t, "config", "show"); err != nil {
		t.Errorf("config show: %v", err)
	}
}

func TestCLICompletionsInstall(t *testing.T) {
	isolateConfig(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("HOME", t.TempDir())

	if _, err := execWF(t, "completions", "install", "bash"); err != nil {
		t.Fatalf("completions install bash: %v", err)
	}
	dest := filepath.Join(data, "bash-completion", "completions", "wf")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("completion file not written: %v", err)
	}
	if _, err := execWF(t, "completions", "install", "bash"); err == nil {
		t.Error("expected error: completion already exists")
	}
	if _, err := execWF(t, "completions", "install", "bash", "--force"); err != nil {
		t.Errorf("install --force: %v", err)
	}
}

func TestCLICompletionsInvalidShell(t *testing.T) {
	if _, err := execWF(t, "completions", "tcsh"); err == nil {
		t.Error("expected error for invalid shell argument")
	}
}

// --- pure helpers ---

func TestUniqueProjectName(t *testing.T) {
	s := &registry.Store{}
	if got := uniqueProjectName(s, "repo"); got != "repo" {
		t.Errorf("free name = %q, want repo", got)
	}
	_ = s.AddProject(registry.Project{Name: "repo", Path: "/p/repo"})
	if got := uniqueProjectName(s, "repo"); got != "repo-2" {
		t.Errorf("taken once = %q, want repo-2", got)
	}
	_ = s.AddProject(registry.Project{Name: "repo-2", Path: "/p/repo2"})
	if got := uniqueProjectName(s, "repo"); got != "repo-3" {
		t.Errorf("taken twice = %q, want repo-3", got)
	}
}

func TestToJSON(t *testing.T) {
	views := []workspace.View{
		{Worktree: registry.Worktree{Project: "p", Branch: "feat", Base: "main", Path: "/wt"}},
		{Worktree: registry.Worktree{Project: "p", Branch: "bad"}, StatErr: errors.New("boom")},
	}
	views[0].Stat.Ahead = 2
	views[0].Stat.Dirty = true

	out := toJSON(views)
	if len(out) != 2 {
		t.Fatalf("toJSON len = %d, want 2", len(out))
	}
	if out[0].Branch != "feat" || !out[0].Active || !out[0].Dirty || out[0].Ahead != 2 {
		t.Errorf("first view = %+v", out[0])
	}
	if out[1].Error != "boom" || out[1].Active {
		t.Errorf("second view = %+v", out[1])
	}
}
