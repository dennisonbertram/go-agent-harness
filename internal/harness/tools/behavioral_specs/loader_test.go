package behavioral_specs_test

import (
	"strings"
	"testing"

	"go-agent-harness/internal/harness/tools/behavioral_specs"
)

// TestLoadSpec_ExistingTool verifies that loading a known spec (bash.md) populates all sections.
func TestLoadSpec_ExistingTool(t *testing.T) {
	spec, err := behavioral_specs.LoadSpec("bash")
	if err != nil {
		t.Fatalf("LoadSpec(\"bash\") unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("LoadSpec(\"bash\") returned nil spec, want non-nil")
	}
	if strings.TrimSpace(spec.WhenToUse) == "" {
		t.Error("spec.WhenToUse: got empty string, want non-empty")
	}
	if strings.TrimSpace(spec.WhenNotToUse) == "" {
		t.Error("spec.WhenNotToUse: got empty string, want non-empty")
	}
	if len(spec.BehavioralRules) == 0 {
		t.Error("spec.BehavioralRules: got empty slice, want at least one rule")
	}
	if len(spec.CommonMistakes) == 0 {
		t.Error("spec.CommonMistakes: got empty slice, want at least one mistake")
	}
	if len(spec.Examples) == 0 {
		t.Error("spec.Examples: got empty slice, want at least one example")
	}
}

// TestLoadSpec_NonExistentTool verifies graceful degradation — returns nil, no error.
func TestLoadSpec_NonExistentTool(t *testing.T) {
	spec, err := behavioral_specs.LoadSpec("nonexistent_tool_xyz_404")
	if err != nil {
		t.Fatalf("LoadSpec(nonexistent) unexpected error: %v", err)
	}
	if spec != nil {
		t.Errorf("LoadSpec(nonexistent): got non-nil spec %+v, want nil", spec)
	}
}

// TestFormatSpec_AllSectionsEnabled verifies that all sections appear when all config toggles are true.
func TestFormatSpec_AllSectionsEnabled(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:       "use for X",
		WhenNotToUse:    "not for Y",
		BehavioralRules: []string{"Rule 1", "Rule 2"},
		CommonMistakes:  []string{"**BadPattern**: description here"},
		Examples: []behavioral_specs.SpecExample{
			{Label: "Test", Wrong: "bad way", Right: "good way"},
		},
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled:              true,
		IncludeWhenNotToUse:  true,
		IncludeAntiPatterns:  true,
		IncludeCommonMistakes: true,
		MaxSpecLength:        0,
	}
	result := behavioral_specs.FormatSpec(spec, cfg)

	if !strings.Contains(result, "When to Use") {
		t.Error("FormatSpec: expected 'When to Use' section, not found")
	}
	if !strings.Contains(result, "When NOT to Use") {
		t.Error("FormatSpec: expected 'When NOT to Use' section, not found")
	}
	if !strings.Contains(result, "Behavioral Rules") {
		t.Error("FormatSpec: expected 'Behavioral Rules' section, not found")
	}
	if !strings.Contains(result, "Common Mistakes") {
		t.Error("FormatSpec: expected 'Common Mistakes' section, not found")
	}
	if !strings.Contains(result, "Examples") {
		t.Error("FormatSpec: expected 'Examples' section, not found")
	}
}

// TestFormatSpec_WhenNotToUseDisabled verifies the toggle hides that section.
func TestFormatSpec_WhenNotToUseDisabled(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:    "use for X",
		WhenNotToUse: "not for Y",
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled:             true,
		IncludeWhenNotToUse: false,
	}
	result := behavioral_specs.FormatSpec(spec, cfg)

	if strings.Contains(result, "When NOT to Use") {
		t.Error("FormatSpec with IncludeWhenNotToUse=false: 'When NOT to Use' section should be absent, but found")
	}
	if !strings.Contains(result, "When to Use") {
		t.Error("FormatSpec: 'When to Use' section should still be present")
	}
}

// TestFormatSpec_AntiPatternsDisabled verifies the anti-patterns toggle hides Common Mistakes.
func TestFormatSpec_AntiPatternsDisabled(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:      "use for X",
		CommonMistakes: []string{"**BadPattern**: description"},
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled:               true,
		IncludeWhenNotToUse:   true,
		IncludeAntiPatterns:   false,
		IncludeCommonMistakes: false,
	}
	result := behavioral_specs.FormatSpec(spec, cfg)

	if strings.Contains(result, "Common Mistakes") {
		t.Error("FormatSpec with IncludeAntiPatterns=false: 'Common Mistakes' section should be absent, but found")
	}
}

