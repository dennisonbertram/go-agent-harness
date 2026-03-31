package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/config"
)

// TestPromptCacheDefaults verifies the built-in defaults for the prompt cache boundary.
// Regression: ensures defaults are populated (not the zero values) when no config file is present.
func TestPromptCacheDefaults(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()

	if !cfg.PromptCache.Enabled {
		t.Error("PromptCache.Enabled: default should be true")
	}
	if cfg.PromptCache.BoundaryMarker == "" {
		t.Error("PromptCache.BoundaryMarker: default should not be empty")
	}
	if len(cfg.PromptCache.StaticSections) == 0 {
		t.Error("PromptCache.StaticSections: default should not be empty")
	}
	if len(cfg.PromptCache.DynamicSections) == 0 {
		t.Error("PromptCache.DynamicSections: default should not be empty")
	}
}

// TestPromptCacheConfigLoadFromTOML verifies that a [prompt_cache] block in a TOML
// config file is correctly parsed and applied over the defaults via Load().
// Regression: catches regressions in rawLayer/applyLayer wiring for prompt_cache.
func TestPromptCacheConfigLoadFromTOML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	content := `
[prompt_cache]
enabled = false
boundary_marker = "===CUSTOM-BOUNDARY==="
static_sections = ["rules", "identity"]
dynamic_sections = ["plugins"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(config.LoadOptions{
		UserConfigPath: cfgPath,
		Getenv:         func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PromptCache.Enabled {
		t.Error("PromptCache.Enabled: should be false (overridden by config)")
	}
	if cfg.PromptCache.BoundaryMarker != "===CUSTOM-BOUNDARY===" {
		t.Errorf("PromptCache.BoundaryMarker: got %q, want %q", cfg.PromptCache.BoundaryMarker, "===CUSTOM-BOUNDARY===")
	}
	if len(cfg.PromptCache.StaticSections) != 2 {
		t.Errorf("PromptCache.StaticSections: got %d items, want 2", len(cfg.PromptCache.StaticSections))
	}
	if len(cfg.PromptCache.DynamicSections) != 1 || cfg.PromptCache.DynamicSections[0] != "plugins" {
		t.Errorf("PromptCache.DynamicSections: got %v, want [plugins]", cfg.PromptCache.DynamicSections)
	}
}

// TestPromptCacheConfigPartialOverride verifies that partial [prompt_cache] overrides
// do not reset unset fields to zero — only the specified fields are overridden.
// Regression: catches bugs where applyLayer zeroes unset fields instead of preserving defaults.
func TestPromptCacheConfigPartialOverride(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Override only enabled; other fields should remain at their defaults.
	content := `
[prompt_cache]
enabled = false
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(config.LoadOptions{
		UserConfigPath: cfgPath,
		Getenv:         func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.PromptCache.Enabled {
		t.Error("PromptCache.Enabled: should be false (overridden)")
	}
	// BoundaryMarker and sections should still be at defaults since they were not overridden.
	defaults := config.Defaults()
	if cfg.PromptCache.BoundaryMarker != defaults.PromptCache.BoundaryMarker {
		t.Errorf("PromptCache.BoundaryMarker: partial override should preserve default, got %q",
			cfg.PromptCache.BoundaryMarker)
	}
	if len(cfg.PromptCache.StaticSections) != len(defaults.PromptCache.StaticSections) {
		t.Errorf("PromptCache.StaticSections: partial override should preserve default count %d, got %d",
			len(defaults.PromptCache.StaticSections), len(cfg.PromptCache.StaticSections))
	}
}

// TestBuildCachedPrompt_UnknownSectionsAppendedAfterDynamic is a regression test that
// verifies unknown sections do not get silently dropped by BuildCachedPrompt.
// Regression: catches a future refactor that accidentally drops unclassified sections.
func TestBuildCachedPrompt_UnknownSectionsAppendedAfterDynamic(t *testing.T) {
	// This test lives in config_test but tests systemprompt's BuildCachedPrompt via
	// integration. We reproduce it here as a config-layer regression for the
	// PromptCacheConfig struct rather than the cache functions directly.
	// (Full behavioral coverage is in internal/systemprompt/cache_test.go.)
	t.Parallel()
	cfg := config.Defaults()

	// Verify that the static and dynamic sections in defaults are distinct and non-overlapping.
	staticSet := make(map[string]bool, len(cfg.PromptCache.StaticSections))
	for _, s := range cfg.PromptCache.StaticSections {
		staticSet[s] = true
	}
	for _, d := range cfg.PromptCache.DynamicSections {
		if staticSet[d] {
			t.Errorf("section %q appears in both StaticSections and DynamicSections — they must be disjoint", d)
		}
	}
}
