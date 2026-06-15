package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
)

// fakeTTY forces stdinIsTTY to v for the duration of the test, so the
// auto-register prompt can be exercised (or suppressed) deterministically.
func fakeTTY(t *testing.T, v bool) {
	t.Helper()
	prev := stdinIsTTY
	stdinIsTTY = func() bool { return v }
	t.Cleanup(func() { stdinIsTTY = prev })
}

// execWFIn runs the root command with args, feeding in as stdin and capturing
// combined output. It is execWF plus an injected interactive answer.
func execWFIn(t *testing.T, in string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(in))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func projectCount(t *testing.T, cfg string) int {
	t.Helper()
	s, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	return len(s.Projects)
}

func TestAddAutoRegisterPromptYes(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, true)

	out, err := execWFIn(t, "y\n", "add", "feat")
	if err != nil {
		t.Fatalf("add with prompt yes: %v\n%s", err, out)
	}
	// The prompt must name the path so a wrong-folder run is obvious.
	if !strings.Contains(out, repo) {
		t.Errorf("prompt should show the repo path %q; got:\n%s", repo, out)
	}
	if !strings.Contains(out, "Registered project") {
		t.Errorf("expected registration confirmation; got:\n%s", out)
	}
	if n := projectCount(t, cfg); n != 1 {
		t.Fatalf("expected 1 registered project, got %d", n)
	}
	if views := listJSON(t); len(views) != 1 || views[0]["branch"] != "feat" {
		t.Fatalf("workspace not created: %+v", views)
	}
}

func TestAddAutoRegisterPromptNo(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, true)

	out, err := execWFIn(t, "n\n", "add", "feat")
	if err == nil {
		t.Fatalf("expected error when registration declined; got:\n%s", out)
	}
	if n := projectCount(t, cfg); n != 0 {
		t.Errorf("declining should not register; got %d projects", n)
	}
	if views := listJSON(t); len(views) != 0 {
		t.Errorf("declining should not create a workspace; got %+v", views)
	}
}

func TestAddAutoRegisterYesFlag(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, false) // no terminal: --yes must still register

	out, err := execWF(t, "add", "feat", "--yes")
	if err != nil {
		t.Fatalf("add --yes: %v\n%s", err, out)
	}
	if n := projectCount(t, cfg); n != 1 {
		t.Fatalf("expected 1 registered project, got %d", n)
	}
}

func TestAddUnregisteredNonInteractive(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, false) // not a terminal and no --yes

	out, err := execWF(t, "add", "feat")
	if err == nil {
		t.Fatalf("expected error in non-interactive unregistered repo; got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "wf project add") && !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should hint at how to register; got: %v", err)
	}
	if n := projectCount(t, cfg); n != 0 {
		t.Errorf("non-interactive run must not register silently; got %d projects", n)
	}
}

func TestInitRegistersPromptYes(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, true)

	out, err := execWFIn(t, "y\n", "init")
	if err != nil {
		t.Fatalf("init with prompt yes: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Wrote ") {
		t.Errorf("init should still write the template; got:\n%s", out)
	}
	if !strings.Contains(out, "Registered project") {
		t.Errorf("init should register on yes; got:\n%s", out)
	}
	if n := projectCount(t, cfg); n != 1 {
		t.Fatalf("expected 1 registered project, got %d", n)
	}
}

func TestInitRegisterDeclined(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)
	t.Chdir(repo)
	fakeTTY(t, true)

	out, err := execWFIn(t, "n\n", "init")
	if err != nil {
		t.Fatalf("declining registration must not fail init: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Not registered") {
		t.Errorf("expected a hint after declining; got:\n%s", out)
	}
	if n := projectCount(t, cfg); n != 0 {
		t.Errorf("declining should not register; got %d projects", n)
	}
}
