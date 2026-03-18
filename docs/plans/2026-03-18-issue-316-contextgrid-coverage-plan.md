# Plan: Issue 316 Context Grid Coverage

## Context

- Problem: `cmd/harnesscli/tui/components/contextgrid` has direct production usage but no package tests, leaving rendering semantics unpinned.
- User impact: regressions in token clamping, width handling, or displayed usage text could silently slip through broader TUI tests.
- Constraints: keep scope limited to issue `#316`; follow strict TDD and preserve existing overlay behavior outside the context grid component.

## Scope

- In scope:
  - add direct package tests for context-grid rendering behavior
  - cover default total-token fallback, usage clamping, width handling, and rendered text shape
  - make the smallest production fix needed for any failing regression
- Out of scope:
  - broader TUI overlay refactors
  - unrelated coverage work in other TUI components

## Test Plan (TDD)

- New failing tests to add first:
  - default total-token fallback and percentage formatting
  - negative and over-limit token clamping
  - narrow-width rendering behavior
- Existing tests to update:
  - none expected
- Regression tests required:
  - `go test ./cmd/harnesscli/tui/components/contextgrid`
  - `go test ./cmd/harnesscli/tui/...`
  - `./scripts/test-regression.sh`

## Cross-Surface Impact Map

- None. This task does not touch provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: narrow-width fixes could degrade the default wider layout or break Unicode bar rendering.
- Mitigation: assert the default 60-cell bar separately and keep width truncation rune-safe.
