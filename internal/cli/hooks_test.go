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
	if count != 5 {
		t.Fatalf("got %d set-status entries, want 5", count)
	}
	for _, ev := range []string{"UserPromptSubmit", "PostToolUse", "Notification", "Stop", "SessionEnd"} {
		if events[ev] != 1 {
			t.Errorf("event %s has %d entries, want 1", ev, events[ev])
		}
	}
	// The absolute self path is embedded in the command.
	if !strings.Contains(ourHooksJSON("/usr/local/bin/wf"), "/usr/local/bin/wf") {
		t.Error("self path not embedded in hook command")
	}
}

// The "your turn" / teardown remap is the heart of this feature: Stop now drives
// ready (not done), and a new SessionEnd hook drives done.
func TestMergeHooksStopAndSessionEnd(t *testing.T) {
	s := mergeHooks(map[string]any{}, "/bin/wf")
	if got := setStatusCmdFor(t, s, "Stop"); !strings.Contains(got, "set-status ready") {
		t.Errorf("Stop hook = %q, want it to call set-status ready", got)
	}
	if got := setStatusCmdFor(t, s, "SessionEnd"); !strings.Contains(got, "set-status done") {
		t.Errorf("SessionEnd hook = %q, want it to call set-status done", got)
	}
}

// setStatusCmdFor returns the set-status command string wf installed under event.
func setStatusCmdFor(t *testing.T, settings map[string]any, event string) string {
	t.Helper()
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	for _, graw := range groups {
		g, _ := graw.(map[string]any)
		entries, _ := g["hooks"].([]any)
		for _, eraw := range entries {
			e, _ := eraw.(map[string]any)
			if c, _ := e["command"].(string); strings.Contains(c, "set-status") {
				return c
			}
		}
	}
	return ""
}

func TestMergeHooksIdempotent(t *testing.T) {
	s := mergeHooks(map[string]any{}, "/bin/wf")
	s = mergeHooks(s, "/bin/wf")
	if count, _ := countSetStatus(t, s); count != 5 {
		t.Errorf("after double merge got %d entries, want 5 (no duplicates)", count)
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

// oldInstall builds a settings map matching a pre-upgrade install: the four
// original hooks with Stop→done and no SessionEnd.
func oldInstall(self string) map[string]any {
	grp := func(matcher, cmd string) map[string]any {
		g := map[string]any{"hooks": []any{hookEntry(cmd)}}
		if matcher != "" {
			g["matcher"] = matcher
		}
		return g
	}
	return map[string]any{"hooks": map[string]any{
		"UserPromptSubmit": []any{grp("", hookCommand(self, "working"))},
		"PostToolUse":      []any{grp("", hookCommand(self, "working"))},
		"Notification":     []any{grp("permission_prompt|elicitation_dialog", hookCommand(self, "waiting"))},
		"Stop":             []any{grp("", hookCommand(self, "done"))},
	}}
}

func TestHookDriftAndInstalled(t *testing.T) {
	if hooksInstalled(map[string]any{}) {
		t.Error("empty settings should report no hooks installed")
	}
	old := oldInstall("/bin/wf")
	if !hooksInstalled(old) {
		t.Error("old install should report hooks installed")
	}
	// Stop flipped done→ready and SessionEnd is new; the other three still match.
	drift := hookDrift(old)
	if len(drift) != 2 || drift[0] != "Stop" || drift[1] != "SessionEnd" {
		t.Errorf("drift = %v, want [Stop SessionEnd]", drift)
	}
	// A current install has no drift, and a path-only difference is NOT drift
	// (drift is judged on the reported state, not the embedded binary path).
	if d := hookDrift(mergeHooks(map[string]any{}, "/some/other/path/wf")); len(d) != 0 {
		t.Errorf("current install (different path) drift = %v, want none", d)
	}
}

// On launch, an already-opted-in user's stale hooks are brought up to date and a
// one-line notice names what changed; re-running is a no-op.
func TestAutoUpdateHooksUpgradesOldInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, _ := settingsPath()
	if err := saveSettings(path, oldInstall("/old/bin/wf")); err != nil {
		t.Fatal(err)
	}

	msg := autoUpdateHooks()
	if !strings.Contains(msg, "Stop") || !strings.Contains(msg, "SessionEnd") {
		t.Errorf("notice = %q, want it to name Stop and SessionEnd", msg)
	}
	settings, _ := loadSettings(path)
	if count, _ := countSetStatus(t, settings); count != 5 {
		t.Errorf("entries after auto-update = %d, want 5", count)
	}
	hooks := asMap(settings["hooks"])
	if st, _ := installedHookState(hooks, "Stop", ""); st != "ready" {
		t.Errorf("Stop after update = %q, want ready", st)
	}
	if _, ok := installedHookState(hooks, "SessionEnd", ""); !ok {
		t.Error("SessionEnd not added by auto-update")
	}
	if msg2 := autoUpdateHooks(); msg2 != "" {
		t.Errorf("second auto-update = %q, want \"\" (already current)", msg2)
	}
}

// Auto-update never installs from absence — a fresh machine or a deliberate
// `hooks uninstall` (no wf hooks, perhaps other hooks present) is left untouched.
func TestAutoUpdateHooksLeavesUninstalledAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, _ := settingsPath()
	// A user with their own unrelated hook but none of wf's.
	if err := saveSettings(path, map[string]any{"hooks": map[string]any{
		"PostToolUse": []any{map[string]any{"matcher": "Write|Edit", "hooks": []any{map[string]any{"type": "command", "command": "gofmt -w"}}}},
	}}); err != nil {
		t.Fatal(err)
	}
	if msg := autoUpdateHooks(); msg != "" {
		t.Errorf("auto-update with no wf hooks = %q, want \"\"", msg)
	}
	settings, _ := loadSettings(path)
	if count, _ := countSetStatus(t, settings); count != 0 {
		t.Errorf("auto-update installed wf hooks from absence: %d entries", count)
	}
}

// The dashboard's startup tasks include the hook auto-update as the first task:
// running it upgrades an already-opted-in user's stale install and reports what
// changed, exactly as the synchronous path did before — now off the launch path.
func TestStartupTasksIncludeHookAutoUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, _ := settingsPath()
	if err := saveSettings(path, oldInstall("/old/bin/wf")); err != nil {
		t.Fatal(err)
	}

	tasks := startupTasks()
	if len(tasks) == 0 {
		t.Fatal("startupTasks returned none")
	}
	msg := tasks[0]()
	if !strings.Contains(msg, "Stop") || !strings.Contains(msg, "SessionEnd") {
		t.Errorf("first startup task notice = %q, want it to name the upgraded hooks", msg)
	}
	// And it actually wrote the upgrade through.
	settings, _ := loadSettings(path)
	if count, _ := countSetStatus(t, settings); count != 5 {
		t.Errorf("entries after the startup task ran = %d, want 5", count)
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
	if count, _ := countSetStatus(t, settings); count != 5 {
		t.Errorf("installed entries = %d, want 5", count)
	}

	if _, err := execWF(t, "hooks", "uninstall"); err != nil {
		t.Fatalf("hooks uninstall: %v", err)
	}
	settings, _ = loadSettings(path)
	if count, _ := countSetStatus(t, settings); count != 0 {
		t.Errorf("entries after uninstall = %d, want 0", count)
	}
}
