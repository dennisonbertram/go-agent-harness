# Issue #234 Grooming: feat(subagent) — Per-Run Tool Filtering, System Prompt, and Permissions Forwarding

## Summary
Wire up the existing allowed_tools, system_prompt, and permissions fields so they actually forward to RunRequest and constrain the runner — currently these fields are accepted by the HTTP endpoint but never used.

## Already Addressed?
**Partially** — Infrastructure exists but forwarding chain is broken:
- RunRequest has SystemPrompt and Permissions fields but missing AllowedTools field
- HTTP endpoint accepts allowed_tools but never forwards to RunRequest
- RunForkedSkill() interface called by skill tool but not implemented in runner
- SkillConstraint lacks IsBootstrap flag (all constraints deactivate after skill completes)
- filteredToolsForRun() enforcement mechanism works but only activates reactively

## Clarity
**4/5** — Clear problem statement and solution. Sparse on implementation details but enough to act on.

## Acceptance Criteria
**Explicit** — 9 well-defined criteria:
- AllowedTools []string field added to RunRequest
- HTTP handler forwards allowed_tools to RunRequest
- filteredToolsForRun() uses AllowedTools when non-empty
- RunForkedSkill() implemented in runner
- IsBootstrap flag on SkillConstraint
- System prompt forwarded to forked runs
- Permissions forwarded to forked runs
- Tests for all new paths
- Race detector clean

## Scope
**Atomic** — 4 files primarily affected: types.go, runner.go, http.go, skill.go.

## Blockers
None.

## Recommended Labels
enhancement, well-specified, small

## Effort
**Small** — 1–2 person-days.

## Recommendation
**well-specified** — Ready to implement immediately. Prerequisite for #235 and #237.
