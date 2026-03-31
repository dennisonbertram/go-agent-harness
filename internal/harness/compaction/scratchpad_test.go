package compaction_test

import (
	"strings"
	"testing"

	"go-agent-harness/internal/harness/compaction"
)

// ---------------------------------------------------------------------------
// BT-001: TestStripScratchpad_RemovesAnalysisBlock
// ---------------------------------------------------------------------------

func TestStripScratchpad_RemovesAnalysisBlock(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	input := `<analysis>
This is step-by-step thinking about what to preserve.
We need to keep the user's name and their task.
</analysis>
<summary>
User: Alice. Task: implement feature X. Status: in progress.
</summary>`

	result := compaction.StripScratchpad(input, cfg)

	if strings.Contains(result, "<analysis>") {
		t.Error("expected <analysis> block to be removed, but it is still present")
	}
	if strings.Contains(result, "step-by-step thinking") {
		t.Error("expected analysis content to be stripped, but found it in output")
	}
	if !strings.Contains(result, "User: Alice") {
		t.Errorf("expected summary content to be present in output, got: %q", result)
	}
	if strings.Contains(result, "<summary>") {
		t.Errorf("expected <summary> tags to be stripped, leaving only content; got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-002: TestStripScratchpad_NoAnalysisBlock
// ---------------------------------------------------------------------------

func TestStripScratchpad_NoAnalysisBlock(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	input := `<summary>
No analysis was included. Just a direct summary.
</summary>`

	result := compaction.StripScratchpad(input, cfg)

	if !strings.Contains(result, "No analysis was included") {
		t.Errorf("expected summary content to be preserved, got: %q", result)
	}
	if strings.Contains(result, "<summary>") {
		t.Errorf("expected summary tags to be stripped from output, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-003: TestStripScratchpad_NoSummaryTags
// ---------------------------------------------------------------------------

func TestStripScratchpad_NoSummaryTags(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	input := `This is raw output with no XML tags at all. Just plain text.`

	result := compaction.StripScratchpad(input, cfg)

	// When no summary tags found, must return full output as-is (graceful degradation)
	if result != input {
		t.Errorf("expected full output returned when no summary tags found, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-004: TestStripScratchpad_MultipleSummaryBlocks
// ---------------------------------------------------------------------------

func TestStripScratchpad_MultipleSummaryBlocks(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	input := `<analysis>thinking...</analysis>
<summary>Part one of summary.</summary>
<summary>Part two of summary.</summary>`

	result := compaction.StripScratchpad(input, cfg)

	if !strings.Contains(result, "Part one of summary") {
		t.Errorf("expected first summary block content in output, got: %q", result)
	}
	if !strings.Contains(result, "Part two of summary") {
		t.Errorf("expected second summary block content in output, got: %q", result)
	}
	if strings.Contains(result, "thinking...") {
		t.Errorf("expected analysis content to be stripped, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-005: TestStripScratchpad_NestedTags
// ---------------------------------------------------------------------------

func TestStripScratchpad_NestedTags(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	// The analysis block contains XML-like content that is NOT an analysis tag.
	input := `<analysis>
Consider this XML snippet: <foo>bar</foo> and <baz attr="1">val</baz>
These look like tags but are inside analysis.
</analysis>
<summary>
Final summary without nested tags.
</summary>`

	result := compaction.StripScratchpad(input, cfg)

	if strings.Contains(result, "<foo>") {
		t.Errorf("expected content inside analysis block to be stripped, got: %q", result)
	}
	if !strings.Contains(result, "Final summary without nested tags") {
		t.Errorf("expected summary content in output, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-006: TestStripScratchpad_Disabled
// ---------------------------------------------------------------------------

func TestStripScratchpad_Disabled(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: false, // stripping is disabled
	}

	input := `<analysis>important thinking here</analysis>
<summary>The actual summary.</summary>`

	result := compaction.StripScratchpad(input, cfg)

	// When strip_scratchpad=false, return output unmodified
	if result != input {
		t.Errorf("expected unmodified output when StripScratchpad=false, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-007: TestStripScratchpad_CustomTags
// ---------------------------------------------------------------------------

func TestStripScratchpad_CustomTags(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "think",
		SummaryTag:      "output",
		StripScratchpad: true,
	}

	input := `<think>My private reasoning goes here.</think>
<output>Final distilled output for the context.</output>`

	result := compaction.StripScratchpad(input, cfg)

	if strings.Contains(result, "private reasoning") {
		t.Errorf("expected custom scratchpad tag content to be stripped, got: %q", result)
	}
	if !strings.Contains(result, "Final distilled output") {
		t.Errorf("expected custom summary tag content to be preserved, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-008: TestExtractSummary_ValidOutput
// ---------------------------------------------------------------------------

func TestExtractSummary_ValidOutput(t *testing.T) {
	t.Parallel()

	input := `<analysis>scratch</analysis>
<summary>
  Clean summary text here.
</summary>`

	content, err := compaction.ExtractSummary(input, "summary")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(content, "Clean summary text here") {
		t.Errorf("expected extracted summary content, got: %q", content)
	}
}

// ---------------------------------------------------------------------------
// BT-009: TestExtractSummary_MissingSummaryTag
// ---------------------------------------------------------------------------

func TestExtractSummary_MissingSummaryTag(t *testing.T) {
	t.Parallel()

	input := `This is output with no summary tags at all.`

	_, err := compaction.ExtractSummary(input, "summary")
	if err == nil {
		t.Error("expected error when summary tag is missing, got nil")
	}
}

// ---------------------------------------------------------------------------
// BT-010: TestWrapCompactionPrompt_AddsInstructions
// ---------------------------------------------------------------------------

func TestWrapCompactionPrompt_AddsInstructions(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	base := "Summarize the conversation history."
	result := compaction.WrapCompactionPrompt(base, cfg)

	// The wrapper must instruct the model to think inside <analysis> tags
	if !strings.Contains(result, "<analysis>") {
		t.Errorf("expected wrap to mention <analysis> tag, got: %q", result)
	}
	// The wrapper must instruct the model to put the final output in <summary> tags
	if !strings.Contains(result, "<summary>") {
		t.Errorf("expected wrap to mention <summary> tag, got: %q", result)
	}
	// The base prompt must still be present
	if !strings.Contains(result, base) {
		t.Errorf("expected base prompt to be preserved in output, got: %q", result)
	}
	// The instructions must indicate analysis will be discarded
	lc := strings.ToLower(result)
	if !strings.Contains(lc, "discard") && !strings.Contains(lc, "discarded") && !strings.Contains(lc, "only the") {
		t.Errorf("expected instruction that analysis block will be discarded, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-011: TestWrapCompactionPrompt_Disabled
// ---------------------------------------------------------------------------

func TestWrapCompactionPrompt_Disabled(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         false, // scratchpad feature disabled
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	base := "Summarize the conversation history."
	result := compaction.WrapCompactionPrompt(base, cfg)

	// When disabled, returns base prompt unmodified
	if result != base {
		t.Errorf("expected base prompt returned unmodified when disabled, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// BT-012: TestScratchpadConfig_Defaults
// ---------------------------------------------------------------------------

func TestScratchpadConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := compaction.DefaultScratchpadConfig()

	if cfg.ScratchpadTag == "" {
		t.Error("expected default ScratchpadTag to be non-empty")
	}
	if cfg.ScratchpadTag != "analysis" {
		t.Errorf("expected default ScratchpadTag=%q, got %q", "analysis", cfg.ScratchpadTag)
	}
	if cfg.SummaryTag == "" {
		t.Error("expected default SummaryTag to be non-empty")
	}
	if cfg.SummaryTag != "summary" {
		t.Errorf("expected default SummaryTag=%q, got %q", "summary", cfg.SummaryTag)
	}
	// Default should have scratchpad enabled
	if !cfg.Enabled {
		t.Error("expected default Enabled=true")
	}
	// Default should have stripping enabled
	if !cfg.StripScratchpad {
		t.Error("expected default StripScratchpad=true")
	}
}

// ---------------------------------------------------------------------------
// BT-013: TestScratchpadConfig_FromTOML (config integration)
// ---------------------------------------------------------------------------

func TestScratchpadConfig_FromTOML(t *testing.T) {
	t.Parallel()

	// This test verifies that config.CompactionConfig maps into ScratchpadConfig.
	// We call the constructor/mapper that converts TOML-layer config into ScratchpadConfig.
	tomlCfg := compaction.CompactionConfig{
		ScratchpadEnabled: true,
		ScratchpadTag:     "think",
		SummaryTag:        "result",
		StripScratchpad:   false,
	}

	cfg := compaction.ScratchpadConfigFromCompaction(tomlCfg)

	if cfg.ScratchpadTag != "think" {
		t.Errorf("expected ScratchpadTag=%q, got %q", "think", cfg.ScratchpadTag)
	}
	if cfg.SummaryTag != "result" {
		t.Errorf("expected SummaryTag=%q, got %q", "result", cfg.SummaryTag)
	}
	if cfg.Enabled != true {
		t.Errorf("expected Enabled=true from TOML scratchpad_enabled=true")
	}
	if cfg.StripScratchpad != false {
		t.Errorf("expected StripScratchpad=false from TOML strip_scratchpad=false")
	}
}

// ---------------------------------------------------------------------------
// Regression tests
// ---------------------------------------------------------------------------

// TestStripScratchpad_EmptyInput verifies graceful handling of empty string.
func TestStripScratchpad_EmptyInput(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	result := compaction.StripScratchpad("", cfg)
	if result != "" {
		t.Errorf("expected empty output for empty input, got: %q", result)
	}
}

// TestStripScratchpad_OnlyScratchpad verifies analysis-only output is stripped to empty.
func TestStripScratchpad_OnlyScratchpad(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "analysis",
		SummaryTag:      "summary",
		StripScratchpad: true,
	}

	// Output has only an analysis block, no summary. Should return full output (graceful).
	input := `<analysis>All my thinking with no summary tag.</analysis>`
	result := compaction.StripScratchpad(input, cfg)

	// No summary tags found → return full output (graceful degradation, same as BT-003)
	if result != input {
		t.Errorf("expected full output when no summary tag present, got: %q", result)
	}
}

// TestWrapCompactionPrompt_CustomTags verifies the wrapped prompt uses the configured tags.
func TestWrapCompactionPrompt_CustomTags(t *testing.T) {
	t.Parallel()

	cfg := compaction.ScratchpadConfig{
		Enabled:         true,
		ScratchpadTag:   "think",
		SummaryTag:      "output",
		StripScratchpad: true,
	}

	result := compaction.WrapCompactionPrompt("base prompt", cfg)

	if !strings.Contains(result, "<think>") {
		t.Errorf("expected custom scratchpad tag <think> in wrapped prompt, got: %q", result)
	}
	if !strings.Contains(result, "<output>") {
		t.Errorf("expected custom summary tag <output> in wrapped prompt, got: %q", result)
	}
}

// TestExtractSummary_WhitespaceStripping verifies that leading/trailing whitespace
// is trimmed from extracted summary content.
func TestExtractSummary_WhitespaceStripping(t *testing.T) {
	t.Parallel()

	input := `<summary>

  Summary with leading and trailing whitespace.

</summary>`

	content, err := compaction.ExtractSummary(input, "summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(content, "\n") || strings.HasPrefix(content, " ") {
		t.Errorf("expected leading whitespace trimmed, got: %q", content)
	}
	if strings.HasSuffix(content, "\n") || strings.HasSuffix(content, " ") {
		t.Errorf("expected trailing whitespace trimmed, got: %q", content)
	}
	if !strings.Contains(content, "Summary with leading and trailing whitespace") {
		t.Errorf("expected actual content preserved, got: %q", content)
	}
}
