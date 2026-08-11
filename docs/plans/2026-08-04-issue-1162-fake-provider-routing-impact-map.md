# Cross-Surface Impact Map: Issue #1162 fake-provider routing

## Task

- Task / issue: #1162 `HARNESS_PROVIDER=fake` must prevent real-provider resolution.
- Plan link: `2026-08-04-issue-1162-fake-provider-routing-plan.md`.
- Owner: harnessd/Runner routing.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `cmd/harnessd/main.go` reads `HARNESS_PROVIDER`, constructs the default provider, and assembles `RunnerConfig`; `internal/harness/runner.go` resolves each run provider.
- Owning packages/types/functions and source of truth: `runnerConfigOptions`, `buildRunnerConfig`, `harness.RunnerConfig`, `Runner.resolveProvider`, and `resolveProviderCandidates`.
- Callers, consumers, events, and downstream data: HTTP/TUI/GUI runs emit `provider.resolved`; registry clients may invoke external provider APIs.
- Similar abstractions searched: `DefaultProviderName`, `HARNESS_PROVIDER`, `resolveProvider`, `resolveProviderCandidates`, and `TestResolveDefaultProvider_FakePath`.
- Search commands/evidence: `rg -n -C 5 "DefaultProviderName|resolveProvider|HARNESS_PROVIDER|TestResolveDefaultProvider_FakePath" cmd/harnessd internal/harness` finds fake clearing at main.go:905-911 and catalog-first lookup at runner.go:2997-3030.
- Duplication/ownership conclusion: a single assembly flag is the smallest source of truth; it avoids treating fake as a catalog provider.

## Config, API, CLI, and Tools

- User-facing config added or changed: existing `HARNESS_PROVIDER=fake` becomes authoritative for execution only.
- Defaults / fallbacks: non-fake defaults and fallback rules remain unchanged; fake mode does not require `AllowFallback`.
- Environment variables, config files, or saved settings touched: only the existing environment variable is interpreted in assembly.
- Endpoints, request fields, response fields, or server wiring affected: existing run endpoint resolves fake; model/provider catalog endpoints stay unchanged.
- CLI commands, tools, wire formats, or integrations affected: deterministic TUI/GUI/API runs inherit the mode; no wire-format change.
- Error states / validation changes: absent fixture models no longer produce catalog lookup errors under explicit fake.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None; no persisted state changes.
- Backward/forward compatibility and versioning: ordinary catalog routing remains unchanged when fake is not explicitly configured.
- Partial rollout and mixed-version behavior: process-local startup setting; a mixed fleet is observable by provider-resolved events.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: no new goroutines or ownership changes; config reload retains the mode through shared assembly.
- Authentication, authorization, permissions, trust, privacy, and secrets: prevents fake acceptance prompts from leaving the local process or consuming configured-provider credentials.
- Failure modes, recovery, idempotency, and data repair: fake turn-file errors remain startup errors; no repair needed.

## Product and Integration Surfaces

- Server/runtime: harnessd passes the authoritative mode into each Runner config.
- TUI/web/macOS/other clients: no client changes; all use the same run endpoint and gain deterministic routing.
- Provider/model/tool catalog and routing: catalog remains loaded for metadata/tools/pricing while routing selects fake first.
- External systems and automation: real provider client factory must not run under fake.
- UX states, keyboard/focus/accessibility/motion: None; no UI change.

## Deployment and Operations

- Deployment/migration order and feature flags: deploy as normal; `HARNESS_PROVIDER=fake` is the existing operator switch.
- Logs, metrics, traces, alerts, and support diagnostics: existing provider-resolved event should report `fake`; tests prove it.
- Rollback triggers and recovery steps: roll back if non-fake catalog selection changes; no data rollback.
- Runbooks and operator docs: clarify fake does not depend on permissive fallback when catalogs/credentials are present.

## Regression Tests

- Characterization and first expected red test: daemon assembly with catalog/OpenAI client, known model and absent model, both `allow_fallback=false`; pre-fix resolves real or fails.
- New acceptance tests required: completion, `provider_name=fake`, catalog endpoint visibility, zero real factory/client calls.
- Edge, negative, failure, lifecycle, and security tests: absent model is the strict no-fallback security case; existing non-fake catalog coverage is retained.
- Integration/e2e/real-path proof: real harnessd HTTP server fixture exercises startup, run API, and catalog endpoint.
- Cross-surface regressions to guard: shared config reload assembly and normal catalog routing.
- Exact targeted and full commands: `go test ./cmd/harnessd -run TestFakeProviderOverride -count=1`; same with `-race`; `go test ./cmd/harnessd -count=1`; same with `-race`; `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan and impact map are complete; no public feature claims change.
- Implementation notes/logs/indexes after code: append the four durable logs; update plans/logs/runbook indexes.
- Training/onboarding/release notes: deterministic acceptance runbook carries the safety boundary.

## Warning Check

- No blank sections. Surfaces without changes are explicitly marked None with rationale.
