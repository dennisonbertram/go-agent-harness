// Package compaction provides utilities for processing LLM-generated compaction output.
// It supports an <analysis> scratchpad pattern where the model thinks step-by-step
// inside a scratchpad block before emitting a final summary.
package compaction

import (
	"fmt"
	"regexp"
	"strings"
)

// CompactionConfig is the TOML-layer configuration for the [compaction] section.
// This struct maps directly from TOML fields via the config package.
type CompactionConfig struct {
	// ScratchpadEnabled controls whether the <analysis> scratchpad is enabled.
	ScratchpadEnabled bool `toml:"scratchpad_enabled"`
	// ScratchpadTag is the XML tag name used for the scratchpad block (default: "analysis").
	ScratchpadTag string `toml:"scratchpad_tag"`
	// SummaryTag is the XML tag name used for the final summary (default: "summary").
	SummaryTag string `toml:"summary_tag"`
	// StripScratchpad controls whether the scratchpad block is stripped before
	// injecting the output into the context (default: true).
	StripScratchpad bool `toml:"strip_scratchpad"`
}

// ScratchpadConfig is the resolved runtime configuration for the scratchpad feature.
// It is derived from CompactionConfig after applying defaults.
type ScratchpadConfig struct {
	// Enabled controls whether the scratchpad feature is active.
	Enabled bool
	// ScratchpadTag is the XML tag name for the scratchpad block.
	ScratchpadTag string
	// SummaryTag is the XML tag name for the final summary.
	SummaryTag string
	// StripScratchpad controls whether the scratchpad block is stripped from output.
	StripScratchpad bool
}

// DefaultScratchpadConfig returns the default ScratchpadConfig with canonical defaults.
func DefaultScratchpadConfig() ScratchpadConfig {
	return ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}
}

// ScratchpadConfigFromCompaction converts a TOML CompactionConfig into a ScratchpadConfig.
func ScratchpadConfigFromCompaction(cc CompactionConfig) ScratchpadConfig {
	return ScratchpadConfig{
		Enabled:         cc.ScratchpadEnabled,
		ScratchpadTag:   cc.ScratchpadTag,
		SummaryTag:      cc.SummaryTag,
		StripScratchpad: cc.StripScratchpad,
	}
}

// StripScratchpad removes <scratchpadTag>...</scratchpadTag> blocks from compaction output
// and returns only the <summaryTag>...</summaryTag> content.
//
// Behaviour:
//   - When cfg.StripScratchpad is false, returns output unmodified.
//   - When cfg.Enabled is false, returns output unmodified.
//   - When no summary tags are found, returns the full output (graceful degradation).
//   - When multiple summary blocks are present, concatenates their content.
func StripScratchpad(output string, cfg ScratchpadConfig) string {
	if !cfg.Enabled || !cfg.StripScratchpad {
		return output
	}

	// Extract all <summary>...</summary> blocks.
	summaryContents, found := extractAllTagContents(output, cfg.SummaryTag)
	if !found {
		// Graceful degradation: no summary tags found, return full output.
		return output
	}

	return strings.Join(summaryContents, "\n")
}

// ExtractSummary extracts the text content between <summaryTag>...</summaryTag> tags.
// Returns an error if no summary tags are found.
// Leading and trailing whitespace is trimmed from the extracted content.
func ExtractSummary(output string, summaryTag string) (string, error) {
	contents, found := extractAllTagContents(output, summaryTag)
	if !found {
		return "", fmt.Errorf("no <%s> tags found in compaction output", summaryTag)
	}
	return strings.TrimSpace(strings.Join(contents, "\n")), nil
}

// WrapCompactionPrompt adds scratchpad instructions to the compaction prompt.
// When cfg.Enabled is false, the base prompt is returned unmodified.
//
// The wrapped prompt instructs the model to:
//  1. Think step-by-step inside <scratchpadTag> tags.
//  2. Output the final summary inside <summaryTag> tags.
//  3. Note that the scratchpad block will be discarded.
func WrapCompactionPrompt(basePrompt string, cfg ScratchpadConfig) string {
	if !cfg.Enabled {
		return basePrompt
	}

	instructions := fmt.Sprintf(
		"First, think step-by-step inside <%[1]s> tags about what information is critical to preserve.\n"+
			"Then output your final summary inside <%[2]s> tags.\n\n"+
			"Your <%[1]s> block will be discarded — only the <%[2]s> content will be retained in context.\n\n",
		cfg.ScratchpadTag,
		cfg.SummaryTag,
	)
	return instructions + basePrompt
}

// extractAllTagContents finds all occurrences of <tag>...</tag> and returns their
// inner text contents. Returns (nil, false) if no matches are found.
// The regex uses a non-greedy match to handle multiple blocks correctly.
func extractAllTagContents(s, tag string) ([]string, bool) {
	// Build a regex like `(?s)<tag>(.*?)</tag>` where (?s) enables dot-matches-newline.
	pattern := fmt.Sprintf(`(?s)<%s>(.*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag))
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil, false
	}

	contents := make([]string, 0, len(matches))
	for _, m := range matches {
		contents = append(contents, strings.TrimSpace(m[1]))
	}
	return contents, true
}
