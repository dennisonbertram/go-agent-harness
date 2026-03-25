# Plan: Issue #428 Timed-Out Subrun Cancellation

## Context

- Problem: `RunPrompt(...)` and `RunForkedSkill(...)` wait on a spawned child run through `waitForTerminalResult(...)`, but when the parent context ends they return `ctx.Err()` without actively cancelling the child run.
- User impact: `/v1/agents` timeouts and parent cancellation paths can leave child runs consuming tokens and mutating state after the caller has already been told the work stopped.
- Constraints: Keep the fix narrow, preserve terminal-result behavior for already-finished runs, and follow strict TDD with regression coverage before implementation.

## Scope

- In scope:
  - runner wait-path cancellation behavior for spawned subruns
  - direct regression coverage for `RunPrompt(...)` and `RunForkedSkill(...)`
  - `/v1/agents` timeout coverage proving the spawned run is cancelled
- Out of scope:
  - broader runner lifecycle refactors
  - unrelated timeout, allowlist, or bootstrap bugs

## Test Plan (TDD)

- New failing tests to add first:
  - `RunPrompt(...)` cancels the spawned run when the parent context is cancelled
  - `RunForkedSkill(...)` cancels the spawned run when the parent context is cancelled
  - `/v1/agents` timeout cancels the spawned run instead of only returning `408`
- Existing tests to update:
  - extend orchestration/server timeout coverage around existing context-cancellation tests
- Regression tests required:
  - preserve terminal-result behavior when the child already finished
  - verify cancelled child runs actually reach `RunStatusCancelled`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not touch provider/model flow plumbing, so no separate impact map is required.

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

- Risk: Cancelling on parent timeout could clobber a child run that already reached a terminal state but has not been observed yet.
- Mitigation: Check the current run state before issuing `CancelRun(runID)` and keep returning the terminal fork result when the child is already done.
