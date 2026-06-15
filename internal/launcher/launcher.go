// Package launcher adapts "open a workspace" to the environment. M1 ships the
// universal backend (path / copy-to-clipboard / open-in-editor) that works in
// any terminal; the tmux window backend arrives in M3.
package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/stack-bound/workflow/internal/config"
)

// Universal is the no-tmux launcher backend.
type Universal struct {
	cfg *config.Global
}

// NewUniversal returns a universal launcher using the given global config.
func NewUniversal(cfg *config.Global) *Universal {
	return &Universal{cfg: cfg}
}

// CopyPath copies a workspace path to the clipboard, honoring a configured
// clipboard_cmd when present, otherwise the built-in clipboard.
func (u *Universal) CopyPath(path string) error {
	if cmd := u.cfg.ClipboardCmd; cmd != "" {
		c := exec.Command("sh", "-c", cmd)
		c.Stdin = strings.NewReader(path)
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("clipboard command failed: %w", err)
		}
		return nil
	}
	if err := clipboard.WriteAll(path); err != nil {
		return fmt.Errorf("copy to clipboard failed (set clipboard_cmd or install xclip/xsel/wl-clipboard): %w", err)
	}
	return nil
}

// EditorCommand builds (without running) the command to open path in the
// resolved editor. The CLI runs it directly; the TUI runs it via
// tea.ExecProcess so a terminal editor can take over the screen cleanly.
func (u *Universal) EditorCommand(path string) *exec.Cmd {
	editor := u.cfg.ResolveEditor()
	// Support editors configured with arguments, e.g. "code -n".
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...)
}

// OpenInEditor opens path in the resolved editor.
func (u *Universal) OpenInEditor(path string) error {
	c := u.EditorCommand(path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("open editor %q: %w", u.cfg.ResolveEditor(), err)
	}
	return nil
}