// TestFormatSpec_MaxLength verifies output is truncated to MaxSpecLength characters.
func TestFormatSpec_MaxLength(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:       strings.Repeat("x", 500),
		WhenNotToUse:    strings.Repeat("y", 500),
		BehavioralRules: []string{strings.Repeat("z", 500)},
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled:             true,
		IncludeWhenNotToUse: true,
		MaxSpecLength:       200,
	}
	result := behavioral_specs.FormatSpec(spec, cfg)

	if len(result) > 200 {
		t.Errorf("FormatSpec with MaxSpecLength=200: got len=%d, want <= 200", len(result))
	}
}

// TestFormatSpec_ZeroMaxLength verifies no truncation occurs when MaxSpecLength is 0.
func TestFormatSpec_ZeroMaxLength(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:    strings.Repeat("a", 3000),
		WhenNotToUse: strings.Repeat("b", 3000),
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled:             true,
		IncludeWhenNotToUse: true,
		MaxSpecLength:       0, // 0 = unlimited
	}
	result := behavioral_specs.FormatSpec(spec, cfg)

	if len(result) < 5000 {
		t.Errorf("FormatSpec with MaxSpecLength=0: got len=%d, want >= 5000 (no truncation)", len(result))
	}
}

// TestLoadSpec_AllPriorityToolsHaveSpecs is a regression test that verifies all
// 10 priority tool specs exist and are non-empty. If any spec file is deleted or
// renamed, this test fails immediately.
func TestLoadSpec_AllPriorityToolsHaveSpecs(t *testing.T) {
	priorityTools := []string{
		"bash",
		"file_edit",
		"file_read",
		"file_write",
		"run_agent",
		"find_tool",
		"web_search",
		"grep",
		"glob",
		"apply_patch",
	}
	for _, name := range priorityTools {
		t.Run(name, func(t *testing.T) {
			spec, err := behavioral_specs.LoadSpec(name)
			if err != nil {
				t.Fatalf("LoadSpec(%q) unexpected error: %v", name, err)
			}
			if spec == nil {
				t.Fatalf("LoadSpec(%q) returned nil, want non-nil spec", name)
			}
			if strings.TrimSpace(spec.WhenToUse) == "" {
				t.Errorf("LoadSpec(%q).WhenToUse is empty", name)
			}
		})
	}
}

// TestFormatSpec_DisabledReturnsEmpty is a regression test that if the Enabled
// toggle is false, FormatSpec must return an empty string — not partial output.
func TestFormatSpec_DisabledReturnsEmpty(t *testing.T) {
	spec := &behavioral_specs.BehavioralSpec{
		WhenToUse:    "use for X",
		WhenNotToUse: "not for Y",
	}
	cfg := behavioral_specs.BehavioralSpecConfig{
		Enabled: false,
	}
	result := behavioral_specs.FormatSpec(spec, cfg)
	if result != "" {
		t.Errorf("FormatSpec with Enabled=false: got %q, want empty string", result)
	}
}

// TestParseMarkdownSections verifies the markdown parser correctly populates all struct fields.
func TestParseMarkdownSections(t *testing.T) {
	markdown := `## When to Use
- Positive example one
- Positive example two

## When NOT to Use
- Negative example one

## Behavioral Rules
1. Rule number one
2. Rule number two
3. Rule number three

## Common Mistakes
- **FirstAntiPattern**: This is what goes wrong
- **SecondAntiPattern**: Another common error

## Examples
### WRONG
Do it the bad way
### RIGHT
Do it the good way
`
	spec, err := behavioral_specs.ParseMarkdown(markdown)
	if err != nil {
		t.Fatalf("ParseMarkdown() unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("ParseMarkdown() returned nil, want non-nil")
	}
	if !strings.Contains(spec.WhenToUse, "Positive example one") {
		t.Errorf("WhenToUse: got %q, want to contain 'Positive example one'", spec.WhenToUse)
	}
	if !strings.Contains(spec.WhenNotToUse, "Negative example one") {
		t.Errorf("WhenNotToUse: got %q, want to contain 'Negative example one'", spec.WhenNotToUse)
	}
	if len(spec.BehavioralRules) != 3 {
		t.Errorf("BehavioralRules: got %d rules, want 3", len(spec.BehavioralRules))
	}
	if len(spec.CommonMistakes) != 2 {
		t.Errorf("CommonMistakes: got %d mistakes, want 2", len(spec.CommonMistakes))
	}
	if len(spec.Examples) != 1 {
		t.Errorf("Examples: got %d examples, want 1", len(spec.Examples))
	}
	if spec.Examples[0].Wrong == "" {
		t.Error("Examples[0].Wrong: got empty, want non-empty")
	}
	if spec.Examples[0].Right == "" {
		t.Error("Examples[0].Right: got empty, want non-empty")
	}
}
