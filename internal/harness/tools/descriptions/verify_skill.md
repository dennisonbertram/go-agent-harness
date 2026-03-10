Verify a registered skill by running automated structural checks against its SKILL.md file.

Checks performed:
1. Skill exists in the registry
2. SKILL.md file is readable
3. YAML frontmatter is valid and parseable
4. Required fields are present: name and description must be non-empty
5. Body content is non-empty and substantive (> 50 characters)

On success, marks the skill as verified with verifiedBy: "automated" and returns a JSON report
of all check results and an overall pass/fail. On failure, returns the failing checks with
explanations so the skill can be improved and re-verified.

Use this after creating or updating a skill to confirm it is structurally sound before relying on it.
