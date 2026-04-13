# Plan: 2026-03-29 Training Test Fixes

## Context

- Problem:
  - `go test ./...` still failed after the repo-structure cleanup because several training packages had correctness bugs and one test deadlocked.
- User impact:
  - The repository could not claim a clean test baseline, which makes later refactors harder to trust.
- Constraints:
  - Keep fixes scoped to the failing training packages.
  - Prefer correctness and determinism over preserving overly clever concurrency patterns in training code.

## Scope

- In scope:
  - Fix `tmp/training-pubsub`.
  - Fix `tmp/training-skiplist`.
  - Fix `tmp/training-regex`.
  - Fix `training-regex`.
  - Fix the deadlocking/incorrect trie training package tests and delete contract.
- Out of scope:
  - Broader product runtime refactors.
  - Reworking the isolated `playground/` examples.

## Test Plan (TDD)

- New failing tests to add first:
  - None. Existing package tests already reproduced the failures clearly.
- Existing tests to update:
  - `training-trie/trie_test.go` to remove the deadlocking `t.Parallel()` pattern.
- Regression tests required:
  - `go test ./tmp/training-pubsub ./tmp/training-skiplist`
  - `go test ./tmp/training-regex ./training-regex`
  - `go test ./training-trie`
  - `go test ./...`

## Cross-Surface Impact Map

- None.
  - This task only touches isolated training packages and their tests.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] For provider/model flow work, add or update the one-page impact map before implementation.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk:
  - Fixes in training code could accidentally overfit the visible tests while hiding deeper contract problems.
- Mitigation:
  - Keep each fix aligned to the package’s obvious public contract and confirm with focused package reruns plus the repo-wide suite.
