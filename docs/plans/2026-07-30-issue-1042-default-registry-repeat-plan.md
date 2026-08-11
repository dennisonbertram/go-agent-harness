# Plan: Make the Default Registry Contract Test Repeatable

## Context

- Governing GitHub issue: #1042.
- Problem: `TestDefaultRegistry_Functions` registers a fixed name in a
  process-global registry, so the second `-count` invocation fails before it
  can test its own duplicate-registration contract.
- User impact: an unrelated workspace package failure blocks the accepted
  cron/callback GUI merge chain.
- Constraint: keep production registry semantics and every top-level API
  assertion unchanged.

## Scope

- In scope: allocate a collision-free name for each test invocation.
- Out of scope: production reset/unregister APIs, registry ownership, package
  serialization, and runtime behavior.

## Documentation Contract

- Feature status: test-only bug repair in implementation.
- Public docs affected: none.
- Evidence: engineering and long-term logs, plan, impact map, plans index.

## Test Plan (TDD)

- Red: existing focused test at `-count=2` fails deterministically on the second
  invocation with `ErrAlreadyExists`.
- Green: focused normal/race at `-count=100`; each invocation still asserts
  first registration, duplicate rejection, listing, and provisioning.
- Full: workspace normal/race and repository normal/race/coverage gate.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1042-default-registry-repeat-impact-map.md`.

## Implementation Checklist

- [x] Create contract-complete bug #1042.
- [x] Capture deterministic red evidence.
- [x] Record registry ownership/search evidence.
- [x] Write plan and impact map before code.
- [x] Implement invocation-unique identity.
- [x] Run focused stress and full gates.
- [ ] Merge through a closing PR.

## Verification

- Focused normal and race tests passed at `-count=100`.
- The complete workspace package passed normal and race tests at `-count=5`.
- `./scripts/test-regression.sh` passed normal, race, and the 85.6% coverage
  gate with zero uncovered functions.

## Risks and Mitigations

- Risk: unique names could stop testing duplicate rejection.
- Mitigation: register the same invocation-local name twice and retain the
  explicit `ErrAlreadyExists` assertion.
- Risk: a timing-derived name could collide.
- Mitigation: use a process-local atomic sequence, not wall-clock time.
