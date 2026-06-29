package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// This file wires up `wf hooks`, which installs the Claude Code lifecycle hooks
// that call `wf set-status`. The hooks live in ~/.claude/settings.json and map:
//
//	UserPromptSubmit / PostToolUse  -> working
//	Notification (permission_prompt|elicitation_dialog) -> waiting
//	Stop -> ready ("your turn", end of every turn)
//	SessionEnd -> done (revert/idle, session teardown)
//
// `wf set-status` infers the workspace from the hook's cwd and decorates the
// agent's current tmux window, so a single global install safely covers every
// current and future workspace — and any borrowed tab — without touching
// anything else.

// desiredHook is one hook wf manages.
type desiredHook struct {
	event   string // Claude Code event name
	matcher string // matcher for the group ("" = all)
	state   string // wf state passed to set-status
}

func desiredHooks() []desiredHook {
	return []desiredHook{
		{"UserPromptSubmit", "", "working"},
		{"PostToolUse", "", "working"},
		{"Notification", "permission_prompt|elicitation_dialog", "waiting"},
		{"Stop", "", "ready"},
		{"SessionEnd", "", "done"},
	}
}

func newHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage the Claude Code hooks that report agent status to wf",
		Long: "Install or remove the Claude Code lifecycle hooks that drive wf's " +
			"live agent-status icons. The hooks call `wf set-status`, which infers " +
			"the workspace from the working directory, so one global install in " +
			"~/.claude/settings.json covers every workspace.",
	}
	cmd.AddCommand(newHooksInstallCmd(), newHooksUninstallCmd(), newHooksPrintCmd())
	return cmd
}

func newHooksInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the status hooks into ~/.claude/settings.json (idempotent)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := settingsPath()
			if err != nil {
				return err
			}
			settings, err := loadSettings(path)
			if err != nil {
				return err
			}
			settings = mergeHooks(settings, selfPath())
			if err := saveSettings(path, settings); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed wf status hooks into %s\n", path)
			return nil
		},
	}
}

func newHooksUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove wf's status hooks from ~/.claude/settings.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := settingsPath()
			if err != nil {
				return err
			}
			settings, err := loadSettings(path)
			if err != nil {
				return err
			}
			settings = removeHooks(settings)
			if err := saveSettings(path, settings); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed wf status hooks from %s\n", path)
			return nil
		},
	}
}

func newHooksPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "Print the hook JSON wf installs (for manual setup)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ourHooksJSON(selfPath()))
			return nil
		},
	}
}

// --- settings.json I/O ---

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func selfPath() string {
	if p, err := os.Executable(); err == nil && p != "" {
		return p
	}
	return "wf"
}

func loadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func saveSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp settings: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit settings: %w", err)
	}
	return nil
}

// --- pure merge/remove logic ---

func hookCommand(self, state string) string {
	// Quote the binary path so a path with spaces survives the shell the hook
	// runs in. %q matches shell double-quote semantics for ordinary paths.
	return fmt.Sprintf("%q set-status %s", self, state)
}

func hookEntry(cmd string) map[string]any {
	return map[string]any{"type": "command", "command": cmd}
}

// mergeHooks layers wf's hooks onto an existing settings map, preserving every
// other key and the user's own hooks. It is idempotent: wf's entry within a
// group is matched by its command containing "set-status", so re-running just
// rewrites it rather than duplicating.
func mergeHooks(settings map[string]any, self string) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}
	hooks := asMap(settings["hooks"])
	for _, d := range desiredHooks() {
		groups := asSlice(hooks[d.event])
		groups = upsertGroup(groups, d.matcher, hookCommand(self, d.state))
		hooks[d.event] = groups
	}
	settings["hooks"] = hooks
	return settings
}

