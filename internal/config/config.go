// Package config handles WorkFlow's global configuration, per-repo
// .workFlow.yaml files, and XDG path resolution.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// appDir is the directory name used under the XDG config root.
const appDir = "workFlow"

// Global is the user-wide configuration, stored at <configdir>/config.yaml.
type Global struct {
	// Editor used by "open"/open-in-editor. Falls back to $VISUAL, $EDITOR,
	// then an auto-detected editor when empty.
	Editor string `yaml:"editor,omitempty"`
	// ClipboardCmd, when set, is run as `sh -c <cmd>` with the path on stdin
	// to copy a workspace path. When empty, a built-in clipboard is used.
	ClipboardCmd string `yaml:"clipboard_cmd,omitempty"`
	// DefaultBase is the fallback base branch when neither the CLI nor the
	// per-repo config specifies one.
	DefaultBase string `yaml:"default_base,omitempty"`
	// WorktreeDir is a default base directory for worktrees. When empty a
	// sibling directory ("<repo>_worktrees") is used.
	WorktreeDir string `yaml:"worktree_dir,omitempty"`
}

// Repo is the optional per-repository configuration read from .workFlow.yaml
// at the repo root.
type Repo struct {
	// WorktreeDir overrides where this repo's worktrees are created.
	WorktreeDir string `yaml:"worktree_dir,omitempty"`
	// Base is the default base branch for new workspaces in this repo.
	Base string `yaml:"base,omitempty"`
	// Setup commands are run (via sh -c) in each new worktree after creation.
	Setup []string `yaml:"setup,omitempty"`
	// Copy lists repo-root-relative files copied into each new worktree.
	Copy []string `yaml:"copy,omitempty"`
	// Symlink lists repo-root-relative files symlinked into each new worktree.
	Symlink []string `yaml:"symlink,omitempty"`
}

// Dir returns the WorkFlow config directory, creating it if necessary.
func Dir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	dir := filepath.Join(root, appDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// GlobalPath returns the path to the global config file.
func GlobalPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// RegistryPath returns the path to the JSON registry store.
func RegistryPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "registry.json"), nil
}

// LoadGlobal reads the global config, returning defaults when it is absent.
func LoadGlobal() (*Global, error) {
	path, err := GlobalPath()
	if err != nil {
		return nil, err
	}
	g := &Global{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return g, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}
	if err := yaml.Unmarshal(data, g); err != nil {
		return nil, fmt.Errorf("parse global config: %w", err)
	}
	return g, nil
}

// SaveGlobal writes the global config to disk.
func SaveGlobal(g *Global) error {
	path, err := GlobalPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(g)
	if err != nil {
		return fmt.Errorf("encode global config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}
	return nil
}

// RepoConfigPath returns the path to a repo's .workFlow.yaml.
func RepoConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".workFlow.yaml")
}

// LoadRepo reads .workFlow.yaml from a repo root, returning an empty config
// when the file is absent.
func LoadRepo(repoRoot string) (*Repo, error) {
	r := &Repo{}
	data, err := os.ReadFile(RepoConfigPath(repoRoot))
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read repo config: %w", err)
	}
	if err := yaml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse repo config: %w", err)
	}
	return r, nil
}

// ResolveEditor returns the editor command to use, honoring config then the
// standard environment variables, then a detected fallback.
func (g *Global) ResolveEditor() string {
	if g.Editor != "" {
		return g.Editor
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	for _, cand := range []string{"code", "vim", "vi", "nano"} {
		if _, err := exec.LookPath(cand); err == nil {
			return cand
		}
	}
	return "vi"
}

// ExampleRepoYAML returns a documented example .workFlow.yaml with base as the
// default base branch (detected at init time; the user can edit it later).
func ExampleRepoYAML(base string) string {
	return fmt.Sprintf(`# .workFlow.yaml — per-repository WorkFlow settings.
# All fields are optional.

# Default base branch for new workspaces in this repo (detected at init; edit freely).
base: %s

# Where worktrees for this repo are created.
# Default: a sibling directory "<repo>_worktrees".
# worktree_dir: ../myrepo_worktrees

# Commands run (via sh -c) inside each new worktree after it is created.
setup:
  # - npm install

# Repo-root-relative files copied into each new worktree.
copy:
  # - .env.example

# Repo-root-relative files symlinked into each new worktree.
symlink:
  # - .env
`, base)
}
