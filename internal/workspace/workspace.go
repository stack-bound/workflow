// Package workspace orchestrates the worktree lifecycle, tying together git,
// the registry, and per-repo config. It is the engine the CLI (and later the
// dashboard and agents) drive.
package workspace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
)

// Manager is the workspace engine.
type Manager struct {
	registryPath string
	global       *config.Global
}

// New returns a Manager backed by the given registry path and global config.
func New(registryPath string, global *config.Global) *Manager {
	return &Manager{registryPath: registryPath, global: global}
}

// AddOptions configures creating a workspace.
type AddOptions struct {
	Branch  string // branch name (also drives the worktree dir slug)
	Project string // project name; empty means infer from cwd
	Base    string // base branch; empty means resolve from config/git
	NoSetup bool   // skip per-repo setup commands and file copy/symlink
}

// View joins a registered worktree with its live git status.
type View struct {
	Worktree registry.Worktree
	Stat     git.Stat
	StatErr  error
}

// ErrUnregisteredRepo is returned when project resolution lands inside a git
// repository whose root is not registered as a project. It carries the repo
// root so callers (the CLI) can offer to register it. This is distinct from
// "not in a git repo at all", which stays a plain error.
type ErrUnregisteredRepo struct {
	Root string
}

func (e *ErrUnregisteredRepo) Error() string {
	return fmt.Sprintf("git repo %s is not registered as a project", e.Root)
}

// Active reports whether a workspace is still in play: dirty, ahead of base,
// or carrying an open PR. The inverse (clean + merged) is cleanable.
func (v View) Active() bool {
	return v.Stat.Dirty || v.Stat.Ahead > 0 || v.Worktree.PRRef != ""
}

// Add creates a branch + worktree, runs setup, copies/symlinks files, and
// registers the workspace.
func (m *Manager) Add(opts AddOptions) (*registry.Worktree, error) {
	if opts.Branch == "" {
		return nil, fmt.Errorf("a branch name is required")
	}

	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	proj, err := m.resolveProject(store, opts.Project)
	if err != nil {
		return nil, err
	}

	repoCfg, err := config.LoadRepo(proj.Path)
	if err != nil {
		return nil, err
	}

	base := m.resolveBase(opts.Base, repoCfg, proj.Path)
	path, err := m.worktreePath(proj, repoCfg, opts.Branch)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("worktree path already exists: %s", path)
	}

	newBranch := !git.BranchExists(proj.Path, opts.Branch)
	// When starting a new branch, the base is git's start-point. Vet it first so
	// a stale/renamed default surfaces as a clear, actionable message instead of
	// git's raw "fatal: invalid reference: <base>".
	if newBranch && !git.RefExists(proj.Path, base) {
		avail := ""
		if branches, err := git.LocalBranches(proj.Path); err == nil && len(branches) > 0 {
			avail = " (available: " + strings.Join(branches, ", ") + ")"
		}
		return nil, fmt.Errorf("base branch %q does not exist in project %q%s; set 'base:' in .workFlow.yaml or pass --base=<branch>", base, proj.Name, avail)
	}
	if err := git.WorktreeAdd(proj.Path, path, opts.Branch, base, newBranch); err != nil {
		return nil, err
	}

	if !opts.NoSetup {
		if err := applyFileOps(proj.Path, path, repoCfg); err != nil {
			return nil, fmt.Errorf("worktree created but file setup failed: %w", err)
		}
		if err := runSetup(path, repoCfg.Setup); err != nil {
			return nil, fmt.Errorf("worktree created but setup commands failed: %w", err)
		}
	}

	wt := registry.Worktree{
		Project:   proj.Name,
		Path:      path,
		Branch:    opts.Branch,
		Base:      base,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := registry.WithLock(m.registryPath, func(s *registry.Store) error {
		return s.AddWorktree(wt)
	}); err != nil {
		return nil, fmt.Errorf("worktree created but registration failed: %w", err)
	}
	return &wt, nil
}

// ProjectView groups a registered project with its workspace views. It is the
// shape the dashboard's project → worktree tree renders.
type ProjectView struct {
	Project    registry.Project
	Main       MainCheckout
	Workspaces []View
}

// MainCheckout is the live status of a project's primary checkout — the repo at
// the project root. The dashboard renders it as the project's own row so the
// base branch can be opened (a tmux window or editor) without first carving off
// a worktree. Unlike a worktree it has no base to diff against, so it carries
// only the branch it is on and whether its working tree is dirty.
type MainCheckout struct {
	Path   string // the project root (== Project.Path)
	Branch string // branch currently checked out at the root
	Dirty  bool   // uncommitted changes in the root checkout
	Err    error  // status couldn't be derived (path missing, not a repo, …)
}

