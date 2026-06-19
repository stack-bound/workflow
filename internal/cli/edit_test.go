package cli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/workspace"
)

// wf edit --list prints detected editors with their ids (so users know what to
// put in default_ide). A custom editor guarantees at least one row.
func TestEditList(t *testing.T) {
	isolateConfig(t)
	if err := config.SaveGlobal(&config.Global{
		IDEs: []config.IDESpec{{ID: "noop", Name: "Noop", Cmd: "true"}},
	}); err != nil {
		t.Fatal(err)
	}
	out, err := execWF(t, "edit", "--list")
	if err != nil {
		t.Fatalf("edit --list: %v", err)
	}
	if !strings.Contains(out, "noop") || !strings.Contains(out, "ID") {
		t.Errorf("edit --list output = %q", out)
	}
}

// With a repo default and autolaunch set, `wf edit <branch>` opens it directly
// (no picker, no TTY needed). The `true` editor keeps it deterministic.
func TestEditAutolaunch(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no 'true' binary available")
	}
	isolateConfig(t)
	if err := config.SaveGlobal(&config.Global{
		IDEs: []config.IDESpec{{ID: "noop", Name: "Noop", Cmd: "true"}},
	}); err != nil {
		t.Fatal(err)
	}
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatal(err)
	}
	// Pin the default + autolaunch in the project's .workFlow.yaml.
	if err := config.SetRepoIDE(repo, "noop", true); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "edit", "feat", "--project", "proj"); err != nil {
		t.Errorf("edit autolaunch: %v", err)
	}
}

// resolveEditTarget maps a branch argument to its worktree path and project.
func TestResolveEditTargetBranch(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatal(err)
	}
	m := workspace.New(regPathFor(cfg), &config.Global{})
	dir, proj, err := resolveEditTarget(m, []string{"feat"}, "proj")
	if err != nil {
		t.Fatal(err)
	}
	if proj != "proj" {
		t.Errorf("project = %q, want proj", proj)
	}
	if !strings.Contains(dir, "feat") {
		t.Errorf("dir = %q, want it to be the feat worktree", dir)
	}
	// An unknown branch is an error.
	if _, _, err := resolveEditTarget(m, []string{"nope"}, "proj"); err == nil {
		t.Error("expected an error for an unknown branch")
	}
}
