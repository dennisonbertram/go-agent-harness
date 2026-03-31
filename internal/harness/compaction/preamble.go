// Package compaction provides utilities for the context compaction / summarization
// pipeline, including the adversarial no-tools preamble that instructs the LLM
// not to call any tools during a summarization inference turn.
package compaction

import "strings"

// DefaultNoToolsPreamble is the adversarial instruction prepended to compaction
// prompts when NoToolsPreambleEnabled is true and no custom text is configured.
// It is intentionally emphatic to deter tool-calling behaviour in models that
// aggressively default to tool use.
const DefaultNoToolsPreamble = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools or functions.
This is a summarization task — you have exactly ONE turn to produce a text summary.
Tool calls will be REJECTED and will waste your only turn — you will fail the task.
Any attempt to use tools will result in task failure with no retry.`

// CompactionConfig holds configuration for the context compaction pipeline.
type CompactionConfig struct {
	// NoToolsPreambleEnabled controls whether the adversarial no-tools preamble
	// is prepended to every compaction/summarization prompt. When true, the
	// runner adds DefaultNoToolsPreamble (or NoToolsPreambleText if non-empty)
	// before the compaction prompt, discouraging the LLM from emitting tool calls
	// during the summarization inference turn.
	NoToolsPreambleEnabled bool `toml:"no_tools_preamble_enabled"`

	// NoToolsPreambleText is the custom preamble text to use when
	// NoToolsPreambleEnabled is true. When empty, DefaultNoToolsPreamble is used.
	NoToolsPreambleText string `toml:"no_tools_preamble_text"`
}

// NoToolsPreamble returns the preamble text to use for compaction prompts.
// Returns an empty string when preamble is disabled. Returns NoToolsPreambleText
// when non-empty, otherwise DefaultNoToolsPreamble.
func NoToolsPreamble(cfg CompactionConfig) string {
	if !cfg.NoToolsPreambleEnabled {
		return ""
	}
	if cfg.NoToolsPreambleText != "" {
		return cfg.NoToolsPreambleText
	}
	return DefaultNoToolsPreamble
}

// PrependPreamble adds the no-tools preamble before the compaction prompt.
// When preamble is disabled, the prompt is returned unmodified.
// When enabled, the preamble and prompt are separated by a double newline.
func PrependPreamble(prompt string, cfg CompactionConfig) string {
	p := NoToolsPreamble(cfg)
	if p == "" {
		return prompt
	}
	return p + "\n\n" + prompt
}

// HasToolCalls checks if a compaction response contains tool call patterns.
// It detects both Anthropic-style <tool_use> XML tags and OpenAI-style
// {"name": ...} function call JSON patterns.
// Returns false for empty strings or plain text responses.
func HasToolCalls(response string) bool {
	if response == "" {
		return false
	}
	// Detect Anthropic-style <tool_use> XML tag.
	if strings.Contains(response, "<tool_use") {
		return true
	}
	// Detect OpenAI-style function call JSON: {"name": ...} pattern.
	// We look for the literal substring `"name":` (with optional spacing)
	// within the response to avoid false positives on arbitrary JSON.
	if strings.Contains(response, `"name":`) || strings.Contains(response, `"name" :`) {
		return true
	}
	return false
}
