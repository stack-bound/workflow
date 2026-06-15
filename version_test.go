package workflow

import (
	"strings"
	"testing"
)

func TestVersionNonEmptyAndPrefixed(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version() returned empty string")
	}
	if v != strings.TrimSpace(v) {
		t.Errorf("Version() has surrounding whitespace: %q", v)
	}
	// Whether this is a release or a stamped dev build, Version() always begins
	// with the base value from the VERSION file (or "dev" when it is empty).
	base := strings.TrimSpace(versionFile)
	if base == "" {
		base = "dev"
	}
	if !strings.HasPrefix(v, base) {
		t.Errorf("Version() = %q, want prefix %q", v, base)
	}
}

func TestComposeVersion(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		rev      string
		modified bool
		want     string
	}{
		{"release build has no vcs metadata", "0.2.2", "", false, "0.2.2"},
		{"release ignores modified flag without a rev", "0.2.2", "", true, "0.2.2"},
		{"clean dev build", "0.2.2", "abc1234", false, "0.2.2-dev+abc1234"},
		{"dirty dev build", "0.2.2", "abc1234", true, "0.2.2-dev+abc1234.dirty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeVersion(tt.base, tt.rev, tt.modified); got != tt.want {
				t.Errorf("composeVersion(%q, %q, %v) = %q, want %q",
					tt.base, tt.rev, tt.modified, got, tt.want)
			}
		})
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	saved := versionFile
	t.Cleanup(func() { versionFile = saved })

	// An empty VERSION file makes the base "dev"; the prefix holds whether or not
	// the test binary carries VCS stamping.
	versionFile = "   \n\t "
	if got := Version(); !strings.HasPrefix(got, "dev") {
		t.Errorf("empty VERSION → Version() = %q, want prefix %q", got, "dev")
	}

	versionFile = "  1.2.3\n"
	if got := Version(); !strings.HasPrefix(got, "1.2.3") {
		t.Errorf("Version() = %q, want prefix %q", got, "1.2.3")
	}
}
