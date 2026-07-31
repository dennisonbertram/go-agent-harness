# Cross-Surface Impact Map: Issue #1067 Terminal Publication Atomicity

## Task

- Task / issue: #1067, terminal status visible before matching replay event.
- Plan: `2026-07-31-issue-1067-terminal-status-event-atomicity-plan.md`.
- Owner: Codex.
- Status: exact-head durability-retention hardening implemented and verified
  locally through focused stress, affected normal/race/vet, and the unchanged
  full repository regression gate; hosted checks remain pending promotion.

## Current Ownership, Callers, and Data Flow

- Entry points: `completeRun`, `failRun`, `failRunMaxSteps`,
  `failRunMaxTurns`, and `cancelledRun`.
- Source of truth: `Runner` owns `runState.run.Status`, `runState.events`,
  `runState.terminated`, terminal event/status durability markers, recorder
  channels, and run subscribers;
  `eventJournal` owns append/store/fanout ordering.
- Callers/consumers: step-engine completion/provider/tool/budget/cancellation
  paths; `GetRun`; `Subscribe`; run HTTP/SSE routes; CLI, TUI, and macOS
  transcript/lifecycle consumers.
- Similar abstractions searched: `rg -n
  "setStatus|completeRun|failRun|cancelledRun|publishTerminal|GetRun|Subscribe"
  internal/harness internal/server`. No second terminal lifecycle owner exists.
- Duplication conclusion: repair the shared Runner transition; do not add
  provider-, server-, or client-specific compensation.

## Config, API, CLI, and Tools

- Config/env/defaults: `MaxCompletedRetention` retains its value/default and now
  also bounds the unresolved store-backed terminal durability backlog.
- Endpoints/request/response/wire formats: unchanged `Run`, `Event`, event IDs,
  SSE names, payload schema, and HTTP routes. When the durability backlog is at
  its cap after bounded recovery, `POST /v1/runs` and
  `POST /v1/runs/{id}/continue` return HTTP 503 with
  `terminal_durability_unavailable`.
- CLI/tools/integrations: no command changes; terminal polling and stream
  consumers gain a stronger ordering guarantee.
- Error states: completed/failed/cancelled values and payloads are unchanged;
  `TerminalDurabilityBackpressureError` is the typed degraded-admission error.

## Persistence and Compatibility

- Schemas/migrations/caches/generated data: none.
- Store order: matching retained terminal `AppendEvent` is bounded and precedes
  recorder dispatch, conditional terminal status `UpdateRun`, in-memory status
  publication, and subscriber fanout. `UpdateRun` is attempted only after
  `AppendEvent` reports success. An append error leaves durable status
  non-terminal; an update error can leave a durable terminal event with a
  non-terminal durable run row. In either case, bounded in-memory terminal
  publication/fanout completes. This is a one-way invariant, not a two-record
  transaction; third-party stores must return errors without partial writes if
  they need the same durable-read characterization as the built-in stores.
- Recorder: terminal JSONL remains queued after all prior events, closed once,
  and drained before terminal transition returns. An explicitly suppressed
  `StorageModeNone` terminal also closes and drains the recorder before status
  visibility even though no terminal event is appended.
- Retention: a store-backed terminal state is prunable only after event
  persistence (or explicit `StorageModeNone` suppression) and final status
  persistence are both acknowledged. No-store states remain process-local and
  outside durable pruning/backpressure.
- Compatibility: event/status values and replay IDs remain stable. The new 503
  is an intentional fail-closed availability change during persistence outage.
- Mixed-version behavior: process-local; older daemons retain the race until
  upgraded, with no data migration.

## Lifecycle, Security, and Reliability

- Concurrency: the winning terminal event seals the ledger before status is
  updated; competing terminal helpers cannot overwrite it with a mismatched
  status. Every status snapshot/persist/commit sequence shares a per-run mutex,
  so a delayed running/waiting write cannot overwrite terminal state. A
  validated Continue source requires a temporary in-state reservation across
  unlocked durability recovery so a concurrent Start prune cannot remove it
  before the existing single-winner continuation commit.
- Cancellation/retries/cleanup: cooperative cancellation and idempotency stay
  unchanged; workspace/tool/MCP cleanup remains before terminal publication.
  Status-only durability gaps retry safely as idempotent `UpdateRun` overwrites
  under one total deadline of at most 250 ms. Ambiguous failed event appends are
  counted but not retried because a third-party store may have applied them.