// mainCheckoutFor derives the live status of a project's root checkout. A
// missing path or a non-repo root degrades to a populated Err rather than an
// error return, so one broken project never blanks the whole ledger.
func mainCheckoutFor(path string) MainCheckout {
	mc := MainCheckout{Path: path}
	if _, err := os.Stat(path); err != nil {
		mc.Err = fmt.Errorf("project path missing")
		return mc
	}
	if !git.IsRepo(path) {
		mc.Err = fmt.Errorf("not a git repository")
		return mc
	}
	branch, err := git.CurrentBranch(path)
	if err != nil {
		mc.Err = err
		return mc
	}
	mc.Branch = branch
	if dirty, err := git.Dirty(path); err == nil {
		mc.Dirty = dirty
	}
	return mc
}

// viewFor joins a worktree with its live git status.
func viewFor(w registry.Worktree) View {
	v := View{Worktree: w}
	if _, err := os.Stat(w.Path); err == nil {
		v.Stat, v.StatErr = git.Stats(w.Path, w.Base)
	} else {
		v.StatErr = fmt.Errorf("worktree path missing")
	}
	return v
}

// List returns every registered workspace joined with live git status.
func (m *Manager) List() ([]View, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(store.Worktrees))
	for _, w := range store.Worktrees {
		views = append(views, viewFor(w))
	}
	return views, nil
}

// Ledger returns every registered project with its workspaces (live status),
// including projects that currently have no workspaces.
func (m *Manager) Ledger() ([]ProjectView, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectView, 0, len(store.Projects))
	for _, p := range store.Projects {
		pv := ProjectView{Project: p, Main: mainCheckoutFor(p.Path)}
		for _, w := range store.WorktreesForProject(p.Name) {
			pv.Workspaces = append(pv.Workspaces, viewFor(w))
		}
		out = append(out, pv)
	}
	return out, nil
}

// MainDiff returns the uncommitted diff of a project's root checkout — the
// working-tree changes the base row would discard if launched-and-edited. It
// diffs against HEAD (the checkout's own tip) rather than a base branch, since
// the trunk has no base of its own.
func (m *Manager) MainDiff(project string) (string, error) {
	root, err := m.ProjectRoot(project)
	if err != nil {
		return "", err
	}
	return git.Diff(root, "HEAD")
}

// Diff returns the cumulative diff of a workspace against its base branch.
func (m *Manager) Diff(ref, projectFlag string) (string, error) {
	wt, err := m.Resolve(ref, projectFlag)
	if err != nil {
		return "", err
	}
	return git.Diff(wt.Path, wt.Base)
}

// Path resolves a workspace reference to its filesystem path.
func (m *Manager) Path(ref, projectFlag string) (string, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return "", err
	}
	wt, err := m.resolveWorktree(store, ref, projectFlag)
	if err != nil {
		return "", err
	}
	return wt.Path, nil
}

// ProjectRoot returns the filesystem root of a registered project by name. It
// is where that project's .workFlow.yaml (and thus its editor preferences)
// live, shared by all of the project's worktrees.
func (m *Manager) ProjectRoot(name string) (string, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return "", err
	}
	p := store.FindProject(name)
	if p == nil {
		return "", fmt.Errorf("no project named %q", name)
	}
	return p.Path, nil
}

// RenameProject changes a registered project's name, retargeting all of its
// worktrees so the registry stays consistent. The repo on disk and the
// project's worktree directories are untouched — only the registry name moves.
func (m *Manager) RenameProject(oldName, newName string) error {
	return registry.WithLock(m.registryPath, func(s *registry.Store) error {
		return s.RenameProject(oldName, newName)
	})
}

// RemoveProject unregisters a project. Without force it refuses while the
// project still has worktrees; with force it also drops those worktree
// registrations. Like `wf project rm`, it never touches the repo or worktree
// directories on disk — it only edits the registry.
func (m *Manager) RemoveProject(name string, force bool) error {
	return registry.WithLock(m.registryPath, func(s *registry.Store) error {
		return s.RemoveProject(name, force)
	})
}

// ProjectForDir returns the project that owns dir: the worktree containing dir,
// else the registered project whose tree contains dir. Returns "" when dir
// belongs to no known project (e.g. an unregistered directory).
func (m *Manager) ProjectForDir(dir string) (string, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return "", err
	}
	if w := store.WorktreeByPath(dir); w != nil {
		return w.Project, nil
	}
	if p, err := m.resolveProject(store, ""); err == nil {
		return p.Name, nil
	}
	return "", nil
}

