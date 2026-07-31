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
  after successful registration. Unresolved timeout handling never waits for a
  stalled notifier; an already-accepted answer waits for notification
  completion or parent cancellation before it is returned.

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
- A third exact-head review found that fully independent answer selection let
  `run.resumed` overtake blocked wait-event persistence. A blocking-store
  regression reproduced that reverse order. Brokers now buffer quick answers
  but do not consume them until notification finishes; cancellation and timeout
  can still return independently if the notifier never completes.
- A fourth exact-head review found that timeout cleanup could overwrite a
  checkpoint already resumed by a timely answer, while a late stale wait-status
  write could overwrite the durable terminal status. New deterministic
  regressions cover both cases. Checkpoint terminal transitions are serialized,
  expiry is pending-only, and status persistence repairs stale writes using a
  monotonic per-run version.
- A fifth exact-head review found asymmetric in-memory answer handling and
  false-success reporting when checkpoint expiry won. Regressions now require a
  timely buffered in-memory answer to survive notification deadline and require
  a losing checkpoint resume to return `ErrAlreadyResolved`.
- A sixth independent review traced the sentinel across every caller. The final
  contract requires both checkpoint AskUser deadline branches to recover an
  already-accepted answer; run input and approval/deny races to retain their
  existing no-pending API semantics; and generic checkpoint resume to return
  `409 already_resolved` on repeated, expired, or denied records without
  mutating the durable terminal snapshot. All concurrency tests use bounded
  gates and receives.
- A seventh exact-head review found that accepted-answer recovery at the
  notifier deadline could return while pending-state publication was still
  blocked. New deterministic regressions require both brokers to preserve the
  accepted answer without returning it until notification finishes, preventing
  `run.resumed` from overtaking `run.waiting_for_user`.
- An independent concurrency review then required four final hardening
  contracts: every notifier persistence/publication step honors the supplied
  deadline context without converting accepted input into a synthetic timeout;
  checkpoint resolution uses per-record coordination plus durable pending-only
  CAS across Service instances; run-status persistence serializes per run and
  snapshots after acquiring that lock; and a runner-side pending observer
  supplies exactly-once wait visibility when a broker omits `OnPending`.
- Final hardening changed pending notification from once-on-attempt to
  serialized once-on-success. Deterministic regressions require retry after an
  immediate waiting `UpdateRun` or `AppendEvent` failure, drain an observer
  whose publication began before the tool returned, and preserve exactly one
  visible wait/resume. Strict event persistence is scoped to waiting
  publication; ordinary events retain best-effort persistence. A failed strict
  append rolls back its final sequence allocation so run SSE replay remains
  contiguous and `Last-Event-ID` returns only unseen events. Intentional
  redaction drops complete publication without retrying to the deadline.
  Cross-Service waiter polling remains registered across transient store reads
  and retries until it observes resolution, receives local notification, or
  the caller context ends.
