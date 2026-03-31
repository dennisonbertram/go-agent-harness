package verification

import (
	"errors"
	"strings"
)

// Verdict represents the outcome of a verification run.
type Verdict string

const (
	VerdictPass    Verdict = "PASS"
	VerdictFail    Verdict = "FAIL"
	VerdictPartial Verdict = "PARTIAL"
)

// ErrNoVerdict is returned by ParseVerdict when no verdict line is found.
var ErrNoVerdict = errors.New("no VERDICT line found in verification output")

// ParseVerdict extracts the verdict from verification output.
//
// It looks for an exact line matching "VERDICT: PASS", "VERDICT: FAIL", or
// "VERDICT: PARTIAL". The line must not have additional decoration (e.g. markdown
// bold markers). If no such line is found, ErrNoVerdict is returned.
func ParseVerdict(output string) (Verdict, error) {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "VERDICT: PASS":
			return VerdictPass, nil
		case "VERDICT: FAIL":
			return VerdictFail, nil
		case "VERDICT: PARTIAL":
			return VerdictPartial, nil
		}
	}
	return "", ErrNoVerdict
}

// HasExecutableEvidence reports whether the verification output contains at least
// one "Command run:" block, indicating the agent actually ran something.
func HasExecutableEvidence(output string) bool {
	return strings.Contains(output, "Command run:")
}

// ValidateVerification checks that verification output meets the evidence requirements
// specified in cfg. It returns a slice of human-readable violation strings.
// An empty slice means the output is compliant.
func ValidateVerification(output string, cfg VerificationConfig) []string {
	var violations []string

	if !cfg.RequireExecutableEvidence {
		return violations
	}

	// The evidence requirement only applies to PASS verdicts. A FAIL may occur
	// before the agent could run anything (e.g. build failure, missing file).
	verdict, err := ParseVerdict(output)
	if err != nil {
		// No verdict found — cannot evaluate evidence requirement.
		return violations
	}

	if verdict == VerdictPass && !HasExecutableEvidence(output) {
		violations = append(violations,
			"PASS verdict requires at least one 'Command run:' block; "+
				"reading code without running it is Verification Avoidance")
	}

	return violations
}
