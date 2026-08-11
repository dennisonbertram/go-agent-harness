# Plan: Workflow terminal-history SSE completion

## Context

- Governing GitHub issue: #1237
- Problem: `GET /v1/workflow-runs/{id}/events` writes persisted terminal history
  then waits forever on a live subscription channel that plural workflows do not
  close on completion/failure.
- User impact: a late API, TUI, or GUI observer can receive the correct terminal
  event but never receive HTTP stream completion.
- Constraints: preserve frame schema/order and the nonterminal live stream;
  do not touch #1236's `workflows.Engine.Subscribe` handshake.

## Scope

- In scope: return from the workflow HTTP SSE handler immediately after writing
  `workflow.completed` or `workflow.failed` from replay history, with endpoint
  regression coverage.
- Out of scope: event schema, persistence, auth, terminal-channel lifecycle,
  script-workflow endpoints, and #1236.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: None; this preserves the public SSE wire format.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering log and plans index.

## Test Plan (TDD)

- New failing tests to add first: terminal `workflow.completed` and
  `workflow.failed` history must flush exactly once and return while an open
  live channel remains unread; a nonterminal-history control must remain open
  for a live event and then terminate on its live terminal frame.
- Existing tests to update: `internal/server/http_workflows_test.go`.
- Regression tests required: bounded handler-return/cancellation cleanup,
  exact frame ordering, and focused normal/race server tests.

## Cross-Surface Impact Map

- `docs/plans/2026-08-07-issue-1237-workflow-sse-terminal-impact-map.md`

## Implementation Checklist

- [x] Define acceptance criteria in the issue and plan.
- [x] Record architecture/caller search in #1237 and impact map.
- [x] Complete impact analysis before code.
- [x] Write and record failing endpoint regression tests.
- [x] Implement the minimal terminal-history return.
- [x] Update logs/indexes and reconcile evidence.
- [x] Run focused, race, package, and full regression gates.

## Risks and Mitigations

- Risk: a blanket history return could cut off a nonterminal stream.
- Mitigation: only terminal event types return; control test proves a later
  live frame and live terminal remain observable in order.
- Rollout/rollback: no migration; deploy as a narrow server behavior fix and
  revert the single terminal-history branch if necessary.
