# Issue #237 Grooming: feat(agents) — Built-in Profile System with Self-Improving Efficiency Review Loop

## Summary
Define named reusable subagent configurations (tool allowlist, model, max_steps, system prompt, cost ceiling) so calling agents pick a profile by name instead of configuring from scratch. Includes a post-run efficiency review loop that improves profiles over time.

## Already Addressed?
**No.** No profiles package, registry, or efficiency review loop exists anywhere in the codebase.

## Clarity
**2/5** — Well-motivated but 6 open design questions unresolved, and the self-improving efficiency review loop scope-creeps significantly.

Key ambiguities:
1. Storage design options (hardcoded vs YAML vs DB) not decided
2. Profile interaction with per-run overrides (#234) undefined
3. Self-improving review loop is a separate subsystem inflating scope
4. Dependency on #236 (config propagation) not formally marked as blocker
5. No clear "built-in vs user-defined" boundary for profile discovery
6. Acceptance criteria partial, not exhaustive

## Acceptance Criteria
**Partial** — Some criteria defined but design questions leave gaps.

## Scope
**Needs splitting** — Recommend 3 sub-issues:
1. Profile definition + registry + run_agent tool lookup
2. HTTP API for profile CRUD
3. Post-run efficiency review loop (separate feature)

## Blockers
- **#236** (config propagation) — profiles depend on deterministic config injection to subagent workspaces
- **#234** (per-run tool filtering) — profiles tool allowlists depend on this infrastructure

## Recommended Labels
enhancement, needs-clarification, large, blocked

## Effort
**Large** (4–6 days as written) / **Medium** (if split into sub-issues)

## Recommendation
**needs-clarification** — Resolve open design questions, formally mark #236 and #234 as blockers, split into sub-issues before implementation.
