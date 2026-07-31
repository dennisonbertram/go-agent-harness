# Plan: Make Terminal Status and Event Publication Atomic

## Context

- Governing GitHub issue: #1067.
- Problem: terminal helpers publish `Run.Status` before the event journal has
  prepared, persisted, and exposed the matching terminal event. A concurrent
  `GetRun` can therefore return completed, failed, or cancelled while an
  immediate `Subscribe` replay still ends at a non-terminal event.
- User impact: API polling, SSE reconnect, CLI/TUI, and macOS clients can
  briefly render terminal state without the authoritative terminal transcript
  or its preceding causal/error evidence.
- Constraints: preserve terminal sealing, recorder drain ordering, bounded
  store writes, run-independent availability, current event/status schemas,
  cleanup ordering, and the explicit `StorageModeNone` terminal-redaction
  policy. The store API has no cross-record transaction, so promise the
  testable one-way invariant rather than two-way event/status atomicity.

## Scope

- In scope: one shared Runner terminal-transition seam for completed, failed,
  cancelled, max-step failed, and max-turn failed paths; deterministic
  concurrency and replay regressions; real HTTP poll-then-replay proof.
- Out of scope: PR #1060/#1055 changes, cron/callback behavior, conversation
  cursor redesign, GUI visual changes, provider routing, schemas, and workflow
  timing issue #1049.

## Documentation Contract

- Feature status: review hardening implemented and fully verified locally;
  hosted checks pending.
- Public docs affected: none; existing terminal event/status wire formats stay
  unchanged.
- Spec docs before code: this plan and its linked impact map.
- Implementation notes after code: engineering, observational, system, and
  long-term logs plus the plans index and active plan.

## Test Plan (TDD)

- First red: a deterministic phase barrier pauses each completed, failed, and
  cancelled helper after the old status write but before terminal event
  publication. Concurrent `GetRun` plus `Subscribe` must never observe that
  forbidden state.
- Causal control: on an error-chain-enabled failure, the required
  `error.context` snapshot must precede `run.failed` before failed status is
  observable.
- Store/recorder controls: block terminal append and prove unrelated
  conversations progress while later events in the target conversation cannot
  overtake. On append error, never attempt a durable terminal status update but
  still complete bounded in-memory terminal publication/fanout. On status
  update error or context timeout after a successful append, keep the durable
  run non-terminal while the live Runner and subscribers complete. For an
  explicit `StorageModeNone` drop, drain the recorder before status visibility.
- Concurrency control: race competing terminal transitions and require the
  winning status to match the single sealed terminal event; hold a terminal at
  the pre-fanout boundary and prove a later same-conversation event cannot
  overtake it for an existing conversation subscriber.
- Real path: HTTP `GET /v1/runs/{id}` followed immediately by run-event SSE
  replay contains the matching terminal event for all three statuses.
- Focused stress: normal and race at `-count=100`.
- Affected gates: `internal/harness` and `internal/server` normal/race and vet.
- Repository gate: unchanged foreground non-TTY
  `./scripts/test-regression.sh`.
- Hosted gates: required PR checks, including `test-fast` and `test-race`.

## Cross-Surface Impact Map

- See `2026-07-31-issue-1067-terminal-status-event-atomicity-impact-map.md`.

## Implementation Checklist

- [x] Link contract-complete bug #1067 before implementation.
- [x] Record current ownership, callers, sources of truth, and search evidence.
- [x] Write this plan and impact map before code.
- [x] Add and confirm the deterministic failing regressions.
- [x] Implement the smallest shared terminal-transition repair.
- [x] Confirm focused stress and affected normal/race/vet gates.
- [x] Confirm the unchanged repository regression gate on the final diff.
- [x] Prove the HTTP poll-then-replay path.
- [x] Update all required logs and documentation status.
- [ ] Open one closing PR, push its exact head, and request `@codex` review.
- [ ] Confirm hosted checks are green; do not merge.

## Risks and Mitigations

- Risk: holding `Runner.mu` across persistence would block unrelated queries.
- Mitigation: retain out-of-lock store/recorder I/O, serialize only the target
  conversation across the terminal sequence, and test unrelated `GetRun` and
  unrelated conversation-journal responsiveness while persistence is blocked.
- Risk: concurrent terminal helpers could publish one event and a different
  status.
- Mitigation: serialize each run's complete terminal helper lifecycle, make the
  shared transition return whether it won terminal sealing, and update status
  only for that winner.
- Risk: moving status later could reorder cleanup, causal snapshots, audit,
  profile persistence, backup, or pruning.
- Mitigation: pin causal/event order and keep cleanup before terminal
  transition, operational side effects after the matching status/event pair.
- Risk: the store API cannot atomically append an event and update the run row.
- Mitigation: enforce the one-way invariant that retained terminal status is
  never attempted unless terminal `AppendEvent` reported success. If append
  fails, durable status stays non-terminal; if final `UpdateRun` fails after a
  successful append, the durable event may lead the durable status. Both
  failures remain bounded and non-fatal to in-memory status/fanout. This does
  not claim two-way transactional atomicity or infer whether a third-party store
  applied a write before returning an error.
- Risk: recorder or store delay could weaken availability or lifecycle order.
- Mitigation: use context-bounded terminal store calls, keep external I/O out
  of the global conversation mutex, preserve target-conversation sequencing,
  drain retained and suppressed terminal recorders, and reclaim idle keyed
  sequence locks.
- Risk: an explicit terminal `StorageModeNone` policy intentionally removes the
  matching event from replay.
- Mitigation: preserve and test the existing redaction exception and scope the
  stronger replay implication to terminal events retained by policy.

## Verification Evidence

- Semantic red:
  `go test ./internal/harness -run
  '^TestTerminalStatusNeverPrecedesTerminalReplayEvent$' -count=1` failed all
  three cases: completed before `run.completed`, failed after `error.context`
  but before `run.failed`, and cancelled before `run.cancelled`.
- Review reds: blocked terminal append serialized an unrelated conversation;
  append failure still persisted terminal status; delayed non-terminal status
  overwrote terminal status; context-blocking final status persistence stranded
  the transition; an explicit terminal redaction drop exposed status before
  recorder drain; and keyed sequence entries were never reclaimed.
- Focused current green: all terminal publication/failure-policy regressions
  and HTTP replay passed normal and race at `-count=100`.
- Affected current green: complete harness/server normal and race passed;
  affected `go vet` passed.
- Real path: HTTP terminal polling followed immediately by Last-Event-ID run
  SSE replay passed for completed, failed, and cancelled.
- Repository: a fresh uninterrupted foreground non-TTY
  `./scripts/test-regression.sh` passed normal, race, and coverage
  (`total=85.6%`, `zero-functions=0`). Its immediately preceding invocation
  passed normal/race but returned red in coverage without retaining the hidden
  diagnostic; the unchanged coverage command and gate then passed before the
  complete clean rerun.
