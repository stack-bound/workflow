// Package config handles WorkFlow's global configuration, per-repo
// .workFlow.yaml files, and XDG path resolution.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// appDir is the directory name used under the XDG config root.
const appDir = "workFlow"

// RepoURL is the home of the WorkFlow (wf) CLI. It is referenced from generated
// .workFlow.yaml files so a reader who finds one can discover the tool.
const RepoURL = "https://github.com/stack-bound/workflow"

// Global is the user-wide configuration, stored at <configdir>/config.yaml.
type Global struct {
	// ClipboardCmd, when set, is run as `sh -c <cmd>` with the path on stdin
	// to copy a workspace path. When empty, a built-in clipboard is used.
	ClipboardCmd string `yaml:"clipboard_cmd,omitempty"`
	// DefaultBase is the fallback base branch when neither the CLI nor the
	// per-repo config specifies one.
	DefaultBase string `yaml:"default_base,omitempty"`
	// WorktreeDir is a default base directory for worktrees. When empty a
	// sibling directory ("<repo>_worktrees") is used.
	WorktreeDir string `yaml:"worktree_dir,omitempty"`
	// IDEs lists user-defined editors that augment the built-in catalog the
	// "wf edit" picker probes for. Use it for an editor wf does not know about.
	IDEs []IDESpec `yaml:"ides,omitempty"`
	// DefaultIDE is the fallback editor id (a catalog or custom id) used by the
	// picker when a repo does not pin its own default_ide.
	DefaultIDE string `yaml:"default_ide,omitempty"`
	// Status tunes the agent-status icons/colors shown in tmux tabs, the
	// dashboard, and the sidebar. Absent means defaults (see StatusConfig).
	Status *StatusConfig `yaml:"status,omitempty"`
}

// IDESpec is a user-defined editor merged into the built-in catalog. Cmd is the
// launch command line (split on spaces; the target directory is appended); GUI
// marks a windowed app that launches detached rather than a terminal editor.
type IDESpec struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name,omitempty"`
	Cmd  string `yaml:"cmd"`
	GUI  bool   `yaml:"gui,omitempty"`
}

// Repo is the optional per-repository configuration read from .workFlow.yaml
// at the repo root.
type Repo struct {
	// WorktreeDir overrides where this repo's worktrees are created.
	WorktreeDir string `yaml:"worktree_dir,omitempty"`
	// Base is the default base branch for new workspaces in this repo.
	Base string `yaml:"base,omitempty"`
	// DefaultIDE is the editor id (catalog or custom) pre-selected by the picker
	// for this repo's workspaces. Set it from the picker or by hand.
	DefaultIDE string `yaml:"default_ide,omitempty"`
	// Autolaunch, when true, makes "wf edit" (and the dashboard's edit key)
	// launch DefaultIDE straight away instead of opening the picker.
	Autolaunch bool `yaml:"autolaunch,omitempty"`
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

// SetRepoIDE records the picker's choice in the repo's .workFlow.yaml: the
// default editor id and whether to autolaunch it. It updates the two keys in
// place via a yaml.Node round-trip so existing settings and comments survive,
// and scaffolds a minimal file when none exists yet.
func SetRepoIDE(repoRoot, defaultIDE string, autolaunch bool) error {
	path := RepoConfigPath(repoRoot)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		content := fmt.Sprintf("# .workFlow.yaml — per-repository WorkFlow settings.\n"+
			"# Created by the wf (WorkFlow) CLI. Learn more: %s\n\n"+
			"default_ide: %s\nautolaunch: %t\n", RepoURL, defaultIDE, autolaunch)
		return os.WriteFile(path, []byte(content), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read repo config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse repo config: %w", err)
	}
	m := documentMapping(&doc)
	setScalar(m, "default_ide", defaultIDE, "!!str")
	setScalar(m, "autolaunch", fmt.Sprintf("%t", autolaunch), "!!bool")

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("encode repo config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write repo config: %w", err)
	}
	return nil
}

// documentMapping returns the mapping node at the root of a parsed YAML
// document, initializing an empty document/mapping when the file was blank.
func documentMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return doc.Content[0]
}

// setScalar sets key to a scalar value in a mapping node, replacing the value
// when the key already exists (keeping its key node, and thus its comments) or
// appending the pair otherwise.
func setScalar(m *yaml.Node, key, value, tag string) {
	val := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			val.HeadComment = m.Content[i+1].HeadComment
			val.LineComment = m.Content[i+1].LineComment
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

// ExampleRepoYAML returns a documented example .workFlow.yaml with base as the
// default base branch (detected at init time; the user can edit it later).
func ExampleRepoYAML(base string) string {
	return fmt.Sprintf(`# .workFlow.yaml — per-repository WorkFlow settings.
# Created by the wf (WorkFlow) CLI. Learn more: %s
# All fields are optional.

# Default base branch for new workspaces in this repo (detected at init; edit freely).
base: %s

# Default editor for "wf edit" in this repo (a catalog or custom id; see: wf edit --list).
# Set it from the picker, or by hand. autolaunch: true opens it without the picker.
# default_ide: goland
# autolaunch: false

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
`, RepoURL, base)
}
