package config

import "time"

// Look is the resolved visual for one status state: a glyph plus an ANSI-256
// color number (as a string; "" means no color / revert to default). The same
// pair drives the tmux tab and the lipgloss dashboard/sidebar, so colors are
// kept as bare numbers usable by both ("colourN" in tmux, lipgloss.Color("N")).
type Look struct {
	Glyph string
	Color string
}

// ResolvedStatus is the fully-resolved status presentation: a Look per state,
// the tmux color mode, the decoration scope, and the staleness TTL.
type ResolvedStatus struct {
	Look      map[string]Look // keyed by state string: idle | working | waiting | ready
	ColorMode string          // tab | glyph | none
	Scope     string          // all | wf
	TTL       time.Duration
}

// StatusConfig is the user-tunable status presentation, stored under `status:`
// in the global config. All fields are optional; absent values fall back to the
// nerdfont preset and sensible defaults.
type StatusConfig struct {
	// Preset selects a built-in glyph set: nerdfont (default), emoji, or ascii.
	Preset string `yaml:"preset,omitempty"`
	// ColorMode is how the tmux tab is colored: tab (default, whole tab), glyph
	// (inline color on the icon only), or none.
	ColorMode string `yaml:"color_mode,omitempty"`
	// Scope limits which tmux tabs get decorated: all (default — the current
	// window for any agent, anywhere) or wf (only when cwd resolves to a
	// registered worktree or project base, leaving unrelated tabs untouched).
	Scope string `yaml:"scope,omitempty"`
	// TTL is how long a working status stays "live" before being shown as idle
	// (default 30m). Only working ages out — waiting/ready persist. Any Go
	// duration string, e.g. "90s", "10m"; <= 0 disables the downgrade.
	TTL string `yaml:"ttl,omitempty"`
	// Glyphs overrides individual state glyphs (keys: idle/working/waiting/ready).
	Glyphs map[string]string `yaml:"glyphs,omitempty"`
	// Colors overrides individual state colors as ANSI-256 numbers (as strings).
	Colors map[string]string `yaml:"colors,omitempty"`
}

const (
	stateIdle    = "idle"
	stateWorking = "working"
	stateWaiting = "waiting"
	stateReady   = "ready"

	scopeAll = "all"
	scopeWF  = "wf"

	// ScopeWF is the resolved "wf" scope value (decorate only registered
	// worktrees/bases), exported so callers can branch on ResolvedStatus.Scope.
	ScopeWF = scopeWF

	defaultPreset    = "nerdfont"
	defaultColorMode = "tab"
	defaultScope     = scopeAll
	defaultTTL       = 30 * time.Minute
)

// presets are the built-in glyph sets. Idle is always a branch-like glyph so a
// wf-opened window always carries an icon (the slot never goes empty, so the
// tab layout never shifts). The nerdfont glyphs are the documented default;
// switch to emoji/ascii in config if your font lacks them.
var presets = map[string]map[string]string{
	defaultPreset: {
		stateIdle:    "",          // powerline branch
		stateWorking: "\U000f06a9", // md robot
		stateWaiting: "",          // fa hourglass
		stateReady:   "\uf0f3",     // fa bell ("your turn")
	},
	"emoji": {
		stateIdle:    "🌿",
		stateWorking: "🤖",
		stateWaiting: "⏳",
		stateReady:   "🔔",
	},
	"ascii": {
		stateIdle:    "-",
		stateWorking: "*",
		stateWaiting: "?",
		stateReady:   "!",
	},
}

// defaultColors maps state → ANSI-256 number. Idle is "" (no color): the branch
// glyph renders in the terminal's normal color, and in color_mode=tab an idle
// window reverts to the default tab style.
var defaultColors = map[string]string{
	stateIdle:    "",
	stateWorking: "11", // bright yellow
	stateWaiting: "9",  // bright red
	stateReady:   "10", // bright green ("your turn")
}

func validColorMode(m string) bool {
	return m == "tab" || m == "glyph" || m == "none"
}

func validScope(s string) bool {
	return s == scopeAll || s == scopeWF
}

// Resolve produces the effective status presentation, layering per-state
// overrides on top of the chosen preset and filling defaults. It never errors:
// an unknown preset/mode or an unparseable TTL falls back to its default.
func (c *StatusConfig) Resolve() ResolvedStatus {
	if c == nil {
		c = &StatusConfig{}
	}
	glyphs, ok := presets[c.Preset]
	if !ok {
		glyphs = presets[defaultPreset]
	}
	look := make(map[string]Look, 4)
	for _, st := range []string{stateIdle, stateWorking, stateWaiting, stateReady} {
		g := glyphs[st]
		if ov, ok := c.Glyphs[st]; ok {
			g = ov
		}
		col := defaultColors[st]
		if ov, ok := c.Colors[st]; ok {
			col = ov
		}
		look[st] = Look{Glyph: g, Color: col}
	}

	mode := c.ColorMode
	if !validColorMode(mode) {
		mode = defaultColorMode
	}

	scope := c.Scope
	if !validScope(scope) {
		scope = defaultScope
	}

	ttl := defaultTTL
	if c.TTL != "" {
		if d, err := time.ParseDuration(c.TTL); err == nil {
			ttl = d
		}
	}

	return ResolvedStatus{Look: look, ColorMode: mode, Scope: scope, TTL: ttl}
}

// StatusLook resolves the status presentation from the global config, applying
// defaults when no `status:` block is present.
func (g *Global) StatusLook() ResolvedStatus {
	if g == nil || g.Status == nil {
		return (&StatusConfig{}).Resolve()
	}
	return g.Status.Resolve()
}