- Locks/resources: terminal store/recorder waits remain outside `Runner.mu` and
  the global conversation journal lock. A refcounted target-conversation lock
  prevents same-conversation overtaking while unrelated journals progress, and
  deletes itself when owners and queued waiters drain.
- Auth/permissions/privacy/secrets: no boundary change after searches through
  run routes and redaction/audit paths; terminal payload redaction remains
  owned by the event journal. Explicit terminal `StorageModeNone` remains the
  documented exception: it seals and publishes status without replaying the
  intentionally suppressed event.
- Failure/recovery: append and status writes use bounded contexts and failures
  remain non-fatal to already-admitted work. Both failure classes count toward
  the same finite admission gate. At the configured cap, Start/Continue retries
  recoverable status gaps without holding Runner/status/conversation locks,
  then rejects admission if the backlog remains full. Already-admitted work may
  temporarily exceed the numeric cap but the excess is bounded by the finite
  active/queued population admitted before outage detection. No two-way durable
  atomicity is claimed without a transactional store API. A successful status
  retry prunes newly durable candidates immediately, restoring the configured
  completed-retention bound before admission reopens.

## Product and Integration Surfaces

- Server/runtime: `GetRun` terminal now implies immediate `Subscribe` replay
  contains the matching event. Start/Continue expose the explicit degraded 503;
  Continue preserves not-found/non-completed source error precedence.
- TUI/web/macOS/other clients: terminal badges, failure text, exit codes, and
  transcript state no longer disagree during the publication window; no client
  code changes.
- Provider/model/tool catalogs/routing: none; provider failure is only a caller.
- External systems/automation: internal StartRun callers inherit the typed
  fail-closed error through their existing error paths; cron/callback/workflow
  request schemas and successful semantics are unchanged.
- UX/accessibility/focus/motion: no visual or interaction change.

## Deployment and Operations

- Deployment/migrations/flags: ordinary daemon rollout; no migration or flag.
- Observability: deterministic regression records transition phases without
  logging prompts, event payload secrets, or credentials. Operators can
  correlate store append/update errors with 503
  `terminal_durability_unavailable`; successful status retry reopens admission.
- Rollback: revert if healthy stores spuriously return 503, the total retry
  exceeds 250 ms, source error precedence changes, terminal fanout deadlocks,
  or unrelated Runner/conversation work blocks. During a real store outage,
  stop new producers and preserve the process before rollback because removing
  the gate reintroduces truthful-state eviction or unbounded protected memory.
- Runbooks/operator docs: no command changes; the issue and implementation logs
  carry the degraded-mode recovery policy.

## Regression Tests

- First red: phase barrier proves old completed/failed/cancelled status can win
  before terminal history.
- Acceptance: terminal status implies one matching replay event; failed causal
  snapshot precedes `run.failed`; competing terminal transitions match the
  winning sealed event; later same-conversation events cannot overtake terminal
  fanout to an existing conversation subscriber.
- Store/recorder: blocked append allows unrelated conversations but not target
  overtaking; append error prevents terminal status persistence; status update
  error/timeout preserves live publication; retained and suppressed terminal
  recorder paths drain before status visibility.
- Retention/admission: several already-admitted completions remain visible at
  retention 1 when final status updates fail and their durable rows remain
  non-terminal; append- and status-pending runs both close admission at the cap;
  concurrent callers reject during outage and recover after status persistence;
  the retry uses one unlocked deadline; concurrent Start recovery cannot prune
  a reserved Continue source; reservation cleanup and single-winner behavior
  remain bounded; StorageModeNone and no-store policies remain explicit.
- Integration: HTTP poll immediately followed by run SSE replay for all three
  statuses.
- Exact gates: focused normal/race stress `-count=100`; harness/server
  normal/race/vet; unchanged foreground non-TTY regression; hosted checks.

## Documentation and Handoff

- Plans/specs: issue-specific plan and this map.
- Logs/indexes: engineering, observational, system, long-term, plans index, and
  active plan.
- Public/training/release docs: none because no new route, schema, or command is
  introduced.

## Warning Check

- Every cross-surface heading is resolved. Unaffected surfaces are explicitly
  named with search and data-flow rationale above.
