# Issue #1004 — Cron execution linkage and overlap admission

## Context

- Governing GitHub issue: #1004.
- Problem: a real `cronsd` → remote `harnessd` canary persisted a successful
  execution with a null `run_id` because the executor reduced a structured run
  admission to output prose.
- User impact: cron history cannot reliably link a visible scheduled turn to
  the run that continued its conversation; overlapping scoped fires can also
  reorder turns.
- Constraints: preserve #1002's scope/CAS lifecycle, retain shell execution
  semantics, do not cherry-pick unmerged #1003 transport work, and use strict
  red-green tests.

## Scope

- In scope for this PR-sized embedded-scheduler slice: typed executor outcome,
  immediate durable `run_id`, explicit lifecycle state transitions, default-on
  no-overlap admission keyed by tenant/agent/conversation, skipped execution
  evidence, and monotonic run-tracking persistence.
- Restart reconciliation is generic in this slice: nonterminal rows reload on
  startup, no-run-ID rows fail closed by retaining their scope lease (the
  process may have crashed between external `StartRun` success and durable link
  persistence), linked rows hold their scope lease, and any `RunObserver`
  terminalizes them idempotently. The
  #1003 remote adapter must implement that already-defined observer contract;
  it is not copied into this worktree.
- Startup restores durable no-overlap leases synchronously but performs
  terminal observation asynchronously. This applies to embedded and remote
  paths: a recovered live run must never block daemon readiness.

## Test Plan (TDD)

- First red: a harness execution returns a deliberately misleading output
  summary but a structured run ID; scheduler history must persist the ID before
  terminal finalization and never parse the summary.
- Add deterministic overlapping fires for same and different scope keys;
  assert skip evidence vs independent concurrent execution.
- Add SQLite monotonic comparison tests for older completion after newer
  completion; assert job run timestamps and optimistic version never regress.
- Run focused normal/race `internal/cron`, then affected server/embedded tests;
  repository regression is required before promotion.

## Implementation Checklist

- [x] Read issue, #1003/#1010 coordination, architecture and runbooks.
- [x] Write cross-surface impact map.
- [x] Add failing behavior tests.
- [x] Implement typed outcome and lifecycle persistence.
- [x] Implement overlap admission, restart reconciliation, and monotonic tracking.
- [x] Add logs and documentation index entries.
- [x] Add durable cross-process admission, observer-unavailable handling, and
  run-link retry regressions from merge review.
- [x] Record embedded missing-method and isolated pre-fix remote-start reds.
- [x] Split startup lease restoration from asynchronous remote/embedded
  terminal observation; repeated post-bind notifications are idempotent.
- [x] Make asynchronous reconciliation scheduler-owned and shutdown-joined:
  post-bind work is rejected after Stop, cancellation is nonterminal, and Stop
  waits for remote/embedded observers before persistence teardown.
- [x] Run focused normal/race verification.
- [ ] Rerun the repository regression and commit one clean candidate. A prior
  full race gate timed out in `cmd/harnessd`; that result is not waived by the
  focused normal/race passes and requires targeted diagnosis before final gate
  acceptance.

## Shutdown repair evidence (2026-08-02)

- Red-first replay used a disposable exact-`5583f04d` worktree with the
  test-only shutdown patch. `TestScheduler_StopCancelsAndJoinsPostBindObserverRegression`
  failed because `Stop` returned before the observer acknowledged exit;
  `TestScheduler_ReconcileAfterExecutorBoundAfterStopIsNoop` failed because a
  post-stop bind still loaded jobs. The remote HTTP test requires host-local
  listener permission and is green-tested below.
- Green: each direct normal test passed for post-bind Stop/join, post-stop
  no-op, recovered remote polling cancellation, existing post-bind terminal
  reconciliation, and existing remote asynchronous startup. Each corresponding
  direct `go test -race ./internal/cron -run <test> -count=10 -timeout 30s`
  command passed.
- The historical repository-wide race timeout in `cmd/harnessd` remains
  unwaived. These focused results do not substitute for the final full gate.

## Rollout / rollback

The execution wire shape is additive. No-overlap is default-on in the embedded
scheduler; it produces a durable skipped history row rather than silently
dismissing a fire. Rollback can restore the old executor adapter while
retaining the additional history fields; no destructive migration is required.
