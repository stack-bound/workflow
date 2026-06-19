// Package launcher adapts "open a workspace" to the environment. The universal
// backend handles copy-to-clipboard in any terminal; the tmux window backend
// jumps to a workspace's window. Opening a workspace in an editor lives in the
// ide package and the "wf edit" command / dashboard picker.
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
