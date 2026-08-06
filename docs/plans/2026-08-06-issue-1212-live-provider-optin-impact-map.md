# Cross-Surface Impact Map: Issue #1212 live provider fetch opt-in

## Task

- Task / issue: #1212 — test: make live provider fetch smoke explicitly opt-in.
- Plan link: `2026-08-06-issue-1212-live-provider-optin-plan.md`.
- Owner: `internal/modelstore/live_manual_test.go`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestLiveFetchAgainstRealProviders`; Go includes this `_test.go` file in ordinary package runs.
- Source of truth: the test reads `OPENAI_API_KEY` / `OPENROUTER_API_KEY`, then passes each credential to `NewFetcher().Fetch` at the real provider base URL.
- Callers/consumers: `scripts/test-regression.sh` invokes `go test ./...` in normal, race, and coverage phases; no production caller consumes this test-local decision.
- Similar abstractions searched: `rg -n 'TestLiveFetchAgainstRealProviders|OPENAI_API_KEY|OPENROUTER_API_KEY|live.*provider|real provider' internal/modelstore scripts docs README.md .github`; local fetch behavior is covered by `httptest` in `fetch_test.go` and environment isolation by `t.Setenv` in package tests.
- Conclusion: retain one test-local gate rather than add production configuration or a second smoke runner.

## Config, API, CLI, and Tools

- User-facing config: test-only `HARNESS_TEST_LIVE_PROVIDERS=1`; it is not read by the application.
- Defaults/fallbacks: absent or non-`1` flag disables every live subtest even when credentials exist; absent credential still skips its individual provider when opted in.
- Environment: provider credentials remain environment-only and are never persisted or logged.
- Endpoints/wire/CLI/tools: None — existing real URLs are used only by the intentionally invoked test command.
- Errors/validation: no product error contract changes.

## Persistence and Compatibility

- Schema/migration/cache/generated data: None — test-only logic.
- Compatibility: ordinary test runs become offline; documented explicit flag plus existing credential preserves intentional real smoke coverage.
- Mixed rollout: None.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/retries/cleanup: none changed; the existing fetcher timeout remains untouched.
- Security/privacy: flag is an explicit acknowledgement before credentialed network use; tests never print credential values.
- Failure/recovery: normal regression cannot fail on provider availability; intentionally enabled smoke reports the existing fetch failure.

## Product and Integration Surfaces

- Server/runtime: None — production `modelstore` code is unchanged.
- TUI/web/macOS/clients: None — searched provider references are test/runbook-only for this behavior.
- Provider/model/tool routing: no catalog/routing change; tests retain OpenAI and OpenRouter smoke targets.
- External systems/UX: only the manual test's outbound provider calls are gated; no client UX change.

## Deployment and Operations

- Deployment/migration: no runtime release action or feature flag.
- Observability/support: `go test` skip text and the runbook command make the boundary diagnosable without disclosing credentials.
- Rollback: revert the isolated commit; no data repair is needed.
- Runbook: add the exact opt-in command to `docs/runbooks/testing.md` after it exists.

## Regression Tests

- First expected red: a table-driven `TestLiveProviderFetchEnabled` fails to compile until the narrow gate exists.
- Acceptance/negative cases: credential only disabled; flag only disabled; flag plus credential enabled; non-`1` flag disabled.
- Lifecycle/security: no-network test invokes only the pure gate, proving no provider endpoint is needed.
- Real path: preserved but deliberately not run in CI/local regression: `HARNESS_TEST_LIVE_PROVIDERS=1 OPENAI_API_KEY=... go test ./internal/modelstore -run '^TestLiveFetchAgainstRealProviders$/openai$' -v`.
- Commands: focused normal/race tests, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Pre-code: this plan and map; indexed in `docs/plans/INDEX.md`.
- Post-code: testing runbook, engineering/observational/system/long-term logs, and relevant indexes.
- Training/release notes: None — no public product contract changes.
