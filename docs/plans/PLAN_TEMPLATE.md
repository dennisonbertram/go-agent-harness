# Plan: <feature or bugfix name>

## Context

- Problem:
- User impact:
- Constraints:

## Scope

- In scope:
- Out of scope:

## Test Plan (TDD)

- New failing tests to add first:
- Existing tests to update:
- Regression tests required:

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- Create a one-page impact map from `IMPACT_MAP_TEMPLATE.md` covering:
  - Config
  - Server API
  - TUI state
  - Regression tests
- A blank heading is a warning. Write `None` with rationale when a surface is truly unaffected.

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

- Risk:
- Mitigation:
