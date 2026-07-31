# Cross-Surface Impact Map: Issue #1062 Provider-Key Matrix Health Wait

## Task

- Task / issue: contention-tolerant provider-key matrix health wait, #1062.
- Plan: `2026-07-31-issue-1062-provider-key-matrix-health-wait-plan.md`.
- Owner: Codex.
- Status: implemented and verified locally and in hosted checks on PR #1063.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestMatrix_ProviderAPIKeyCapture`.
- Owner/source of truth: `awaitHealthy` polls the fixture's `/healthz`; the
  provider factory closure separately captures `openai.Config.APIKey`.
- Callers/consumers: this test starts `runWithSignals`, waits for health, sends
  an interrupt, then asserts the captured key.
- Similar abstraction: `runMatrixTest` already gives the same startup path ten
  seconds under suite contention.
- Search evidence: `rg -n "func awaitHealthy|awaitHealthy\\("
  cmd/harnessd/main_test.go` and the three-second deadline search recorded in
  #1062.
- Ownership conclusion: the failure is the fixture's caller-supplied budget,
  not provider capture or production startup.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: none; only an in-test environment
  map and fake key are used.
- Endpoints/wire formats/server wiring: unchanged; `/healthz` remains the
  required readiness signal.
- CLI/tools/integrations/error validation: none.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, and ownership: none.
- Backward/forward compatibility and mixed versions: none because no runtime
  artifact changes.

## Lifecycle, Security, and Reliability

- Concurrency: tolerate shared runner scheduling delay while retaining an
  explicit deadline.
- Cancellation/cleanup: interrupt and bounded graceful-shutdown assertions are
  unchanged.
- Authentication/privacy/secrets: fake fixture key only; no log or credential
  behavior changes.
- Failure/recovery: genuine failure to return HTTP 200 still fails with the
  address and timeout.

## Product and Integration Surfaces

- Server/runtime, TUI, web, macOS, providers, catalogs, tools, external
  automation, and accessibility: no production impact after repository search.
- GitHub Actions: the race suite stops measuring a three-second scheduling
  accident for this fixture.

## Deployment and Operations

- Deployment/migration/flags: none.
- Diagnostics: preserve the exact `awaitHealthy` failure message.
- Rollback: revert the one-line test budget change if focused evidence shows it
  masks a deterministic startup defect.
- Operator docs/runbooks: none.

## Regression Tests

- Characterization/red: hosted run `30583930460`, job `91010705523`.
- Acceptance: the existing provider-key test reaches real health and captures
  the exact fake key under normal and race stress.
- Negative/lifecycle: health remains mandatory; graceful shutdown stays
  bounded.
- Integration/real path: matrix slice plus the full repository regression gate.
- Exact commands: focused normal/race `-count=100`, matrix normal/race, and
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public specs/docs: none because runtime behavior is unchanged.
- Update engineering/long-term logs, plan index, issue #1062, and PR #1063
  verification/linkage.
- Training/release notes: none.

## Warning Check

- Every surface is mapped above; production surfaces are explicitly unaffected
  because the change is confined to `cmd/harnessd/main_test.go`.
