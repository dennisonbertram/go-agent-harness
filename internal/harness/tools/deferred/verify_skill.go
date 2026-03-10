package deferred

import (
	tools "go-agent-harness/internal/harness/tools"
)

// VerifySkillTool returns a deferred tool that validates a skill's SKILL.md structure
// and marks it as verified in the store if all checks pass.
//
// Checks performed:
//  1. Skill exists in the registry
//  2. SKILL.md file is readable
//  3. YAML frontmatter is valid and parseable
//  4. Required fields (name, description) are non-empty
//  5. Body content is non-empty and substantive (> 50 characters)
//
// On success, marks the skill as verified with verifiedBy: "automated" and returns
// a JSON report of all check results and an overall pass/fail.
func VerifySkillTool(verifier tools.SkillVerifier) tools.Tool {
	return tools.VerifySkillTool(verifier)
}
