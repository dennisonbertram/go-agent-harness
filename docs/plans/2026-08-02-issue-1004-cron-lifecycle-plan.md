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
- [x] Fence terminal reconciliation persistence against Stop: cancellation
  wins without terminalizing/releasing the recovered lease, while a terminal
  commit already inside the gate completes before Stop returns. Definitive job
  absence terminalizes; canceled/transient job lookup retains the active row.
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

## Terminal persistence lifecycle fence evidence (2026-08-02)

- Strict reds on exact `9181311` were run individually before production code:
  cancel-wins wrote a terminal row after Stop, commit-wins let Stop return
  during a blocked terminal write, and canceled/transient `GetJob` failures
  both incorrectly terminalized an active row.
- Fix: remote/embedded observation remains outside `lifecycleMu`; only the
  terminal persistence, local lease release, and run-tracking write share the
  Stop gate. `IsJobNotFound` is the sole job-lookup error allowed to classify
  an execution as unavailable; cancellation and transient errors preserve it.
- Each new regression passed directly in normal mode and under
  `go test -race ... -count=20`. The eight prior restart/terminal lifecycle
  regressions passed normal and race x20. The sandbox denied the real
  `httptest` IPv6 listener (`bind: operation not permitted`); the same normal
  and race commands passed host-local. This is environmental, not a waived
  product result.
- The earlier repository-wide `cmd/harnessd` race timeout remains unwaived and
  is still required before final acceptance.

## Rollout / rollback

## Live observation shutdown evidence (2026-08-02)

- Exact `6838e25` test-only replays were red for live embedded cancellation,
  real `RemoteRunStarter` polling cancellation, stop-wins retention, and a
  terminal `UpdateExecution` failure that still touched the job. Commit-wins
  and shell in-flight drain passed at base and are preserved controls.
- Fix: live terminal observation is detached from dispatch onto a
  scheduler-owned cancellable context and joined by Stop. Terminal persistence
  is lifecycle-fenced; observer error/unobserved/cancel and write failure are
  nonterminal and preserve row/link/lease.
- Expanded direct normal and `-race -count=20` live matrix passes. The prior
  repository-wide race timeout is not waived and remains a final gate.

## Recovered observer-error evidence (2026-08-02)

- Recovery now has the same terminal contract as live observation: only
  `observed=true` with a nil observer error is terminal. A 503, stream/transport
  error, unavailable observer, or scheduler cancellation leaves the active
  execution, structured `RunID`, and local/durable no-overlap lease unchanged;
  it does not call `TouchJobRun`.
- Reconciliation continues after a nonterminal observation result. Thus a
  transient error for one recovered row cannot starve a later row that already
  reports a durable terminal success or failure.
- Exact `1d699808` test-only replay was red for linked recovered 503 and
  stream errors, plus a mixed error-and-success batch. Explicit terminal
  failure, unobserved, and stop/cancel are retained controls. Focused normal
  and race x20 recovery matrices pass; repository-wide regression remains a
  required, unwaived acceptance gate.

## Embedded replay-observation evidence (2026-08-03)

- A real race remained in `cronRunStarter.ObserveRun`: Runner records a terminal
  event in replay before it commits terminal status and fans out to subscribers.
  A cron observer that subscribed in that interval discarded terminal replay,
  read `running`, and then waited forever on a live channel that would never
  receive that already-snapshotted terminal event.
- The bridge now treats terminal replay only as a synchronization hint and
  waits for `Runner.GetRun` to report an authoritative completed, failed, or
  cancelled status. A low-rate context-owned status poll is also the fallback
  when storage policy intentionally suppresses terminal replay/live events. It
  never derives cron output or success from event payload.
- Deterministic seam tests cover completed/failed/cancelled replay, cancellation
  before status commit, closed live stream, and live terminal delivery. The
  full embedded conversation test passes `-count=100` after the repair.

The execution wire shape is additive. No-overlap is default-on in the embedded
scheduler; it produces a durable skipped history row rather than silently
dismissing a fire. Rollback can restore the old executor adapter while
retaining the additional history fields; no destructive migration is required.
