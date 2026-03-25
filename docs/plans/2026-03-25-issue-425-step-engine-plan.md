# Plan: Issue #425 step engine extraction

## Context

- Problem: `internal/harness/runner.go` still owns the full step loop, mixing provider calls, hook application, tool dispatch, accounting, memory observation, compaction, and steering at the same level.
- User impact: the most change-prone runner logic is hard to reason about locally, so even small loop changes risk regressions across unrelated behaviors.
- Constraints: preserve existing run semantics and event order, keep the extraction internal and narrow, follow strict TDD, and avoid changing public contracts.

## Scope

- In scope:
  - characterize current step-boundary behavior in tests
  - extract the step loop into a focused internal step-engine helper
  - preserve provider orchestration, hooks, tool execution, accounting, compaction, memory observe triggers, and steering drain timing
- Out of scope:
  - redesigning the run state model
  - changing HTTP/event contracts
  - changing tool policy semantics
  - altering preflight or event-journal extraction work

## Test Plan (TDD)

- New failing tests to add first:
  - direct characterization for step-boundary ordering between `run.step.started`, steering drain, and `llm.turn.requested`
  - direct characterization for step-loop preservation when a run spans multiple steps and completes normally
  - direct characterization for step-loop preservation when tool execution and accounting occur in the same turn
- Existing tests to update:
  - `internal/harness/runner_test.go`
  - `internal/harness/runner_steer_test.go`
- Regression tests required:
  - `go test ./internal/harness -run 'TestRunner(EmitsStepStartedAndCompletedEvents|.*SteerRun.*|.*UsageDelta.*)' -count=1`
  - `go test ./internal/harness -count=1`

## Cross-Surface Impact Map

- Not required. This task does not change provider/model flow contracts, gateway routing, or server/TUI provider plumbing.

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

- Risk: the extraction could subtly shift step-boundary event ordering or the timing of steering and compaction decisions.
- Mitigation: pin the current loop contract with direct tests first, then move the logic into a narrowly scoped internal engine and rerun the targeted harness tests.
