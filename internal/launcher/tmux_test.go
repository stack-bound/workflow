package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// isolatedTmux starts a private tmux server on a unique socket and points $TMUX
// at it, so the tmux package's bare `tmux` calls resolve to this server — never
// the user's default one. Teardown kills only this server and removes its
// socket. It returns the socket name for direct assertions.
func isolatedTmux(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("wf_launch_%d", os.Getpid())
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

func windowNames(t *testing.T, socket string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", socket, "list-windows", "-F", "#{window_name}").CombinedOutput()
	if err != nil {
		t.Fatalf("list-windows: %v\n%s", err, out)
	}
	return string(out)
}

func TestTmuxBackendLifecycle(t *testing.T) {
	socket := isolatedTmux(t)
	lt := NewTmux()
	dir := t.TempDir()

	// EnsureWindow creates a window the first time and is idempotent after.
	made, err := lt.EnsureWindow(dir, "feat")
	if err != nil {
		t.Fatalf("EnsureWindow: %v", err)
	}
	if !made {
		t.Error("first EnsureWindow should create a window")
	}
	if !strings.Contains(windowNames(t, socket), "feat") {
		t.Errorf("window 'feat' not created; have:\n%s", windowNames(t, socket))
	}
	if made, _ := lt.EnsureWindow(dir, "feat"); made {
		t.Error("second EnsureWindow should be a no-op")
	}

	// Open on an existing workspace jumps without creating a duplicate.
	if err := lt.Open(dir, "feat"); err != nil {
		t.Errorf("Open existing: %v", err)
	}
	if n := strings.Count(windowNames(t, socket), "feat"); n != 1 {
		t.Errorf("expected exactly one 'feat' window, got %d", n)
	}

	// Close kills it; a second close is a no-op.
	closed, err := lt.Close(dir)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed {
		t.Error("Close should report the window closed")
	}
	if strings.Contains(windowNames(t, socket), "feat") {
		t.Error("window still present after Close")
	}
	if closed, _ := lt.Close(dir); closed {
		t.Error("second Close should be a no-op")
	}

	// Open on a fresh workspace creates the window and jumps to it.
	dir2 := t.TempDir()
	if err := lt.Open(dir2, "feat2"); err != nil {
		t.Errorf("Open new: %v", err)
	}
	if !strings.Contains(windowNames(t, socket), "feat2") {
		t.Errorf("Open did not create 'feat2'; have:\n%s", windowNames(t, socket))
	}
}
