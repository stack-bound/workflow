package cli

import (
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
)

func TestProjectRenameCLI(t *testing.T) {
	cfg := isolateConfig(t)
	repo := gitRepo(t)

	if _, err := execWF(t, "project", "add", repo, "--name", "old"); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if _, err := execWF(t, "project", "rename", "old", "new"); err != nil {
		t.Fatalf("project rename: %v", err)
	}

	s, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if s.FindProject("old") != nil {
		t.Error("old project name still present after rename")
	}
	if s.FindProject("new") == nil {
		t.Error("renamed project not found under new name")
	}

	// Renaming an unknown project surfaces an error.
	if _, err := execWF(t, "project", "rename", "ghost", "x"); err == nil {
		t.Error("expected error renaming an unknown project")
	}
}
