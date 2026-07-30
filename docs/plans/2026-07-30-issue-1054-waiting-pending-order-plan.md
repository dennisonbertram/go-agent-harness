# Plan: Publish waiting-for-user only after pending input exists

## Context

- Governing GitHub issue: #1054
- Problem: The runner exposes `waiting_for_user` and its event before either
  built-in AskUserQuestion broker has registered pending input.
- User impact: TUI and native GUI clients can react to the wait state and
  receive a transient `no pending input`, leaving the question unavailable
  until a retry.
- Constraints: Preserve tool/event order, support in-memory and durable
  checkpoint brokers, and keep denial/cancellation/timeout paths bounded.

## Scope

- In scope: A typed post-registration notification on AskUserQuestion requests,
  broker implementations, tool propagation, runner status/event publication,
  lifecycle regressions, and documentation.
- Out of scope: Question/answer schema, approval broker behavior, and scheduler
  semantics.

## Documentation Contract

- Feature status: `implemented`
- Public docs affected: None; this tightens an existing lifecycle contract.
- Spec docs to update before code: This plan and impact map.
- Implementation notes to add after code: Engineering and long-term-thinking
  logs with TDD and verification evidence.

## Test Plan (TDD)

- New failing tests to add first: Gate broker registration and prove the current
  runner publishes `waiting_for_user` while `PendingInput` still fails.
- Existing tests to update: In-memory/checkpoint broker lifecycle tests, core
  AskUserQuestion tool forwarding test, and runner lifecycle order test.
- Regression tests required: Denied AskUserQuestion status restoration,
  cancellation, timeout, event order, repeated normal/race focused suites, and
  the complete repository gate.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1054-waiting-pending-order-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: A broker invokes the readiness hook before pending state is readable.
- Mitigation: Broker-level assertions call `Pending` inside the hook.
- Risk: A hook executes more than once or after cancellation.
- Mitigation: Each broker starts exactly one context-bound notifier immediately
  after successful registration; answer and deadline handling never wait for a
  stalled notifier.

## Verification

- Expected red: deterministic gated-broker test observed
  `waiting_for_user` before registration while `PendingInput` failed.
- Both broker backends prove pending input is readable inside `OnPending`; the
  core tool proves typed notifier propagation.
- Focused AskUser/wait lifecycle suites passed 100 normal and 100 race
  repetitions.
- Complete `internal/harness/...` normal and race suites passed.
- `./scripts/test-regression.sh` passed normal, race, and coverage with 85.6%
  total coverage and zero uncovered functions.
- Exact-head review found that notification time was outside the timeout
  countdown. New regressions blocked both broker notifiers beyond their
  deadlines and failed until timers/contexts started before notification; each
  passed 10 normal and 10 race repetitions, followed by complete harness and
  repository gates.
- A second exact-head review strengthened the finding: merely starting the
  clock still let a never-returning notifier hang `Ask`. The regressions now
  require timeout while notification remains blocked. Brokers launch the typed
  notifier with the same deadline context and continue waiting for
  answer/cancellation independently.
