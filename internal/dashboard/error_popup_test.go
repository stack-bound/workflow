package dashboard

import (
	"errors"
	"strings"
	"testing"
)

// A failed suspend-and-run action (merge/add/rm/forget) must surface the
// engine's real message as a popup, not the bare "exit status 1" status line.
func TestActionMsgRaisesErrorPopup(t *testing.T) {
	detail := "wf: workspace workflow/fix/colours has uncommitted changes; commit or stash before merging\n"
	m, cmd := step(readyModel(t), actionMsg{
		err:      errors.New("exit status 1"),
		errTitle: "Merge failed",
		detail:   detail,
		refresh:  true,
	})
	if m.mode != modeError {
		t.Fatalf("mode = %v, want modeError", m.mode)
	}
	if m.errTitle != "Merge failed" {
		t.Errorf("errTitle = %q", m.errTitle)
	}
	want := "workspace workflow/fix/colours has uncommitted changes; commit or stash before merging"
	if m.errBody != want {
		t.Errorf("errBody = %q, want %q", m.errBody, want)
	}
	if strings.Contains(m.errBody, "exit status 1") || strings.Contains(m.errBody, "wf: ") {
		t.Errorf("errBody leaked exit code or prefix: %q", m.errBody)
	}
	if m.status != "" || m.statusErr {
		t.Errorf("status not cleared: status=%q err=%v", m.status, m.statusErr)
	}
	if cmd == nil {
		t.Error("refresh action should still return a refresh command")
	}

	// The popup renders as a red card carrying the title and body over the ledger.
	got := m.View()
	// The body wraps, so assert on words that survive a line break rather than a
	// contiguous phrase.
	if !strings.Contains(got, "Merge failed") || !strings.Contains(got, "uncommitted") {
		t.Errorf("error popup missing title/body:\n%s", got)
	}
	if !strings.Contains(got, "dismiss") {
		t.Errorf("error popup missing help line:\n%s", got)
	}
}

// An untitled action error (a direct engine call like copy/open) keeps the
// existing one-line status behaviour rather than popping a box.
func TestActionMsgUntitledErrorStaysStatusLine(t *testing.T) {
	m, _ := step(readyModel(t), actionMsg{err: errors.New("clipboard boom")})
	if m.mode == modeError {
		t.Fatal("untitled error should not enter modeError")
	}
	if !m.statusErr || !strings.Contains(m.status, "failed") {
		t.Errorf("status = %q err=%v, want a failed status line", m.status, m.statusErr)
	}
}

func TestHandleErrorKeyDismisses(t *testing.T) {
	m := readyModel(t)
	m.mode = modeError
	m.errTitle, m.errBody = "Merge failed", "boom"
	m, cmd := step(m, runeKey("x")) // any key dismisses
	if m.mode != modeLedger {
		t.Errorf("mode = %v, want modeLedger after dismiss", m.mode)
	}
	if m.errTitle != "" || m.errBody != "" {
		t.Errorf("popup content not cleared: title=%q body=%q", m.errTitle, m.errBody)
	}
	if cmd != nil {
		t.Errorf("dismiss should issue no command, got %v", cmd)
	}
}

func TestEngineError(t *testing.T) {
	cases := []struct {
		name   string
		detail string
		err    error
		want   string
	}{
		{
			name:   "strips wf prefix from the failure line",
			detail: "wf: workspace a/b has uncommitted changes; commit or stash before merging\n",
			err:    errors.New("exit status 1"),
			want:   "workspace a/b has uncommitted changes; commit or stash before merging",
		},
		{
			name:   "ignores preceding progress output and takes the last wf line",
			detail: "Cloning into 'repo'...\nremote: Counting objects\nwf: could not create worktree: boom\n",
			err:    errors.New("exit status 1"),
			want:   "could not create worktree: boom",
		},
		{
			name:   "falls back to the process error when no wf line is present",
			detail: "some opaque git failure\n",
			err:    errors.New("exit status 128"),
			want:   "exit status 128",
		},
		{
			name:   "falls back to trimmed detail when there is no error",
			detail: "  lonely detail  \n",
			err:    nil,
			want:   "lonely detail",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := engineError(tc.detail, tc.err); got != tc.want {
				t.Errorf("engineError = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestActionErrTitle(t *testing.T) {
	cases := map[string]string{
		"merge":   "Merge failed",
		"add":     "Add failed",
		"rm":      "Remove failed",
		"forget":  "Forget failed",
		"unknown": "Action failed",
	}
	for sub, want := range cases {
		if got := actionErrTitle([]string{sub, "branch"}); got != want {
			t.Errorf("actionErrTitle(%q) = %q, want %q", sub, got, want)
		}
	}
	if got := actionErrTitle(nil); got != "Action failed" {
		t.Errorf("actionErrTitle(nil) = %q, want %q", got, "Action failed")
	}
}
