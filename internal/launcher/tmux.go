package launcher

import (
	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/tmux"
)

// IdleName builds the initial window name for a wf-opened workspace: the idle
// (branch) status glyph prefixed to the branch. Every wf window therefore
// carries an icon from creation, so the tab layout never shifts when an agent
// later flips it to working/waiting and back.
func IdleName(g *config.Global, branch string) string {
	look := g.StatusLook()
	l := look.Look["idle"]
	return tmux.WindowName(l.Glyph, branch, look.ColorMode, l.Color)
}

// Tmux is the tmux-window launcher backend, used when WorkFlow runs inside a
// tmux session. It is a guest: it creates and selects real windows in the
// user's current session (one per workspace) and never owns or bootstraps a
// session of its own. Windows are matched by the worktree path tag, so these
// operations are safe to call repeatedly.
type Tmux struct{}

// NewTmux returns a tmux launcher backend.
func NewTmux() *Tmux { return &Tmux{} }

// EnsureWindow creates a detached window for the workspace if one is not already
// open, naming it after the branch. It reports whether a window was created.
func (t *Tmux) EnsureWindow(path, name string) (bool, error) {
	w, err := tmux.FindByWorkspace(path)
	if err != nil {
		return false, err
	}
	if w != nil {
		return false, nil
	}
	if _, err := tmux.NewWindow(path, name); err != nil {
		return false, err
	}
	return true, nil
}

// Open jumps to the workspace's window, creating it first when absent.
func (t *Tmux) Open(path, name string) error {
	w, err := tmux.FindByWorkspace(path)
	if err != nil {
		return err
	}
	id := ""
	if w != nil {
		id = w.ID
	} else {
		id, err = tmux.NewWindow(path, name)
		if err != nil {
			return err
		}
	}
	return tmux.SelectWindow(id)
}

// Close kills the workspace's window if one is open, keeping the worktree and
// branch. It reports whether a window was closed.
func (t *Tmux) Close(path string) (bool, error) {
	w, err := tmux.FindByWorkspace(path)
	if err != nil {
		return false, err
	}
	if w == nil {
		return false, nil
	}
	if err := tmux.KillWindow(w.ID); err != nil {
		return false, err
	}
	return true, nil
}
