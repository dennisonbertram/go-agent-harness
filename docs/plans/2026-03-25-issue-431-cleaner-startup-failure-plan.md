# Plan: Issue #431 Conversation Cleaner Startup Failure Cleanup

## Context

- Problem: `cmd/harnessd/main.go` starts the conversation-retention cleaner but only cancels it on the normal signal-driven shutdown path, so later startup failures can bypass cleanup and `go vet` reports a possible context leak.
- User impact: operators can hit bootstrap failures that leave a background cleaner context alive longer than intended, and the codebase currently fails a useful static check.
- Constraints:
  - Follow strict TDD.
  - Keep the change narrow and avoid a broad bootstrap refactor.
  - Preserve existing shutdown behavior except for guaranteed cleanup on startup-error paths.

## Scope

- In scope:
  - Add failing regression coverage for startup failure after cleaner initialization.
  - Introduce the smallest test seam needed to observe cleaner cancellation.
  - Ensure cleaner cleanup runs on all exits after initialization.
- Out of scope:
  - General `harnessd` bootstrap decomposition.
  - Unrelated startup or shutdown behavior changes.

## Test Plan (TDD)

- New failing tests to add first:
  - startup failure after conversation-cleaner init cancels the cleaner context before `runWithSignals` returns.
- Existing tests to update:
  - keep the existing clean shutdown cleaner test green.
- Regression tests required:
  - `go test ./cmd/harnessd`
  - `go vet ./internal/... ./cmd/...`

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

- Risk: a test-only seam for cleaner startup could accidentally broaden runtime behavior.
- Mitigation: keep the seam unexported, default it to the real cleaner, and use it only to characterize the startup-failure cleanup path.
