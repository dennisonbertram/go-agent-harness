# Plan: Issue #332 runner orchestration coverage

## Context

- Problem: `SubmitInput`, `RunPrompt`, and `RunForkedSkill` still rely on incidental coverage for several public orchestration edges, making future runner extraction riskier than it should be.
- User impact: regressions in input submission, prompt-run waiting behavior, or forked-skill result mapping could ship without a direct failing test.
- Constraints: strict TDD, stay scoped to issue `#332`, avoid production behavior changes unless a real uncovered bug is exposed, and keep the work limited to runner orchestration semantics.

## Scope

- In scope:
  - direct `SubmitInput` error-mapping coverage
  - `RunPrompt` terminal-history and stream-closure return behavior
  - `RunForkedSkill` terminal success/failure result mapping
- Out of scope:
  - broader runner refactors
  - provider/model plumbing changes
  - unrelated coverage issues in other packages

## Test Plan (TDD)

- New failing tests to add first:
  - `SubmitInput` returns `ErrInvalidRunInput` for broker validation failures.
  - `SubmitInput` returns `ErrNoPendingInput` when the broker reports no pending question.
  - `RunPrompt` returns output immediately when terminal history already exists.
  - `RunPrompt` returns output when the subscription stream closes after non-terminal history.
  - `RunForkedSkill` returns terminal output for completed sub-runs and terminal error text for failed sub-runs.
- Existing tests to update:
  - extend runner orchestration coverage files instead of adding broad new integration tests.
- Regression tests required:
  - exact error/result assertions for each public orchestration helper touched by the issue.

## Cross-Surface Impact Map

- None. This task does not touch provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.

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

- Risk: existing async runner behavior could make stream-closure tests flaky.
- Mitigation: drive the tests with deterministic providers and explicit subscription closure instead of timing-sensitive sleeps where possible.
