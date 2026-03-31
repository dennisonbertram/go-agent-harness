package systemprompt

import (
	"fmt"
	"strings"
)

// OutputEfficiencyConfig controls the word-count anchors injected into the
// system prompt to limit assistant verbosity between tool calls and in final
// responses.
type OutputEfficiencyConfig struct {
	// Enabled controls whether the output efficiency section is included in
	// the system prompt. When false, FormatEfficiencyAnchors returns "".
	Enabled bool

	// MaxWordsBetweenToolCalls is the soft cap on assistant words in the text
	// segment between successive tool calls. 0 means no limit.
	MaxWordsBetweenToolCalls int

	// MaxWordsFinalResponse is the soft cap on assistant words in the final
	// response to the user. 0 means no limit.
	MaxWordsFinalResponse int

	// CustomInstruction, when non-empty, replaces the auto-generated anchor
	// text entirely. The caller supplies whatever wording they prefer.
	CustomInstruction string
}

// DefaultOutputEfficiencyConfig returns the default OutputEfficiencyConfig:
//   - Enabled: true
//   - MaxWordsBetweenToolCalls: 25
//   - MaxWordsFinalResponse: 100
//   - CustomInstruction: "" (auto-generate)
func DefaultOutputEfficiencyConfig() OutputEfficiencyConfig {
	return OutputEfficiencyConfig{
		Enabled:                  true,
		MaxWordsBetweenToolCalls: 25,
		MaxWordsFinalResponse:    100,
		CustomInstruction:        "",
	}
}

// FormatEfficiencyAnchors returns the system prompt section for output efficiency.
//
// Rules:
//   - If cfg.Enabled is false, returns "".
//   - If cfg.CustomInstruction is non-empty, returns it verbatim.
//   - Otherwise, generates a standard three-bullet anchor block using the
//     configured word limits.
func FormatEfficiencyAnchors(cfg OutputEfficiencyConfig) string {
	if !cfg.Enabled {
		return ""
	}
	if custom := strings.TrimSpace(cfg.CustomInstruction); custom != "" {
		return custom
	}

	return fmt.Sprintf(
		"## Output Efficiency\n"+
			"- Between tool calls, use at most %d words. Be direct and action-oriented.\n"+
			"- In final responses to the user, use at most %d words. Lead with the answer.\n"+
			"- Do not restate what you just did unless the user asks. Do not narrate your process.",
		cfg.MaxWordsBetweenToolCalls,
		cfg.MaxWordsFinalResponse,
	)
}

// CountWords counts whitespace-separated words in s.
// Multiple consecutive whitespace characters (spaces, tabs, newlines) are
// treated as a single delimiter. Returns 0 for empty or whitespace-only input.
func CountWords(s string) int {
	return len(strings.Fields(s))
}

// ExceedsWordLimit reports whether text contains more words than limit.
// When limit is 0, it is treated as unlimited and the function always
// returns false. Exact equality (word count == limit) is NOT considered
// exceeding the limit.
func ExceedsWordLimit(text string, limit int) bool {
	if limit <= 0 {
		return false
	}
	return CountWords(text) > limit
}
