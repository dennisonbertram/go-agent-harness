# Plan: Runner Concurrency Invariants

## Context

- Problem: A review on the recent runner forensics work called out that the code fixes landed, but the deeper concurrency and lifecycle invariants are still too implicit.
- User impact: Future recorder, compaction, or forensic changes can regress ordering, ownership, or state-transition guarantees without an obvious place in the code/tests that says what must stay true.
- Constraints:
  - Preserve current runner behavior.
  - Prefer invariant-focused tests over broad refactors.
  - Avoid touching unrelated dirty worktree files.

## Scope

- In scope:
  - Make the runner's concurrency/lifecycle invariants explicit in code comments near the affected state and helpers.
  - Add regression coverage for the recorder ledger invariant.
  - Tighten test comments around the message-state source-of-truth invariant.
  - Update logs and plan trackers.
- Out of scope:
  - New concurrency architecture changes.
  - Refactoring unrelated runner subsystems.
  - Reopening already-fixed review items as behavior changes.

## Test Plan (TDD)

- New failing tests to add first:
  - `TestEventLedgerInvariant_JSONLMatchesInMemoryHistory`
- Existing tests to update:
  - `TestCompactRunSurvivesConcurrentExecute`
  - `TestCompactRunAtStepBoundary`
- Regression tests required:
  - Recorder ledger remains complete and ordered relative to in-memory history.
  - Existing compaction regression tests continue to defend `state.messages` as the only source of truth.
  - Existing forensic isolation tests continue to defend payload ownership boundaries.

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

- Risk: Invariant wording drifts away from the actual code paths.
- Mitigation: Put the invariant block next to `runState`, `emit`, and message replacement helpers, and back it with a recorder regression test.
