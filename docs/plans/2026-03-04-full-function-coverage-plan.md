# Plan: Full Function Test Coverage

## Context

- Problem: Test suite passes, but several functions are not exercised (`cmd/harnessd` entrypoints, runner failure helpers, and HTTP error handlers).
- User impact: Lower confidence in behavior under failure/edge conditions.
- Constraints:
  - Keep production behavior unchanged.
  - Use strict TDD additions and keep scope focused on tests + minimal refactor for testability.

## Scope

- In scope:
  - Add tests for all currently uncovered functions.
  - Refactor `cmd/harnessd/main.go` minimally to make entrypoint logic testable.
  - Add missing failure-path tests in harness/server packages.
- Out of scope:
  - Performance/load testing.
  - End-to-end integration against live OpenAI API.

## Test Plan (TDD)

- New failing tests to add first:
  - `main` behavior for success and failure paths (exit behavior).
  - `run`/startup path helper behavior and env helper tests.
  - Runner failure paths (`failRun`, `mustJSON` fallback).
  - HTTP handler branches (`health`, method-not-allowed, invalid JSON, missing run).
- Existing tests to update:
  - Extend `internal/server/http_test.go`.
  - Extend `internal/harness/runner_test.go`.
- Regression tests required:
  - Ensure added testability hooks do not alter runtime behavior.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Testability refactor in `main` could unintentionally alter startup/shutdown behavior.
- Mitigation: Keep functional flow identical and add explicit tests around exit/run delegation paths.
