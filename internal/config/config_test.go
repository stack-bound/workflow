package config

import (
	"os"
	"path/filepath"
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
	if g == nil || g.Editor != "" || g.DefaultBase != "" {
		t.Errorf("expected zero-value Global, got %+v", g)
	}
}

func TestSaveLoadGlobalRoundTrip(t *testing.T) {
	isolate(t)
	in := &Global{Editor: "code -n", ClipboardCmd: "pbcopy", DefaultBase: "trunk", WorktreeDir: "/wt"}
	if err := SaveGlobal(in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if *got != *in {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, in)
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

func TestResolveEditor(t *testing.T) {
	t.Run("config wins", func(t *testing.T) {
		t.Setenv("VISUAL", "vis")
		t.Setenv("EDITOR", "ed")
		g := &Global{Editor: "myeditor"}
		if got := g.ResolveEditor(); got != "myeditor" {
			t.Errorf("got %q, want myeditor", got)
		}
	})
	t.Run("VISUAL over EDITOR", func(t *testing.T) {
		t.Setenv("VISUAL", "vis")
		t.Setenv("EDITOR", "ed")
		g := &Global{}
		if got := g.ResolveEditor(); got != "vis" {
			t.Errorf("got %q, want vis", got)
		}
	})
	t.Run("EDITOR when no VISUAL", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "ed")
		g := &Global{}
		if got := g.ResolveEditor(); got != "ed" {
			t.Errorf("got %q, want ed", got)
		}
	})
	t.Run("fallback never empty", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		g := &Global{}
		if got := g.ResolveEditor(); got == "" {
			t.Error("ResolveEditor fallback returned empty")
		}
	})
}

func TestExampleRepoYAMLIsValid(t *testing.T) {
	var r Repo
	if err := yaml.Unmarshal([]byte(ExampleRepoYAML()), &r); err != nil {
		t.Fatalf("example YAML does not parse: %v", err)
	}
	if r.Base != "main" {
		t.Errorf("example base = %q, want main", r.Base)
	}
}
