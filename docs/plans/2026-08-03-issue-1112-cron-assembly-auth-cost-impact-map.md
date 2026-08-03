# Cross-Surface Impact Map: Issue #1112

## Task

- Task / issue: #1112 authenticated cron assembly race timeout.
- Plan link: `2026-08-03-issue-1112-cron-assembly-auth-cost-plan.md`.
- Owner: Codex.
- Status: in implementation; test-fixture reliability only.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestSchedulerHarnessExecutorRemoteStarterAuthenticatedHarnessdAssembly`.
- Source of truth: production `store.GenerateAPIKey` owns cost-12 credentials;
  the test owns its synthetic persisted hash and may rehash it at minimum cost.
- Flow retained: scheduler -> `HarnessExecutor` -> `RemoteRunStarter` -> bearer
  auth -> durable cron reservation -> reserved Runner start -> authenticated
  observation -> durable execution terminal/linkage assertions.
- Similar abstractions searched: `internal/store/apikey_test_helpers_test.go`
  and `internal/server/http_subagents_tenant_test.go` use `bcrypt.MinCost` for
  synthetic race-safe credentials.
- Duplication conclusion: keep a local helper because cross-package `_test.go`
  helpers are not importable and production must not expose a weak-key API.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: None; existing finite test config
  remains five-second request and ten-second job timeout.
- Endpoints/request/response/wire formats: None changed.
- CLI, model tools, slash commands, catalogs, and validation: None changed;
  searched cron remote starter and harnessd endpoint ownership.

## Persistence and Compatibility

- Schemas, migrations, durable production rows, caches: None changed.
- Compatibility and mixed versions: None; fixture-only hash replacement.
- Idempotency/run linkage: existing real durable reservation and typed
  execution `RunID` assertions remain intact.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/retries: no production change. Race instrumentation
  no longer consumes the transport deadline on deliberately expensive bcrypt.
- Authentication/authorization/privacy/secrets: real bearer extraction, scope,
  and tenant enforcement remain exercised with a random synthetic token. Only
  its test-local hash cost changes; production cost remains 12.
- Failure/recovery/idempotency: no new retry or fallback. Existing correlation
  key and one reserved run identity remain the acceptance boundary.

## Product and Integration Surfaces

- Server/runtime: exercised but unchanged.
- TUI/web/macOS/other clients: None changed; no client code or payload changes.
- Provider/model/tool routing: deterministic assembled provider unchanged.
- External automation: full regression becomes stable under aggregate race.
- UX/accessibility: None.

## Deployment and Operations

- Deployment/migration/feature flags: None.
- Logs/metrics/runbooks: engineering and observational logs will record the
  classification; public remote-cronsd behavior remains unchanged.
- Rollback: revert the test/docs commit.

## Regression Tests

- First deterministic red: assert the stored assembly key uses
  `bcrypt.MinCost`; current fixture reports cost 12.
- Acceptance: authenticated assembly completes with one linked run, exact
  scope, and terminal output under normal/race repetition.
- Adjacent coverage: complete `internal/cron` normal/race and repository
  regression gate.
- Security negative coverage: unchanged remote starter/server auth tests cover
  invalid/under-scoped keys; this test continues to use the real middleware.

## Documentation and Handoff

- Specs/public docs: None; behavior is unchanged.
- Implementation evidence: this plan/map, long-term-thinking, engineering and
  observational logs, plus plans/logs indexes.
- Warning check: every surface is explicit; all `None` entries have searched
  fixture-only rationale.
