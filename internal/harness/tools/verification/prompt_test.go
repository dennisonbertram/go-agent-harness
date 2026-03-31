package verification_test

import (
	"strings"
	"testing"

	"go-agent-harness/internal/harness/tools/verification"
)

// TestFormatVerificationGuidance_Enabled verifies that enabled guidance contains
// anti-pattern names and the verdict format instruction.
func TestFormatVerificationGuidance_Enabled(t *testing.T) {
	cfg := verification.DefaultVerificationConfig()
	guidance := verification.FormatVerificationGuidance(cfg)

	if guidance == "" {
		t.Fatal("FormatVerificationGuidance() returned empty string with defaults enabled")
	}

	// Should contain verdict format instruction
	if !strings.Contains(guidance, "VERDICT:") {
		t.Error("FormatVerificationGuidance() missing 'VERDICT:' format instruction")
	}

	// Should contain anti-pattern section
	if !strings.Contains(guidance, "Anti-Pattern") {
		t.Error("FormatVerificationGuidance() missing anti-pattern section")
	}
}

// TestFormatVerificationGuidance_Disabled verifies that when anti-patterns are disabled,
// the function returns an empty string.
func TestFormatVerificationGuidance_Disabled(t *testing.T) {
	cfg := verification.VerificationConfig{
		RequireExecutableEvidence: false,
		NamedAntiPatternsEnabled:  false,
		VerdictFormat:             "",
	}
	guidance := verification.FormatVerificationGuidance(cfg)
	if guidance != "" {
		t.Errorf("FormatVerificationGuidance() = %q, want empty string when disabled", guidance)
	}
}

// TestFormatVerificationGuidance_ContainsAllAntiPatterns verifies all 3 named anti-patterns
// appear in the guidance text.
func TestFormatVerificationGuidance_ContainsAllAntiPatterns(t *testing.T) {
	cfg := verification.DefaultVerificationConfig()
	guidance := verification.FormatVerificationGuidance(cfg)

	expectedNames := []string{
		"Verification Avoidance",
		"First-80% Seduction",
		"Narration Over Evidence",
	}
	for _, name := range expectedNames {
		if !strings.Contains(guidance, name) {
			t.Errorf("FormatVerificationGuidance() missing anti-pattern name %q", name)
		}
	}
}

// TestFormatVerificationGuidance_ContainsCommandRunInstruction verifies the guidance
// includes "Command run:" as a required check element.
func TestFormatVerificationGuidance_ContainsCommandRunInstruction(t *testing.T) {
	cfg := verification.DefaultVerificationConfig()
	guidance := verification.FormatVerificationGuidance(cfg)

	if !strings.Contains(guidance, "Command run:") {
		t.Error("FormatVerificationGuidance() missing 'Command run:' instruction")
	}
}

// TestFormatVerificationGuidance_ContainsOutputObserved verifies the guidance
// includes "Output observed:" as a required check element.
func TestFormatVerificationGuidance_ContainsOutputObserved(t *testing.T) {
	cfg := verification.DefaultVerificationConfig()
	guidance := verification.FormatVerificationGuidance(cfg)

	if !strings.Contains(guidance, "Output observed:") {
		t.Error("FormatVerificationGuidance() missing 'Output observed:' instruction")
	}
}