// ProjectIDEPrefs returns a project's configured default editor id and whether
// to autolaunch it, read from the project's .workFlow.yaml.
func (m *Manager) ProjectIDEPrefs(project string) (defaultID string, autolaunch bool, err error) {
	root, err := m.ProjectRoot(project)
	if err != nil {
		return "", false, err
	}
	rc, err := config.LoadRepo(root)
	if err != nil {
		return "", false, err
	}
	return rc.DefaultIDE, rc.Autolaunch, nil
}

// SetProjectDefaultIDE records a project's default editor and autolaunch flag in
// its .workFlow.yaml.
func (m *Manager) SetProjectDefaultIDE(project, ideID string, autolaunch bool) error {
	root, err := m.ProjectRoot(project)
	if err != nil {
		return err
	}
	return config.SetRepoIDE(root, ideID, autolaunch)
}

// Resolve returns the worktree matching ref (and optional project).
func (m *Manager) Resolve(ref, projectFlag string) (*registry.Worktree, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	return m.resolveWorktree(store, ref, projectFlag)
}

// Remove removes a workspace: its worktree, its branch, and its registration.
func (m *Manager) Remove(ref, projectFlag string, force bool) (*registry.Worktree, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	wt, err := m.resolveWorktree(store, ref, projectFlag)
	if err != nil {
		return nil, err
	}
	proj := store.FindProject(wt.Project)
	if proj == nil {
		return nil, fmt.Errorf("project %q for workspace not found", wt.Project)
	}

	if err := git.WorktreeRemove(proj.Path, wt.Path, force); err != nil {
		return nil, err
	}
	if git.BranchExists(proj.Path, wt.Branch) {
		if err := git.DeleteBranch(proj.Path, wt.Branch, force); err != nil {
			return nil, fmt.Errorf("worktree removed but branch deletion failed: %w", err)
		}
	}
	if err := registry.WithLock(m.registryPath, func(s *registry.Store) error {
		s.RemoveWorktree(wt.Path)
		return nil
	}); err != nil {
		return nil, err
	}
	// The worktree is gone; drop its agent-status file too (best-effort).
	_ = status.Remove(wt.Project, wt.Branch, wt.Path)
	return wt, nil
}

// Merge merges a workspace's branch into its base, then removes the worktree,
// deletes the branch, and unregisters it.
func (m *Manager) Merge(ref, projectFlag string) (*registry.Worktree, error) {
	store, err := registry.Load(m.registryPath)
	if err != nil {
		return nil, err
	}
	wt, err := m.resolveWorktree(store, ref, projectFlag)
	if err != nil {
		return nil, err
	}
	proj := store.FindProject(wt.Project)
	if proj == nil {
		return nil, fmt.Errorf("project %q for workspace not found", wt.Project)
	}

	// Refuse before touching base if the workspace has uncommitted work, so a
	// merge is all-or-nothing rather than merging history then failing to clean
	// up a dirty worktree.
	if st, err := git.Stats(wt.Path, wt.Base); err == nil && st.Dirty {
		return nil, fmt.Errorf("workspace %s/%s has uncommitted changes; commit or stash before merging", wt.Project, wt.Branch)
	}

	if err := git.Merge(proj.Path, wt.Base, wt.Branch); err != nil {
		return nil, err
	}
	if err := git.WorktreeRemove(proj.Path, wt.Path, false); err != nil {
		return nil, fmt.Errorf("merged, but removing the worktree failed: %w", err)
	}
	if err := git.DeleteBranch(proj.Path, wt.Branch, false); err != nil {
		return nil, fmt.Errorf("merged and worktree removed, but deleting the branch failed: %w", err)
	}
	if err := registry.WithLock(m.registryPath, func(s *registry.Store) error {
		s.RemoveWorktree(wt.Path)
		return nil
	}); err != nil {
		return nil, err
	}
	// The worktree is gone; drop its agent-status file too (best-effort).
	_ = status.Remove(wt.Project, wt.Branch, wt.Path)
	return wt, nil
}

