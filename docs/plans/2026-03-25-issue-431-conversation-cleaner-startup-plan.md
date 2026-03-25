# Plan: Issue #431 conversation cleaner startup cleanup

## Context

- Problem: `cmd/harnessd/main.go` starts the conversation retention cleaner with its own cancel function, but startup errors after initialization can return before that cancel function is called.
- User impact: failed `harnessd` startups can leak the cleaner context/goroutine longer than intended, and `go vet` already flags the path as a possible context leak.
- Constraints: keep the fix narrow, preserve existing startup/shutdown behavior, follow strict TDD, and verify with targeted `go test` and `go vet`.

## Scope

- In scope:
  - reproduce the `go vet` warning
  - add a failing regression test for a startup-failure path after the cleaner starts
  - implement the smallest cleanup change that guarantees the cleaner cancel function runs on all exit paths after initialization
  - update logs/docs required by repo workflow
- Out of scope:
  - broad `harnessd` bootstrap decomposition
  - unrelated startup refactors
  - changing conversation retention behavior itself

## Test Plan (TDD)

- New failing tests to add first:
  - `cmd/harnessd/main_test.go`: startup failure after conversation cleaner initialization still cancels the cleaner context
- Existing tests to update:
  - none unless the smallest safe seam requires a helper or hook
- Regression tests required:
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./cmd/harnessd`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go vet ./internal/... ./cmd/...`

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

- Risk: a test seam for the startup-failure path could accidentally pull in a wider bootstrap refactor.
- Mitigation: add a tiny injected hook/helper only if needed to deterministically force the post-cleaner failure path.
