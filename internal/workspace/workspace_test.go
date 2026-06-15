package workspace

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"feature-x":         "feature-x",
		"feature/foo":       "feature-foo",
		"Feature/Foo_Bar":   "feature-foo-bar",
		"fix/ISSUE-123":     "fix-issue-123",
		"--weird//branch--": "weird-branch",
		"UPPER":             "upper",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestViewActive(t *testing.T) {
	tests := []struct {
		name string
		v    View
		want bool
	}{
		{"clean+merged", View{}, false},
		{"dirty", func() View { v := View{}; v.Stat.Dirty = true; return v }(), true},
		{"ahead", func() View { v := View{}; v.Stat.Ahead = 2; return v }(), true},
		{"open-pr", func() View { v := View{}; v.Worktree.PRRef = "42"; return v }(), true},
	}
	for _, tc := range tests {
		if got := tc.v.Active(); got != tc.want {
			t.Errorf("%s: Active() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
