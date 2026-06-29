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

// ResolveProjectByCwd returns the registered project whose root path equals cwd
// or is an ancestor of it, choosing the longest (innermost) match. It mirrors
// ResolveByCwd over Projects so an agent at a project's base checkout (the repo
// root, which is not itself a worktree) still resolves to its project.
//
// Worktrees live as siblings of the root, never inside it, so a worktree never
// matches here; callers still try ResolveByCwd first and only fall back to this.
func ResolveProjectByCwd(store *registry.Store, cwd string) *registry.Project {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	var best *registry.Project
	bestLen := -1
	for i := range store.Projects {
		p := store.Projects[i].Path
		if p == "" {
			continue
		}
		if abs == p || strings.HasPrefix(abs, p+string(os.PathSeparator)) {
			if len(p) > bestLen {
				best = &store.Projects[i]
				bestLen = len(p)
			}
		}
	}
	return best
}
