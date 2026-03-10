Apply a registered skill by name. Skills are pre-configured specializations
(e.g. code-review, TDD, refactoring, migration) that provide detailed,
domain-specific instructions and optionally constrain which tools are allowed.

Pass the skill name as the first word, followed by optional arguments.
Examples: "deploy staging", "code-review", "tdd --strict".

When a user references a skill by name or a slash command (e.g. "/deploy"),
use this tool to apply the matching skill. Available skills are listed below.

Built-in actions:
- "list" — list all available skills with their verification status ([verified] or [unverified])
- "verify <skill_name> [verified_by]" — mark a skill as verified, writing verification
  metadata (verified, verified_at, verified_by) back to the skill file. The optional
  verified_by argument identifies who performed the verification (defaults to "agent").

Unverified skills display a warning when applied. Use "verify" to mark a skill as trusted
after reviewing its instructions.
