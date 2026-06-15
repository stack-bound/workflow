// Package registry is WorkFlow's durable store: the only persisted truth is
// the set of registered projects and worktrees. Everything else (git stats,
// open windows, agent status) is derived live elsewhere.
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Version is the current on-disk schema version.
const Version = 1

// Project is a git repo registered on this host.
type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	AddedAt string `json:"added_at"`
}

// Worktree is a git worktree on its own branch, owned by a Project.
type Worktree struct {
	Project    string `json:"project"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	Base       string `json:"base"`
	CreatedAt  string `json:"created_at"`
	PRRef      string `json:"pr_ref,omitempty"`
	StatusSlot string `json:"status_slot,omitempty"`
}

// Store is the full persisted registry.
type Store struct {
	Version   int        `json:"version"`
	Projects  []Project  `json:"projects"`
	Worktrees []Worktree `json:"worktrees"`
}

// Load reads the registry from path, returning an empty store if it is absent.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Store{Version: Version}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	if s.Version == 0 {
		s.Version = Version
	}
	return &s, nil
}

// Save atomically writes the store to path (temp file + rename).
func Save(path string, s *Store) error {
	s.Version = Version
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp registry: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp registry: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit registry: %w", err)
	}
	return nil
}

// WithLock runs fn against the registry under an exclusive advisory lock,
// loading before and saving after. It serializes concurrent CLI invocations
// so load-modify-save cycles do not clobber each other.
func WithLock(path string, fn func(*Store) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open registry lock: %w", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	s, err := Load(path)
	if err != nil {
		return err
	}
	if err := fn(s); err != nil {
		return err
	}
	return Save(path, s)
}

// FindProject returns the project with the given name, or nil.
func (s *Store) FindProject(name string) *Project {
	for i := range s.Projects {
		if s.Projects[i].Name == name {
			return &s.Projects[i]
		}
	}
	return nil
}

// ProjectByPath returns the project at the given absolute path, or nil.
func (s *Store) ProjectByPath(path string) *Project {
	for i := range s.Projects {
		if s.Projects[i].Path == path {
			return &s.Projects[i]
		}
	}
	return nil
}

// AddProject registers a project. It errors on a duplicate name or path.
func (s *Store) AddProject(p Project) error {
	if s.FindProject(p.Name) != nil {
		return fmt.Errorf("project %q already registered", p.Name)
	}
	if s.ProjectByPath(p.Path) != nil {
		return fmt.Errorf("a project is already registered at %s", p.Path)
	}
	s.Projects = append(s.Projects, p)
	return nil
}

// RemoveProject removes a project by name. It errors if the project still has
// worktrees unless force is set.
func (s *Store) RemoveProject(name string, force bool) error {
	if s.FindProject(name) == nil {
		return fmt.Errorf("no project named %q", name)
	}
	if !force && len(s.WorktreesForProject(name)) > 0 {
		return fmt.Errorf("project %q still has worktrees; remove them first or use --force", name)
	}
	out := s.Projects[:0]
	for _, p := range s.Projects {
		if p.Name != name {
			out = append(out, p)
		}
	}
	s.Projects = out
	if force {
		s.removeWorktreesForProject(name)
	}
	return nil
}

// WorktreesForProject returns all worktrees belonging to a project.
func (s *Store) WorktreesForProject(name string) []Worktree {
	var out []Worktree
	for _, w := range s.Worktrees {
		if w.Project == name {
			out = append(out, w)
		}
	}
	return out
}

func (s *Store) removeWorktreesForProject(name string) {
	out := s.Worktrees[:0]
	for _, w := range s.Worktrees {
		if w.Project != name {
			out = append(out, w)
		}
	}
	s.Worktrees = out
}

// AddWorktree registers a worktree. It errors on a duplicate path.
func (s *Store) AddWorktree(w Worktree) error {
	for _, existing := range s.Worktrees {
		if existing.Path == w.Path {
			return fmt.Errorf("a worktree is already registered at %s", w.Path)
		}
	}
	s.Worktrees = append(s.Worktrees, w)
	return nil
}

// RemoveWorktree removes a worktree by path. Returns false if not found.
func (s *Store) RemoveWorktree(path string) bool {
	found := false
	out := s.Worktrees[:0]
	for _, w := range s.Worktrees {
		if w.Path == path {
			found = true
			continue
		}
		out = append(out, w)
	}
	s.Worktrees = out
	return found
}

// FindWorktrees returns worktrees matching branch, optionally scoped to a
// project (pass "" to search all projects).
func (s *Store) FindWorktrees(branch, project string) []Worktree {
	var out []Worktree
	for _, w := range s.Worktrees {
		if w.Branch != branch {
			continue
		}
		if project != "" && w.Project != project {
			continue
		}
		out = append(out, w)
	}
	return out
}
