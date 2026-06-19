package ide

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
)

// lookPathSet returns a LookPath stub that "finds" only the named binaries.
func lookPathSet(found ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, f := range found {
		set[f] = true
	}
	return func(bin string) (string, error) {
		if set[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", exec.ErrNotFound
	}
}

func find(t *testing.T, ides []IDE, id string) IDE {
	t.Helper()
	i, ok := Find(ides, id)
	if !ok {
		t.Fatalf("expected to detect %q in %+v", id, ides)
	}
	return i
}

func TestDetectPathLaunchers(t *testing.T) {
	d := &Detector{GOOS: "linux", LookPath: lookPathSet("goland", "vim")}
	ides := d.Detect(nil)

	g := find(t, ides, "goland")
	if !g.GUI || len(g.Exec) != 1 || g.Exec[0] != "goland" {
		t.Errorf("goland detected wrong: %+v", g)
	}
	v := find(t, ides, "vim")
	if v.GUI {
		t.Errorf("vim should be a terminal editor: %+v", v)
	}
	if _, ok := Find(ides, "code"); ok {
		t.Error("code should not be detected when not on PATH")
	}
}

func TestDetectLinuxDesktopFallback(t *testing.T) {
	dir := t.TempDir()
	desktop := "[Desktop Entry]\nName=VSCodium\nExec=codium --unity-launch %F\nType=Application\n"
	if err := os.WriteFile(filepath.Join(dir, "codium.desktop"), []byte(desktop), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &Detector{GOOS: "linux", LookPath: lookPathSet(), DesktopDirs: []string{dir}}
	ides := d.Detect(nil)

	c := find(t, ides, "codium")
	if want := []string{"codium", "--unity-launch"}; len(c.Exec) != 2 || c.Exec[0] != want[0] || c.Exec[1] != want[1] {
		t.Errorf("desktop Exec parsed wrong (field codes should be stripped): %+v", c.Exec)
	}
}

func TestDetectMacApp(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "GoLand.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := &Detector{GOOS: "darwin", LookPath: lookPathSet(), AppDirs: []string{dir}}
	ides := d.Detect(nil)

	g := find(t, ides, "goland")
	want := []string{"open", "-na", "GoLand.app", "--args"}
	if len(g.Exec) != len(want) {
		t.Fatalf("mac launch argv wrong: %+v", g.Exec)
	}
	for i := range want {
		if g.Exec[i] != want[i] {
			t.Fatalf("mac launch argv = %+v, want %+v", g.Exec, want)
		}
	}
}

func TestDetectMergesCustomIDEs(t *testing.T) {
	d := &Detector{GOOS: "linux", LookPath: lookPathSet("vim")}
	g := &config.Global{IDEs: []config.IDESpec{
		{ID: "myed", Name: "My Editor", Cmd: "myed --new", GUI: true},
		{ID: "bad"}, // no command — skipped
	}}
	ides := d.Detect(g)

	m := find(t, ides, "myed")
	if !m.GUI || m.Name != "My Editor" || len(m.Exec) != 2 || m.Exec[0] != "myed" {
		t.Errorf("custom ide wrong: %+v", m)
	}
	if _, ok := Find(ides, "bad"); ok {
		t.Error("a custom ide without a command should be skipped")
	}
}

func TestDetectCustomOverridesCatalog(t *testing.T) {
	d := &Detector{GOOS: "linux", LookPath: lookPathSet("code")}
	g := &config.Global{IDEs: []config.IDESpec{
		{ID: "code", Name: "Custom Code", Cmd: "mycode"},
	}}
	ides := d.Detect(g)

	// Only one "code" entry, and it's the custom override.
	count := 0
	for _, i := range ides {
		if i.ID == "code" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected a single 'code' entry, got %d: %+v", count, ides)
	}
	c := find(t, ides, "code")
	if c.Name != "Custom Code" || c.Exec[0] != "mycode" {
		t.Errorf("custom did not override catalog: %+v", c)
	}
}

func TestLaunchCmd(t *testing.T) {
	cmd := LaunchCmd(IDE{Exec: []string{"code", "-n"}}, "/work/tree")
	if cmd.Path == "" {
		t.Fatal("expected a resolved command path")
	}
	args := cmd.Args
	want := []string{"code", "-n", "/work/tree"}
	if len(args) != len(want) {
		t.Fatalf("args = %+v, want %+v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %+v, want %+v", args, want)
		}
	}
}

func TestFindMissing(t *testing.T) {
	if _, ok := Find(nil, "nope"); ok {
		t.Error("Find on empty slice should miss")
	}
}
