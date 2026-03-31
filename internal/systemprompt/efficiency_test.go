package systemprompt_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/systemprompt"
)

// ---------------------------------------------------------------------------
// CountWords tests
// ---------------------------------------------------------------------------

func TestCountWords_SimpleString(t *testing.T) {
	got := systemprompt.CountWords("hello world")
	if got != 2 {
		t.Errorf("CountWords(\"hello world\"): got %d, want 2", got)
	}
}

func TestCountWords_EmptyString(t *testing.T) {
	got := systemprompt.CountWords("")
	if got != 0 {
		t.Errorf("CountWords(\"\"): got %d, want 0", got)
	}
}

func TestCountWords_MultipleSpaces(t *testing.T) {
	got := systemprompt.CountWords("hello   world")
	if got != 2 {
		t.Errorf("CountWords(\"hello   world\"): got %d, want 2", got)
	}
}

func TestCountWords_Newlines(t *testing.T) {
	got := systemprompt.CountWords("hello\nworld\n")
	if got != 2 {
		t.Errorf("CountWords(\"hello\\nworld\\n\"): got %d, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// ExceedsWordLimit tests
// ---------------------------------------------------------------------------

func TestExceedsWordLimit_Under(t *testing.T) {
	// 5 words, limit 10 → false
	text := "one two three four five"
	if systemprompt.ExceedsWordLimit(text, 10) {
		t.Errorf("ExceedsWordLimit(5 words, 10): got true, want false")
	}
}

func TestExceedsWordLimit_Over(t *testing.T) {
	// 15 words, limit 10 → true
	text := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen"
	if !systemprompt.ExceedsWordLimit(text, 10) {
		t.Errorf("ExceedsWordLimit(15 words, 10): got false, want true")
	}
}

func TestExceedsWordLimit_Exact(t *testing.T) {
	// 10 words, limit 10 → false (equal is NOT over the limit)
	text := "one two three four five six seven eight nine ten"
	if systemprompt.ExceedsWordLimit(text, 10) {
		t.Errorf("ExceedsWordLimit(10 words, 10): got true, want false (exact limit is allowed)")
	}
}

func TestExceedsWordLimit_ZeroLimit(t *testing.T) {
	// limit 0 means unlimited → always false
	text := "one two three four five six seven eight nine ten eleven"
	if systemprompt.ExceedsWordLimit(text, 0) {
		t.Errorf("ExceedsWordLimit(11 words, 0): got true, want false (0 means unlimited)")
	}
}

// ---------------------------------------------------------------------------
// DefaultOutputEfficiencyConfig tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := systemprompt.DefaultOutputEfficiencyConfig()
	if !cfg.Enabled {
		t.Error("DefaultOutputEfficiencyConfig().Enabled: got false, want true")
	}
	if cfg.MaxWordsBetweenToolCalls != 25 {
		t.Errorf("DefaultOutputEfficiencyConfig().MaxWordsBetweenToolCalls: got %d, want 25", cfg.MaxWordsBetweenToolCalls)
	}
	if cfg.MaxWordsFinalResponse != 100 {
		t.Errorf("DefaultOutputEfficiencyConfig().MaxWordsFinalResponse: got %d, want 100", cfg.MaxWordsFinalResponse)
	}
	if cfg.CustomInstruction != "" {
		t.Errorf("DefaultOutputEfficiencyConfig().CustomInstruction: got %q, want empty string", cfg.CustomInstruction)
	}
}

// ---------------------------------------------------------------------------
// FormatEfficiencyAnchors tests
// ---------------------------------------------------------------------------

func TestFormatEfficiencyAnchors_Enabled(t *testing.T) {
	cfg := systemprompt.OutputEfficiencyConfig{
		Enabled:                  true,
		MaxWordsBetweenToolCalls: 25,
		MaxWordsFinalResponse:    100,
	}
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	if got == "" {
		t.Fatal("FormatEfficiencyAnchors(enabled=true): got empty string, want non-empty")
	}
	if !strings.Contains(got, "25") {
		t.Errorf("FormatEfficiencyAnchors: expected output to contain '25' (tool call limit), got:\n%s", got)
	}
	if !strings.Contains(got, "100") {
		t.Errorf("FormatEfficiencyAnchors: expected output to contain '100' (final response limit), got:\n%s", got)
	}
}

func TestFormatEfficiencyAnchors_Disabled(t *testing.T) {
	cfg := systemprompt.OutputEfficiencyConfig{
		Enabled:                  false,
		MaxWordsBetweenToolCalls: 25,
		MaxWordsFinalResponse:    100,
	}
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	if got != "" {
		t.Errorf("FormatEfficiencyAnchors(enabled=false): got %q, want empty string", got)
	}
}

func TestFormatEfficiencyAnchors_CustomInstruction(t *testing.T) {
	custom := "Be very brief. No more than 5 words between tool calls."
	cfg := systemprompt.OutputEfficiencyConfig{
		Enabled:                  true,
		MaxWordsBetweenToolCalls: 25,
		MaxWordsFinalResponse:    100,
		CustomInstruction:        custom,
	}
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	if got != custom {
		t.Errorf("FormatEfficiencyAnchors(custom): got %q, want %q", got, custom)
	}
}

func TestFormatEfficiencyAnchors_CustomLimits(t *testing.T) {
	cfg := systemprompt.OutputEfficiencyConfig{
		Enabled:                  true,
		MaxWordsBetweenToolCalls: 50,
		MaxWordsFinalResponse:    200,
	}
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	if !strings.Contains(got, "50") {
		t.Errorf("FormatEfficiencyAnchors(custom limits): expected output to contain '50', got:\n%s", got)
	}
	if !strings.Contains(got, "200") {
		t.Errorf("FormatEfficiencyAnchors(custom limits): expected output to contain '200', got:\n%s", got)
	}
	// Also verify the default values are NOT present (to confirm the custom limits took effect)
	if strings.Contains(got, "25 words") {
		t.Errorf("FormatEfficiencyAnchors(custom limits): output should not contain '25 words' (default), got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// TOML parsing test
// ---------------------------------------------------------------------------

// outputEfficiencyTOML mirrors the shape of the [output_efficiency] TOML block
// for parsing tests, without importing internal/config.
type outputEfficiencyTOML struct {
	OutputEfficiency *struct {
		Enabled                  *bool   `toml:"enabled"`
		MaxWordsBetweenToolCalls *int    `toml:"max_words_between_tool_calls"`
		MaxWordsFinalResponse    *int    `toml:"max_words_final_response"`
		CustomInstruction        *string `toml:"custom_instruction"`
	} `toml:"output_efficiency"`
}

func TestOutputEfficiencyConfig_FromTOML(t *testing.T) {
	raw := `
[output_efficiency]
enabled = true
max_words_between_tool_calls = 30
max_words_final_response = 150
custom_instruction = ""
`
	var parsed outputEfficiencyTOML
	if _, err := toml.Decode(raw, &parsed); err != nil {
		t.Fatalf("toml.Decode failed: %v", err)
	}
	if parsed.OutputEfficiency == nil {
		t.Fatal("parsed.OutputEfficiency is nil, want non-nil")
	}
	oe := parsed.OutputEfficiency
	if oe.Enabled == nil || !*oe.Enabled {
		t.Errorf("enabled: got %v, want true", oe.Enabled)
	}
	if oe.MaxWordsBetweenToolCalls == nil || *oe.MaxWordsBetweenToolCalls != 30 {
		t.Errorf("max_words_between_tool_calls: got %v, want 30", oe.MaxWordsBetweenToolCalls)
	}
	if oe.MaxWordsFinalResponse == nil || *oe.MaxWordsFinalResponse != 150 {
		t.Errorf("max_words_final_response: got %v, want 150", oe.MaxWordsFinalResponse)
	}
}

// ---------------------------------------------------------------------------
// Regression tests
// ---------------------------------------------------------------------------

// TestFormatEfficiencyAnchors_OutputContainsDoNotRestate ensures the generated
// anchor text instructs the agent not to restate prior actions — a key
// behavioral requirement of the efficiency feature.
func TestFormatEfficiencyAnchors_OutputContainsDoNotRestate(t *testing.T) {
	cfg := systemprompt.DefaultOutputEfficiencyConfig()
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	// The output should instruct the agent not to restate or narrate.
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "restat") && !strings.Contains(lower, "narrat") {
		t.Errorf("FormatEfficiencyAnchors output should mention 'restate' or 'narrate' to guide agent behavior, got:\n%s", got)
	}
}

// TestCountWords_TabSeparated ensures tab characters count as whitespace delimiters.
func TestCountWords_TabSeparated(t *testing.T) {
	got := systemprompt.CountWords("hello\tworld")
	if got != 2 {
		t.Errorf("CountWords(\"hello\\tworld\"): got %d, want 2", got)
	}
}

// TestExceedsWordLimit_EmptyText ensures empty text never exceeds any limit.
func TestExceedsWordLimit_EmptyText(t *testing.T) {
	if systemprompt.ExceedsWordLimit("", 5) {
		t.Error("ExceedsWordLimit(\"\", 5): got true, want false (empty text can't exceed limit)")
	}
}

// TestFormatEfficiencyAnchors_DefaultUsesDefaultValues ensures that using
// DefaultOutputEfficiencyConfig() produces output with both default limits.
// Regression: if DefaultOutputEfficiencyConfig values ever change, this fails.
func TestFormatEfficiencyAnchors_DefaultUsesDefaultValues(t *testing.T) {
	cfg := systemprompt.DefaultOutputEfficiencyConfig()
	got := systemprompt.FormatEfficiencyAnchors(cfg)
	if !strings.Contains(got, "25") {
		t.Errorf("FormatEfficiencyAnchors(default): expected output to contain default limit '25', got:\n%s", got)
	}
	if !strings.Contains(got, "100") {
		t.Errorf("FormatEfficiencyAnchors(default): expected output to contain default limit '100', got:\n%s", got)
	}
}
