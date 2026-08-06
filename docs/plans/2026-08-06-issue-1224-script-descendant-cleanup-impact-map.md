# Cross-Surface Impact Map: Issue #1224 script descendant fixture

## Task

- Task / issue: #1224 deterministic descendant cleanup regression.
- Plan link: `2026-08-06-issue-1224-script-descendant-cleanup-plan.md`.
- Owner: `internal/harness/tools/script` test fixture.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestScriptHandler_TimeoutKillsDescendantHoldingStdio` starts a
  shell fixture through the existing public script loader/handler.
- Source of truth: `makeScriptHandler` owns process-group cleanup; the test
  only observes its result and child PID barrier.
- Consumers: local/CI normal, race, coverage, and full regression gates.
- Search evidence: `rg -n "descendant|pid|ready|Timeout|timeout_seconds"
  internal/harness/tools/script/loader_test.go`; issue #1224 maps the runtime
  cancellation branch at `loader.go:256-263`.
- Conclusion: test fixture timing/control flow changes; runtime owner is not
  changed.

## Config, API, CLI, and Tools

- Config/API/CLI: None. Test fixture declares a longer local timeout only.
- Tool contract/errors: existing configured-timeout test retains external
  timeout behavior; cancellation result remains the existing handler error.

## Persistence and Compatibility

- Schemas/migrations/state: None; temp files hold only fixture PID/barrier.
- Compatibility/rollback: revert one test/docs commit; no durable data.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: test parent context is the deterministic cancel
  trigger after child publication; readiness returns early on handler result.
- Auth/privacy/secrets: None; fixture retains existing restricted handler env.
- Failure/recovery: cleanup defer kills the observed PID defensively; assertion
  proves production group cleanup completed.

## Product and Integration Surfaces

- Server/runtime/TUI/web/macOS: None; no production source changes.
- Provider/catalog/automation: None; shell fixture executes locally.
- UX/accessibility: None; no product client surface changes.

## Deployment and Operations

- Deployment/flags: None.
- Diagnostics: result-aware readiness includes the actual early handler result.
- Rollback/runbook: revert PR; ordinary regression command remains unchanged.

## Regression Tests

- First red: readiness helper must accept and report early handler completion.
- Tests: long fixture timeout, proven start then parent cancellation, prompt
  result, descendant death, retained independent configured timeout test.
- Commands: focused normal and `-race` under external `GOCACHE`, then
  `TMPDIR=/private/tmp GOCACHE=<outside-worktree> ./scripts/test-regression.sh`.

## Documentation and Handoff

- No public docs. Add plan/map/log entries and indexes; PR `Closes #1224`.

## Warning Check

- Every relevant surface is covered; unaffected product/runtime surfaces are
  explicitly test-only and unchanged.
