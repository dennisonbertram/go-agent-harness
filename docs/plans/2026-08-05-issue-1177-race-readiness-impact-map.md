# Cross-Surface Impact Map: Issue #1177

## Task

- Task / issue: #1177 harnessd race-readiness fixtures.
- Plan link: `2026-08-05-issue-1177-race-readiness-plan.md`.
- Owner: harnessd test suite.
- Status: implemented; awaiting review/CI.

## Current Ownership, Callers, and Data Flow

- Entry points: the two `TestRunWithSignals*Memory*` fixtures in
  `cmd/harnessd/main_test.go`.
- Owning source of truth: `runMatrixTestWithListener` injects the listener
  seam into `runWithSignalsWithDeps`, captures the actual `listener.Addr()`,
  waits for `/healthz`, then delivers the real interrupt and awaits shutdown.
- Callers/consumers: Go test normal/race lanes only; no runtime consumers.
- Similar abstractions searched: `runMatrixTest`, `runMatrixTestWithListener`,
  and `freeLocalAddr` usages in the same test file.
- Conclusion: these two tests were outliers and should use the established
  ownership-aware test harness rather than predict a port.

## Config, API, CLI, and Tools

- User-facing config: none. Test-local `HARNESS_ADDR` changes from a predicted
  released port to `127.0.0.1:0`; both fixtures retain their three-second
  health diagnostic deadline and all memory/provider inputs.
- Defaults/fallbacks, endpoints, CLI/tools, and error mapping: none. `/healthz`
  is existing test readiness only.

## Persistence and Compatibility

- Schemas, migrations, caches, and compatibility: none; no production code or
  persisted test fixtures change.

## Lifecycle, Security, and Reliability

- Concurrency: removes reserve-close-rebind ownership race; startup readiness
  derives from the actual daemon listener.
- Security/privacy/secrets: unchanged; fixture dummy API keys remain local.
- Failure/recovery: early daemon failures are surfaced by the helper rather
  than being hidden until a guessed-address timeout.

## Product and Integration Surfaces

- Server/runtime: production daemon behavior unchanged; only its injected test
  listener seam is consumed.
- TUI/web/macOS, providers/models/tools, external automation, and UX: none,
  with source search limited to `cmd/harnessd/main_test.go`.

## Deployment and Operations

- Deployment/feature flags: none.
- Diagnostics: test failures retain helper listener/startup diagnostics.
- Rollback: revert the two fixture migrations; no data or service recovery.
- Runbooks: existing testing runbook remains authoritative.

## Regression Tests

- Characterization: hosted race CI failed both exact fixtures at the 3-second
  guessed-address health deadline.
- Acceptance: run each unchanged configuration through actual-listener health
  and graceful shutdown in focused normal/race stress.
- Edge/failure: existing helper tests continue proving actual listener identity
  and early-startup failure handling.
- Exact commands: `go test ./cmd/harnessd -run '^(TestRunWithSignalsObservationalMemoryFallsBackToOpenAIAPIKey|TestRunWithSignalsMemoryProviderModeFromProjectConfig)$' -count=20`; same with `-race`; complete `go test ./cmd/harnessd -count=1`; complete `go test -race ./cmd/harnessd -count=1`; and `./scripts/test-regression.sh` serially.

## Documentation and Handoff

- Specs/public docs: none.
- Implementation notes: plan/index and engineering, observational, system, and
  long-term-thinking logs record the test-only boundary and evidence.
- Training/release notes: none; CI fixture reliability is internal.
