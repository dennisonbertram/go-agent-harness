# Plan: Issue #431 conversation cleaner startup cleanup

## Context

- Problem: `cmd/harnessd/main.go` starts the conversation-retention cleaner with `context.WithCancel(...)`, but startup errors can return before the cancel function is called.
- User impact: startup failures can leak the cleaner goroutine/context, and `go vet` already flags the path as a real lifecycle bug.
- Constraints: keep the change small, use strict TDD, and preserve existing `harnessd` startup and shutdown behavior aside from cleanup correctness.

## Scope

- In scope:
  - add regression coverage for startup failure after cleaner initialization
  - ensure the cleaner cancel function is always used after initialization
  - verify `cmd/harnessd` tests and vet checks
- Out of scope:
  - broader `harnessd` bootstrap decomposition
  - unrelated shutdown-order refactors

## Test Plan (TDD)

- New failing tests to add first:
  - startup failure after conversation cleaner initialization still invokes cleaner cancellation
- Existing tests to update:
  - `cmd/harnessd/main_test.go`
- Regression tests required:
  - `go test ./cmd/harnessd`
  - `go vet ./internal/... ./cmd/...`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This issue does not touch provider/model flow surfaces, so no impact map is required.

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

- Risk: regression coverage could rely on timing or goroutine counting and become flaky.
- Mitigation: inject a tiny test seam around cleaner context creation so the test can assert cancellation deterministically.
