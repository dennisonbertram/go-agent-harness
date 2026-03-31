package compaction_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/harness/compaction"
)

// TestNoToolsPreamble_DefaultText verifies that NoToolsPreamble returns the
// DefaultNoToolsPreamble constant when no custom text is configured.
func TestNoToolsPreamble_DefaultText(t *testing.T) {
	t.Parallel()

	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
		NoToolsPreambleText:    "",
	}

	got := compaction.NoToolsPreamble(cfg)
	if got != compaction.DefaultNoToolsPreamble {
		t.Errorf("expected DefaultNoToolsPreamble, got %q", got)
	}
	// Extra assertion: the default preamble must be non-empty and contain the key adversarial phrase
	if !strings.Contains(got, "TEXT ONLY") {
		t.Error("DefaultNoToolsPreamble must contain 'TEXT ONLY'")
	}
}

// TestNoToolsPreamble_CustomText verifies that NoToolsPreamble returns the
// custom text when configured, not the default.
func TestNoToolsPreamble_CustomText(t *testing.T) {
	t.Parallel()

	customText := "DO NOT USE TOOLS. Summary only."
	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
		NoToolsPreambleText:    customText,
	}

	got := compaction.NoToolsPreamble(cfg)
	if got != customText {
		t.Errorf("expected custom preamble %q, got %q", customText, got)
	}
	// Verify it is NOT the default when a custom text is set
	if got == compaction.DefaultNoToolsPreamble {
		t.Error("expected custom preamble, got DefaultNoToolsPreamble")
	}
}

// TestNoToolsPreamble_Disabled verifies that NoToolsPreamble returns an empty
// string when preamble is disabled, regardless of configured text.
func TestNoToolsPreamble_Disabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  compaction.CompactionConfig
	}{
		{
			name: "disabled with empty text",
			cfg: compaction.CompactionConfig{
				NoToolsPreambleEnabled: false,
				NoToolsPreambleText:    "",
			},
		},
		{
			name: "disabled with custom text",
			cfg: compaction.CompactionConfig{
				NoToolsPreambleEnabled: false,
				NoToolsPreambleText:    "custom preamble",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compaction.NoToolsPreamble(tc.cfg)
			if got != "" {
				t.Errorf("expected empty string when disabled, got %q", got)
			}
		})
	}
}

// TestPrependPreamble_AddsBeforePrompt verifies that the preamble text appears
// before the original prompt content when enabled.
func TestPrependPreamble_AddsBeforePrompt(t *testing.T) {
	t.Parallel()

	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
		NoToolsPreambleText:    "",
	}
	prompt := "Please summarize the conversation."

	got := compaction.PrependPreamble(prompt, cfg)

	// Preamble must appear before the prompt
	preambleIdx := strings.Index(got, compaction.DefaultNoToolsPreamble)
	promptIdx := strings.Index(got, prompt)
	if preambleIdx < 0 {
		t.Error("preamble not found in output")
	}
	if promptIdx < 0 {
		t.Error("prompt not found in output")
	}
	if preambleIdx > promptIdx {
		t.Errorf("preamble (at %d) should appear before prompt (at %d)", preambleIdx, promptIdx)
	}
}

// TestPrependPreamble_Disabled verifies that the prompt is returned unmodified
// when preamble is disabled.
func TestPrependPreamble_Disabled(t *testing.T) {
	t.Parallel()

	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: false,
		NoToolsPreambleText:    "",
	}
	prompt := "Please summarize the conversation."

	got := compaction.PrependPreamble(prompt, cfg)
	if got != prompt {
		t.Errorf("expected prompt unchanged, got %q", got)
	}
}

// TestPrependPreamble_SeparatedByNewlines verifies that a double newline
// separates the preamble from the prompt content.
func TestPrependPreamble_SeparatedByNewlines(t *testing.T) {
	t.Parallel()

	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
		NoToolsPreambleText:    "STOP. No tools.",
	}
	prompt := "Summarize the conversation."

	got := compaction.PrependPreamble(prompt, cfg)

	// The preamble and prompt must be separated by at least two newlines
	if !strings.Contains(got, "STOP. No tools.\n\nSummarize the conversation.") {
		t.Errorf("expected double newline separator between preamble and prompt, got: %q", got)
	}
}

// TestHasToolCalls_DetectsToolUseXML verifies that HasToolCalls returns true
// when the response contains an Anthropic-style <tool_use> XML tag.
func TestHasToolCalls_DetectsToolUseXML(t *testing.T) {
	t.Parallel()

	responses := []string{
		`<tool_use>{"name": "bash", "input": {}}</tool_use>`,
		`Some text before <tool_use> and some after`,
		`<tool_use name="read">some content</tool_use>`,
	}
	for _, r := range responses {
		if !compaction.HasToolCalls(r) {
			t.Errorf("expected HasToolCalls=true for %q, got false", r)
		}
	}
}

// TestHasToolCalls_DetectsFunctionCall verifies that HasToolCalls returns true
// when the response contains OpenAI-style function call JSON patterns.
func TestHasToolCalls_DetectsFunctionCall(t *testing.T) {
	t.Parallel()

	responses := []string{
		`{"name": "bash", "arguments": {"cmd": "ls"}}`,
		`{"name":"read_file","arguments":{}}`,
		`Some prefix {"name": "tool"} some suffix`,
	}
	for _, r := range responses {
		if !compaction.HasToolCalls(r) {
			t.Errorf("expected HasToolCalls=true for %q, got false", r)
		}
	}
}