// resolveProject finds the project by flag, else infers it from the cwd.
func (m *Manager) resolveProject(store *registry.Store, projectFlag string) (*registry.Project, error) {
	if projectFlag != "" {
		p := store.FindProject(projectFlag)
		if p == nil {
			return nil, fmt.Errorf("no project named %q (see: wf project ls)", projectFlag)
		}
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, rootErr := git.RepoRoot(cwd)
	if rootErr == nil {
		if p := store.ProjectByPath(root); p != nil {
			return p, nil
		}
	}
	// Fall back to an ancestor match (cwd inside a registered project tree).
	abs, _ := filepath.Abs(cwd)
	for i := range store.Projects {
		if abs == store.Projects[i].Path || strings.HasPrefix(abs, store.Projects[i].Path+string(os.PathSeparator)) {
			return &store.Projects[i], nil
		}
	}
	// Inside a git repo, just not registered: signal precisely so the CLI can
	// offer to register it. Outside any repo, stay a plain error.
	if rootErr == nil {
		return nil, &ErrUnregisteredRepo{Root: root}
	}
	return nil, fmt.Errorf("could not determine project; run inside a registered project or pass --project")
}

// resolveWorktree matches a workspace by branch, optionally scoped to project.
func (m *Manager) resolveWorktree(store *registry.Store, ref, projectFlag string) (*registry.Worktree, error) {
	if ref == "" {
		return nil, fmt.Errorf("a workspace (branch) name is required")
	}
	matches := store.FindWorktrees(ref, projectFlag)
	switch len(matches) {
	case 1:
		w := matches[0]
		return &w, nil
	case 0:
		return nil, fmt.Errorf("no workspace with branch %q", ref)
	default:
		var names []string
		for _, w := range matches {
			names = append(names, w.Project+"/"+w.Branch)
		}
		return nil, fmt.Errorf("workspace %q is ambiguous (%s); use --project", ref, strings.Join(names, ", "))
	}
}

// resolveBase picks the base branch: explicit flag, then per-repo, then global,
// then the repo's detected default branch.
func (m *Manager) resolveBase(flag string, repoCfg *config.Repo, repoPath string) string {
	if flag != "" {
		return flag
	}
	if repoCfg.Base != "" {
		return repoCfg.Base
	}
	if m.global.DefaultBase != "" {
		return m.global.DefaultBase
	}
	if b, err := git.DefaultBranch(repoPath); err == nil {
		return b
	}
	// Last resort: prefer whatever is checked out over a literal "main", which
	// may not exist in this repo.
	if b, err := git.CurrentBranch(repoPath); err == nil && b != "" {
		return b
	}
	return "main"
}

// worktreePath computes where a new worktree should live.
func (m *Manager) worktreePath(proj *registry.Project, repoCfg *config.Repo, branch string) (string, error) {
	slug := Slug(branch)
	if slug == "" {
		return "", fmt.Errorf("branch %q produces an empty directory name", branch)
	}

	baseDir := repoCfg.WorktreeDir
	if baseDir == "" {
		baseDir = m.global.WorktreeDir
	}
	if baseDir == "" {
		// Sibling directory: <parent>/<repo>_worktrees
		parent := filepath.Dir(proj.Path)
		baseDir = filepath.Join(parent, filepath.Base(proj.Path)+"_worktrees")
	} else if !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(proj.Path, baseDir)
	}
	return filepath.Join(baseDir, slug), nil
}

// Slug converts a branch name into a filesystem-safe directory name.
func Slug(branch string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(branch) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// runSetup runs each setup command via sh -c in the worktree, streaming output.
func runSetup(dir string, cmds []string) error {
	for _, cmd := range cmds {
		if strings.TrimSpace(cmd) == "" {
			continue
		}
		c := exec.Command("sh", "-c", cmd)
		c.Dir = dir
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("setup command %q: %w", cmd, err)
		}
	}
	return nil
}

// applyFileOps copies and symlinks configured files from the repo root into a
// new worktree.
func applyFileOps(repoRoot, worktree string, repoCfg *config.Repo) error {
	for _, rel := range repoCfg.Copy {
		if err := copyInto(repoRoot, worktree, rel); err != nil {
			return err
		}
	}
	for _, rel := range repoCfg.Symlink {
		src := filepath.Join(repoRoot, rel)
		dst := filepath.Join(worktree, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return fmt.Errorf("symlink %s: %w", rel, err)
		}
	}
	return nil
}

func copyInto(repoRoot, worktree, rel string) error {
	srcPath := filepath.Join(repoRoot, rel)
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("copy %s: %w", rel, err)
	}
	defer func() { _ = src.Close() }()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	dstPath := filepath.Join(worktree, rel)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("copy %s: %w", rel, err)
	}
	// Surface a close error: for the write target it can report a flush failure
	// that the copy itself did not see.
	if err := dst.Close(); err != nil {
		return fmt.Errorf("copy %s: %w", rel, err)
	}
	return nil
}
