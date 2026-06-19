package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// isolate points config at a private XDG config root for the test.
func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	return root
}

func TestDirCreatesUnderXDG(t *testing.T) {
	root := isolate(t)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, appDir)
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("Dir() did not create %q: %v", dir, err)
	}
}

func TestGlobalAndRegistryPaths(t *testing.T) {
	root := isolate(t)
	gp, err := GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, appDir, "config.yaml"); gp != want {
		t.Errorf("GlobalPath() = %q, want %q", gp, want)
	}
	rp, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, appDir, "registry.json"); rp != want {
		t.Errorf("RegistryPath() = %q, want %q", rp, want)
	}
}

func TestLoadGlobalMissingReturnsDefaults(t *testing.T) {
	isolate(t)
	g, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || g.DefaultIDE != "" || g.DefaultBase != "" {
		t.Errorf("expected zero-value Global, got %+v", g)
	}
}

func TestSaveLoadGlobalRoundTrip(t *testing.T) {
	isolate(t)
	in := &Global{ClipboardCmd: "pbcopy", DefaultBase: "trunk", WorktreeDir: "/wt", DefaultIDE: "goland"}
	if err := SaveGlobal(in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if got.ClipboardCmd != in.ClipboardCmd || got.DefaultBase != in.DefaultBase ||
		got.WorktreeDir != in.WorktreeDir || got.DefaultIDE != in.DefaultIDE {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, in)
	}
}

// A stale "editor" key from before the IDE model is silently ignored on load
// (the field no longer exists) and is dropped the next time we save.
func TestLoadGlobalIgnoresStaleEditorKey(t *testing.T) {
	root := isolate(t)
	if _, err := Dir(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, appDir, "config.yaml")
	if err := os.WriteFile(path, []byte("editor: vim\ndefault_base: trunk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if g.DefaultBase != "trunk" {
		t.Errorf("default_base = %q, want trunk", g.DefaultBase)
	}
	if err := SaveGlobal(g); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "editor:") {
		t.Errorf("stale editor key survived a save: %s", data)
	}
}

func TestLoadGlobalCustomIDEs(t *testing.T) {
	root := isolate(t)
	if _, err := Dir(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, appDir, "config.yaml")
	yamlText := "ides:\n  - id: myed\n    name: My Editor\n    cmd: myed --flag\n    gui: true\n"
	if err := os.WriteFile(path, []byte(yamlText), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.IDEs) != 1 || g.IDEs[0].ID != "myed" || g.IDEs[0].Cmd != "myed --flag" || !g.IDEs[0].GUI {
		t.Errorf("custom ides parsed wrong: %+v", g.IDEs)
	}
}

func TestLoadGlobalParseError(t *testing.T) {
	root := isolate(t)
	if _, err := Dir(); err != nil { // ensure dir exists
		t.Fatal(err)
	}
	path := filepath.Join(root, appDir, "config.yaml")
	if err := os.WriteFile(path, []byte("editor: [unterminated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGlobal(); err == nil {
		t.Error("expected parse error for malformed config")
	}
}

func TestRepoConfigPath(t *testing.T) {
	if got := RepoConfigPath("/repo"); got != filepath.Join("/repo", ".workFlow.yaml") {
		t.Errorf("RepoConfigPath = %q", got)
	}
}

func TestLoadRepoMissingReturnsEmpty(t *testing.T) {
	r, err := LoadRepo(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || len(r.Setup) != 0 || r.Base != "" {
		t.Errorf("expected empty Repo, got %+v", r)
	}
}

func TestLoadRepoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	yamlText := "base: develop\nworktree_dir: ../wt\nsetup:\n  - npm install\ncopy:\n  - .env.example\nsymlink:\n  - .env\n"
	if err := os.WriteFile(RepoConfigPath(dir), []byte(yamlText), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Base != "develop" || r.WorktreeDir != "../wt" {
		t.Errorf("scalar fields wrong: %+v", r)
	}
	if len(r.Setup) != 1 || r.Setup[0] != "npm install" {
		t.Errorf("setup = %+v", r.Setup)
	}
	if len(r.Copy) != 1 || len(r.Symlink) != 1 {
		t.Errorf("copy/symlink = %+v / %+v", r.Copy, r.Symlink)
	}
}

func TestLoadRepoParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(RepoConfigPath(dir), []byte("setup: [bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepo(dir); err == nil {
		t.Error("expected parse error for malformed repo config")
	}
}

func TestSetRepoIDECreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := SetRepoIDE(dir, "goland", true); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.DefaultIDE != "goland" || !r.Autolaunch {
		t.Errorf("after create: %+v", r)
	}
}

func TestSetRepoIDEPreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	// A hand-written config with a comment and unrelated settings.
	yamlText := "# my repo config\nbase: develop\nsetup:\n  - npm install\n"
	if err := os.WriteFile(RepoConfigPath(dir), []byte(yamlText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetRepoIDE(dir, "code", false); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Base != "develop" || len(r.Setup) != 1 || r.Setup[0] != "npm install" {
		t.Errorf("existing settings lost: %+v", r)
	}
	if r.DefaultIDE != "code" || r.Autolaunch {
		t.Errorf("ide settings wrong: %+v", r)
	}
	data, err := os.ReadFile(RepoConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# my repo config") {
		t.Errorf("comment did not survive: %s", data)
	}
}

func TestSetRepoIDEUpdatesInPlace(t *testing.T) {
	dir := t.TempDir()
	if err := SetRepoIDE(dir, "code", true); err != nil {
		t.Fatal(err)
	}
	if err := SetRepoIDE(dir, "goland", false); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.DefaultIDE != "goland" || r.Autolaunch {
		t.Errorf("update in place wrong: %+v", r)
	}
	data, err := os.ReadFile(RepoConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	// The key should appear exactly once, not be duplicated on update.
	if n := strings.Count(string(data), "default_ide:"); n != 1 {
		t.Errorf("default_ide appears %d times, want 1: %s", n, data)
	}
}

func TestExampleRepoYAMLIsValid(t *testing.T) {
	out := ExampleRepoYAML("development")
	var r Repo
	if err := yaml.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("example YAML does not parse: %v", err)
	}
	if r.Base != "development" {
		t.Errorf("example base = %q, want development", r.Base)
	}
	if !strings.Contains(out, RepoURL) {
		t.Errorf("example YAML missing back-reference to %q", RepoURL)
	}
}
