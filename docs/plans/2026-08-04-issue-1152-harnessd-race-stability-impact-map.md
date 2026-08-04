# Cross-Surface Impact Map: Issue #1152

## Task

- Task / issue: #1152 harnessd race-stable test fixtures.
- Plan link: `2026-08-04-issue-1152-harnessd-race-stability-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: the five named tests in `cmd/harnessd/main_test.go` call
  `runWithSignals`/`runWithSignalsWithDeps`.
- Owning packages/types/functions and source of truth: test-owned environment
  maps control `callbacksEnabled`; production reads it at `main.go:506` before
  persistence bootstrap and callback-manager startup.
- Callers, consumers, events, and downstream data: provider factory invocation,
  injected cleaner start/cancellation, and server listener acquisition are
  causal test readiness boundaries; shutdown still travels through real daemon
  cleanup.
- Similar abstractions searched: `runMatrixTestWithListener`,
  `heldConversationCleaner`, and existing provider-start channels.
- Search commands/evidence: `rg` over `cmd/harnessd` for the five tests,
  callbacks, listener, health, and readiness.
- Duplication/ownership conclusion: reuse existing test signals; do not add a
  production readiness API.

## Config, API, CLI, and Tools

- User-facing config added or changed: none; tests explicitly set existing
  `HARNESS_ENABLE_CALLBACKS=false` only when callbacks are not under test.
- Defaults / fallbacks: production default remains true from #1150.
- Environment variables, config files, or saved settings touched: test maps
  only.
- Endpoints, request fields, response fields, server wiring affected: none.
- CLI commands, tools, wire formats, or integrations affected: none.
- Error states / validation changes: none.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: none.
- Backward/forward compatibility and versioning: none; default callback
  persistence remains covered by callback-specific tests.
- Partial rollout and mixed-version behavior: none.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: replace
  sleep-based test observations with provider/cleaner/listener causal signals;
  retain real cancellation and shutdown ordering.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  none; test keys remain synthetic.
- Failure modes, recovery, idempotency, and data repair: tests retain bounded
  failure diagnostics and do not change runtime recovery.

## Product and Integration Surfaces

- Server/runtime: test fixture only; real `runWithSignals` lifecycle remains
  the subject.
- TUI/web/macOS/other clients: none; no protocol or rendering change.
- Provider/model/tool catalog and routing: model lookup fixtures retain their
  existing catalog assertions.
- External systems and automation: hosted race reliability is the affected CI
  integration.
- UX states, keyboard/focus/accessibility/motion: none.

## Deployment and Operations

- Deployment/migration order and feature flags: none.
- Logs, metrics, traces, alerts, and support diagnostics: durable engineering,
  observational, and system log entries record the fixture boundary.
- Rollback triggers and recovery steps: revert only test-fixture edits if they
  obscure a subsequently proven production ordering defect.
- Runbooks and operator docs: no public/operator change.

## Regression Tests

- Characterization and first expected red test: compile/run red after adding
  explicit fixture readiness requirements, before their helper/wiring exists.
- New acceptance tests required: five target paths start and terminate through
  semantic boundaries with callbacks disabled.
- Edge, negative, failure, lifecycle, and security tests: retained cleaner
  startup-failure and cancellation acknowledgement behavior.
- Integration/e2e/real-path proof: real harnessd startup, listener, provider,
  and signal shutdown under `-race` repetition.
- Cross-surface regressions to guard: callback-enabled matrix and shutdown
  tests remain untouched.
- Exact targeted and full commands: targeted normal/race repetition, `go test
  ./cmd/harnessd -race`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: plan and map above.
- Implementation notes/logs/indexes after code: plans/logs indexes and three
  durable logs.
- Training/onboarding/release notes: none; internal fixture reliability only.

## Warning Check

- Every surface is mapped. Unaffected product/runtime interfaces are explicit
  because the scope is test-only.
