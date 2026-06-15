package status

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/stack-bound/workflow/internal/registry"
)

// ResolveByCwd returns the registered worktree whose path equals cwd or is an
// ancestor of it, choosing the longest (innermost) match when worktrees nest.
// It returns nil when cwd is not inside any registered worktree.
//
// This is how `wf set-status` figures out which workspace a Claude Code hook
// belongs to: the hook runs in the session's cwd, which is inside the worktree.
// It is pure path matching against the registry — no git shell-out — so it is
// cheap enough to run on every tool call.
func ResolveByCwd(store *registry.Store, cwd string) *registry.Worktree {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	var best *registry.Worktree
	bestLen := -1
	for i := range store.Worktrees {
		p := store.Worktrees[i].Path
		if abs == p || strings.HasPrefix(abs, p+string(os.PathSeparator)) {
			if len(p) > bestLen {
				best = &store.Worktrees[i]
				bestLen = len(p)
			}
		}
	}
	return best
}
