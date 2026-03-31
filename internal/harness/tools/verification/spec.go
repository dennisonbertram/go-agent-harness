// Package verification provides types, validators, and prompt formatters for
// the verification/testing tool description system.
//
// It defines named anti-patterns agents must avoid, a verdict type and parser,
// executable-evidence checks, and a function to format verification guidance text
// that is embedded in tool descriptions.
package verification

// VerificationConfig controls how verification output is evaluated and how
// verification guidance is presented in tool descriptions.
type VerificationConfig struct {
	// RequireExecutableEvidence requires that PASS verdicts include at least one
	// "Command run:" block. A PASS without evidence is treated as a violation.
	RequireExecutableEvidence bool

	// VerdictFormat is the expected verdict line format (e.g. "VERDICT: {PASS|FAIL|PARTIAL}").
	VerdictFormat string

	// NamedAntiPatternsEnabled includes the named anti-pattern list in tool description prompts.
	NamedAntiPatternsEnabled bool
}

// DefaultVerificationConfig returns sensible production defaults:
// evidence is required for PASS verdicts and anti-patterns are shown.
func DefaultVerificationConfig() VerificationConfig {
	return VerificationConfig{
		RequireExecutableEvidence: true,
		VerdictFormat:             "VERDICT: {PASS|FAIL|PARTIAL}",
		NamedAntiPatternsEnabled:  true,
	}
}

// AntiPattern describes a named failure mode that agents commonly fall into
// during verification tasks.
type AntiPattern struct {
	// Name is the short, memorable identifier for this anti-pattern.
	Name string

	// Description explains what the anti-pattern is and why it is harmful.
	Description string

	// Detection describes how to identify this anti-pattern in verification output.
	Detection string
}

// AntiPatterns returns the canonical list of named verification anti-patterns.
// There are exactly three, each with a memorable name, description, and detection hint.
func AntiPatterns() []AntiPattern {
	return []AntiPattern{
		{
			Name:        "Verification Avoidance",
			Description: "Reading code instead of running it. Inspecting source to confirm it \"looks right\" is not verification — it is speculation.",
			Detection:   "PASS verdict without a `Command run:` block. If nothing was executed, the pass is a skip.",
		},
		{
			Name:        "First-80% Seduction",
			Description: "Declaring success when the easy happy-path cases work but edge cases, error paths, and boundary conditions remain untested.",
			Detection:   "All checks are happy-path inputs with expected outputs; no error, boundary, or negative test cases present.",
		},
		{
			Name:        "Narration Over Evidence",
			Description: "Describing what should happen instead of showing what did happen. Predictions are not observations.",
			Detection:   "Verification output contains prose like 'this should work' or 'the function will return' with no actual command output.",
		},
	}
}
