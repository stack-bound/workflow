package status

import (
	"path/filepath"
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
)

func TestResolveByCwd(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "wt", "feature")
	inner := filepath.Join(outer, "nested") // a worktree nested inside another
	other := filepath.Join(root, "wt", "bugfix")

	store := &registry.Store{Worktrees: []registry.Worktree{
		{Project: "p", Branch: "feature", Path: outer},
		{Project: "p", Branch: "nested", Path: inner},
		{Project: "p", Branch: "bugfix", Path: other},
	}}

	cases := []struct {
		name string
		cwd  string
		want string // expected branch, "" for nil
	}{
		{"exact match", outer, "feature"},
		{"subdir of worktree", filepath.Join(outer, "src", "pkg"), "feature"},
		{"nested picks innermost", filepath.Join(inner, "src"), "nested"},
		{"other worktree", other, "bugfix"},
		{"outside any", filepath.Join(root, "elsewhere"), ""},
		{"sibling-prefix not matched", outer + "-x", ""},
	}
	for _, c := range cases {
		got := ResolveByCwd(store, c.cwd)
		switch {
		case c.want == "" && got != nil:
			t.Errorf("%s: got %q, want nil", c.name, got.Branch)
		case c.want != "" && (got == nil || got.Branch != c.want):
			t.Errorf("%s: got %v, want %q", c.name, got, c.want)
		}
	}
}

func TestResolveProjectByCwd(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "repoA")
	repoB := filepath.Join(root, "repoB")

	store := &registry.Store{Projects: []registry.Project{
		{Name: "A", Path: repoA},
		{Name: "B", Path: repoB},
		{Name: "noPath"}, // empty path is skipped, never matches
	}}

	cases := []struct {
		name string
		cwd  string
		want string // expected project name, "" for nil
	}{
		{"exact root", repoA, "A"},
		{"nested in root", filepath.Join(repoA, "internal", "cli"), "A"},
		{"other project", repoB, "B"},
		{"outside any", filepath.Join(root, "elsewhere"), ""},
		{"sibling-prefix not matched", repoA + "-x", ""},
	}
	for _, c := range cases {
		got := ResolveProjectByCwd(store, c.cwd)
		switch {
		case c.want == "" && got != nil:
			t.Errorf("%s: got %q, want nil", c.name, got.Name)
		case c.want != "" && (got == nil || got.Name != c.want):
			t.Errorf("%s: got %v, want %q", c.name, got, c.want)
		}
	}
}
