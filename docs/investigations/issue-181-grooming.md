# Issue #181 Grooming: workspace: define Workspace interface and package scaffold

Date: 2026-03-11
Verdict: **WELL-SPECIFIED** ✅

## Summary
No existing workspace abstraction in codebase. Only reference is `HARNESS_WORKSPACE` env var in `cmd/harnessd/main.go` (sets filesystem path for skills/config). Clean slate.

## Evaluation

| Dimension | Result |
|-----------|--------|
| Already addressed? | No |
| Clarity | Excellent — explicit interface + Options struct defined in issue |
| Acceptance criteria | All explicit and measurable |
| Scope | Atomic — interface + registry only, no implementations |
| Blockers | None |
| Effort | Small (2-4 hours) |

## Notes for Implementation
- Consider whether Options needs Owner/CreatedAt/Labels for future use
- Whether Workspace needs Name()/ID() accessor methods
- Whether factory should support runtime registration (plugin style) or just predefined types
- Whether to define specific error types (e.g. ErrWorkspaceNotFound)
- Factory registry map needs mutex for concurrent registration

## Labels
- `well-specified`, `small`, `workspace`
