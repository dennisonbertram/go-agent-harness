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
- Exact-head review finding: terminal event success was tracked for pruning but
  terminal status update success was discarded. A failed `UpdateRun` could
  therefore make pruning evict truthful live state while the durable row stayed
  non-terminal; permanent append/update failures could also grow protected
  memory without an admission bound.
- Exact-head concurrent review finding: Continue validated its completed source,
  released `Runner.mu`, and performed bounded durability recovery before its
  single-winner mutation. A concurrent Start recovery could prune that source
  through the shared prune path, turning a valid continuation into a spurious
  `ErrRunNotFound`.
- Hosted aggregate review finding: race run `30656467482` exposed another stale
  test assumption in `TestRunnerHookErrorFailOpen`: `collectRunEvents` returned
  terminal replay history while the valid later completed-status commit was
  still in progress. The helper's 215 references and its configurable-timeout
  equivalent were audited; no caller intentionally observes that window.

## Scope

- In scope: one shared Runner terminal-transition seam for completed, failed,
  cancelled, max-step failed, and max-turn failed paths; deterministic
  concurrency and replay regressions; event/status-aware safe pruning; bounded
  degraded admission and recovery; explicit Start/Continue HTTP 503 mapping;
  real HTTP poll-then-replay proof; test-helper settlement semantics that keep
  terminal event collection and terminal status waiting distinct.
- Out of scope: PR #1060/#1055 changes, cron/callback behavior, conversation
  cursor redesign, GUI visual changes, provider routing, schemas, and workflow
  timing issue #1049.

## Documentation Contract

- Feature status: durability-retention hardening remains implemented; the
  hosted settled-helper repair passes focused stress, `make test-race`, and the
  unchanged full repository regression gate.
- Public API behavior: event/status wire formats stay unchanged. During a full
  terminal durability backlog, Start/Continue now return documented HTTP 503
  `terminal_durability_unavailable` after bounded recovery fails.
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
- Retention/admission controls: with retention 1, pre-admit several runs and
  fail terminal `UpdateRun`; every truthful terminal state must remain in
  memory while durable rows remain non-terminal. Count both append- and
  status-pending states toward one gate; reject concurrent Start admissions and
  Continue admission with a typed error/HTTP 503 at the cap; recover concurrent
  callers after status persistence returns; preserve source error precedence,
  intentional StorageModeNone suppression, and no-store behavior. Pin one
  shared retry deadline of at most 250 ms with no store I/O under Runner,
  status, event-journal, or conversation locks.
- Continuation reservation control: deterministically block Continue's first
  recovery write after source validation, let a concurrent Start recover and
  prune, then require Continue to retain its source through the single-winner
  handoff. The reservation must apply to every completed-run prune caller and
  release on all success/error paths without holding locks across store I/O.
- Settled collection control: pause terminal publication after replay history is
  visible but before status commit, call the shared event collector, and prove
  it does not return until a terminal status is independently observable. Keep
  one total bounded deadline and retain the exact collected event assertions.
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
- [x] Track terminal status durability separately and require event plus status
  resolution before store-backed pruning.
- [x] Add finite append/status backlog admission, bounded status recovery, typed
  errors, and Start/Continue HTTP 503 mappings.
- [x] Prove concurrent outage/recovery, unlocked deadline, StorageModeNone, and
  no-store behavior.
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
- Risk: protecting event/status-pending terminal states from pruning could make
  a permanent store outage consume memory without bound.
- Mitigation: both failure classes consume the `MaxCompletedRetention` backlog.
  At the cap, new admissions retry status-only gaps under one shared deadline
  and otherwise fail closed. Already-admitted work may finish and remain visible,
  but growth stops at the finite population admitted before outage detection.
  Ambiguous append failures are never replayed in-process.
- Risk: degraded admission could mask normal Continue errors or block unrelated
  state while retrying.
- Mitigation: validate unknown/non-completed sources before the global gate,
  revalidate for the single-winner mutation, map only the typed error to 503,
  and prove the retry holds no Runner/status/conversation lock.
- Risk: validation followed by unlocked recovery lets another admission's prune
  delete the continuation source before revalidation.
- Mitigation: reserve the validated source under `Runner.mu`, exclude reserved
  sources in the shared prune implementation used by every caller, and release
  plus re-prune on every return path. The reservation does not grant the
  continuation winner status; the existing write-lock revalidation still does.
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
- Exact-head P1 reds: at retention 1, append-pending admission remained open;
  status-update-failed terminal runs were pruned despite non-terminal durable
  rows; and both Start/Continue mapped the new typed degraded state to 400.
- Exact-head concurrent P1 red: a phase hook paused Continue after source
  validation, concurrent Start recovery pruned that oldest source at retention
  1, and resumed Continue failed deterministically with `run not found`.
- Exact-head focused green: append/status pending runs remain visible, both
  close admission at the cap, 16 concurrent callers reject then recover under
  race, successful status recovery immediately restores the retention bound,
  status recovery uses one unlocked total deadline, Start/Continue return the
  explicit 503, and StorageModeNone/no-store controls pass.
- Concurrent focused green: the source reservation is respected by the shared
  prune path, releases after backpressure, and preserves exactly one continuation
  winner. The new regression, cleanup control, and existing winner control pass
  normal/race at `-count=100`.
- Focused current green: all terminal publication/failure-policy regressions
  and HTTP replay passed normal and race at `-count=100`.
- Affected current green: complete harness/server normal and race passed;
  affected `go vet` passed.
- Real path: HTTP terminal polling followed immediately by Last-Event-ID run
  SSE replay passed for completed, failed, and cancelled.
- Gate-test review: the complete race gate exposed a replay-first test that read
  status before the later commit. Its independent failed-status wait now matches
  the one-way contract, and the affected race gate passes.
- Hosted settled-helper red: race run `30656467482` reproduced the next stale
  immediate-status assumption in `TestRunnerHookErrorFailOpen`. A deterministic
  pre-status barrier then proved the shared collector returned terminal history
  while status remained running.
- Helper audit/green: all 215 shared collector references, the configurable
  timeout variant, and the separate snapshot helper family were classified.
  No shared collector consumer requires the transition window. Settlement now
  preserves event history and independently requires terminal status within one
  total deadline; the deterministic test, hook family, and timeout caller pass
  normal/race at `-count=100`, and hosted-equivalent `make test-race` passes.
- Repository: the final direct foreground non-TTY
  `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-go-cache
  ./scripts/test-regression.sh` passed normal, full race, and coverage with
  `coveragegate: PASS (total=85.7%, min=80.0%, zero-functions=0)` on the
  settled-helper diff.
