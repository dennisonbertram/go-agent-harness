package verification_test

import (
	"testing"

	"go-agent-harness/internal/harness/tools/verification"
)

// TestParseVerdict_Pass verifies "VERDICT: PASS" parses to VerdictPass.
func TestParseVerdict_Pass(t *testing.T) {
	v, err := verification.ParseVerdict("VERDICT: PASS")
	if err != nil {
		t.Fatalf("ParseVerdict() unexpected error: %v", err)
	}
	if v != verification.VerdictPass {
		t.Errorf("ParseVerdict() = %q, want %q", v, verification.VerdictPass)
	}
}

// TestParseVerdict_Fail verifies "VERDICT: FAIL" parses to VerdictFail.
func TestParseVerdict_Fail(t *testing.T) {
	v, err := verification.ParseVerdict("VERDICT: FAIL")
	if err != nil {
		t.Fatalf("ParseVerdict() unexpected error: %v", err)
	}
	if v != verification.VerdictFail {
		t.Errorf("ParseVerdict() = %q, want %q", v, verification.VerdictFail)
	}
}

// TestParseVerdict_Partial verifies "VERDICT: PARTIAL" parses to VerdictPartial.
func TestParseVerdict_Partial(t *testing.T) {
	v, err := verification.ParseVerdict("VERDICT: PARTIAL")
	if err != nil {
		t.Fatalf("ParseVerdict() unexpected error: %v", err)
	}
	if v != verification.VerdictPartial {
		t.Errorf("ParseVerdict() = %q, want %q", v, verification.VerdictPartial)
	}
}

// TestParseVerdict_NotFound verifies that output with no verdict line returns an error.
func TestParseVerdict_NotFound(t *testing.T) {
	_, err := verification.ParseVerdict("No verdict here.\nJust some output.")
	if err == nil {
		t.Error("ParseVerdict() expected error for missing verdict, got nil")
	}
}

// TestParseVerdict_BoldMarkdown verifies that "**VERDICT: PASS**" (bold markdown) returns an error.
// The verdict must be exact, not decorated.
func TestParseVerdict_BoldMarkdown(t *testing.T) {
	_, err := verification.ParseVerdict("**VERDICT: PASS**")
	if err == nil {
		t.Error("ParseVerdict() expected error for bold-decorated verdict '**VERDICT: PASS**', got nil")
	}
}

// TestParseVerdict_MultiLine verifies verdict is correctly extracted when it appears on the last line
// with other content before it.
func TestParseVerdict_MultiLine(t *testing.T) {
	output := `### Check: server starts
**Command run:** go build ./...
**Output observed:** (no output)
**Result: PASS**

### Check: tests pass
**Command run:** go test ./...
**Output observed:** ok  go-agent-harness  0.42s
**Result: PASS**

VERDICT: PASS`

	v, err := verification.ParseVerdict(output)
	if err != nil {
		t.Fatalf("ParseVerdict() unexpected error: %v", err)
	}
	if v != verification.VerdictPass {
		t.Errorf("ParseVerdict() = %q, want %q", v, verification.VerdictPass)
	}
}

// TestHasExecutableEvidence_WithCommandRun verifies that text containing "Command run:" returns true.
func TestHasExecutableEvidence_WithCommandRun(t *testing.T) {
	output := "**Command run:** go test ./...\n**Output observed:** ok  package  0.1s"
	if !verification.HasExecutableEvidence(output) {
		t.Error("HasExecutableEvidence() = false for text with 'Command run:', want true")
	}
}

// TestHasExecutableEvidence_WithoutCommandRun verifies plain text without command blocks returns false.
func TestHasExecutableEvidence_WithoutCommandRun(t *testing.T) {
	output := "I reviewed the code and it looks correct. The function returns the right value."
	if verification.HasExecutableEvidence(output) {
		t.Error("HasExecutableEvidence() = true for plain text without command blocks, want false")
	}
}

// TestValidateVerification_AllValid verifies that proper verification output with evidence
// produces no violations.
func TestValidateVerification_AllValid(t *testing.T) {
	output := `### Check: binary compiles
**Command run:** go build ./cmd/harnessd
**Output observed:** (no output, exit 0)
**Result: PASS**

VERDICT: PASS`

	cfg := verification.DefaultVerificationConfig()
	violations := verification.ValidateVerification(output, cfg)
	if len(violations) != 0 {
		t.Errorf("ValidateVerification() returned violations for valid output: %v", violations)
	}
}

// TestValidateVerification_MissingEvidence verifies that a PASS verdict without a "Command run:"
// block produces a violation when RequireExecutableEvidence is true.
func TestValidateVerification_MissingEvidence(t *testing.T) {
	output := `I looked at the code and it looks correct. The function does what it says.

VERDICT: PASS`

	cfg := verification.DefaultVerificationConfig() // RequireExecutableEvidence = true
	violations := verification.ValidateVerification(output, cfg)
	if len(violations) == 0 {
		t.Error("ValidateVerification() returned no violations for PASS without command evidence, want at least 1")
	}
}

// TestValidateVerification_EvidenceNotRequired verifies that when RequireExecutableEvidence is false,
// a PASS without command blocks produces no violations.
func TestValidateVerification_EvidenceNotRequired(t *testing.T) {
	output := `I looked at the code and it looks correct.

VERDICT: PASS`

	cfg := verification.VerificationConfig{
		RequireExecutableEvidence: false,
		NamedAntiPatternsEnabled:  true,
		VerdictFormat:             "VERDICT: {PASS|FAIL|PARTIAL}",
	}
	violations := verification.ValidateVerification(output, cfg)
	if len(violations) != 0 {
		t.Errorf("ValidateVerification() returned violations when evidence not required: %v", violations)
	}
}

// TestValidateVerification_FailVerdictNoEvidenceRequired verifies that a FAIL verdict without
// command evidence does not produce a violation (evidence requirement only applies to PASS).
func TestValidateVerification_FailVerdictNoEvidenceRequired(t *testing.T) {
	output := `I tried to read the code but it had compilation errors.

VERDICT: FAIL`

	cfg := verification.DefaultVerificationConfig() // RequireExecutableEvidence = true
	violations := verification.ValidateVerification(output, cfg)
	// A FAIL without evidence should not be a violation — agent may not be able to run it
	if len(violations) != 0 {
		t.Errorf("ValidateVerification() returned violations for FAIL verdict without evidence: %v", violations)
	}
}
