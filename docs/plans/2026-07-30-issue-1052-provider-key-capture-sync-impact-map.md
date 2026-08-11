# Cross-Surface Impact Map: Provider API-key capture synchronization

## Task

- Task / issue: #1052
- Plan link: `2026-07-30-issue-1052-provider-key-capture-sync-plan.md`
- Owner: Codex
- Status: Implemented; promotion pending

## Current Ownership, Callers, and Data Flow

- Entry points: `TestMatrix_ProviderAPIKeyCapture`.
- Owning packages/types/functions and source of truth:
  `cmd/harnessd/main_test.go`; injected `getenv` supplies the sentinel and the
  injected `newProvider` factory receives `openai.Config`.
- Callers, consumers, events, and downstream data: Test-only call to
  `runWithSignals`; no production caller changes.
- Similar abstractions searched: `awaitHealthy`, `runWithSignals`, and injected
  provider factories in `cmd/harnessd`.
- Search commands/evidence:
  `rg -n "awaitHealthy|ProviderAPIKeyCapture|runWithSignals" cmd/harnessd`.
- Duplication/ownership conclusion: The provider factory is already the
  authoritative observation point; HTTP health is redundant for this contract.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults / fallbacks: None.
- Environment variables, config files, or saved settings touched: Test-local
  `OPENAI_API_KEY` sentinel remains unchanged.
- Endpoints, request fields, response fields, or server wiring affected: None.
- CLI commands, tools, wire formats, or integrations affected: None.
- Error states / validation changes: None.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: None; test-only.
- Partial rollout and mixed-version behavior: None.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership:
  Synchronize directly with provider-factory invocation, then deliver the
  existing interrupt and require bounded `runWithSignals` completion.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  The sentinel key remains test-local and is never logged.
- Failure modes, recovery, idempotency, and data repair: A missing factory
  invocation or non-terminating server fails with a bounded diagnostic.

## Product and Integration Surfaces

- Server/runtime: Production code unchanged.
- TUI/web/macOS/other clients: None.
- Provider/model/tool catalog and routing: Provider factory observation only;
  routing behavior unchanged.
- External systems and automation: GitHub Actions race reliability improves.
- UX states, keyboard/focus/accessibility/motion: None.

## Deployment and Operations

- Deployment/migration order and feature flags: None.
- Logs, metrics, traces, alerts, and support diagnostics: None.
- Rollback triggers and recovery steps: Revert if exact-head normal/race checks
  or local full regression fail.
- Runbooks and operator docs: None.

## Regression Tests

- Characterization and first expected red test: Test waits for an explicit
  provider-invocation signal that is initially never published.
- New acceptance tests required: Updated provider-key capture test.
- Edge, negative, failure, lifecycle, and security tests: Existing bounded
  graceful-shutdown failure remains.
- Integration/e2e/real-path proof: Hosted normal/race jobs on the exact PR head.
- Cross-surface regressions to guard: Whole `cmd/harnessd` normal/race suites
  and repository normal/race/coverage gate.
- Exact targeted and full commands:
  `go test ./cmd/harnessd -run TestMatrix_ProviderAPIKeyCapture -count=100`;
  `go test -race ./cmd/harnessd -run TestMatrix_ProviderAPIKeyCapture -count=100`;
  `go test ./cmd/harnessd -count=1`;
  `go test -race ./cmd/harnessd -count=1`;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: Plan and impact map.
- Implementation notes/logs/indexes after code: Plan/index, engineering log,
  long-term-thinking log.
- Training/onboarding/release notes: None; test-only.

## Warning Check

- Every surface is either mapped or explicitly marked unaffected with rationale.
