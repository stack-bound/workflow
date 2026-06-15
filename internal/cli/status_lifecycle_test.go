package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
)

func TestStatusFileRemovedOnRm(t *testing.T) {
	cfg := isolateConfig(t)
	wt := addWorktreeForStatus(t, cfg)
	if err := status.Write(wt.Project, wt.Branch, wt.Path, status.Working); err != nil {
		t.Fatal(err)
	}

	if _, err := execWF(t, "rm", "feat", "--project", "proj", "--force"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if _, ok, _ := status.ReadFor(wt.Project, wt.Branch, wt.Path); ok {
		t.Error("status file survived rm")
	}
}

func TestStatusFileRemovedOnMerge(t *testing.T) {
	cfg := isolateConfig(t)
	wt := addWorktreeForStatus(t, cfg)
	if err := status.Write(wt.Project, wt.Branch, wt.Path, status.Working); err != nil {
		t.Fatal(err)
	}
	// Commit something so the merge has work to fast-forward.
	if err := os.WriteFile(filepath.Join(wt.Path, "feature.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, wt.Path, "add", ".")
	gitCmd(t, wt.Path, "commit", "-m", "feature")

	if _, err := execWF(t, "merge", "feat", "--project", "proj"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, ok, _ := status.ReadFor(wt.Project, wt.Branch, wt.Path); ok {
		t.Error("status file survived merge")
	}
}

func TestStatusResetToIdleOnClose(t *testing.T) {
	cfg := isolateConfig(t)
	socket := startIsolatedTmux(t)
	_ = socket

	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatalf("add: %v", err)
	}
	store, err := registry.Load(regPathFor(cfg))
	if err != nil {
		t.Fatal(err)
	}
	wt := store.FindWorktrees("feat", "proj")[0]

	// Pretend an agent was working, then close the window.
	if err := status.Write(wt.Project, wt.Branch, wt.Path, status.Working); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "close", "feat", "--project", "proj"); err != nil {
		t.Fatalf("close: %v", err)
	}
	st, ok, err := status.ReadFor(wt.Project, wt.Branch, wt.Path)
	if err != nil || !ok {
		t.Fatalf("ReadFor after close ok=%v err=%v", ok, err)
	}
	if st.State != status.Idle {
		t.Errorf("state after close = %q, want idle", st.State)
	}
}

func TestCreatedWindowsCarryIdleGlyph(t *testing.T) {
	cfg := isolateConfig(t)
	socket := startIsolatedTmux(t)

	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_ = cfg

	idleGlyph := (&config.Global{}).StatusLook().Look["idle"].Glyph
	want := idleGlyph + " feat"
	if got := tmuxWindows(t, socket); !strings.Contains(got, want) {
		t.Errorf("created window name missing idle glyph; want substring %q in:\n%s", want, got)
	}
}
