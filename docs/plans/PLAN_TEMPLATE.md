# Plan: <feature or bugfix name>

## Context

- Governing GitHub issue:
- Problem:
- User impact:
- Constraints:

## Scope

- In scope:
- Out of scope:

## Documentation Contract

- Feature status: `planned` | `in implementation` | `implemented` | `deferred` | `rejected`
- Public docs affected:
- Spec docs to update before code:
- Implementation notes to add after code:

## Test Plan (TDD)

- New failing tests to add first:
- Existing tests to update:
- Regression tests required:

## Cross-Surface Impact Map

- Required for every non-minor task. Reconcile the structured issue's analysis
  and create a one-page artifact from `IMPACT_MAP_TEMPLATE.md` for complex work.
- Cover current ownership/callers and data flow; config/API/CLI; persistence;
  lifecycle/concurrency; security/privacy; product clients;
  provider/model/tool catalogs; deployment/observability; compatibility;
  tests; and documentation.
- A blank heading is a warning. Write `None` with rationale when a surface is truly unaffected.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] Link a contract-complete structured GitHub issue before implementation.
- [ ] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [ ] Document feature status and exact contract before code.
- [ ] Complete and reconcile the cross-surface impact map before implementation.
- [ ] Add characterization coverage before structural refactors.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs, status ledgers, and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk:
- Mitigation:
