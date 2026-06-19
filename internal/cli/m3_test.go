package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
)

// The tmux-only commands must refuse cleanly (not panic, not act) when there is
// no tmux session. isolateConfig clears $TMUX, so this is the no-tmux path.
func TestTmuxCommandsRequireTmux(t *testing.T) {
	isolateConfig(t)
	for _, args := range [][]string{
		{"close", "feat"},
		{"resurrect"},
		{"sidebar"},
	} {
		out, err := execWF(t, args...)
		if err == nil {
			t.Errorf("%v should error without tmux; out=%q", args, out)
			continue
		}
		if !strings.Contains(err.Error(), "tmux") {
			t.Errorf("%v error = %q, want it to mention tmux", args, err)
		}
	}
}

// Outside tmux, `open` launches the workspace's default editor. A custom
// "no-op" editor (the `true` binary) keeps this deterministic across machines.
func TestOpenLaunchesDefaultEditor(t *testing.T) {
	cfg := isolateConfig(t)
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no 'true' binary available")
	}
	if err := config.SaveGlobal(&config.Global{
		DefaultIDE: "noop",
		IDEs:       []config.IDESpec{{ID: "noop", Name: "Noop", Cmd: "true"}},
	}); err != nil {
		t.Fatal(err)
	}
	_ = cfg
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "open", "feat", "--project", "proj"); err != nil {
		t.Errorf("open (editor launch): %v", err)
	}
}

// startIsolatedTmux brings up a private tmux server and points $TMUX at it, so
// the tmux-aware commands act on this server (never the user's default one). It
// returns the socket name for direct assertions and registers teardown.
func startIsolatedTmux(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("wf_cli_%d", os.Getpid())
	raw := func(args ...string) []byte {
		t.Helper()
		out, err := exec.Command(bin, append([]string{"-L", socket}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return out
	}
	raw("new-session", "-d", "-s", "wf", "-x", "80", "-y", "24")
	sockPath := strings.TrimSpace(string(raw("display-message", "-t", "wf", "-p", "#{socket_path}")))
	t.Setenv("TMUX", sockPath+",0,0")
	t.Cleanup(func() {
		_ = exec.Command(bin, "-L", socket, "kill-server").Run()
		_ = os.Remove(sockPath)
	})
	return socket
}

func tmuxWindows(t *testing.T, socket string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", socket, "list-windows", "-F", "#{window_id} #{window_name} #{@wf_workspace}").CombinedOutput()
	if err != nil {
		t.Fatalf("list-windows: %v\n%s", err, out)
	}
	return string(out)
}

// TestTmuxCommandsWithServer drives the window-creating success paths (add,
// resurrect, close, rm cleanup) against an isolated tmux server. The worktree
// path is read from the registry rather than `wf path`/`wf add` stdout, which
// those commands print to os.Stdout (not the captured cobra buffer).
func TestTmuxCommandsWithServer(t *testing.T) {
	cfg := isolateConfig(t)        // isolate XDG (also clears TMUX)…
	socket := startIsolatedTmux(t) // …then point TMUX at a private server

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
	wts := store.FindWorktrees("feat", "proj")
	if len(wts) != 1 {
		t.Fatalf("want 1 worktree, got %d", len(wts))
	}
	wtPath := wts[0].Path
	hasWindow := func() bool { return strings.Contains(tmuxWindows(t, socket), wtPath) }

	// add created a window tagged with the worktree path.
	if !hasWindow() {
		t.Fatalf("add did not create a window; windows:\n%s", tmuxWindows(t, socket))
	}

	// resurrect with the window already open reports nothing created.
	if rout, err := execWF(t, "resurrect"); err != nil || !strings.Contains(rout, "0 created, 1 already open") {
		t.Errorf("resurrect (open) = %q err=%v", rout, err)
	}

	// close kills the window but keeps the workspace.
	if cout, err := execWF(t, "close", "feat", "--project", "proj"); err != nil || !strings.Contains(cout, "Closed") {
		t.Errorf("close = %q err=%v", cout, err)
	}
	if hasWindow() {
		t.Error("close left the window behind")
	}
	if got := listJSON(t); len(got) != 1 {
		t.Errorf("close should keep the workspace; got %d", len(got))
	}

	// resurrect now rebinds the missing window from the registry.
	if rout, err := execWF(t, "resurrect"); err != nil || !strings.Contains(rout, "rebound proj/feat") {
		t.Errorf("resurrect (rebind) = %q err=%v", rout, err)
	}
	if !hasWindow() {
		t.Error("resurrect did not recreate the window")
	}

	// rm cleans up the window too.
	if _, err := execWF(t, "rm", "feat", "--project", "proj", "--force"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if hasWindow() {
		t.Error("rm left the window behind")
	}
}
