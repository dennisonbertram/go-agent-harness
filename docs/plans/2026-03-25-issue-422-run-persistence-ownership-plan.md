# Plan: Issue #422 Run Persistence Ownership

## Context

- Problem: The runner already persists initial run records, but the HTTP layer duplicates `CreateRun` calls for direct run creation and external-trigger start/continue flows.
- User impact: Persistence ownership is unclear, duplicate writes complicate store semantics, and future transports could repeat the same mistake.
- Constraints: Keep the change narrow, preserve response contracts, and follow strict TDD with regression coverage.

## Scope

- In scope:
  - Add failing tests for duplicate `CreateRun` calls on HTTP-backed run creation paths.
  - Remove transport-layer duplicate inserts where the runner already persists the run.
  - Keep store errors non-fatal in the same places they are today.
- Out of scope:
  - Store API redesign.
  - Broader HTTP transport decomposition.
  - Persistence behavior unrelated to initial run creation.

## Test Plan (TDD)

- New failing tests to add first:
  - `POST /v1/runs` persists exactly once when a store is configured.
  - External-trigger `start` persists exactly once.
  - External-trigger `continue` persists exactly once.
- Existing tests to update:
  - Server persistence tests around run creation and trigger handling, if needed for current helpers.
- Regression tests required:
  - Targeted `go test ./internal/server ./internal/harness`.
  - Repo regression script rerun before final handoff.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This issue does not change provider/model flow behavior, so no impact map is required.

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

- Risk: Removing the HTTP-side insert could accidentally change behavior on a path where the runner does not actually persist yet.
- Mitigation: Pin single-write behavior in failing tests for both direct HTTP and external-trigger flows before touching production code.
