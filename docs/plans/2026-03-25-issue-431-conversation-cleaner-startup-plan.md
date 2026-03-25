# Plan: Issue #431 conversation cleaner startup cleanup

## Context

- Problem: `cmd/harnessd/main.go` starts the conversation-retention cleaner but only cancels it in the normal signal-driven shutdown path, leaving later startup failures with a leaked cancel path and a `go vet` warning.
- User impact: startup failures can leave background cleanup work alive longer than intended, and the static-check signal for lifecycle correctness is currently red.
- Constraints:
  - Strict TDD with a failing regression test first.
  - Keep startup behavior unchanged except for cleanup correctness.
  - Stay scoped to issue `#431` and avoid broader bootstrap refactors unless a tiny seam is required for testability.

## Scope

- In scope:
  - Reproduce the current vet warning and targeted `cmd/harnessd` baseline.
  - Add a regression test for a startup-failure path after cleaner initialization.
  - Implement the minimal fix that guarantees cleaner cancellation on all post-init exits.
  - Update logs and the GitHub issue/PR with verification details.
- Out of scope:
  - General `harnessd` modularization work from issue `#426`.
  - Unrelated test cleanup outside the targeted verification path.

## Test Plan (TDD)

- New failing tests to add first:
  - A `cmd/harnessd` regression test that initializes conversation persistence plus the cleaner, forces a later startup failure, and asserts the cleanup path runs.
- Existing tests to update:
  - `cmd/harnessd/main_test.go` shutdown-related coverage if the new failure-path test can share helpers with the existing cleaner shutdown test.
- Regression tests required:
  - `go test ./cmd/harnessd`
  - `go vet ./internal/... ./cmd/...`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not touch provider/model flow behavior, so no impact map is required.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: The startup-failure seam may be awkward to reach directly through `runWithSignals`.
- Mitigation: Allow a tiny lifecycle-focused extraction only if needed to make the regression deterministic and easy to assert.
