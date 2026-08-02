# Impact Map — Issue #1004 cron lifecycle

## Task

- Task / issue: #1004 cron execution lifecycle.
- Plan link: `2026-08-02-issue-1004-cron-lifecycle-plan.md`.
- Owner: Codex implementation worktree.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `Scheduler.fireJob*`, `TriggerJob`, embedded cron adapter, and
  cronsd scheduler use `cron.Executor` implementations.
- Source of truth: `cron_executions` is persisted by `SQLiteStore`; `Job`
  stores schedule/scope/run tracking. `HarnessExecutor` starts runs through
  `RunStarter`.
- Consumers: cron history API, harness cron tools, TUI/native Activity and
  transcript correlation. #1003's remote transport implements the same
  `RunStarter` contract but is not in this base.
- Search evidence: `rg DispatchExecutor|Execution|TouchJobRun|RunID
  internal/cron cmd/harnessd` on 2026-08-02.
- Conclusion: scheduler owns admission and execution persistence; executor
  owns structured start outcome. No output-text parsing is permitted.

## Config, API, CLI, and Tools

- Config: no new external configuration in this slice; default-on scope
  overlap admission is SQLite-durable across embedded scheduler processes.
- API: existing history serialization obtains additive execution status/run ID
  fields from the shared `Execution` value. No route/path change.
- Tools/clients: cron history is the contract consumed by GUI/TUI; rendered
  proof remains #1010 integration work.

## Persistence and Compatibility

- Existing columns include run ID, status, error text and duration. Status
  values are additive (`starting`, `skipped`) and old clients remain able to
  render strings.
- `TouchJobRun` changes to a monotonic SQL update so mixed binaries cannot move
  timestamps or `updated_at` backward. Existing rows need no migration.

## Lifecycle, Security, and Reliability

- Scheduler atomically reserves a scope key in SQLite before accepting
  same-conversation work and releases its local lease only after terminal
  persistence; another scope can proceed. A failed run-link write retries and
  then fails closed without releasing the durable active lease.
- A successful harness start must persist its structured ID immediately.
  Startup reloads active rows, retains both linked and ambiguous-unlinked
  scope leases, and finalizes only when the generic observer is available.
- Scope derives only from durable Job fields. No new credentials or authority
  boundary is introduced.
- Restart recovery has synchronous durable lease restoration plus asynchronous
  observation. Execution-ID ownership prevents repeated readiness/bind calls
  from double-counting or double-releasing local scope.
- The generic observer/reconciliation contract is implemented here. #1003's
  remote adapter must adopt it; no remote implementation is guessed here.

## Product and Integration Surfaces

- Server/runtime: embedded scheduler and persistence are changed; remote
  cronsd is compatible through the generic executor outcome interface.
- TUI/macOS: no direct source change; history fields are a prerequisite for
  #1007/#1009 controls and #1010 proof.
- Provider/model/tool catalog: none; scheduled run scope and prompt remain
  exactly as #1002/#1001 defined.

## Deployment and Operations

- Logs retain execution IDs and report skip admission. Cron history provides
  operator-visible machine-readable reason.
- Rollback is source-level only; rows remain valid and no migration rollback
  is needed.

## Regression Tests

- First red: structured harness outcome run ID is persisted independently of
  output summary.
- Add same/different scope overlap and monotonic SQLite completion tests.
- Commands: `go test ./internal/cron -run
  'Test.*(Execution|Overlap|RunID|Monotonic)' -count=1`; corresponding race
  run; then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Plan/impact map and long-term/engineering logs describe the canary defect,
  ownership and explicit remote reconciliation boundary.
- `docs/plans/INDEX.md` and docs log index remain maintained.
