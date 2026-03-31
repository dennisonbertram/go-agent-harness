package symphd

import (
	"fmt"
	"strings"
)

// SynthesisDoctrineConfig holds configuration for the coordinator synthesis doctrine.
type SynthesisDoctrineConfig struct {
	// Enabled controls whether the synthesis doctrine is included in coordinator prompts.
	Enabled bool

	// RequireFileReferences controls whether the doctrine requires specific file paths
	// in worker prompts.
	RequireFileReferences bool

	// RequireLineReferences controls whether the doctrine requires specific line numbers
	// in worker prompts (stricter than RequireFileReferences).
	RequireLineReferences bool

	// CustomDoctrineText overrides the default synthesis doctrine text when non-empty.
	// When set and Enabled is true, only the custom text is returned.
	CustomDoctrineText string
}

// SynthesisAntiPattern describes a named coordinator anti-pattern with a
// wrong and a right example.
type SynthesisAntiPattern struct {
	// Name is the short label for the anti-pattern (e.g. `"Based on your findings"`).
	Name string

	// Wrong is an example of the anti-pattern to avoid.
	Wrong string

	// Right is an example of the corrected behavior.
	Right string
}

// DefaultSynthesisDoctrineConfig returns sensible defaults:
// enabled=true, file_refs=true, line_refs=false.
func DefaultSynthesisDoctrineConfig() SynthesisDoctrineConfig {
	return SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
		RequireLineReferences: false,
		CustomDoctrineText:    "",
	}
}

// FormatSynthesisDoctrine generates the synthesis doctrine text for coordinator prompts.
// Returns an empty string when cfg.Enabled is false.
// Returns cfg.CustomDoctrineText verbatim when it is non-empty and Enabled is true.
// Otherwise generates the full default doctrine including anti-patterns and requirements.
func FormatSynthesisDoctrine(cfg SynthesisDoctrineConfig) string {
	if !cfg.Enabled {
		return ""
	}
	if cfg.CustomDoctrineText != "" {
		return cfg.CustomDoctrineText
	}

	var sb strings.Builder

	sb.WriteString("## Synthesis Requirements\n\n")
	sb.WriteString("Synthesis is your most important job. Before delegating implementation to a worker,\n")
	sb.WriteString("you must prove you understood the research by including specific evidence in the worker prompt.\n\n")

	sb.WriteString("### Anti-Patterns (AVOID THESE)\n\n")

	for _, ap := range SynthesisAntiPatterns() {
		sb.WriteString(fmt.Sprintf("**%q** — NEVER write vague references to research output.\n", ap.Name))
		sb.WriteString(fmt.Sprintf("  WRONG: %q\n", ap.Wrong))
		sb.WriteString(fmt.Sprintf("  RIGHT: %q\n\n", ap.Right))
	}

	sb.WriteString("### Requirements\n")

	if cfg.RequireFileReferences {
		sb.WriteString("- Every worker prompt MUST include at least one specific file path\n")
		sb.WriteString("- File paths MUST be relative to the repository root\n")
	}

	if cfg.RequireLineReferences {
		sb.WriteString("- When referencing code, include line numbers or function names\n")
	} else {
		sb.WriteString("- When referencing code, include line numbers or function names\n")
	}

	sb.WriteString("- When referencing behavior, include the exact error message or test name\n")

	return sb.String()
}

// SynthesisAntiPatterns returns the three canonical coordinator anti-patterns.
func SynthesisAntiPatterns() []SynthesisAntiPattern {
	return []SynthesisAntiPattern{
		{
			Name:  "Based on your findings",
			Wrong: "Based on your findings, update the config file.",
			Right: "Update internal/config/config.go — add a MaxRetries field to the ServerConfig struct at line 47, matching the pattern used by Timeout on line 45.",
		},
		{
			Name:  "See the research above",
			Wrong: "Implement the changes discussed in the research phase.",
			Right: "Create internal/cache/lru.go implementing an LRU cache with Get/Put/Delete methods. Follow the interface defined in internal/cache/cache.go:12-18.",
		},
		{
			Name:  "Fix the bug",
			Wrong: "Fix the race condition in the worker pool.",
			Right: "In internal/pool/worker.go:89, the jobs channel is read without holding mu. Add mu.RLock()/mu.RUnlock() around the channel receive on line 89-91.",
		},
	}
}
