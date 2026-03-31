package symphd

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestFormatSynthesisDoctrine_Enabled verifies that an enabled config
// produces a non-empty doctrine string containing anti-patterns.
func TestFormatSynthesisDoctrine_Enabled(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
		RequireLineReferences: false,
		CustomDoctrineText:    "",
	}
	got := FormatSynthesisDoctrine(cfg)
	if got == "" {
		t.Error("FormatSynthesisDoctrine with Enabled=true returned empty string, want non-empty")
	}
	if !strings.Contains(got, "Synthesis") {
		t.Errorf("doctrine does not contain 'Synthesis': %q", got)
	}
}

// TestFormatSynthesisDoctrine_Disabled verifies that a disabled config
// produces an empty string.
func TestFormatSynthesisDoctrine_Disabled(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled: false,
	}
	got := FormatSynthesisDoctrine(cfg)
	if got != "" {
		t.Errorf("FormatSynthesisDoctrine with Enabled=false returned %q, want empty string", got)
	}
}

// TestFormatSynthesisDoctrine_CustomText verifies that when CustomDoctrineText is set,
// the returned string equals the custom text (and not the default doctrine).
func TestFormatSynthesisDoctrine_CustomText(t *testing.T) {
	const custom = "My custom synthesis doctrine text."
	cfg := SynthesisDoctrineConfig{
		Enabled:            true,
		CustomDoctrineText: custom,
	}
	got := FormatSynthesisDoctrine(cfg)
	if got != custom {
		t.Errorf("FormatSynthesisDoctrine with CustomDoctrineText: got %q, want %q", got, custom)
	}
}

// TestFormatSynthesisDoctrine_ContainsAntiPatterns verifies that all three
// anti-pattern names appear in the doctrine output.
func TestFormatSynthesisDoctrine_ContainsAntiPatterns(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
	}
	got := FormatSynthesisDoctrine(cfg)

	patterns := SynthesisAntiPatterns()
	for _, p := range patterns {
		if !strings.Contains(got, p.Name) {
			t.Errorf("doctrine does not contain anti-pattern name %q", p.Name)
		}
	}
}

// TestFormatSynthesisDoctrine_ContainsWrongExamples verifies that WRONG examples
// appear in the doctrine output.
func TestFormatSynthesisDoctrine_ContainsWrongExamples(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
	}
	got := FormatSynthesisDoctrine(cfg)

	patterns := SynthesisAntiPatterns()
	for _, p := range patterns {
		if !strings.Contains(got, p.Wrong) {
			t.Errorf("doctrine does not contain WRONG example for anti-pattern %q: %q", p.Name, p.Wrong)
		}
	}
}

// TestFormatSynthesisDoctrine_ContainsRightExamples verifies that RIGHT examples
// appear in the doctrine output.
func TestFormatSynthesisDoctrine_ContainsRightExamples(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
	}
	got := FormatSynthesisDoctrine(cfg)

	patterns := SynthesisAntiPatterns()
	for _, p := range patterns {
		if !strings.Contains(got, p.Right) {
			t.Errorf("doctrine does not contain RIGHT example for anti-pattern %q: %q", p.Name, p.Right)
		}
	}
}

// TestFormatSynthesisDoctrine_RequireFileReferences verifies that when
// RequireFileReferences is true, the doctrine mentions file path requirements.
func TestFormatSynthesisDoctrine_RequireFileReferences(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               true,
		RequireFileReferences: true,
	}
	got := FormatSynthesisDoctrine(cfg)

	// Must mention the file path requirement explicitly.
	if !strings.Contains(got, "file path") {
		t.Errorf("doctrine with RequireFileReferences=true does not mention 'file path': %q", got)
	}
}

// TestSynthesisAntiPatterns_ReturnsThree verifies exactly 3 anti-patterns are defined.
func TestSynthesisAntiPatterns_ReturnsThree(t *testing.T) {
	got := SynthesisAntiPatterns()
	if len(got) != 3 {
		t.Errorf("SynthesisAntiPatterns() returned %d patterns, want 3", len(got))
	}
}

// TestSynthesisAntiPatterns_AllHaveWrongAndRight verifies every anti-pattern
// has non-empty Wrong and Right fields.
func TestSynthesisAntiPatterns_AllHaveWrongAndRight(t *testing.T) {
	patterns := SynthesisAntiPatterns()
	for i, p := range patterns {
		if p.Wrong == "" {
			t.Errorf("anti-pattern[%d] %q has empty Wrong field", i, p.Name)
		}
		if p.Right == "" {
			t.Errorf("anti-pattern[%d] %q has empty Right field", i, p.Name)
		}
	}
}

