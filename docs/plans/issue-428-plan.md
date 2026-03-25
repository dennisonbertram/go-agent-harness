# Plan: Issue #428 timed-out subrun cancellation

## Context

- Problem: `RunPrompt(...)` and `RunForkedSkill(...)` wait on child runs through `waitForTerminalResult(...)`, but if the parent context times out or is cancelled the helper returns immediately without cancelling the spawned child run.
- User impact: timed-out agent and skill calls can keep consuming provider/tool resources after the caller already received a timeout or cancellation.
- Constraints: keep the change scoped to subrun lifecycle plumbing, preserve normal terminal result behavior, and follow strict TDD.

## Scope

- In scope:
  - characterize the current cancellation leak with failing tests
  - make parent cancellation actively cancel the spawned child run
  - keep regression coverage for both `RunPrompt(...)` and `RunForkedSkill(...)`
  - verify the relevant harness/server surfaces
- Out of scope:
  - broader runner lifecycle refactors
  - changing unrelated timeout handling semantics
  - fixing unrelated pre-existing regression failures

## Test Plan (TDD)

- New failing tests to add first:
  - `waitForTerminalResult(...)` cancels the spawned run when `ctx.Done()` fires
  - `RunPrompt(...)` parent cancellation transitions the child run to `cancelled`
  - `RunForkedSkill(...)` parent cancellation transitions the child run to `cancelled`
- Existing tests to update:
  - extend orchestration coverage in `internal/harness/runner_orchestration_test.go`
  - add or extend `/v1/agents` timeout coverage in `internal/server/http_agents_test.go` if the current tests do not pin the cancellation side effect
- Regression tests required:
  - `go test ./internal/harness`
  - `go test ./internal/server`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- None for this bugfix; the change is limited to runner subrun cancellation behavior plus server timeout verification.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: cancelling too aggressively could race with a child run that already reached a terminal state.
- Mitigation: keep `CancelRun` idempotent and preserve the existing terminal-history fast path before invoking cancellation on parent timeout.
