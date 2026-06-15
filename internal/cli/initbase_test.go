package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
)

// gitRepoWithBranches makes a repo on main with the extra branches added.
func gitRepoWithBranches(t *testing.T, extra ...string) string {
	t.Helper()
	repo := gitRepo(t)
	for _, b := range extra {
		gitCmd(t, repo, "branch", b)
	}
	return repo
}

func TestDetectInitBaseOrdersDevelopmentFirst(t *testing.T) {
	repo := gitRepoWithBranches(t, "development", "master")

	def, cands := detectInitBase(repo)
	if def != "development" {
		t.Errorf("default base = %q, want development", def)
	}
	if want := []string{"development", "main", "master"}; !reflect.DeepEqual(cands, want) {
		t.Errorf("candidates = %v, want %v", cands, want)
	}
}

func TestDetectInitBaseSingleBranch(t *testing.T) {
	repo := gitRepo(t) // only main

	def, cands := detectInitBase(repo)
	if def != "main" {
		t.Errorf("default base = %q, want main", def)
	}
	if want := []string{"main"}; !reflect.DeepEqual(cands, want) {
		t.Errorf("candidates = %v, want %v", cands, want)
	}
}

func TestDetectInitBaseOriginHEADWins(t *testing.T) {
	repo := gitRepoWithBranches(t, "development")
	// Point origin/HEAD at master even though master is not a local branch:
	// git's tracked default must still take precedence and lead the list.
	origin := t.TempDir()
	gitCmd(t, origin, "init", "--bare", "-b", "master")
	gitCmd(t, repo, "remote", "add", "origin", origin)
	gitCmd(t, repo, "push", "origin", "main:master")
	gitCmd(t, repo, "remote", "set-head", "origin", "master")

	def, cands := detectInitBase(repo)
	if def != "master" {
		t.Errorf("default base = %q, want master (origin/HEAD)", def)
	}
	if len(cands) == 0 || cands[0] != "master" {
		t.Errorf("candidates should lead with master; got %v", cands)
	}
}

// readBase loads the base written into a repo's .workFlow.yaml.
func readBase(t *testing.T, repoRoot string) string {
	t.Helper()
	r, err := config.LoadRepo(repoRoot)
	if err != nil {
		t.Fatalf("load repo config: %v", err)
	}
	return r.Base
}

func TestInitPromptWritesChosenBase(t *testing.T) {
	isolateConfig(t)
	repo := gitRepoWithBranches(t, "development", "master")
	t.Chdir(repo)
	fakeTTY(t, true)

	// First line answers the base prompt (empty -> detected default
	// "development"); the rest is consumed by the register prompt.
	out, err := execWFIn(t, "\n", "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if got := readBase(t, repo); got != "development" {
		t.Errorf("written base = %q, want development", got)
	}
}

func TestInitPromptSelectByNumber(t *testing.T) {
	isolateConfig(t)
	repo := gitRepoWithBranches(t, "development", "master")
	t.Chdir(repo)
	fakeTTY(t, true)

	// candidates are [development, main, master]; "2" picks main.
	out, err := execWFIn(t, "2\n", "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if got := readBase(t, repo); got != "main" {
		t.Errorf("written base = %q, want main", got)
	}
}

func TestInitNonInteractiveUsesDetectedBase(t *testing.T) {
	isolateConfig(t)
	repo := gitRepoWithBranches(t, "development", "master")
	t.Chdir(repo)
	fakeTTY(t, false) // no terminal: must not prompt, just detect

	out, err := execWF(t, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if !strings.Contains(out, "base branch: development") {
		t.Errorf("init should report the detected base; got:\n%s", out)
	}
	if got := readBase(t, repo); got != "development" {
		t.Errorf("written base = %q, want development", got)
	}
}

func TestInitYesFlagSkipsBasePrompt(t *testing.T) {
	isolateConfig(t)
	repo := gitRepoWithBranches(t, "development", "master")
	t.Chdir(repo)
	fakeTTY(t, true) // interactive, but --yes must skip the prompt

	out, err := execWF(t, "init", "--yes")
	if err != nil {
		t.Fatalf("init --yes: %v\n%s", err, out)
	}
	if got := readBase(t, repo); got != "development" {
		t.Errorf("written base = %q, want development", got)
	}
}
