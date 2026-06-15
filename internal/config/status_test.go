package config

import (
	"testing"
	"time"
)

func TestStatusResolveDefaults(t *testing.T) {
	r := (&Global{}).StatusLook() // nil Status → defaults
	if r.ColorMode != "tab" {
		t.Errorf("ColorMode = %q, want tab", r.ColorMode)
	}
	if r.TTL != 5*time.Minute {
		t.Errorf("TTL = %v, want 5m", r.TTL)
	}
	// nerdfont preset: idle is the branch glyph, working/waiting are non-empty,
	// and idle has no color (so tab-mode reverts and the glyph stays default).
	if r.Look[stateIdle].Glyph == "" {
		t.Errorf("idle glyph empty; the slot must always carry an icon")
	}
	if r.Look[stateIdle].Color != "" {
		t.Errorf("idle color = %q, want empty", r.Look[stateIdle].Color)
	}
	if r.Look[stateWorking].Color != "11" || r.Look[stateWaiting].Color != "9" {
		t.Errorf("default colors wrong: %+v", r.Look)
	}
}

func TestStatusResolvePresetAndOverrides(t *testing.T) {
	sc := &StatusConfig{
		Preset:    "emoji",
		ColorMode: "glyph",
		TTL:       "90s",
		Glyphs:    map[string]string{stateWorking: "🚀"},
		Colors:    map[string]string{stateWaiting: "200"},
	}
	r := sc.Resolve()
	if r.ColorMode != "glyph" {
		t.Errorf("ColorMode = %q, want glyph", r.ColorMode)
	}
	if r.TTL != 90*time.Second {
		t.Errorf("TTL = %v, want 90s", r.TTL)
	}
	if r.Look[stateWorking].Glyph != "🚀" {
		t.Errorf("working glyph override not applied: %q", r.Look[stateWorking].Glyph)
	}
	if r.Look[stateIdle].Glyph != "🌿" {
		t.Errorf("emoji preset idle glyph wrong: %q", r.Look[stateIdle].Glyph)
	}
	if r.Look[stateWaiting].Color != "200" {
		t.Errorf("waiting color override not applied: %q", r.Look[stateWaiting].Color)
	}
}

func TestStatusResolveBadValuesFallBack(t *testing.T) {
	sc := &StatusConfig{Preset: "nope", ColorMode: "rainbow", TTL: "not-a-duration"}
	r := sc.Resolve()
	if r.ColorMode != "tab" {
		t.Errorf("bad color_mode should fall back to tab, got %q", r.ColorMode)
	}
	if r.TTL != 5*time.Minute {
		t.Errorf("bad ttl should fall back to 5m, got %v", r.TTL)
	}
	// Unknown preset falls back to nerdfont (non-empty idle glyph).
	if r.Look[stateIdle].Glyph == "" {
		t.Errorf("unknown preset should fall back to nerdfont")
	}
}
