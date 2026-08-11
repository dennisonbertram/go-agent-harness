# Cross-Surface Impact Map: Issue #1231 API/SSE filesystem and Git acceptance

## Task

- Task / issue: #1231 default-registry filesystem/Git acceptance.
- Plan link: `2026-08-07-issue-1231-api-filesystem-git-plan.md`.
- Owner: acceptance test infrastructure.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `cmd/harnessd` production composition, `/v1/tools`, `/v1/runs`, `/v1/runs/{id}/events`, and `/v1/runs/{id}`.
- Owning packages/types/functions and source of truth: `internal/harness/tools_default.go` composes the default registry; `internal/acceptance/apisserunner.Runner` captures hash-bound API/SSE evidence; `cmd/harnessd/profile_crud_acceptance_test.go` owns real fake-provider daemon fixtures.
- Callers, consumers, events, and downstream data: provider tool calls enter Runner, produce SSE `tool.call.*` and terminal events, persist run/conversation records, and mutate only the fixture repo.
- Similar abstractions searched: Issue #1201 API/SSE profile acceptance and universal denied/no-mutation lane; generic `test/e2e` server helpers.
- Search commands/evidence: `rg` across `internal/acceptance`, `cmd/harnessd`, `test/e2e`, tool descriptions, and default registry composition.
- Duplication/ownership conclusion: extend the existing acceptance runner rather than add a second HTTP/SSE parser or a static tool manifest.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults / fallbacks: default registry is used exactly as harnessd composes it.
- Environment variables, config files, or saved settings touched: test-only isolated `HARNESS_WORKSPACE` and daemon fixture settings.
- Endpoints, request fields, response fields, or server wiring affected: exercised only; no contract change.
- CLI commands, tools, wire formats, or integrations affected: the 15 named tool calls and raw SSE evidence are asserted; no tool schema change.
- Error states / validation changes: acceptance fails closed on absent inventory entry, bad event order, tool failure, terminal mismatch, missing same-conversation linkage, or failed fixture probe.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None; test reads existing run/conversation state.
- Backward/forward compatibility and versioning: additive test support only.
- Partial rollout and mixed-version behavior: None.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: test owns daemon listener, temp repo, SSE response bodies, and Git fixture cleanup; no retries mask a failure.
- Authentication, authorization, permissions, trust, privacy, and secrets: fake provider and private temporary artifacts only; no credentials or user files.
- Failure modes, recovery, idempotency, and data repair: preserve exact raw evidence and stop without production repair when a tool fails.

## Product and Integration Surfaces

- Server/runtime: real `harnessd` default registry with fake scripted provider.
- TUI/web/macOS/other clients: None; separate children own those proofs.
- Provider/model/tool catalog and routing: live `/v1/tools` reconciliation is authoritative; scripted provider selects only named live tools.
- External systems and automation: local Git executable inside disposable fixture only.
- UX states, keyboard/focus/accessibility/motion: None.

## Deployment and Operations

- Deployment/migration order and feature flags: test-only; no rollout.
- Logs, metrics, traces, alerts, and support diagnostics: retained raw SSE and terminal/store artifacts bind evidence to inventory hash and fixture state.
- Rollback triggers and recovery steps: revert additive acceptance code if it destabilizes test infrastructure; never weaken an assertion.
- Runbooks and operator docs: update acceptance inventory status and indexes after verified code lands.

## Regression Tests

- Characterization and first expected red test: default real-daemon multi-turn sequence has explicit expected call/action order and artifact probes before driver support.
- New acceptance tests required: all 15 named tool rows with exact arguments/results, one conversation, Git and filesystem postconditions, and cleanup.
- Edge, negative, failure, lifecycle, and security tests: missing/misordered tool event and tool result error fail closed; temp fixture cleanup is asserted.
- Integration/e2e/real-path proof: real daemon HTTP plus raw SSE, status/conversation persistence, and external repository probes.
- Cross-surface regressions to guard: existing API runner, default registry, core/deferred tools, and daemon shutdown.
- Exact targeted and full commands: `go test ./internal/acceptance/apisserunner ./cmd/harnessd`; same with `-race`; `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1231-gocache ./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: this plan/map; no public feature claim.
- Implementation notes/logs/indexes after code: plans/logs indexes, engineering log, acceptance-inventory execution-status note.
- Training/onboarding/release notes: no release note; PR carries test-first and artifact evidence.
