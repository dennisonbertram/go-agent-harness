package verification

import "strings"

// FormatVerificationGuidance generates the verification guidance text for tool descriptions.
//
// When cfg.NamedAntiPatternsEnabled is false (or VerdictFormat is empty), an empty
// string is returned so callers can omit the section entirely.
//
// When enabled, the returned string contains:
//   - A named anti-pattern list with name, description, and detection hint for each.
//   - The required output format for each check block.
//   - The final VERDICT line format.
func FormatVerificationGuidance(cfg VerificationConfig) string {
	if !cfg.NamedAntiPatternsEnabled && cfg.VerdictFormat == "" {
		return ""
	}

	var b strings.Builder

	b.WriteString("## Verification Requirements\n\n")

	if cfg.NamedAntiPatternsEnabled {
		b.WriteString("### Named Anti-Patterns (AVOID THESE)\n\n")
		for _, p := range AntiPatterns() {
			b.WriteString("- **")
			b.WriteString(p.Name)
			b.WriteString("**: ")
			b.WriteString(p.Description)
			b.WriteString("\n  Detection: ")
			b.WriteString(p.Detection)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("### Required Output Format\n\n")
	b.WriteString("Each check must include:\n")
	b.WriteString("- `### Check:` — what is being verified\n")
	b.WriteString("- `**Command run:**` — the exact command executed\n")
	b.WriteString("- `**Output observed:**` — the actual output (not predicted)\n")
	b.WriteString("- `**Result: PASS/FAIL**`\n\n")

	b.WriteString("Final line must be exactly: `VERDICT: PASS`, `VERDICT: FAIL`, or `VERDICT: PARTIAL`\n")

	return b.String()
}
