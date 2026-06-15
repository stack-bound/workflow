package workflow

import (
	"strings"
	"testing"
)

func TestVersionMatchesEmbeddedFile(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version() returned empty string")
	}
	if v != strings.TrimSpace(v) {
		t.Errorf("Version() has surrounding whitespace: %q", v)
	}
	// When the embedded VERSION file has content, Version() is its trimmed value.
	if trimmed := strings.TrimSpace(versionFile); trimmed != "" && v != trimmed {
		t.Errorf("Version() = %q, want %q (from VERSION file)", v, trimmed)
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	saved := versionFile
	t.Cleanup(func() { versionFile = saved })

	versionFile = "   \n\t "
	if got := Version(); got != "dev" {
		t.Errorf("empty VERSION → Version() = %q, want dev", got)
	}

	versionFile = "  1.2.3\n"
	if got := Version(); got != "1.2.3" {
		t.Errorf("Version() = %q, want 1.2.3", got)
	}
}
