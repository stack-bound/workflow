// Package status is WorkFlow's per-workspace agent status: the small files an
// agent's hooks write (via `wf set-status`) and the dashboard, sidebar, and
// tmux tab read. It is the single source of truth for "is an agent working,
// waiting on me, or idle in this workspace?".
//
// Status is derived/high-frequency, not durable, so it lives in its own files
// under <config dir>/status/ — never in the registry (which holds only durable
// facts). One file per worktree, keyed by project+branch with a path hash so
// branches that slugify identically across projects never collide.
//
// This package deliberately does NOT import the workspace engine: the engine
// depends on status for cleanup, and a private slug helper here keeps the
// dependency one-directional (no import cycle).
package status

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stack-bound/workflow/internal/config"
)

// State is an agent's status in a workspace.
type State string

const (
	// Idle is the resting state (also what the hook's "done"/SessionEnd maps to):
	// no agent session here.
	Idle State = "idle"
	// Working means an agent is actively running.
	Working State = "working"
	// Waiting means an agent is blocked on user input mid-turn (a permission or
	// elicitation prompt).
	Waiting State = "waiting"
	// Ready means an agent finished its turn and is parked for your reply ("your
	// turn"). Distinct from Idle, which is "no session at all".
	Ready State = "ready"
)

// Normalize maps a raw state string (including the hooks' "done") to a State,
// defaulting unknown values to Idle so a bad hook argument is harmless.
func Normalize(s string) State {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "working":
		return Working
	case "waiting":
		return Waiting
	case "ready":
		return Ready
	default: // "idle", "done", "", and anything unexpected
		return Idle
	}
}

// Status is the on-disk record for one workspace.
type Status struct {
	State   State  `json:"state"`
	TS      int64  `json:"ts"` // unix seconds when last written
	Branch  string `json:"branch"`
	Project string `json:"project"`
}

// slug mirrors workspace.Slug for filesystem-safe filename parts. It is
// duplicated here (not imported) on purpose: the workspace engine depends on
// this package for cleanup, so importing it back would create a cycle. The
// status filename is independent of the worktree directory name, so the two
// sluggers never need to agree — only this package's own write/read/remove must
// agree with itself.
func slug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// shortHash returns the first 8 hex chars of sha256(path), used to disambiguate
// workspaces whose project+branch slugify identically.
func shortHash(worktreePath string) string {
	sum := sha256.Sum256([]byte(worktreePath))
	return hex.EncodeToString(sum[:])[:8]
}

// Key builds the status filename stem for a worktree:
// "<project-slug>__<branch-slug>-<hash>" — readable, with a hash for safety.
func Key(project, branch, worktreePath string) string {
	return fmt.Sprintf("%s__%s-%s", slug(project), slug(branch), shortHash(worktreePath))
}

// Dir returns the status directory path WITHOUT creating it (so reads do not
// litter an empty dir). Use EnsureDir when a writable/watchable dir is needed.
func Dir() (string, error) {
	cfgDir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "status"), nil
}

// EnsureDir returns the status directory, creating it if necessary.
func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create status dir: %w", err)
	}
	return dir, nil
}

// FileForKey returns the absolute status-file path for a key (no dir creation).
func FileForKey(key string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

// FileFor returns the status-file path for a worktree (no dir creation).
func FileFor(project, branch, worktreePath string) (string, error) {
	return FileForKey(Key(project, branch, worktreePath))
}

// baseBranch is the sentinel branch component for a project's base/root
// checkout: empty, so the key is the project slug + the root path hash with no
// branch part. Write (set-status) and read (dashboard) both route through
// WriteBase/ReadBase so they agree on this key without a git call.
const baseBranch = ""

// WriteBase records the agent status for a project's base/root checkout, keyed
// by the project root path with an empty branch component.
func WriteBase(project, projectPath string, st State) error {
	return Write(project, baseBranch, projectPath, st)
}

// ReadBase reads a project's base/root checkout status (the mirror of WriteBase).
func ReadBase(project, projectPath string) (Status, bool, error) {
	return ReadFor(project, baseBranch, projectPath)
}

// Write records a workspace's state, stamping the current time. The write is
// atomic (temp file + rename), mirroring registry.Save.
func Write(project, branch, worktreePath string, st State) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	file, err := FileFor(project, branch, worktreePath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(Status{
		State:   st,
		TS:      time.Now().Unix(),
		Branch:  branch,
		Project: project,
	})
	if err != nil {
		return err
	}
	return atomicWrite(file, data)
}

// Read reads a status file. ok is false (with a nil error) when the file is
// absent, so callers can treat "no agent ran here yet" as plain idle.
func Read(file string) (st Status, ok bool, err error) {
	data, rerr := os.ReadFile(file)
	if os.IsNotExist(rerr) {
		return Status{}, false, nil
	}
	if rerr != nil {
		return Status{}, false, rerr
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return Status{}, false, err
	}
	return st, true, nil
}

// ReadFor reads the status for a worktree.
func ReadFor(project, branch, worktreePath string) (Status, bool, error) {
	file, err := FileFor(project, branch, worktreePath)
	if err != nil {
		return Status{}, false, err
	}
	return Read(file)
}

// Remove deletes a workspace's status file. A missing file is not an error.
func Remove(project, branch, worktreePath string) error {
	file, err := FileFor(project, branch, worktreePath)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Effective applies the staleness rule at render time — but only to Working.
// Working self-refreshes on every PostToolUse, so a stale Working means the
// agent died mid-tool (or a single tool ran longer than the TTL): downgrade it
// to Idle. Waiting and Ready never restamp by nature (they mark "needs you"),
// so any TTL on them is a false clear — they persist until the next state
// change or SessionEnd. Idle is always Idle; ttl <= 0 disables the downgrade.
func Effective(st State, ts int64, ttl time.Duration, now time.Time) State {
	if st == Idle {
		return Idle
	}
	if st != Working {
		return st // waiting/ready persist
	}
	if ttl <= 0 {
		return st
	}
	if now.Sub(time.Unix(ts, 0)) > ttl {
		return Idle
	}
	return st
}

// atomicWrite writes data to path via a temp file + rename in the same dir.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".status-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp status: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp status: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp status: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit status: %w", err)
	}
	return nil
}
