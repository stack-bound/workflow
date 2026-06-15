package sidebar

import (
	"testing"

	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/tmux"
)

func TestBuildEntriesFiltersAndLabels(t *testing.T) {
	store := &registry.Store{
		Worktrees: []registry.Worktree{
			{Project: "alpha", Branch: "feat-1", Path: "/wt/a1"},
		},
	}
	wins := []tmux.Window{
		{ID: "@1", Workspace: "/wt/a1", Name: "feat-1", Active: true},
		{ID: "@2", Workspace: "", Name: "bash"},         // untracked: dropped
		{ID: "@3", Workspace: "/wt/unknown", Name: "x"}, // tagged but not in registry
	}

	es := buildEntries(wins, store)
	if len(es) != 2 {
		t.Fatalf("got %d entries, want 2 (untracked dropped)", len(es))
	}
	// Sorted by project then branch; the registry-known one ("alpha") sorts
	// before the empty-project unknown.
	if es[0].label() != "alpha/feat-1" {
		t.Errorf("entry 0 label = %q, want alpha/feat-1", es[0].label())
	}
	if !es[0].active {
		t.Error("entry 0 should be active")
	}
	// An unmatched window falls back to its tmux window name for a label.
	if es[1].label() != "x" {
		t.Errorf("unmatched entry label = %q, want x", es[1].label())
	}
}

func TestBuildEntriesNilStore(t *testing.T) {
	wins := []tmux.Window{{ID: "@1", Workspace: "/wt/a1", Name: "feat-1"}}
	es := buildEntries(wins, nil)
	if len(es) != 1 || es[0].label() != "feat-1" {
		t.Fatalf("nil-store entries = %+v", es)
	}
}

func TestEntryLabelFallbacks(t *testing.T) {
	if got := (entry{path: "/only/path"}).label(); got != "/only/path" {
		t.Errorf("path-only label = %q", got)
	}
	if got := (entry{name: "win", path: "/p"}).label(); got != "win" {
		t.Errorf("name label = %q, want win", got)
	}
	if got := (entry{project: "p", branch: "b"}).label(); got != "p/b" {
		t.Errorf("project/branch label = %q, want p/b", got)
	}
}
