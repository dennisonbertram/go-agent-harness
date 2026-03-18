# Plan: Issue 316 Context Grid Coverage

## Context

- Problem: `cmd/harnesscli/tui/components/contextgrid` has no direct package tests, so regressions in token normalization, width handling, and rendering can slip past higher-level overlay coverage.
- User impact: `/context` overlay behavior can change silently, especially around default totals, clamping, and progress-bar output.
- Constraints: Stay scoped to issue `#316`, follow strict TDD, avoid behavior changes outside the tested rendering contract, and keep docs/logs current.

## Scope

- In scope:
  - Add direct package tests for `cmd/harnesscli/tui/components/contextgrid`.
  - Cover default total fallback, token clamping, width fallback and bar limits, and rendered text/percentage output.
  - Make any minimal production change needed only if a failing regression test exposes a mismatch with the issue’s acceptance criteria.
- Out of scope:
  - Refactoring unrelated TUI overlay code.
  - Broad UI redesign or changes to unrelated components.

## Test Plan (TDD)

- New failing tests to add first:
  - `View()` falls back to the default context window when `TotalTokens <= 0`.
  - `View()` clamps negative and over-limit `UsedTokens`.
  - `View()` applies width fallback/min/max progress-bar sizing.
  - `View()` renders the expected header, token counts, and percentage text.
- Existing tests to update:
  - None expected.
- Regression tests required:
  - At least one rendering assertion that would fail if the bar glyphs or usage text regress.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- This task does not touch those surfaces, so no impact map is required.

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

- Risk: The component may already satisfy the intended contract, making test-first evidence less explicit.
- Mitigation: Start with assertions that intentionally pin uncovered edge cases and run the package with coverage before and after to prove the added signal.
