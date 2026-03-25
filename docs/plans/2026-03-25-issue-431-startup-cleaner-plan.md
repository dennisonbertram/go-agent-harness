# Plan: Issue #431 startup cleaner cancellation

## Context

- Problem: `cmd/harnessd/main.go` starts the conversation-retention cleaner with a cancelable context, but some startup-error returns can bypass the shutdown block that calls `convCleanerCancel()`.
- User impact: background cleaner goroutines can outlive a failed startup attempt, and `go vet` already flags the path as a possible context leak.
- Constraints: keep the bootstrap behavior unchanged aside from guaranteed cleanup, follow strict TDD, and verify with targeted `go test` plus `go vet`.

## Scope

- In scope:
  - add a failing regression test for a startup failure after the cleaner is started
  - implement the smallest `cmd/harnessd` cleanup fix that cancels the cleaner on all exit paths
  - update logs/docs needed for the issue workflow
- Out of scope:
  - broader `harnessd` bootstrap refactors
  - changing conversation retention semantics
  - unrelated `go vet` findings outside this issue

## Test Plan (TDD)

- New failing tests to add first:
  - a `cmd/harnessd` test that starts the conversation cleaner, forces a startup failure, and asserts the cleaner context is cancelled before `runWithSignals(...)` returns
- Existing tests to update:
  - keep the existing clean-shutdown cleaner test passing
- Regression tests required:
  - `go test ./cmd/harnessd`
  - `go vet ./internal/... ./cmd/...`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This issue does not touch those surfaces, so no impact map is required.

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

- Risk: the regression test could become timing-sensitive if it depends on real goroutine behavior.
- Mitigation: inject a tiny cleaner factory seam so the test can observe cancellation deterministically without waiting on the real background sweep loop.
