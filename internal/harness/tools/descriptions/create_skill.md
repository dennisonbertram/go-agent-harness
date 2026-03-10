Create a new skill file in the configured skills directory. A skill is a reusable
specialization that provides domain-specific instructions to the agent.

The skill file is written as a SKILL.md with YAML frontmatter. Required fields:
- name: machine-readable kebab-case identifier (e.g. "code-review", "deploy")
- description: what the skill does; shown in the skill catalog
- trigger: phrase or condition that should activate this skill

Optional fields in content:
- version: must be 1 (default)
- allowed-tools: list of tools the skill permits
- argument-hint: hint for arguments the skill accepts
- context: "conversation" (default) or "fork"

The tool validates the name format, checks for duplicates, and writes the skill
to disk. Once created, the skill is available for use in future sessions.
