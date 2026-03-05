# Plan: Regression Testing Guardrails

## Context

- Problem: Tests pass today, but there is no automated gate to prevent future regression in coverage and edge-path validation.
- User impact: Future changes could ship with weaker test quality and untested functions.
- Constraints:
  - Keep implementation lightweight and easy to run locally.
  - Enforce via script + CI, not local git hooks.

## Scope

- In scope:
  - Add a reusable regression script that runs core tests, race checks, and coverage checks.
  - Add function-level zero-coverage detection + minimum total coverage threshold validation.
  - Add CI workflow to execute regression suite on PRs and pushes.
  - Add a regression contract test for default tool definitions.
  - Update runbooks and README for new standard workflow.
- Out of scope:
  - Branch protection settings (repository admin configuration).
  - Live integration tests against external APIs.

## Test Plan (TDD)

- New failing tests to add first:
  - Coverage gate parser/validator tests.
  - Tool contract regression test for default registry tools.
- Existing tests to update:
  - N/A.
- Regression tests required:
  - Coverage gate fails on `0.0%` function coverage.
  - Coverage gate fails below configured total threshold.

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

- Risk: Coverage threshold may be too strict for rapid iteration.
- Mitigation: threshold is configurable via environment variable while default remains enforced in CI.
