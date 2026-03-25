# Plan: Issue #422 Run Persistence Ownership

## Context

- Problem: both the runner and the HTTP transport create the initial persisted run record, so `CreateRun` happens twice for the same run on some entrypoints.
- User impact: persistence ownership is ambiguous, duplicate writes hide the real domain boundary, and future transports can easily repeat the same mistake.
- Constraints:
  - strict TDD
  - keep HTTP response shapes unchanged
  - leave store persistence best-effort and non-fatal where it is today

## Scope

- In scope:
  - `POST /v1/runs` persistence ownership
  - external trigger `start` and `continue` persistence ownership
  - explicit regression coverage for runner-owned `ContinueRun` persistence
- Out of scope:
  - store API redesign
  - broader `internal/server` decomposition
  - changes to retrieval/listing contracts

## Test Plan (TDD)

- New failing tests to add first:
  - `TestPostRunsPersistsExactlyOnce`
  - `TestHandleExternalTrigger_StartPersistsExactlyOnce`
  - `TestHandleExternalTrigger_ContinuePersistsExactlyOnce`
  - `TestRunnerStore_ContinueRunCreateRunCalledExactlyOnce`
- Existing tests to update:
  - none required beyond renaming the direct POST persistence test to match the ownership contract
- Regression tests required:
  - `go test ./internal/server ./internal/harness`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- None. This issue changes run-store ownership only and does not alter provider/model flow surfaces.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: removing transport writes could break store-backed historical retrieval if the runner store is not wired.
- Mitigation: share the same store with runner and server in regression tests and verify the persisted run is still retrievable with a single `CreateRun`.
