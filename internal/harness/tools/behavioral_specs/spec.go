// Package behavioral_specs provides types and utilities for tool behavioral specifications.
// Specs are loaded from embedded markdown files and rendered according to config toggles.
package behavioral_specs

// BehavioralSpecConfig holds feature flags and limits for behavioral spec rendering.
// This mirrors config.BehavioralSpecConfig to avoid import cycles; callers convert.
type BehavioralSpecConfig struct {
	// Enabled is the master toggle — when false, FormatSpec returns "".
	Enabled bool
	// IncludeWhenNotToUse controls whether the "When NOT to Use" section is rendered.
	IncludeWhenNotToUse bool
	// IncludeAntiPatterns controls whether named anti-patterns are rendered.
	IncludeAntiPatterns bool
	// IncludeCommonMistakes controls whether the Common Mistakes section is rendered.
	IncludeCommonMistakes bool
	// MaxSpecLength is the maximum character length of a rendered spec. 0 = unlimited.
	MaxSpecLength int
}

// BehavioralSpec holds the structured behavioral specification for a tool.
type BehavioralSpec struct {
	// WhenToUse describes positive usage patterns.
	WhenToUse string
	// WhenNotToUse describes negative patterns and anti-patterns.
	WhenNotToUse string
	// BehavioralRules is an ordered list of constraints and safety rules.
	BehavioralRules []string
	// CommonMistakes is a list of named anti-patterns with descriptions.
	CommonMistakes []string
	// Examples holds WRONG/RIGHT paired examples.
	Examples []SpecExample
}

// SpecExample is a WRONG/RIGHT usage pair.
type SpecExample struct {
	// Label is a short description of the example scenario.
	Label string
	// Wrong is the bad example.
	Wrong string
	// Right is the good example.
	Right string
}
