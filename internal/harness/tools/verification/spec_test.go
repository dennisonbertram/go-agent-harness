package verification_test

import (
	"testing"

	"go-agent-harness/internal/harness/tools/verification"
)

// TestAntiPatterns_ReturnsThreePatterns verifies exactly 3 anti-patterns are returned.
func TestAntiPatterns_ReturnsThreePatterns(t *testing.T) {
	patterns := verification.AntiPatterns()
	if len(patterns) != 3 {
		t.Errorf("AntiPatterns() returned %d patterns, want exactly 3", len(patterns))
	}
}

// TestAntiPatterns_AllHaveNames verifies all anti-patterns have non-empty Name fields.
func TestAntiPatterns_AllHaveNames(t *testing.T) {
	patterns := verification.AntiPatterns()
	for i, p := range patterns {
		if p.Name == "" {
			t.Errorf("AntiPatterns()[%d].Name is empty", i)
		}
	}
}

// TestAntiPatterns_AllHaveDescriptions verifies all anti-patterns have non-empty Description fields.
func TestAntiPatterns_AllHaveDescriptions(t *testing.T) {
	patterns := verification.AntiPatterns()
	for i, p := range patterns {
		if p.Description == "" {
			t.Errorf("AntiPatterns()[%d].Description is empty (Name=%q)", i, p.Name)
		}
	}
}

// TestAntiPatterns_AllHaveDetection verifies all anti-patterns have non-empty Detection fields.
func TestAntiPatterns_AllHaveDetection(t *testing.T) {
	patterns := verification.AntiPatterns()
	for i, p := range patterns {
		if p.Detection == "" {
			t.Errorf("AntiPatterns()[%d].Detection is empty (Name=%q)", i, p.Name)
		}
	}
}

// TestAntiPatterns_VerificationAvoidancePresent verifies the "Verification Avoidance" pattern exists.
func TestAntiPatterns_VerificationAvoidancePresent(t *testing.T) {
	patterns := verification.AntiPatterns()
	for _, p := range patterns {
		if p.Name == "Verification Avoidance" {
			return
		}
	}
	t.Error("AntiPatterns() does not contain a pattern named 'Verification Avoidance'")
}

// TestAntiPatterns_FirstEightyPercentPresent verifies the "First-80% Seduction" pattern exists.
func TestAntiPatterns_FirstEightyPercentPresent(t *testing.T) {
	patterns := verification.AntiPatterns()
	for _, p := range patterns {
		if p.Name == "First-80% Seduction" {
			return
		}
	}
	t.Error("AntiPatterns() does not contain a pattern named 'First-80% Seduction'")
}

// TestAntiPatterns_NarrationOverEvidencePresent verifies the "Narration Over Evidence" pattern exists.
func TestAntiPatterns_NarrationOverEvidencePresent(t *testing.T) {
	patterns := verification.AntiPatterns()
	for _, p := range patterns {
		if p.Name == "Narration Over Evidence" {
			return
		}
	}
	t.Error("AntiPatterns() does not contain a pattern named 'Narration Over Evidence'")
}

// TestDefaultVerificationConfig verifies defaults: require_evidence=true, anti_patterns=true.
func TestDefaultVerificationConfig(t *testing.T) {
	cfg := verification.DefaultVerificationConfig()

	if !cfg.RequireExecutableEvidence {
		t.Error("DefaultVerificationConfig().RequireExecutableEvidence = false, want true")
	}
	if !cfg.NamedAntiPatternsEnabled {
		t.Error("DefaultVerificationConfig().NamedAntiPatternsEnabled = false, want true")
	}
	if cfg.VerdictFormat == "" {
		t.Error("DefaultVerificationConfig().VerdictFormat is empty, want a non-empty format string")
	}
}
