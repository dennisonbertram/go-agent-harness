# Plan: Issue #429 forked child-run failure propagation

## Context

- Problem: several callers treat `RunForkedSkill(...)` as successful whenever the Go `error` is nil, even if the returned `ForkResult` carries a terminal child-run failure in `Error`.
- User impact: `/v1/agents` and fork-context skill tools can report success and surface partial output when the child run actually failed.
- Constraints: keep the fix narrow, preserve healthy success paths, and follow strict TDD with regression coverage on each affected caller surface.

## Scope

- In scope:
  - `/v1/agents` forked skill execution failure handling
  - flat skill-tool fork path failure handling
  - core skill-tool fork path failure handling
  - issue-specific plan/log updates
- Out of scope:
  - broader runner refactors
  - unrelated `allowed_tools` fallback work
  - transport or tool contract redesign

## Test Plan (TDD)

- New failing tests to add first:
  - `/v1/agents` returns an execution error when `RunForkedSkill(...)` returns `ForkResult{Error: ...}` with `err == nil`
  - flat skill-tool fork path returns an error when `ForkResult.Error` is populated
  - core skill-tool fork path returns an error when `ForkResult.Error` is populated
- Existing tests to update:
  - none expected beyond the new focused regressions
- Regression tests required:
  - `go test ./internal/server ./internal/harness/tools ./internal/harness/tools/core`

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

- Risk: error handling drifts between server and tool call sites again.
- Mitigation: centralize the child-failure check in small shared helpers local to each package boundary and pin each caller with regression tests.
