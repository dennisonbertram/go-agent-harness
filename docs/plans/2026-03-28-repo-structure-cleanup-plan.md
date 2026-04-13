# Plan: 2026-03-28 Repository Structure Cleanup

## Context

- Problem:
  - The module root mixed product code with experimental/training snippets.
  - Root-level Go files used multiple package names and multiple `main()` functions, which made the repo harder to navigate and caused `go test ./...` to fail before reaching the real harness packages.
- User impact:
  - Contributors could not tell quickly which directories were product surfaces versus scratch space.
  - Repo verification was coupled to unrelated exploratory code.
- Constraints:
  - Preserve the existing product entrypoints under `cmd/` and package organization under `internal/`.
  - Prefer structural separation over rewriting exploratory snippets.
  - Keep the cleanup easy to understand and hard to regress.

## Scope

- In scope:
  - Remove ad hoc Go source from the module root.
  - Relocate exploratory snippets into a dedicated `playground/` area.
  - Isolate `playground/` behind its own Go module.
  - Add regression coverage for the repo layout.
  - Update repo docs and logs to explain the new structure.
- Out of scope:
  - Rewriting or repairing every playground snippet to product quality.
  - Refactoring runtime behavior in `cmd/`, `internal/`, or `plugins/`.
  - Collapsing every existing top-level training directory in one pass.

## Test Plan (TDD)

- New failing tests to add first:
  - `internal/quality/repostructure/root_layout_test.go` to fail when Go source exists at the repo root.
  - `internal/quality/repostructure/root_layout_test.go` to fail when `playground/` is not isolated behind its own `go.mod`.
- Existing tests to update:
  - None.
- Regression tests required:
  - `go test ./internal/quality/repostructure`
  - `go test ./cmd/... ./internal/... ./plugins/...`

## Cross-Surface Impact Map

- None.
  - This task changes repository structure and documentation, not provider/model/config/TUI flow behavior.

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

- Risk:
  - Moving files could accidentally change how snippet packages compile.
- Mitigation:
  - Keep file groups intact and isolate them behind a separate module so product verification no longer depends on every snippet compiling cleanly.

- Risk:
  - Contributors may reintroduce root-level scratch files over time.
- Mitigation:
  - Enforce the new boundary with `internal/quality/repostructure/root_layout_test.go` and document the intended placement in `README.md` and `playground/README.md`.
