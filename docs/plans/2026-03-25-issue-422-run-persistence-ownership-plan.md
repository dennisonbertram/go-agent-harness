# Plan: Issue #422 run persistence ownership

## Context

- Problem: run-record persistence is owned by both the runner and the HTTP transport, so `POST /v1/runs` and external-trigger start/continue paths can call `CreateRun` twice for the same logical run.
- User impact: duplicate writes make persistence behavior harder to reason about, blur the boundary between transport and domain layers, and raise the risk of store-specific bugs or misleading failure handling.
- Constraints: keep the change small, preserve current response shapes and non-fatal persistence behavior, and follow strict TDD with regression coverage before code changes.

## Scope

- In scope:
  - `POST /v1/runs` persistence ownership
  - external-trigger `start` and `continue` persistence ownership
  - focused regression coverage for single-write behavior and store-backed retrieval
  - issue-specific docs/log updates
- Out of scope:
  - store API redesign
  - broader transport decomposition
  - unrelated runtime or provider behavior

## Test Plan (TDD)

- New failing tests to add first:
  - `POST /v1/runs` persists exactly once when a store is configured
  - external-trigger `start` persists exactly once when a store is configured
  - external-trigger `continue` persists exactly once for the new run record
- Existing tests to update:
  - store-backed HTTP tests that currently describe transport-side persistence should be reworded around runner-owned persistence
- Regression tests required:
  - `go test ./internal/server`
  - `go test ./internal/harness`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This issue is a persistence-ownership fix and does not touch provider/model flow surfaces, so no impact map is required.

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

- Risk: removing HTTP-side inserts could accidentally break store-backed list/get flows if the runner is not the only write path in practice.
- Mitigation: pin both single-write behavior and store-backed retrieval/continuation flows with focused tests before deleting the duplicate transport writes.
