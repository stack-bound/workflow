package cli

import (
	"strings"
	"testing"
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

// Outside tmux, `open` falls through to the editor launcher.
func TestOpenFallsBackToEditor(t *testing.T) {
	isolateConfig(t)
	t.Setenv("EDITOR", "true") // a no-op editor that exists on PATH
	repo := gitRepo(t)
	if _, err := execWF(t, "project", "add", repo, "--name", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "add", "feat", "--project", "proj"); err != nil {
		t.Fatal(err)
	}
	if _, err := execWF(t, "open", "feat", "--project", "proj"); err != nil {
		t.Errorf("open (editor fallback): %v", err)
	}
	if _, err := execWF(t, "open", "feat", "--project", "proj", "--editor"); err != nil {
		t.Errorf("open --editor: %v", err)
	}
}
