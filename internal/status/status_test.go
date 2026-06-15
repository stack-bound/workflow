package status

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := map[string]State{
		"working":   Working,
		"WORKING":   Working,
		" waiting ": Waiting,
		"waiting":   Waiting,
		"done":      Idle,
		"idle":      Idle,
		"":          Idle,
		"garbage":   Idle,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeyDeterministicAndPathScoped(t *testing.T) {
	// Same inputs → same key.
	a := Key("workflow", "feature/x", "/work/wf_worktrees/feature-x")
	b := Key("workflow", "feature/x", "/work/wf_worktrees/feature-x")
	if a != b {
		t.Fatalf("Key not deterministic: %q != %q", a, b)
	}
	// Readable stem: contains slugged project and branch.
	if want := "workflow__feature-x-"; a[:len(want)] != want {
		t.Errorf("Key = %q, want prefix %q", a, want)
	}
	// Two branches that slugify identically but live at different paths get
	// different keys (the hash disambiguates).
	c := Key("workflow", "feature/x", "/work/a")
	d := Key("workflow", "feature-x", "/work/b")
	if c == d {
		t.Errorf("expected different keys for different worktree paths, both %q", c)
	}
}

func TestWriteReadRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	const proj, branch, path = "workflow", "feature/x", "/work/wf_worktrees/feature-x"
	if err := Write(proj, branch, path, Working); err != nil {
		t.Fatalf("Write: %v", err)
	}
	st, ok, err := ReadFor(proj, branch, path)
	if err != nil || !ok {
		t.Fatalf("ReadFor ok=%v err=%v", ok, err)
	}
	if st.State != Working || st.Branch != branch || st.Project != proj {
		t.Errorf("read back %+v", st)
	}
	if st.TS == 0 {
		t.Errorf("timestamp not set")
	}

	// File is in the status subdir.
	file, _ := FileFor(proj, branch, path)
	if filepath.Base(filepath.Dir(file)) != "status" {
		t.Errorf("status file not under status/: %s", file)
	}

	if err := Remove(proj, branch, path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok, _ := ReadFor(proj, branch, path); ok {
		t.Errorf("status still present after Remove")
	}
}

func TestReadMalformed(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(file, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Read(file); ok || err == nil {
		t.Errorf("malformed read ok=%v err=%v, want false + error", ok, err)
	}
}

func TestReadAndRemoveMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, ok, err := ReadFor("p", "b", "/none"); ok || err != nil {
		t.Errorf("missing read: ok=%v err=%v, want false/nil", ok, err)
	}
	if err := Remove("p", "b", "/none"); err != nil {
		t.Errorf("Remove missing: %v", err)
	}
}

func TestEffective(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ttl := 5 * time.Minute
	fresh := now.Add(-time.Minute).Unix()
	stale := now.Add(-10 * time.Minute).Unix()

	cases := []struct {
		name string
		st   State
		ts   int64
		ttl  time.Duration
		want State
	}{
		{"fresh working stays", Working, fresh, ttl, Working},
		{"stale working downgrades", Working, stale, ttl, Idle},
		{"fresh waiting stays", Waiting, fresh, ttl, Waiting},
		{"stale waiting downgrades", Waiting, stale, ttl, Idle},
		{"idle stays idle", Idle, stale, ttl, Idle},
		{"ttl disabled keeps state", Working, stale, 0, Working},
	}
	for _, c := range cases {
		if got := Effective(c.st, c.ts, c.ttl, now); got != c.want {
			t.Errorf("%s: Effective = %q, want %q", c.name, got, c.want)
		}
	}
}
