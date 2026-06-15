// Package workflow exposes build-time metadata shared across the wf binary.
package workflow

import (
	_ "embed"
	"strings"
)

// versionFile is the contents of the repo-root VERSION file, embedded at build
// time. The release flow rewrites VERSION and tags the commit, so the embedded
// value always matches the released tag — no ldflags wiring required.
//
//go:embed VERSION
var versionFile string

// Version returns the application version read from the VERSION file. It falls
// back to "dev" when the file is empty (e.g. an unreleased local checkout).
func Version() string {
	if v := strings.TrimSpace(versionFile); v != "" {
		return v
	}
	return "dev"
}
