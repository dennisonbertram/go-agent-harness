# Cross-Surface Impact Map: Issue #830 Anthropic Retry Fixture Budget

## Task

- Task / issue: #830
- Plan link: `2026-08-04-issue-830-anthropic-retry-fixture-plan.md`
- Owner: `internal/provider/anthropic` tests
- Status: implemented

## Current Ownership, Callers, and Data Flow

- Entry points: `TestClientRetriesOn429` and `TestClientRetriesOn503`.
- Source of truth: `testRetryConfig` in `client_test.go`; both clients assign
  it through `newTestClient` before calling `Complete` against `httptest`.
- Search evidence: `rg -n "testRetryConfig|TestClientRetriesOn(429|503)|MaxTotal|retry budget" internal/provider/anthropic`.
- Conclusion: one package-local fixture owns the failure budget; production
  `provider.DefaultRetryConfig` is not modified.

## Config, API, CLI, and Tools

- None: no user-facing config, environment, endpoint, request/response,
  CLI, tool, or validation contract changes. The test-only fixture keeps its
  existing attempts, delay bounds, and disabled jitter.

## Persistence and Compatibility

- None: no schema, stored data, migration, cache, versioning, or mixed-version
  impact. The test's HTTP status contract is unchanged.

## Lifecycle, Security, and Reliability

- Reliability: the test fixture receives a larger bounded wall-clock budget
  for scheduling/coverage contention; retry production behavior is unchanged.
- Security: None; no credential, permission, trust, privacy, or secret path.
- Recovery/idempotency: the existing 429/503 retry and two-attempt assertions
  remain the characterization boundary.

## Product and Integration Surfaces

- Server/runtime, clients, catalog/routing, external systems, and UX: None;
  only an in-package `httptest` fixture changes.

## Deployment and Operations

- None: no deploy, migration, feature flag, runtime log/metric, alert, or
  operator runbook change. Rollback is reverting the fixture-only line.

## Regression Tests

- Characterization: unchanged focused 429/503 normal and race stress with the
  historical 100ms fixture; issue #830 records the full coverage red.
- Acceptance: both 429 and 503 complete successfully with exactly two requests.
- Commands: `go test ./internal/provider/anthropic -run 'TestClientRetriesOn(429|503)$' -count=100`; same with `-race`; full package normal/race;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- No public/spec addition. Record the test-only cause and verification in the
  engineering log and index the plan/log artifacts.