// TestDefaultSynthesisDoctrineConfig verifies the defaults:
// enabled=true, file_refs=true, line_refs=false.
func TestDefaultSynthesisDoctrineConfig(t *testing.T) {
	cfg := DefaultSynthesisDoctrineConfig()
	if !cfg.Enabled {
		t.Error("DefaultSynthesisDoctrineConfig: Enabled=false, want true")
	}
	if !cfg.RequireFileReferences {
		t.Error("DefaultSynthesisDoctrineConfig: RequireFileReferences=false, want true")
	}
	if cfg.RequireLineReferences {
		t.Error("DefaultSynthesisDoctrineConfig: RequireLineReferences=true, want false")
	}
	if cfg.CustomDoctrineText != "" {
		t.Errorf("DefaultSynthesisDoctrineConfig: CustomDoctrineText=%q, want empty", cfg.CustomDoctrineText)
	}
}

// coordinatorTOML is a minimal TOML document used to test [coordinator] TOML parsing.
// The SynthesisDoctrineConfig fields are nested under [coordinator].
const coordinatorTOML = `
[coordinator]
synthesis_doctrine_enabled = true
require_file_references = true
require_line_references = false
custom_doctrine_text = ""
`

// rawCoordinatorLayer is a helper struct that mirrors how [coordinator] would
// be embedded in a parent config structure for TOML parsing tests.
type rawCoordinatorLayer struct {
	Coordinator rawCoordinatorConfig `toml:"coordinator"`
}

type rawCoordinatorConfig struct {
	SynthesisDoctrineEnabled bool   `toml:"synthesis_doctrine_enabled"`
	RequireFileReferences    bool   `toml:"require_file_references"`
	RequireLineReferences    bool   `toml:"require_line_references"`
	CustomDoctrineText       string `toml:"custom_doctrine_text"`
}

// --- Regression Tests ---

// TestRegression_DisabledDoctrineProducesNoOutput ensures that once disabled,
// no doctrine text leaks into prompts regardless of other field values.
func TestRegression_DisabledDoctrineProducesNoOutput(t *testing.T) {
	cfg := SynthesisDoctrineConfig{
		Enabled:               false,
		RequireFileReferences: true,
		RequireLineReferences: true,
		CustomDoctrineText:    "should not appear",
	}
	got := FormatSynthesisDoctrine(cfg)
	if got != "" {
		t.Errorf("disabled doctrine with non-empty custom text produced output %q, want empty string", got)
	}
}

// TestRegression_AntiPatternNamesAreUnique ensures no two anti-patterns share a name,
// which would make the doctrine text ambiguous when checking for presence.
func TestRegression_AntiPatternNamesAreUnique(t *testing.T) {
	patterns := SynthesisAntiPatterns()
	seen := make(map[string]int)
	for i, p := range patterns {
		if prev, ok := seen[p.Name]; ok {
			t.Errorf("anti-pattern[%d] has duplicate name %q (first seen at index %d)", i, p.Name, prev)
		}
		seen[p.Name] = i
	}
}

// TestRegression_DefaultConfigEnabledAndFileRefs ensures the default config never
// silently disables the doctrine or removes file-ref requirements — callers depend
// on these defaults to be protective by default.
func TestRegression_DefaultConfigEnabledAndFileRefs(t *testing.T) {
	cfg := DefaultSynthesisDoctrineConfig()
	doc := FormatSynthesisDoctrine(cfg)
	if doc == "" {
		t.Error("regression: default config produced empty doctrine; default should be enabled")
	}
	if !strings.Contains(doc, "file path") {
		t.Error("regression: default doctrine lost file path requirement text")
	}
}

// TestRegression_CustomTextNotWrapped ensures that custom doctrine text is returned
// verbatim without any wrapping headers or footers added by the formatter.
func TestRegression_CustomTextNotWrapped(t *testing.T) {
	const custom = "Only this text."
	cfg := SynthesisDoctrineConfig{
		Enabled:            true,
		CustomDoctrineText: custom,
	}
	got := FormatSynthesisDoctrine(cfg)
	if got != custom {
		t.Errorf("custom text was wrapped: got %q, want exactly %q", got, custom)
	}
}

// TestSynthesisDoctrineConfig_FromTOML verifies that a [coordinator] TOML
// section can be decoded and maps correctly to SynthesisDoctrineConfig fields.
func TestSynthesisDoctrineConfig_FromTOML(t *testing.T) {
	var layer rawCoordinatorLayer
	if _, err := toml.Decode(coordinatorTOML, &layer); err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}

	got := SynthesisDoctrineConfig{
		Enabled:               layer.Coordinator.SynthesisDoctrineEnabled,
		RequireFileReferences: layer.Coordinator.RequireFileReferences,
		RequireLineReferences: layer.Coordinator.RequireLineReferences,
		CustomDoctrineText:    layer.Coordinator.CustomDoctrineText,
	}

	if !got.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if !got.RequireFileReferences {
		t.Error("RequireFileReferences: got false, want true")
	}
	if got.RequireLineReferences {
		t.Error("RequireLineReferences: got true, want false")
	}
	if got.CustomDoctrineText != "" {
		t.Errorf("CustomDoctrineText: got %q, want empty", got.CustomDoctrineText)
	}
}
