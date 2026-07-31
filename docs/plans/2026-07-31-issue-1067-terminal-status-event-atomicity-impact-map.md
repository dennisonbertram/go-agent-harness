# Cross-Surface Impact Map: Issue #1067 Terminal Publication Atomicity

## Task

- Task / issue: #1067, terminal status visible before matching replay event.
- Plan: `2026-07-31-issue-1067-terminal-status-event-atomicity-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `completeRun`, `failRun`, `failRunMaxSteps`,
  `failRunMaxTurns`, and `cancelledRun`.
- Source of truth: `Runner` owns `runState.run.Status`, `runState.events`,
  `runState.terminated`, recorder channels, and run subscribers;
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

- Config/env/defaults: none.
- Endpoints/request/response/wire formats: unchanged `Run`, `Event`, event IDs,
  SSE names, payload schema, and HTTP routes.
- CLI/tools/integrations: no command changes; terminal polling and stream
  consumers gain a stronger ordering guarantee.
- Error states: unchanged completed/failed/cancelled values and payloads.

## Persistence and Compatibility

- Schemas/migrations/caches/generated data: none.
- Store order: matching terminal `AppendEvent` remains bounded and precedes
  recorder dispatch, terminal status `UpdateRun`, and subscriber fanout;
  status persistence occurs after releasing the per-conversation journal lock.
- Recorder: terminal JSONL remains queued after all prior events, closed once,
  and drained before terminal transition returns.
- Compatibility: additive ordering guarantee only; event/status values and
  replay IDs remain stable.
- Mixed-version behavior: process-local; older daemons retain the race until
  upgraded, with no data migration.

## Lifecycle, Security, and Reliability

- Concurrency: the winning terminal event seals the ledger before status is
  updated; competing terminal helpers cannot overwrite it with a mismatched
  status.
- Cancellation/retries/cleanup: cooperative cancellation and idempotency stay
  unchanged; workspace/tool/MCP cleanup remains before terminal publication.
- Locks/resources: terminal store/recorder waits remain outside `Runner.mu`, so
  unrelated queries are not blocked; status-store I/O also remains outside the
  per-conversation journal lock, so unrelated event journals are not blocked.
- Auth/permissions/privacy/secrets: no boundary change after searches through
  run routes and redaction/audit paths; terminal payload redaction remains
  owned by the event journal. Explicit terminal `StorageModeNone` remains the
  documented exception: it seals and publishes status without replaying the
  intentionally suppressed event.
- Failure/recovery: bounded store failures remain non-fatal and in-memory replay
  remains authoritative for the live Runner; persisted-before-prune guards stay
  intact.

## Product and Integration Surfaces

- Server/runtime: `GetRun` terminal now implies immediate `Subscribe` replay
  contains the matching event.
- TUI/web/macOS/other clients: terminal badges, failure text, exit codes, and
  transcript state no longer disagree during the publication window; no client
  code changes.
- Provider/model/tool catalogs/routing: none; provider failure is only a caller.
- External systems/automation: cron/callback/workflow semantics unchanged.
- UX/accessibility/focus/motion: no visual or interaction change.

## Deployment and Operations

- Deployment/migrations/flags: ordinary daemon rollout; no migration or flag.
- Observability: deterministic regression records transition phases without
  logging prompts, event payload secrets, or credentials.
- Rollback: revert if terminal fanout deadlocks, unrelated `GetRun` blocks,
  cleanup order changes, recorder output truncates, or cancellation regresses.
- Runbooks/operator docs: no public/operator command changes.

## Regression Tests

- First red: phase barrier proves old completed/failed/cancelled status can win
  before terminal history.
- Acceptance: terminal status implies one matching replay event; failed causal
  snapshot precedes `run.failed`; competing terminal transitions match the
  winning sealed event; later same-conversation events cannot overtake terminal
  fanout to an existing conversation subscriber.
- Store/recorder: durability and existing drain barriers preserve non-terminal
  status without blocking unrelated queries; a blocked status-store write does
  not block unrelated event journals.
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
