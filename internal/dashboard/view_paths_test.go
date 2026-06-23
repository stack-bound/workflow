package dashboard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/config"
)

// TestShortenPath rewrites a leading home directory to ~ and leaves other paths
// untouched.
func TestShortenPath(t *testing.T) {
	t.Setenv("HOME", "/home/x")
	for in, want := range map[string]string{
		"/home/x":        "~",
		"/home/x/proj":   "~/proj",
		"/srv/elsewhere": "/srv/elsewhere",
	} {
		if got := shortenPath(in); got != want {
			t.Errorf("shortenPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTruncatePathLeft clips from the left with a leading … and never exceeds
// the target width.
func TestTruncatePathLeft(t *testing.T) {
	if got := truncatePathLeft("/short", 20); got != "/short" {
		t.Errorf("short path should pass through, got %q", got)
	}
	got := truncatePathLeft("/a/very/long/path/name", 8)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("clipped path should start with …: %q", got)
	}
	if w := lipgloss.Width(got); w > 8 {
		t.Errorf("clipped width = %d, want <= 8: %q", w, got)
	}
}

// TestPadPath exercises the three width regimes: truncate when the path nearly
// fits, drop it when there is no room, and show it (home-shortened) on an
// unknown width.
func TestPadPath(t *testing.T) {
	long := "/srv/some/deep/project/root"
	left := strings.Repeat("x", 40)

	if got := (Model{width: 60}).padPath(long, left); !strings.Contains(got, "…") {
		t.Errorf("width 60 should truncate the path: %q", got)
	}
	if got := (Model{width: 44}).padPath(long, left); strings.TrimSpace(got) != "" {
		t.Errorf("tiny width should drop the path: %q", got)
	}
	if got := (Model{width: 0}).padPath(long, left); !strings.Contains(got, "project/root") {
		t.Errorf("unknown width should still show the path: %q", got)
	}
}

// TestCopyPathCmd covers the base-row copy action via a no-op clipboard command,
// so it never touches a real clipboard.
func TestCopyPathCmd(t *testing.T) {
	m := New(nil, &config.Global{ClipboardCmd: "cat >/dev/null"})
	msg := m.copyPathCmd("/srv/workflow", "workflow")()
	am, ok := msg.(actionMsg)
	if !ok || am.err != nil {
		t.Fatalf("copyPathCmd = %#v, want an ok actionMsg", msg)
	}
	if !strings.Contains(am.msg, "workflow") {
		t.Errorf("copy confirmation = %q, want to mention the label", am.msg)
	}
}
