# Issue #1115 Workflow Subscriber Terminal-Close Impact Map

## Task

- Task / issue: deterministic full-buffer subscriber terminal-close regression, #1115
- Plan link: `2026-08-03-issue-1115-workflow-subscriber-plan.md`
- Owner: workflow engine test boundary
- Status: implemented and locally verified; PR review/promotion pending

## Current Ownership, Callers, and Data Flow

- Entry points: `Engine.Start`, `Engine.Subscribe`, workflow script `Context.Log`, and terminal `Engine.emit`.
- Owning packages/types/functions and source of truth: `internal/workflow.Engine`, `subEntry`, `Subscribe`, and `emit` in `internal/workflow/engine.go`.
- Callers, consumers, events, and downstream data: `SourceManager.Subscribe`; the script-workflow SSE handler replays history then reads the live channel; tests exercise both history/live and cancellation concurrency.
- Similar abstractions searched: Runner subscriptions plus workflow `concurrency_bugs_internal_test.go`, `subscribe_lock_scope_test.go`, and `perf_regression_internal_test.go`.
- Search commands/evidence: `rg -n "Subscribe|subscribers|terminal" internal/workflow internal/server`; production terminal close/delete is under `Engine.mu`, and cancel consults the same map under the same mutex.
- Duplication/ownership conclusion: the production invariant has one owner in `Engine.emit`; #1115 changes only the fixture that schedules the existing path.

## Config, API, CLI, and Tools

- User-facing config added or changed: None; test-only synchronization.
- Defaults / fallbacks: None; channel capacity and drop behavior remain unchanged.
- Environment variables, config files, or saved settings touched: None; repository search found no configuration dependency.
- Endpoints, request fields, response fields, or server wiring affected: None; the existing SSE history/live handler is blast-radius context only.
- CLI commands, tools, wire formats, or integrations affected: None.
- Error states / validation changes: None.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: None.
- Backward/forward compatibility and versioning: unchanged workflow event history and live-channel semantics.
- Partial rollout and mixed-version behavior: None; tests do not ship runtime behavior.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: the fixture gates execution until registration, releases once, fills the buffer without draining, then observes ordered buffered values followed by channel closure; cancel remains safe after terminal cleanup.
- Authentication, authorization, permissions, trust, privacy, and secrets: None; no runtime request or data boundary changes.
- Failure modes, recovery, idempotency, and data repair: bounded test failure replaces a scheduling race; no production recovery or data repair changes.

## Product and Integration Surfaces

- Server/runtime: runtime code unchanged; existing terminal close remains the behavior under test.
- TUI/web/macOS/other clients: None; no product client consumes this test seam.
- Provider/model/tool catalog and routing: None; search found no involvement.
- External systems and automation: GitHub fast/race CI gains deterministic evidence.
- UX states, keyboard/focus/accessibility/motion: None; no UI change.

## Deployment and Operations

- Deployment/migration order and feature flags: merge as a standalone test-only baseline repair before #1107; no migration or flag.
- Logs, metrics, traces, alerts, and support diagnostics: durable engineering/observational/system notes only; runtime telemetry unchanged.
- Rollback triggers and recovery steps: revert the test/docs commit if exact-head CI regresses.
- Runbooks and operator docs: testing evidence follows `docs/runbooks/testing.md`; no operator runbook change.

## Regression Tests

- Characterization and first expected red test: with the gate intentionally withheld, the bounded run-terminal assertion fails, proving execution cannot race ahead of registration.
- New acceptance tests required: deterministic pre-terminal subscription, >64-event full buffer, ordered drain, terminal close, and safe cancel.
- Edge, negative, failure, lifecycle, and security tests: cancel-after-terminal and repeated race execution; no security path.
- Integration/e2e/real-path proof: complete `internal/workflow` normal/race tests and repository full regression; no external UI path applies to a test-only correction.
- Cross-surface regressions to guard: concurrent history/live capture and SSE terminal-history short circuit remain covered by existing suites.
- Exact targeted and full commands: focused normal/race `-count=100`, `go test ./internal/workflow`, `go test -race ./internal/workflow`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: issue body, this plan, and this impact map; no public docs.
- Implementation notes/logs/indexes after code: plan status/checklist, engineering/observational/system logs, `docs/plans/INDEX.md`, and `docs/plans/active-plan.md`.
- Training/onboarding/release notes: None; record the test-fixture scheduling lesson in durable logs.

## Warning Check

- Every surface is mapped. `None` entries are supported by the repository searches above and the test-only scope.
