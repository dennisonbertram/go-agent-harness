# Issue #1093 — Conversation-cleaner shutdown impact map

## Task

- Task / issue: #1093, deterministic conversation-cleaner shutdown under race load.
- Plan link: `2026-08-02-issue-1093-cleaner-shutdown-plan.md`.
- Owner: harnessd bootstrap.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `runWithSignalsWithDeps` builds persistence through `buildPersistenceBootstrap`; normal signal and every early return unwind its cleanup.
- Owning types/functions: `harness.ConversationCleaner.Start`, `conversationCleanerStarter`, and `persistenceBootstrap` currently own creation, cancellation, and store closure sequencing.
- Callers/consumers: production `harnessd`; direct cleaner SQLite tests; injected harnessd cleaner fakes.
- Similar abstractions searched: cleaner, `convCleanerCancel`, `runWithSignals`, persistence bootstrap, and `Start` via `rg -n` across Go sources.
- Duplication/ownership conclusion: one bootstrap-owned lifecycle handle must cancel and acknowledge the sole started cleaner before conversation-store closure; no caller should guess cleaner exit timing.

## Config, API, CLI, and Tools

- User-facing config added or changed: none; `HARNESS_CONVERSATION_RETENTION_DAYS` keeps its existing meaning.
- Defaults / fallbacks: unchanged; nonpositive retention remains disabled.
- Environment variables, config files, or saved settings touched: existing conversation DB/retention wiring only.
- Endpoints, request fields, response fields, or server wiring affected: no API contract; HTTP server shutdown now follows cleaner acknowledgement.
- CLI commands, tools, wire formats, or integrations affected: none.
- Error states / validation changes: startup failure now waits for owned cleanup before returning its original error.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: no schema/migration; conversation SQLite store is no longer closed while its cleaner can still access it.
- Backward/forward compatibility and versioning: internal method signature only; no persisted/wire compatibility impact.
- Partial rollout and mixed-version behavior: none; each daemon process owns its own cleaner lifecycle.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: cancellation plus a done acknowledgement are owned exactly once; all paths await before store close.
- Authentication, authorization, permissions, trust, privacy, and secrets: none; retention policy and pinned-conversation handling stay unchanged.
- Failure modes, recovery, idempotency, and data repair: startup failures retain their original error after cleanup; repeated cleanup is safe; no data repair is needed.

## Product and Integration Surfaces

- Server/runtime: harnessd normal signal, server-start failure, and deferred cleanup.
- TUI/web/macOS/other clients: none; no client behavior changes.
- Provider/model/tool catalog and routing: none.
- External systems and automation: none.
- UX states, keyboard/focus/accessibility/motion: none.

## Deployment and Operations

- Deployment/migration order and feature flags: none.
- Logs, metrics, traces, alerts, and support diagnostics: existing cleaner sweep logs retained; deterministic test failures identify lifecycle ordering.
- Rollback triggers and recovery steps: revert the isolated lifecycle change if it alters shutdown behavior; no database rollback.
- Runbooks and operator docs: testing and engineering logs record the acknowledgement invariant.

## Regression Tests

- Characterization and first expected red test: controlled cleaner keeps `done` blocked after cancellation; daemon must not return before acknowledgement.
- New acceptance tests required: normal signal and startup-failure ownership, plus direct cleaner completion semantics.
- Edge, negative, failure, lifecycle, and security tests: zero retention returns an already-complete lifecycle; cancellation during startup sweep exits cleanly.
- Integration/e2e/real-path proof: `runWithSignalsWithDeps` uses a real bound-port startup failure and normal signal.
- Cross-surface regressions to guard: conversation store is closed only after cleaner acknowledgement.
- Exact targeted and full commands: issue-specified race repetitions, `go test ./cmd/harnessd`, `go test ./cmd/harnessd -race`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan and impact map complete; no public docs because behavior is internal.
- Implementation notes/logs/indexes after code: update plan, long-term-thinking, engineering, system/observational logs if architecture changes, and plans index.
- Training/onboarding/release notes: no release-note change.

## Warning Check

- Every required surface is either mapped above or explicitly unaffected with rationale.
