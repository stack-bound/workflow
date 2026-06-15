package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseWindows(t *testing.T) {
	out := "@0\t0\t1\t/work/a\tfeat-a\n@1\t1\t0\t\tbash\n@2\t2\t0\t/work/b\tfeat-b"
	wins := parseWindows(out)
	if len(wins) != 3 {
		t.Fatalf("got %d windows, want 3", len(wins))
	}
	if wins[0].ID != "@0" || wins[0].Index != 0 || !wins[0].Active || wins[0].Workspace != "/work/a" || wins[0].Name != "feat-a" {
		t.Errorf("window 0 = %+v", wins[0])
	}
	if wins[1].Workspace != "" || wins[1].Active {
		t.Errorf("untracked window should have no workspace and not be active: %+v", wins[1])
	}
	if wins[2].Workspace != "/work/b" {
		t.Errorf("window 2 workspace = %q, want /work/b", wins[2].Workspace)
	}
}

func TestParseWindowsEmptyAndMalformed(t *testing.T) {
	if got := parseWindows(""); got != nil {
		t.Errorf("empty input = %+v, want nil", got)
	}
	// A line with too few fields is skipped rather than panicking.
	if got := parseWindows("@0\t0\tonly-three"); len(got) != 0 {
		t.Errorf("malformed line should be skipped, got %+v", got)
	}
}

func TestInsideReflectsEnv(t *testing.T) {
	t.Setenv("TMUX", "")
	if Inside() {
		t.Error("Inside() true with empty TMUX")
	}
	t.Setenv("TMUX", "/tmp/x,1,0")
	if !Inside() {
		t.Error("Inside() false with TMUX set")
	}
}

func TestAvailable(t *testing.T) {
	t.Setenv("TMUX", "")
	if Available() {
		t.Error("Available() true without a tmux session")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return // can't assert the positive case without the tmux binary
	}
	t.Setenv("TMUX", "/tmp/x,1,0")
	if !Available() {
		t.Error("Available() false with TMUX set and tmux on PATH")
	}
}

// --- isolated-server integration ---

// tmuxBin skips the test when tmux is unavailable; otherwise it returns its path.
func tmuxBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not installed")
	}
	return p
}

// isolatedServer starts a private tmux server on a unique socket and points the
// package at it. Teardown kills ONLY that server (never the default one), per
// the repo's tmux-testing rules.
func isolatedServer(t *testing.T) string {
	t.Helper()
	bin := tmuxBin(t)
	socket := fmt.Sprintf("wf_test_%d", os.Getpid())

	rawTmux := func(args ...string) {
		t.Helper()
		full := append([]string{"-L", socket}, args...)
		if out, err := exec.Command(bin, full...).CombinedOutput(); err != nil {
			t.Fatalf("tmux %v: %v\n%s", full, err, out)
		}
	}
	// A detached session gives list-windows/new-window a session to act on.
	rawTmux("new-session", "-d", "-s", "wf", "-x", "80", "-y", "24")
	// Capture the socket path so cleanup can remove the file too — kill-server
	// alone can leave a dead socket inode behind in $TMUX_TMPDIR.
	sockPath, _ := exec.Command(bin, "-L", socket, "display-message", "-t", "wf", "-p", "#{socket_path}").Output()

	serverFlags = []string{"-L", socket}
	t.Cleanup(func() {
		serverFlags = nil
		_ = exec.Command(bin, "-L", socket, "kill-server").Run()
		if p := strings.TrimSpace(string(sockPath)); p != "" {
			_ = os.Remove(p)
		}
	})
	return socket
}

func TestWindowLifecycle(t *testing.T) {
	isolatedServer(t)

	dir := t.TempDir()
	id, err := NewWindow(dir, "feat-x")
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if id == "" {
		t.Fatal("NewWindow returned an empty id")
	}

	// The new window is discoverable by its workspace-path tag.
	w, err := FindByWorkspace(dir)
	if err != nil {
		t.Fatalf("FindByWorkspace: %v", err)
	}
	if w == nil {
		t.Fatal("FindByWorkspace did not find the tagged window")
	}
	if w.ID != id || w.Workspace != dir || w.Name != "feat-x" {
		t.Errorf("found window = %+v, want id=%s workspace=%s name=feat-x", w, id, dir)
	}

	// And it shows up in the open-workspace set.
	open, err := OpenWorkspaces()
	if err != nil {
		t.Fatalf("OpenWorkspaces: %v", err)
	}
	if !open[dir] {
		t.Errorf("open set %v missing %s", open, dir)
	}

	if err := SelectWindow(id); err != nil {
		t.Errorf("SelectWindow: %v", err)
	}

	// Closing removes it from discovery.
	if err := KillWindow(id); err != nil {
		t.Fatalf("KillWindow: %v", err)
	}
	w, err = FindByWorkspace(dir)
	if err != nil {
		t.Fatalf("FindByWorkspace after kill: %v", err)
	}
	if w != nil {
		t.Errorf("window still found after kill: %+v", w)
	}
}
