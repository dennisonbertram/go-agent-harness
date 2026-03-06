# Plan: Terminal Bench Periodic Suite

## Context

- Problem: The repository has regression tests, but no periodic benchmark that exercises the real harness loop against stable terminal tasks.
- User impact: Harness regressions in end-to-end behavior can slip through unit/regression coverage without a recurring benchmark signal.
- Constraints:
  - Keep the benchmark small and deterministic.
  - Reuse Terminal Bench instead of inventing a custom harness.
  - Support local runs and scheduled CI execution.

## Scope

- In scope:
  - Add a small private Terminal Bench dataset tailored to the harness.
  - Add a custom Terminal Bench agent bridge that runs this harness inside task containers.
  - Add a local runner script and a scheduled GitHub Actions workflow.
  - Document usage and update required logs/indexes.
- Out of scope:
  - Large public benchmark ingestion.
  - Leaderboard/reporting automation beyond artifact upload.
  - PR gating on paid benchmark execution.

## Test Plan (TDD)

- New failing tests to add first:
  - Terminal Bench task assertions for Go bugfix, config/docs edits, and shell output generation.
- Existing tests to update:
  - None.
- Regression tests required:
  - The benchmark tasks themselves act as the new periodic smoke suite.

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

- Risk: Terminal Bench CLI behavior or import-path contracts change upstream.
- Mitigation: keep the runner script thin, use a local custom agent, and document the official dependency points.
