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
- Reconciliation is scheduler-owned lifecycle work: `Stop` seals new
  reconciliation admission, cancels its context, joins every existing observer
  before returning, and cancellation retains the durable active row/lease
  rather than persisting a synthetic failed terminal state. A late bind after
  Stop is a no-op.
- Terminal persistence has a separate narrow lifecycle fence: remote/embedded
  observation never holds it, but UpdateExecution, local lease release, and
  TouchJobRun are linearized with Stop. A Stop-winning race preserves the
  active row/lease; a commit-winning race completes the full terminal
  transition before Stop returns. Only definitive `IsJobNotFound` lookup
  results terminalize an otherwise recovered row; cancellation and transient
  lookup errors remain nonterminal.
- Recovered observer results obey the same error semantics as live results:
  only `observed=true && err=nil` may enter terminal persistence. Any observer
  error, unobserved result, or scheduler cancellation retains the active row,
  run link, and scope lease for a later retry; reconciliation continues with
  subsequent rows so one transient provider failure cannot block an unrelated
  known-terminal run.
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
- Add shutdown regression tests for observer cancellation/join, post-stop bind
  no-op, and recovered authenticated remote poll cancellation with zero false
  terminal persistence. The exact pre-fix test-only replay is red; direct
  normal and race x10 focused commands pass. Full repository race remains an
  explicit pending acceptance gate.
- Add deterministic cancel-wins and commit-wins terminal-gate regressions plus
  canceled/transient job-lookup retention. Exact `9181311` reds are recorded;
  each new direct normal and race x20 command passed. The real `httptest`
  listener requires host-local execution because sandbox IPv6 binding is
  denied; the host-local normal/race lifecycle bundle passed.
- Commands: `go test ./internal/cron -run
  'Test.*(Execution|Overlap|RunID|Monotonic)' -count=1`; corresponding race
  run; then `./scripts/test-regression.sh`.
- Live observations: add embedded and real remote cancellation/join, explicit
  stop-wins and commit-wins terminal ordering, `observed=false`, transport
  error, terminal-write failure, and shell drain controls. Base red replay
  distinguishes new lifecycle failures from the already-green commit/shell
  controls; direct normal/race x20 is required before full regression.
- Recovery observations: add linked recovered 503 and stream-error retention,
  unobserved retention, explicit terminal-failure control, and mixed
  error-plus-terminal-success continuation. Exact `1d699808` red replay and
  direct normal/race x20 are required before repository-wide regression.
- Embedded observation: consume a terminal Runner replay event as a hint, then
  obtain completed/failed/cancelled status and summaries from authoritative
  `GetRun`, with a cancellation-bound low-rate status fallback for suppressed
  terminal events; test replay-before-commit, cancellation, closed stream,
  status-only terminal, and live terminal delivery. This bridge-only fix
  changes no remote cronsd or UI API.

## Documentation and Handoff

- Plan/impact map and long-term/engineering logs describe the canary defect,
  ownership and explicit remote reconciliation boundary.
- `docs/plans/INDEX.md` and docs log index remain maintained.
