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
	"github.com/mattnelsonuk/workflow/internal/config"
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

// OpenInEditor opens path in the resolved editor.
func (u *Universal) OpenInEditor(path string) error {
	editor := u.cfg.ResolveEditor()
	// Support editors configured with arguments, e.g. "code -n".
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	c := exec.Command(parts[0], args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("open editor %q: %w", editor, err)
	}
	return nil
}
