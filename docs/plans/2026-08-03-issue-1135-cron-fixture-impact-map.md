# Cross-Surface Impact Map: Issue #1135 cron reconciliation fixture

## Task

- Issue: #1135 — deterministic post-bind scoped-admission proof.
- Plan: `2026-08-03-issue-1135-cron-fixture-plan.md`.
- Owner/status: cron test suite; test-only repair in verification.

## Current Ownership, Callers, and Data Flow

- Entry tests: post-bind embedded recovery and asynchronous remote recovery in
  `internal/cron/scheduler_test.go`.
- Runtime source of truth: `finishObservedExecution` persists a terminal row,
  calls `releaseReconciledScope`, then updates job tracking; asynchronous
  recovery is owned by `Scheduler.reconcileWG`.
- Search evidence: `rg` located `reconcileWG`, `releaseReconciledScope`,
  `reconciledLeases`, and both recovery fixtures. The old test channel was a
  store fixture observation, not the scheduler completion boundary.
- Conclusion: gate only the mock store, then join the existing scheduler
  lifecycle boundary; no duplicate runtime primitive is needed.

## Config, API, CLI, and Tools

- None. No settings, endpoints, data shapes, tools, CLI, or error semantics
  change; tests invoke existing internal scheduler APIs.

## Persistence and Compatibility

- None. The mock store only models the already-existing durable terminal write;
  schemas, migrations, data, and mixed-version behavior are unchanged.

## Lifecycle, Security, and Reliability

- Concurrency: admission is denied while the terminal durable write is blocked;
  `reconcileWG.Wait` establishes release completion before assertions.
- Security/privacy: none; test identifiers are synthetic and no credentials
  change.
- Recovery: both embedded post-bind and authenticated remote recovery retain
  their existing scope/no-overlap contracts.

## Product and Integration Surfaces

- Server/runtime: production source unchanged; test covers existing recovery.
- TUI/web/macOS: none; clients retain lifecycle behavior.
- Providers/tools/external automation: no routing or catalog effect. Remote
  fixture preserves the existing authenticated `/v1/runs/<id>` path.

## Deployment and Operations

- None. Revert is test/docs-only. Hosted race remains the authoritative
  promotion signal; no rollout or rollback behavior changes.

## Regression Tests

- Characterization/red: hosted full race saw post-terminal admission race with
  a still-held recovered scope.
- Acceptance: pre-return same-scope denial, post-`reconcileWG` zero scope/lease,
  and successful next admission for both embedded and remote recovery.
- Commands: focused normal/race stress, `go test ./internal/cron` normal/race,
  and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public specs: none.
- Internal records: plan/map, plan/log indexes, engineering/observational/system
  logs, long-term intent, issue evidence, and PR test evidence.
