# Plan: Issue #1122 native interactive-state ownership

## Context

- Governing GitHub issue: #1122.
- Problem: a timestamp-newer selected run could leave approval, plan, or input UI
  owned by the displaced run visible; its action then targeted `currentRunID`.
- User impact: an operator can accidentally approve, deny, steer, or answer a
  different scheduled continuation.
- Constraints: stack on draft PR #1118; preserve #994 request generations and
  delayed terminal acknowledgement semantics; no server or callback change.

## Scope

- In scope: run-scoped native pending models, synchronous selection/retirement
  invalidation, guarded action APIs, Chat and ToolWalk call sites, regressions.
- Out of scope: harness API/wire changes, callback dispatch, TUI, and #1118's
  external-run selection policy.

## Documentation Contract

- Feature status: implemented; stacked draft pending independent review.
- Public docs affected: None; this is internal native ownership repair.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable logs and indexes.

## Test Plan (TDD)

- New failing tests first: A approval/plan/input then newer B selection clears
  all A UI; stale UI action emits no B endpoint; selected terminal/no-fallback
  clears interaction state; foreign terminal preserves B state.
- Existing tests to update: transcript pending model construction and UI/ToolWalk
  action invocations.
- Regression tests required: strict Swift focused/package/full and repository
  regression gate.

## Cross-Surface Impact Map

- `docs/plans/2026-08-03-issue-1122-interactive-state-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue and exact #1118 head.
- [x] Record ownership/caller search evidence and cross-surface map.
- [x] Write and capture expected-red acceptance tests.
- [x] Add run identity and synchronous ownership invalidation.
- [x] Bind UI and ToolWalk actions to captured run identity.
- [x] Run focused, full Swift, repository regression, and update logs/indexes.
- [ ] Push stacked draft PR with `Closes #1122`; do not merge.

## Risks and Mitigations

- Risk: clearing state could break an in-flight #994 acknowledgement.
- Mitigation: invalidate state ownership only; retain lifecycle generation
  bookkeeping so a terminal completion still releases its own control request.
