package systemprompt

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// PromptCacheConfig holds configuration for the system prompt cache boundary.
type PromptCacheConfig struct {
	// Enabled controls whether the cache boundary optimization is active.
	Enabled bool
	// BoundaryMarker is the text that separates static from dynamic sections.
	// It is stripped from the final prompt output when consumed by callers.
	BoundaryMarker string
	// StaticSections lists section names that appear before the boundary (globally cacheable).
	StaticSections []string
	// DynamicSections lists section names that appear after the boundary (session-specific).
	DynamicSections []string
}

// PromptCacheResult holds the split prompt with cache metadata.
type PromptCacheResult struct {
	// FullPrompt is the complete assembled prompt (with boundary marker when enabled).
	FullPrompt string
	// StaticPart is the content before the boundary marker.
	// Empty when cache is disabled.
	StaticPart string
	// DynamicPart is the content after the boundary marker.
	// Empty when cache is disabled.
	DynamicPart string
	// StaticHash is a deterministic SHA-256 cache key derived from StaticPart.
	// Empty when cache is disabled.
	StaticHash string
	// BoundaryIdx is the byte offset of the boundary marker in FullPrompt.
	// -1 if the marker is absent (e.g. cache disabled or no dynamic sections).
	BoundaryIdx int
}

// DefaultPromptCacheConfig returns sensible defaults for the cache boundary configuration.
func DefaultPromptCacheConfig() PromptCacheConfig {
	return PromptCacheConfig{
		Enabled:        true,
		BoundaryMarker: "---DYNAMIC-BOUNDARY---",
		StaticSections: []string{
			"identity",
			"rules",
			"tool_behavior",
			"tone_style",
		},
		DynamicSections: []string{
			"memory",
			"environment",
			"tool_catalog",
			"plugins",
		},
	}
}

// SplitPromptAtBoundary splits a system prompt into static and dynamic parts at the marker.
// If the marker is not found, the entire prompt is returned as static and dynamic is empty.
func SplitPromptAtBoundary(prompt string, marker string) (static string, dynamic string) {
	idx := strings.Index(prompt, marker)
	if idx == -1 {
		return prompt, ""
	}
	static = prompt[:idx]
	dynamic = prompt[idx+len(marker):]
	return static, dynamic
}

// JoinWithBoundary combines static and dynamic parts with the boundary marker between them.
func JoinWithBoundary(static string, dynamic string, marker string) string {
	return static + marker + dynamic
}

// HashStaticContent generates a deterministic SHA-256 hex cache key from a prompt string.
// Returns a non-empty hex string even for empty input.
func HashStaticContent(static string) string {
	h := sha256.Sum256([]byte(static))
	return fmt.Sprintf("%x", h)
}

// ClassifySection determines whether a section name is "static", "dynamic", or "unknown"
// according to the provided PromptCacheConfig.
func ClassifySection(name string, cfg PromptCacheConfig) string {
	for _, s := range cfg.StaticSections {
		if s == name {
			return "static"
		}
	}
	for _, d := range cfg.DynamicSections {
		if d == name {
			return "dynamic"
		}
	}
	return "unknown"
}

// BuildCachedPrompt assembles the provided sections into a PromptCacheResult that
// respects the cache boundary ordering:
//   - When enabled: static-classified sections first, then the boundary marker, then dynamic sections.
//   - When disabled: sections are joined in iteration order with no boundary marker.
//
// Sections whose names are not listed in StaticSections or DynamicSections ("unknown") are
// appended after the dynamic sections when the cache is enabled.
func BuildCachedPrompt(sections map[string]string, cfg PromptCacheConfig) PromptCacheResult {
	if !cfg.Enabled {
		// Disabled: join all sections in their map iteration order, no boundary.
		var b strings.Builder
		first := true
		for _, content := range sections {
			c := strings.TrimSpace(content)
			if c == "" {
				continue
			}
			if !first {
				b.WriteString("\n\n")
			}
			b.WriteString(c)
			first = false
		}
		return PromptCacheResult{
			FullPrompt:  b.String(),
			StaticPart:  "",
			DynamicPart: "",
			StaticHash:  "",
			BoundaryIdx: -1,
		}
	}

	// Enabled: collect static, dynamic, and unknown sections separately, preserving
	// the declaration order from StaticSections / DynamicSections lists.
	var staticParts []string
	for _, name := range cfg.StaticSections {
		content, ok := sections[name]
		if !ok {
			continue
		}
		c := strings.TrimSpace(content)
		if c == "" {
			continue
		}
		staticParts = append(staticParts, c)
	}

	var dynamicParts []string
	for _, name := range cfg.DynamicSections {
		content, ok := sections[name]
		if !ok {
			continue
		}
		c := strings.TrimSpace(content)
		if c == "" {
			continue
		}
		dynamicParts = append(dynamicParts, c)
	}

	// Sections not found in either list.
	knownNames := make(map[string]bool, len(cfg.StaticSections)+len(cfg.DynamicSections))
	for _, n := range cfg.StaticSections {
		knownNames[n] = true
	}
	for _, n := range cfg.DynamicSections {
		knownNames[n] = true
	}
	var unknownParts []string
	for name, content := range sections {
		if knownNames[name] {
			continue
		}
		c := strings.TrimSpace(content)
		if c != "" {
			unknownParts = append(unknownParts, c)
		}
	}

	staticStr := strings.Join(staticParts, "\n\n")
	dynamicStr := strings.Join(append(dynamicParts, unknownParts...), "\n\n")

	fullPrompt := JoinWithBoundary(staticStr, dynamicStr, cfg.BoundaryMarker)
	boundaryIdx := strings.Index(fullPrompt, cfg.BoundaryMarker)

	return PromptCacheResult{
		FullPrompt:  fullPrompt,
		StaticPart:  staticStr,
		DynamicPart: dynamicStr,
		StaticHash:  HashStaticContent(staticStr),
		BoundaryIdx: boundaryIdx,
	}
}