// removeHooks deletes only wf's hook entries (command contains "set-status"),
// pruning groups and events that become empty, and leaves all other settings
// untouched.
func removeHooks(settings map[string]any) map[string]any {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return settings
	}
	for event, raw := range hooks {
		var keptGroups []any
		for _, graw := range asSlice(raw) {
			g, ok := graw.(map[string]any)
			if !ok {
				keptGroups = append(keptGroups, graw)
				continue
			}
			kept := dropOurEntries(asSlice(g["hooks"]))
			if len(kept) == 0 {
				continue // prune emptied group
			}
			g["hooks"] = kept
			keptGroups = append(keptGroups, g)
		}
		if len(keptGroups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = keptGroups
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	return settings
}

func dropOurEntries(entries []any) []any {
	var kept []any
	for _, eraw := range entries {
		if e, ok := eraw.(map[string]any); ok {
			if c, _ := e["command"].(string); strings.Contains(c, "set-status") {
				continue
			}
		}
		kept = append(kept, eraw)
	}
	return kept
}

// upsertGroup finds the group with the given matcher and upserts wf's entry into
// it, or appends a new group when none matches. Groups with a different matcher
// (e.g. the user's PostToolUse "Write|Edit" hook) are never disturbed.
func upsertGroup(groups []any, matcher, cmd string) []any {
	for i, raw := range groups {
		g, ok := raw.(map[string]any)
		if !ok || groupMatcher(g) != matcher {
			continue
		}
		g["hooks"] = upsertEntry(asSlice(g["hooks"]), cmd)
		groups[i] = g
		return groups
	}
	g := map[string]any{"hooks": []any{hookEntry(cmd)}}
	if matcher != "" {
		g["matcher"] = matcher
	}
	return append(groups, g)
}

func upsertEntry(entries []any, cmd string) []any {
	for i, raw := range entries {
		e, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if c, _ := e["command"].(string); strings.Contains(c, "set-status") {
			e["type"] = "command"
			e["command"] = cmd
			entries[i] = e
			return entries
		}
	}
	return append(entries, hookEntry(cmd))
}

// autoUpdateHooks drift-corrects wf's already-installed Claude Code hooks when
// they have fallen behind the current mapping (e.g. an install from before the
// ready/SessionEnd states existed), returning a one-line notice naming the
// events it brought up to date, or "" when it did nothing. It is best-effort
// (every error is swallowed — a settings file it can't read or write must never
// keep the dashboard from opening) and deliberately conservative:
//
//   - it acts ONLY when wf's hooks are already present, so it never installs from
//     an empty file and never resurrects hooks a user removed with
//     `wf hooks uninstall` (a deliberate uninstall is indistinguishable from a
//     fresh machine — both are "no wf hooks", and neither should be auto-filled);
//   - drift is judged on the STATE each hook reports, not the embedded binary
//     path, so running wf from a different path/symlink never triggers a spurious
//     rewrite on launch (an explicit `wf hooks install` still refreshes the path).
func autoUpdateHooks() string {
	path, err := settingsPath()
	if err != nil {
		return ""
	}
	settings, err := loadSettings(path)
	if err != nil {
		return ""
	}
	if !hooksInstalled(settings) {
		return "" // not opted in (or deliberately uninstalled) — leave it alone
	}
	drift := hookDrift(settings)
	if len(drift) == 0 {
		return "" // already current
	}
	settings = mergeHooks(settings, selfPath())
	if err := saveSettings(path, settings); err != nil {
		return ""
	}
	return "updated agent-status hooks: " + strings.Join(drift, ", ")
}

// hooksInstalled reports whether any of wf's hooks are present, i.e. the user has
// opted into wf managing their Claude Code hooks.
func hooksInstalled(settings map[string]any) bool {
	hooks := asMap(settings["hooks"])
	for _, d := range desiredHooks() {
		if _, ok := installedHookState(hooks, d.event, d.matcher); ok {
			return true
		}
	}
	return false
}

// hookDrift lists the events whose installed set-status hook reports a different
// state than the current mapping wants (or is missing entirely) — what an
// auto-update would bring into line. The path is ignored on purpose (see
// autoUpdateHooks). The order follows desiredHooks for a stable message.
func hookDrift(settings map[string]any) []string {
	hooks := asMap(settings["hooks"])
	var drift []string
	for _, d := range desiredHooks() {
		got, ok := installedHookState(hooks, d.event, d.matcher)
		if !ok || got != d.state {
			drift = append(drift, d.event)
		}
	}
	return drift
}

// installedHookState returns the state arg of wf's set-status hook under the
// given event+matcher group (e.g. "done"), and whether such an entry exists.
func installedHookState(hooks map[string]any, event, matcher string) (string, bool) {
	for _, graw := range asSlice(hooks[event]) {
		g, ok := graw.(map[string]any)
		if !ok || groupMatcher(g) != matcher {
			continue
		}
		for _, eraw := range asSlice(g["hooks"]) {
			e, ok := eraw.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := e["command"].(string); strings.Contains(c, "set-status") {
				return stateArgOf(c), true
			}
		}
	}
	return "", false
}

// stateArgOf extracts the state token following "set-status" in a hook command
// (e.g. `"…/wf" set-status ready` → "ready"), or "" when none follows.
func stateArgOf(cmd string) string {
	i := strings.Index(cmd, "set-status")
	if i < 0 {
		return ""
	}
	if fields := strings.Fields(cmd[i+len("set-status"):]); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// ourHooksJSON renders just wf's hooks as pretty JSON, for `hooks print`.
func ourHooksJSON(self string) string {
	data, _ := json.MarshalIndent(mergeHooks(map[string]any{}, self), "", "  ")
	return string(data)
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func groupMatcher(g map[string]any) string {
	if s, ok := g["matcher"].(string); ok {
		return s
	}
	return ""
}
