// Package workflow exposes build-time metadata shared across the wf binary.
package workflow

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

// versionFile is the contents of the repo-root VERSION file, embedded at build
// time. The release flow rewrites VERSION and tags the commit, so the embedded
// value always matches the released tag — no ldflags wiring required.
//
//go:embed VERSION
var versionFile string

// Version returns the application version.
//
// Official release builds are produced by goreleaser with VCS stamping disabled
// (-buildvcs=false), so they carry no commit metadata and Version returns the
// bare value from the embedded VERSION file (e.g. "0.2.2").
//
// Any build made from a source checkout (`go build`, `go install`, `make build`)
// is stamped by the Go toolchain with the commit it was built from. For those,
// Version appends a "-dev" pre-release tag and "+<short-commit>" build metadata,
// plus a ".dirty" marker when the working tree had uncommitted changes — e.g.
// "0.2.2-dev+abc1234" or "0.2.2-dev+abc1234.dirty" — so a locally built binary is
// always distinguishable from a release.
func Version() string {
	base := strings.TrimSpace(versionFile)
	if base == "" {
		base = "dev"
	}
	rev, modified := vcsInfo()
	return composeVersion(base, rev, modified)
}

// composeVersion assembles the user-facing version string from its parts. An
// empty rev (no VCS stamping) marks a release build and yields the bare base;
// otherwise the commit is attached as a "-dev" build, with ".dirty" appended for
// a modified working tree.
func composeVersion(base, rev string, modified bool) string {
	if rev == "" {
		return base
	}
	v := base + "-dev+" + rev
	if modified {
		v += ".dirty"
	}
	return v
}

// vcsInfo reads the commit hash and dirty flag that the Go toolchain stamps into
// the binary at build time. rev is the short (7-char) commit, or empty when the
// binary was built without VCS stamping (e.g. a release built with
// -buildvcs=false, or a build outside the source tree).
func vcsInfo() (rev string, modified bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	return rev, modified
}
