package sidebar

import (
	"strings"
	"testing"
	"time"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/status"
)

func TestAttachStatus(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := status.Write("alpha", "feat-1", "/wt/a1", status.Working); err != nil {
		t.Fatal(err)
	}
	es := []entry{
		{project: "alpha", branch: "feat-1", path: "/wt/a1"},
		{project: "beta", branch: "feat-2", path: "/wt/b2"}, // no status file
		{path: "/wt/untracked"},                             // no project/branch
	}
	attachStatus(es, 5*time.Minute)

	if es[0].state != status.Working {
		t.Errorf("entry 0 state = %q, want working", es[0].state)
	}
	if es[1].state != "" {
		t.Errorf("entry 1 (no file) state = %q, want empty", es[1].state)
	}
	if es[2].state != "" {
		t.Errorf("untracked entry state = %q, want empty", es[2].state)
	}
}

func TestRenderEntryShowsStatusGlyph(t *testing.T) {
	look := (&config.StatusConfig{}).Resolve() // nerdfont defaults
	m := New("")
	m.look = look
	m.entries = []entry{
		{project: "alpha", branch: "feat-1", path: "/wt/a1", state: status.Working},
		{project: "beta", branch: "feat-2", path: "/wt/b2"}, // idle
	}

	working := look.Look["working"].Glyph
	if got := m.renderEntry(1, m.entries[0]); !strings.Contains(got, working) {
		t.Errorf("working entry missing glyph %q: %q", working, got)
	}

	// Idle entry shows the branch glyph (slot filled), not the old "—".
	idle := look.Look["idle"].Glyph
	got := m.renderEntry(1, m.entries[1])
	if !strings.Contains(got, idle) {
		t.Errorf("idle entry missing branch glyph %q: %q", idle, got)
	}
	if strings.Contains(got, "—") {
		t.Errorf("idle entry still shows the placeholder em dash: %q", got)
	}
}
