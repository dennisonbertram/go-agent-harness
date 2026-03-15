# Issue #238 Grooming: feat(runner) — Agent-Controlled Context Reset with Selective Preservation

## Summary
New reset_context tool that lets agents hard-reset conversation history mid-run while preserving selected content (pinned messages, observational memory). Includes DB recording, ContextReset events, and invariant preservation.

## Already Addressed?
**No.** Zero implementation found:
- No reset_context tool in internal/harness/tools/
- No ContextReset event type
- No run_context_resets DB table

## Clarity
**5/5** — Exceptionally well-written design doc with concrete schema, step-by-step runner behavior, database DDL, and opening message format.

## Acceptance Criteria
**Explicit and complete** — 16 checkboxes covering:
- Tool behavior and input validation
- Invariant preservation (pinned messages, observational memory)
- ContextReset event emission
- DB recording (run_context_resets table)
- Memory integration
- Test coverage requirements

## Scope
**Atomic but non-trivial** — Spans 7 files: tool handler, runner.go, events, DB layer, observational memory, tool description .md, tests. Cohesive enough for a single PR.

## Blockers
None. All dependencies exist (observational memory, runner, DB layer, tool registry).

## Recommended Labels
`enhancement`, `well-specified`, `large`

## Effort
**Large** — 4–6 days.
- Tool handler: 1 day
- Runner integration: 1.5 days
- DB layer: 0.5 days
- Observational memory: 0.5 days
- Testing (concurrency + state machine): 2+ days

## Recommendation
**well-specified** — Ready for implementation. Assign to developer with runner.go familiarity. Test coverage is critical due to concurrency and state-machine aspects.
