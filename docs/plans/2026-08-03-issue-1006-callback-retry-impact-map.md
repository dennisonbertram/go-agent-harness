# Issue #1006 cross-surface impact map

## Task

- Task / issue: #1006 durable callback retry, idempotency, and run linkage.
- Plan link: `2026-08-03-issue-1006-callback-retry-plan.md`.
- Owner: callback runtime slice.
- Status: independent-review repairs implemented; affected normal/race and
  final repository gate verified locally.

## Current Ownership, Callers, and Data Flow

- Entry points: `set_delayed_callback` → `CallbackManager.Set`; timer/recovery
  → `CallbackManager.fire`; `harnessd.callbackRunStarter` → `Runner.StartRun`.
- Sources of truth: `delayed_callbacks` SQLite rows (#1005), in-memory timer
  index, and Runner's durable run store/admission.
- Consumers: callback tools/list/task union, callback event bridge/SSE, the
  same conversation's subsequent Runner transcript, and native/TUI clients via
  existing API/SSE.
- Similar abstraction searched: server `getOrStartCronRun`,
  `CronRunStartStore`, and `Runner.StartRunWithIDContext` implement a durable
  reserved-ID boundary for remote cron only.
- Conclusion: reuse the Runner reserved-ID admission capability for embedded
  callback execution, but do not couple callback state to cron HTTP tables.

## Config, API, CLI, and Tools

- Config: conservative internal retry defaults may be injected only if
  documented/tested; no provider-policy knobs or secrets.
- API/tools: callback list/status JSON and lifecycle event payload become
  additive with attempt, next attempt, safe last-error, and run link. Lease
  owner/token/deadline remain internal.
- CLI/native: no new controls in this issue. Existing consumers must preserve
  unknown/additive fields and show state through current task/list/event paths.
- Validation: callback IDs/scopes remain server-owned; terminal/cancelled rows
  cannot be re-dispatched.

## Persistence and Compatibility

- Migration: extend `delayed_callbacks` additively from #1005 fields with
  dispatch state, attempt/next-attempt, lease owner/until, safe error summary,
  stable reserved run ID, and updated timestamps.
- Compatibility: legacy `pending` rows get deterministic initial dispatch
  metadata and are recovered; existing in-memory manager remains usable in
  unit tests without a durable store.
- Mixed version: old binaries cannot interpret new states, so rollback retains
  rows and a new binary remains required to resume them; no destructive data
  migration.

## Lifecycle, Security, and Reliability

- One worker claims a due row conditionally; recovery reacquires expired
  dispatching leases; only a successful Runner admission becomes `started`.
- Retryable errors use bounded exponential backoff; invalid input/closed runner
  and explicit non-retryable errors fail terminally. Cancellation has one
  documented conditional winner and never cancels a successfully started run.
- Stable callback-derived reserved `run_` ID reaches the Runner admission
  boundary, preventing duplicate embedded runs after timeout/retry/restart.
- Later lifecycle events publish at conversation scope, and startup republishes
  the current durable state before timers restart so replay remains truthful
  after Runner restart without post-terminal run events.
- Error persistence and exposure use a callback-owned summary allowlist; no
  credentials, prompt expansion, or raw provider body is stored. Durable list
  failures propagate to API callers instead of returning partial success.

## Product and Integration Surfaces

- Server/runtime: `harnessd` callback starter must return the Runner-created
  ID and bind scope; harness event bridge carries richer lifecycle payload.
- TUI/web/macOS: no new UI code, but list/task and SSE contracts become able to
  report retry/started/failed state; #1007 owns native controls.
- Provider/catalog: none beyond generic safe error classification.
- External systems: none; callback is embedded, not remote cronsd.

## Deployment and Operations

- Startup migration precedes recovery; recovery remains after Runner/listener
  readiness. Shutdown fences timers/claims and waits admitted dispatches.
- Structured logs/events expose callback ID/state/attempt/run ID, never raw
  secrets. Rollback can set one attempt while retaining rows.
- Operator documentation records state transition and cancellation semantics.

## Regression Tests

- First expected red: retryable start error currently writes `fired`; it must
  instead preserve a durable `retry_wait` row and later start one reserved run.
- Acceptance: retry→started; nonretryable/exhausted→failed; duplicate claim;
  expired lease recovery; run/scope link; cancellation races; events/list data.
- Integration: real harnessd callback continuation remains a later #1010
  proof; this issue supplies deterministic runtime/API lifecycle coverage.
- Commands: focused callback package normal/race; harness/server lifecycle;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update plan/index/logs now; update public/operator behavior only once code
  and tests land. Record exact red/green commands and review outcome in PR.
