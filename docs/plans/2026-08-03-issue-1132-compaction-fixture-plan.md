# Plan: Issue #1132 deterministic compaction-after-wait fixture

## Context

- Governing GitHub issue: #1132.
- Problem: `TestCompactRunWhileWaitingForUserPreservesCompactionAfterResume`
  treated broker registration as proof that the public run state had reached
  `waiting_for_user`. Under hosted race scheduling, that earlier internal
  boundary allowed the following status assertion to observe `running`.
- User impact: this is test evidence for a user-visible wait/compact/resume
  lifecycle; the fixture must prove the public event before inspecting it.
- Constraints: test and documentation only; no runner, broker, API, TUI, GUI,
  persistence, or production lifecycle change.

## Scope

- In scope: subscribe immediately after `StartRun`; wait for the already-public
  `run.waiting_for_user` event before pending-input and status assertions;
  preserve compaction, resume, ordering, and final-message assertions.
- Out of scope: changing event ordering, broker timing, compaction behavior,
  or any production code.

## Documentation Contract

- Feature status: implemented locally; independent review, hosted checks, and
  promotion remain.
- Public docs affected: none; public behavior is unchanged.
- Spec docs to update before code: this plan and linked impact map.
- Implementation notes to add after code: engineering, observational, system,
  and long-term-thinking logs plus indexes.

## Test Plan (TDD)

- Existing expected red: hosted `go test -race` observed the old fixture's
  `waiting_for_user` status assertion while the run was still `running`.
- Existing test to update: `TestCompactRunWhileWaitingForUserPreservesCompactionAfterResume`.
- Regression proof: the test must subscribe immediately after start and use
  `waitForRunEventType(..., EventRunWaitingForUser)` before it reads pending
  input/state, then retain all original compaction/resume/order assertions.

## Cross-Surface Impact Map

See `2026-08-03-issue-1132-compaction-fixture-impact-map.md`.

## Implementation Checklist

- [x] Verify the structured issue and current source-of-truth boundary.
- [x] Write the plan and cross-surface impact map before test edits.
- [x] Update only the test fixture.
- [x] Run focused normal/race x100, harness normal/race, and full regression.
- [x] Update durable logs and indexes.
- [ ] Obtain independent cheap review and publish a PR that closes #1132.

## Risks and Mitigations

- Risk: subscribing after an event loses the boundary. Mitigation: `Subscribe`
  returns history and live stream; the shared helper searches both.
- Risk: a fixture-only repair hides a product ordering defect. Mitigation: it
  waits on the public lifecycle event, then preserves the original externally
  visible state, event-order, compaction, and resumed output assertions.