// TestHasToolCalls_NoToolCalls verifies that HasToolCalls returns false for
// plain text responses that contain no tool call patterns.
func TestHasToolCalls_NoToolCalls(t *testing.T) {
	t.Parallel()

	responses := []string{
		"Here is a summary of the conversation.",
		"The user asked about Go programming and the assistant explained goroutines.",
		"No tools were called in this interaction.",
		`{"key": "value"}`, // JSON without "name" key is not a tool call
	}
	for _, r := range responses {
		if compaction.HasToolCalls(r) {
			t.Errorf("expected HasToolCalls=false for %q, got true", r)
		}
	}
}

// TestHasToolCalls_EmptyString verifies that HasToolCalls returns false for
// an empty string input.
func TestHasToolCalls_EmptyString(t *testing.T) {
	t.Parallel()

	if compaction.HasToolCalls("") {
		t.Error("expected HasToolCalls=false for empty string, got true")
	}
}

// TestCompactionConfig_PreambleDefaults verifies that the zero-value
// CompactionConfig has sensible defaults (disabled by default).
func TestCompactionConfig_PreambleDefaults(t *testing.T) {
	t.Parallel()

	var cfg compaction.CompactionConfig

	// Preamble should be disabled by default (zero value is false)
	if cfg.NoToolsPreambleEnabled {
		t.Error("expected NoToolsPreambleEnabled=false by default")
	}
	// Custom text should be empty by default
	if cfg.NoToolsPreambleText != "" {
		t.Errorf("expected empty NoToolsPreambleText by default, got %q", cfg.NoToolsPreambleText)
	}
	// NoToolsPreamble returns empty when disabled
	if got := compaction.NoToolsPreamble(cfg); got != "" {
		t.Errorf("expected empty NoToolsPreamble for disabled config, got %q", got)
	}
}

// TestCompactionConfig_PreambleFromTOML verifies that CompactionConfig can be
// correctly decoded from TOML with preamble fields.
func TestCompactionConfig_PreambleFromTOML(t *testing.T) {
	t.Parallel()

	tomlStr := `
[compaction]
no_tools_preamble_enabled = true
no_tools_preamble_text = "Custom adversarial preamble."
`

	type container struct {
		Compaction compaction.CompactionConfig `toml:"compaction"`
	}

	var c container
	if _, err := toml.Decode(tomlStr, &c); err != nil {
		t.Fatalf("toml.Decode error: %v", err)
	}

	if !c.Compaction.NoToolsPreambleEnabled {
		t.Error("expected NoToolsPreambleEnabled=true after TOML decode")
	}
	if c.Compaction.NoToolsPreambleText != "Custom adversarial preamble." {
		t.Errorf("expected custom text, got %q", c.Compaction.NoToolsPreambleText)
	}
}

// --- Regression Tests ---

// TestRegression_PreamblePreservesPromptContent verifies that PrependPreamble
// never drops or truncates the original prompt content. If PrependPreamble were
// to return only the preamble or an empty string, this test catches it.
func TestRegression_PreamblePreservesPromptContent(t *testing.T) {
	t.Parallel()

	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
	}
	originalPrompt := "Please summarize the following conversation in three sentences."

	got := compaction.PrependPreamble(originalPrompt, cfg)
	if !strings.Contains(got, originalPrompt) {
		t.Errorf("original prompt not found in output — PrependPreamble dropped content.\nGot: %q", got)
	}
}

// TestRegression_DefaultPreambleContainsCriticalPhrases verifies that the
// DefaultNoToolsPreamble constant contains the key adversarial phrases that are
// essential for deterring tool use. If anyone edits the constant to remove these
// phrases, this test will fail.
func TestRegression_DefaultPreambleContainsCriticalPhrases(t *testing.T) {
	t.Parallel()

	required := []string{
		"TEXT ONLY",
		"Do NOT call any tools",
		"summarization task",
		"task failure",
	}
	for _, phrase := range required {
		if !strings.Contains(compaction.DefaultNoToolsPreamble, phrase) {
			t.Errorf("DefaultNoToolsPreamble missing required phrase %q", phrase)
		}
	}
}

// TestRegression_HasToolCalls_PartialXMLTag verifies that HasToolCalls does not
// false-positive on text that contains a partial substring of "<tool_use" without
// the actual angle-bracket tag. This guards against overly greedy substring matching.
func TestRegression_HasToolCalls_PartialXMLTag(t *testing.T) {
	t.Parallel()

	// "tool_use" without the opening < — should NOT be detected
	plainText := "tool_use is a concept discussed in the conversation"
	if compaction.HasToolCalls(plainText) {
		t.Errorf("expected HasToolCalls=false for %q (no XML tag), got true", plainText)
	}
}

// TestRegression_PrependPreamble_CustomTextTakesPrecedence verifies that a
// custom preamble text is used verbatim and the DefaultNoToolsPreamble is not
// injected alongside it. If implementation accidentally injects both, this fails.
func TestRegression_PrependPreamble_CustomTextTakesPrecedence(t *testing.T) {
	t.Parallel()

	custom := "HALT. No function calls permitted."
	cfg := compaction.CompactionConfig{
		NoToolsPreambleEnabled: true,
		NoToolsPreambleText:    custom,
	}
	prompt := "Summarize."

	got := compaction.PrependPreamble(prompt, cfg)

	// Custom text must appear
	if !strings.Contains(got, custom) {
		t.Errorf("custom preamble not found in output: %q", got)
	}
	// DefaultNoToolsPreamble must NOT appear (the custom text replaced it)
	if strings.Contains(got, compaction.DefaultNoToolsPreamble) {
		t.Errorf("DefaultNoToolsPreamble should not appear when custom text is set: %q", got)
	}
}
