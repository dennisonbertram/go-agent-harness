package memory

// DriftProtectionText returns the drift protection section for injection into
// the system prompt. It returns an empty string when drift protection is
// disabled.
//
// IMPORTANT: This returns a section header (## level), NOT a bullet point.
// Eval results: 0/3 pass rate as bullet, 3/3 as section header.
func DriftProtectionText(cfg MemoryConfig) string {
	if !cfg.DriftProtectionEnabled {
		return ""
	}
	return `## Before Recommending from Memory

Before recommending any approach, pattern, or configuration from memory, you MUST verify
that the memory still reflects the current state of the codebase:

1. Check that referenced files still exist at the specified paths
2. Check that referenced functions/types still have the expected signatures
3. Check that referenced patterns are still used in the current code
4. If anything has changed, update the memory entry instead of recommending stale information

Do NOT recommend from memory without verification. Stale memories cause more harm than no memories.`
}
