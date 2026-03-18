# Plan: Ownership And Copy-Semantics Hardening

## Context

- Problem: recent runner review loops repeatedly found shallow-copy and aliasing bugs in exported or stored harness state.
- User impact: callers can accidentally or concurrently corrupt transcript, tool schema, or event state through shared slices/maps/pointers.
- Constraints: preserve existing nil semantics, keep fixes small and reviewable, and validate with tests before broader regression.

## Scope

- In scope:
  - Add a reusable clone contract for exported/state-storing harness types with mutable fields.
  - Harden registry storage/export of `ToolDefinition.Parameters`.
  - Normalize runner message snapshot reads onto the shared deep-copy path.
  - Add a reusable internal checklist doc for ownership review.
- Out of scope:
  - Reworking unrelated concurrency issues outside ownership/copy boundaries.
  - Refactoring every package in the repo onto clone helpers in one pass.

## Test Plan (TDD)

- New failing tests to add first:
  - Registry tests proving `ToolDefinition.Parameters` is isolated from caller-owned and returned mutations.
  - `ToolDefinition.Clone()` nil-semantics test.
- Existing tests to update:
  - None expected.
- Regression tests required:
  - `go test ./internal/harness`
  - `./scripts/test-regression.sh`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: clone helpers accidentally erase the distinction between `nil` and empty slices/maps.
- Mitigation: keep nil-semantics tests and preserve existing helper behavior where callers already depend on it.
