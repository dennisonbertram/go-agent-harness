# Cross-Surface Impact Map: Issue #1201 API/SSE evidence-runner foundation

## Task

- Task / issue: #1201 real-daemon registry-derived API/SSE evidence runner; foundation child of #1087.
- Plan link: `2026-08-05-issue-1087-api-sse-intent-runner-plan.md`.
- Owner: acceptance infrastructure.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: planned API runner -> live `/v1/tools` -> `inventory.Compile` -> complete API case set -> `/v1/runs` and `/continue` -> per-run SSE -> run status/conversation/artifact probes -> evidence report.
- Owning packages/types/functions and source of truth: `internal/acceptance/inventory` owns hashes/case/evidence validation; `internal/server` owns HTTP/SSE; `cmd/harnessd` owns real composition.
- Callers, consumers, events, and downstream data: operator/CI deterministic lane; later #1090 aggregates artifacts. Raw event IDs bind execution to the observed terminal state.
- Similar abstractions searched: `test/e2e`, `cmd/harnessd/profile_crud_acceptance_test.go`, `scripts/tool-sweep.py`, `scripts/run-bench-smoke.sh`, and `cmd/acceptance-inventory`.
- Search commands/evidence: `rg 'acceptance-inventory|/v1/tools|run-bench-smoke|SSE' cmd internal test scripts docs`; existing sweep is prompt/text-based and cannot satisfy #1087 postconditions.
- Duplication/ownership conclusion: reuse inventory schema and production HTTP routes; do not add a parallel catalog or direct manager calls.

## Config, API, CLI, and Tools

- User-facing config added or changed: additive runner endpoint/base URL and artifact root only; fake deterministic fixture remains default.
- Defaults / fallbacks: absent compatible `/v1/tools` resolver evidence or case coverage is a failure, never an empty/default pass.
- Environment variables, config files, or saved settings touched: isolated test workspace/store only; no persisted user configuration.
- Endpoints, request fields, response fields, or server wiring affected: exercise existing `/v1/tools`, `/v1/runs`, `/continue`, events, status, and state endpoints; no contract changes.
- CLI commands, tools, wire formats, or integrations affected: additive internal/operator runner output based on v2 evidence.
- Error states / validation changes: validation rejects missing case, wrong inventory hash, missing raw evidence/probe, or cleanup failure.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: artifact files only; product stores are isolated fixture roots.
- Backward/forward compatibility and versioning: evidence uses existing `acceptance-evidence/v2` and inventory hash.
- Partial rollout and mixed-version behavior: unsupported/missing resolver evidence fails rather than claiming an incomplete catalog.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: one runner owns a unique daemon/artifact root; bounded streams and exact terminal/cancel handling; no retry of tool behavior.
- Authentication, authorization, permissions, trust, privacy, and secrets: default fixture uses fake provider and disposable paths; artifacts redact request secrets; permission rejections prove no mutation.
- Failure modes, recovery, idempotency, and data repair: preserve raw artifacts and classify failures; do not repair product behavior in this issue.

## Product and Integration Surfaces

- Server/runtime: real daemon/HTTP/SSE boundary exercised.
- TUI/web/macOS/other clients: None; #1088/#1089 own those surfaces.
- Provider/model/tool catalog and routing: registry-derived inventory, fake provider fixture, explicit configured-unavailable dynamic toolsets as N/A.
- External systems and automation: dynamic external tools require explicit safe fixtures/conditions; no live target by default.
- UX states, keyboard/focus/accessibility/motion: None; operator report must distinguish intent probe from a tool event.

## Deployment and Operations

- Deployment/migration order and feature flags: CI-safe deterministic lane first; opt-in live subset remains documented only.
- Logs, metrics, traces, alerts, and support diagnostics: raw SSE, terminal/status/probe, cleanup, and redacted request artifacts.
- Rollback triggers and recovery steps: revert runner/test artifacts only; preserve failure packs; no product migration.
- Runbooks and operator docs: acceptance-inventory runbook gains the execution boundary and invocation after implementation.

## Regression Tests

- Characterization and first expected red test: executor absent; planned test must fail to compile before implementation.
- New acceptance tests required: complete registry-derived plan, ordered continuation, independent state probe, raw SSE/run ID, missing-case preflight, and rejected/no-mutation behavior.
- Edge, negative, failure, lifecycle, and security tests: closed stream/terminal failure, unsupported live inventory, cleanup failure, dynamic N/A, and redaction.
- Integration/e2e/real-path proof: real `harnessd` fixture composed through `runWithSignalsWithDeps`; no direct manager invocation.
- Cross-surface regressions to guard: server continuation/SSE and inventory validator tests.
- Exact targeted and full commands: `go test ./internal/acceptance/... -race -count=1`, `go test ./cmd/harnessd -run Issue1087 -race -count=1`, and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan/impact map only; no public feature claim.
- Implementation notes/logs/indexes after code: runbook, plans/log indexes, and three durable logs.
- Training/onboarding/release notes: no release notes; #1090 owns convergence/orchestration.

## Warning Check

- No headings are blank. Explicitly unaffected client surfaces are delegated to #1088/#1089.
