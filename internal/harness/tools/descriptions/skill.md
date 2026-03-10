Apply a registered skill by name. Skills are pre-configured specializations
(e.g. code-review, TDD, refactoring, migration) that provide detailed,
domain-specific instructions and optionally constrain which tools are allowed.

Pass the skill name as the first word, followed by optional arguments.
Examples: "deploy staging", "code-review", "tdd --strict".

When a user references a skill by name or a slash command (e.g. "/deploy"),
use this tool to apply the matching skill. Available skills are listed below.