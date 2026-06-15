package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// countSetStatus walks a settings map and counts hook entries whose command
// references set-status, returning the events they appear under.
func countSetStatus(t *testing.T, settings map[string]any) (count int, events map[string]int) {
	t.Helper()
	events = map[string]int{}
	hooks, _ := settings["hooks"].(map[string]any)
	for event, raw := range hooks {
		groups, _ := raw.([]any)
		for _, graw := range groups {
			g, _ := graw.(map[string]any)
			entries, _ := g["hooks"].([]any)
			for _, eraw := range entries {
				e, _ := eraw.(map[string]any)
				if c, _ := e["command"].(string); strings.Contains(c, "set-status") {
					count++
					events[event]++
				}
			}
		}
	}
	return count, events
}

func TestMergeHooksFromEmpty(t *testing.T) {
	s := mergeHooks(map[string]any{}, "/usr/local/bin/wf")
	count, events := countSetStatus(t, s)
	if count != 4 {
		t.Fatalf("got %d set-status entries, want 4", count)
	}
	for _, ev := range []string{"UserPromptSubmit", "PostToolUse", "Notification", "Stop"} {
		if events[ev] != 1 {
			t.Errorf("event %s has %d entries, want 1", ev, events[ev])
		}
	}
	// The absolute self path is embedded in the command.
	if !strings.Contains(ourHooksJSON("/usr/local/bin/wf"), "/usr/local/bin/wf") {
		t.Error("self path not embedded in hook command")
	}
}

func TestMergeHooksIdempotent(t *testing.T) {
	s := mergeHooks(map[string]any{}, "/bin/wf")
	s = mergeHooks(s, "/bin/wf")
	if count, _ := countSetStatus(t, s); count != 4 {
		t.Errorf("after double merge got %d entries, want 4 (no duplicates)", count)
	}
}

func TestMergeHooksPreservesUserConfig(t *testing.T) {
	// A user with an unrelated top-level key and an existing PostToolUse hook
	// (matcher "Write|Edit", like this repo's gofmt hook) must keep both.
	settings := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Write|Edit",
					"hooks":   []any{map[string]any{"type": "command", "command": "gofmt -w"}},
				},
			},
		},
	}
	settings = mergeHooks(settings, "/bin/wf")

	if settings["theme"] != "dark" {
		t.Error("unrelated top-level key dropped")
	}
	hooks := settings["hooks"].(map[string]any)
	groups := hooks["PostToolUse"].([]any)
	if len(groups) != 2 {
		t.Fatalf("PostToolUse groups = %d, want 2 (user's Write|Edit + wf's all-tools)", len(groups))
	}
	// The user's gofmt hook survives untouched.
	var foundGofmt bool
	for _, graw := range groups {
		g := graw.(map[string]any)
		for _, eraw := range g["hooks"].([]any) {
			if c, _ := eraw.(map[string]any)["command"].(string); strings.Contains(c, "gofmt") {
				foundGofmt = true
			}
		}
	}
	if !foundGofmt {
		t.Error("user's gofmt hook was clobbered")
	}
}

func TestRemoveHooks(t *testing.T) {
	settings := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Write|Edit",
					"hooks":   []any{map[string]any{"type": "command", "command": "gofmt -w"}},
				},
			},
		},
	}
	settings = mergeHooks(settings, "/bin/wf")
	settings = removeHooks(settings)

	if count, _ := countSetStatus(t, settings); count != 0 {
		t.Errorf("set-status entries after removal = %d, want 0", count)
	}
	// The user's gofmt hook and top-level key remain.
	if settings["theme"] != "dark" {
		t.Error("removeHooks dropped an unrelated key")
	}
	hooks := settings["hooks"].(map[string]any)
	groups, ok := hooks["PostToolUse"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("PostToolUse groups after removal = %v, want the single user group", hooks["PostToolUse"])
	}
	// Events that only held wf hooks are pruned entirely.
	if _, exists := hooks["Stop"]; exists {
		t.Error("empty Stop event not pruned after removal")
	}
}

func TestOurHooksJSONValid(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(ourHooksJSON("/bin/wf")), &m); err != nil {
		t.Fatalf("ourHooksJSON is not valid JSON: %v", err)
	}
}

func TestHooksPrintCommand(t *testing.T) {
	out, err := execWF(t, "hooks", "print")
	if err != nil {
		t.Fatalf("hooks print: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Errorf("hooks print output is not valid JSON: %v\n%s", err, out)
	}
	if !strings.Contains(out, "set-status") {
		t.Errorf("hooks print missing set-status:\n%s", out)
	}
}

func TestHooksInstallUninstallCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execWF(t, "hooks", "install"); err != nil {
		t.Fatalf("hooks install: %v", err)
	}
	path, _ := settingsPath()
	settings, err := loadSettings(path)
	if err != nil {
		t.Fatalf("load after install: %v", err)
	}
	if count, _ := countSetStatus(t, settings); count != 4 {
		t.Errorf("installed entries = %d, want 4", count)
	}

	if _, err := execWF(t, "hooks", "uninstall"); err != nil {
		t.Fatalf("hooks uninstall: %v", err)
	}
	settings, _ = loadSettings(path)
	if count, _ := countSetStatus(t, settings); count != 0 {
		t.Errorf("entries after uninstall = %d, want 0", count)
	}
}
