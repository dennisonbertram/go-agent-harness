# Issue #1098: Deleted-Job Cron Reconciliation Impact Map

## Task

- Task / issue: #1098 deleted-job reconciliation coverage.
- Plan link: `2026-08-03-issue-1098-cron-deleted-reconcile-plan.md`.
- Owner: isolated `codex/issue-1098-cron-deleted-reconcile` worktree.
- Status: implemented test-only coverage repair; full regression green.

## Current Ownership, Callers, and Data Flow

- Entry point: `Scheduler.Start` restores durable active leases and starts
  asynchronous `reconcileExecutionRows` once an observer is bound.
- Owner/source of truth: `internal/cron.Scheduler`, its durable `Store` active
  execution records, `reconciledLeases`, and `activeScopes` admission map.
- Deleted-job path: no startup job exists in `byID`; `Store.GetJob` returns a
  definitive not-found error; `finishUnavailableExecution` persists failure;
  `reconciledScope` retrieves the stored scope key for ordered release.
- Search evidence: `rg 'finishUnavailableExecution|reconciledScope|reconcileExecutionRows|ErrJobNotFound' internal/cron` locates the only owners,
  callers, and existing cancellation/transient tests.
- Conclusion: direct scheduler tests are the narrow owner; no second
  reconciliation coordinator is appropriate.

## Config, API, CLI, and Tools

- No configuration, defaults, environment, HTTP endpoint, CLI command, tool,
  wire format, or validation change.
- Existing clients only observe the already-defined failed execution history;
  no user-facing payload is added.

## Persistence and Compatibility

- Existing `cron_executions` fields (`status`, `finished_at`, `duration_ms`,
  `error_text`, and `run_id`) are used without schema/migration changes.
- The deleted `cron_jobs` row is never touched. Mixed-version behavior is
  unchanged because this slice adds coverage rather than a protocol change.

## Lifecycle, Security, and Reliability

- Terminal persistence is guarded by `lifecycleMu`; Stop/cancellation must not
  synthesize a terminal transition.
- The recovered lease remains present through persistence failure and releases
  only after `UpdateExecution` succeeds. That ordering prevents duplicate
  same-conversation scheduled work.
- Auth, authorization, privacy, and secrets are unaffected; no network path is
  exercised by this test-only slice.

## Product and Integration Surfaces

- Server/runtime: cron scheduler recovery and durable admission only.
- API/TUI/web/macOS: no code change; unchanged execution status is the common
  downstream representation.
- Provider/model/tool catalog and external automation: none; terminal observer
  and callback routing are explicitly outside scope.
- UX/accessibility: none; no rendered control or copy changes.

## Deployment and Operations

- Deploy as a small regression-gate repair before blocked dependent slices.
- Logs capture the invariant and direct coverage evidence; no alert, migration,
  flag, runbook, or operational rollback step changes.

## Regression Tests

- Red tests cover `ErrJobNotFound` and `sql.ErrNoRows`, persistence ordering,
  `RunID` retention, zero deleted-job touches, post-success readmission, and
  failure retention/duplicate denial.
- Existing cancellation, transient lookup, Stop-wins, and commit-wins tests
  are retained as lifecycle controls.
- Exact commands: targeted normal and `-race -count=20`, package coverage and
  function report, then `./scripts/test-regression.sh`.
- Evidence: former zero helpers report 91.7% and 100.0%; the rebased full gate
  passed at 85.6% total with zero uncovered functions.

## Documentation and Handoff

- No public docs change because external contracts are unchanged.
- Update plan/log indexes plus engineering, observational, system, and
  long-term-thinking records. Handoff reports base SHA, red result, final
  diff, focused/full command evidence, and no-push commit SHA.
