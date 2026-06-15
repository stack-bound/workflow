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
//	Stop -> done (idle)
//
// `wf set-status` infers the workspace from the cwd and no-ops outside a wf
// worktree, so a single global install safely covers every current and future
// workspace without touching anything else.

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
		{"Stop", "", "done"},
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
