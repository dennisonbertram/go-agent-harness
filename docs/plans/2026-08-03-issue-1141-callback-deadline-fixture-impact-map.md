# Cross-Surface Impact Map: Issue #1141

## Task

- Task / issue: #1141 callback deadline-release fixture stabilization.
- Plan link: `2026-08-03-issue-1141-callback-deadline-fixture-plan.md`.
- Owner: callback tools test suite.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: three tests in `delayed_callback_retry_red_test.go`.
- Source of truth: `CallbackManager` owns deadline cancellation; SQLite owns token-fenced durable state.
- Callers/consumers: harness callback tests only; production callers untouched.
- Similar abstractions/search: `rg` located all deadline-release tests, `blockingLeaseStore`, `ExtendLease`, and #1121's causal-gate fixture.
- Conclusion: alter test synchronization only.

## Config, API, CLI, and Tools

- None: no config, endpoint, CLI, tool, wire-format, or validation change.

## Persistence and Compatibility

- None in runtime: tests only read existing SQLite durable fields. Compatibility remains unchanged.

## Lifecycle, Security, and Reliability

- Reliability: gates prove ordered heartbeat/deadline/admission cancellation.
- Security/privacy: none; safe error text remains asserted, not changed.
- Recovery: tests retain token/lease clearing and exact reserved run ID checks.

## Product and Integration Surfaces

- Server/runtime: existing callback state machine is characterized.
- TUI/web/macOS: no code change; safe durable state continues to be the shared source for replay/visibility.
- Provider/model/tool routing and external systems: none.

## Deployment and Operations

- No deployment/migration/flag/rollback; fixture-only CI reliability repair.
- Hosted fast/race checks are the operational acceptance evidence.

## Regression Tests

- First red: hosted #1138 fast fixture timeout with 40 ms lease.
- Acceptance: focused three-test normal/race x20; assert admitted -> renewal -> deadline -> starter cancellation -> release ordering.
- Edge/negative: attempt cap remains failed with attempt 1, unchanged RunID, zero retry/token/lease, and safe reason.
- Full command: `./scripts/test-regression.sh`.

## Documentation and Handoff

- No public spec change; plan, map, logs, indexes, and PR test evidence record why the fixture synchronization is causal and runtime-neutral.
