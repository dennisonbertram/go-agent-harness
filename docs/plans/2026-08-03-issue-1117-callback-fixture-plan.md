# Plan: deterministic callback claim fixtures (Issue #1117)

## Context

- Governing GitHub issue: #1117.
- Problem: the duplicate-manager callback fixture used a 30 ms lease, blocked
  `StartCallback`, then slept 100 ms. Under aggregate CI load that can be a
  legitimate sequential expiry/reclaim rather than duplicate ownership.
- User impact: a load-sensitive test blocks reliable acceptance of durable
  callback work and can misdiagnose the production ownership protocol.
- Constraints: this is test-and-documentation-only. The branch is stacked on
  unmerged callback claim head `74e21270`; it must not close #1106 or alter
  callback storage, claim, retry, lease, or recovery semantics.

## Scope

- In scope: give the duplicate-manager fixture its normal non-expiring/default
  lease; remove its unrelated wait-past-lease assertion; retain exclusive
  recovery failure, exactly one starter call, attempt one, and reserved run
  linkage. Strengthen transient SQLite claim-contention coverage with the same
  exact-one-starter assertion.
- Out of scope: production code; callback retry/lease policy; GUI/TUI/API
  behavior; merging either this stacked PR or #1106.

## Documentation Contract

- Feature status: `implemented` only as test-fixture stabilization after the
  test changes and verification complete.
- Public docs affected: none; this does not change product behavior.
- Implementation notes: this plan, impact map, logs, and indexes record the
  hosted symptom and exact stacked provenance.

## Test Plan (TDD)

- First characterization: run the pre-change duplicate-manager fixture under
  normal/race repetition, preserving the known hosted false-positive diagnosis
  rather than treating a timing pass as production proof.
- Existing tests updated: `TestCallbackManagerDuplicateManagersClaimOneDispatch`
  and `TestCallbackManagerRetriesTransientClaimContention`.
- Regression tests: focused normal/race `-count=100`, complete tools package
  normal/race, and foreground `./scripts/test-regression.sh`.

## Implementation Checklist

- [x] Verify structured issue and Sol diagnosis.
- [x] Record architecture/search/provenance and complete impact map.
- [x] Update only the two fixtures with deterministic ownership assertions.
- [x] Run focused normal/race and complete tools package normal/race gates.
- [x] Run the foreground full regression gate.
- [ ] Update logs/indexes, push one closing stacked PR, and stop for review.

## Risks and Mitigations

- Risk: removing the elapsed-lease wait could weaken exclusive-manager proof.
  Mitigation: preserve the direct second `Recover` failure plus exact starter,
  durable attempt, and run-ID assertions; the separate heartbeat tests retain
  lease-expiry behavior.
- Risk: testing unmerged behavior against main masks provenance. Mitigation:
  branch base is explicitly `74e21270` and the PR will state it stacks on #1106.

## Evidence

- Red (pre-change): `go test ./internal/harness/tools -run
  '^TestCallbackManager(DuplicateManagersClaimOneDispatch|RetriesTransientClaimContention)$'
  -count=100 -v` failed at `delayed_callback_retry_red_test.go:494` with
  `attempts = 2, want 1`; the 30 ms fixture lease had expired during its 100 ms
  blocked admission. The corresponding race x100 happened to pass, confirming
  a scheduling-sensitive fixture rather than a waived baseline.
- Green: the two focused tests pass normal x100 in 5.666s and race x100 in
  14.099s. Complete `./internal/harness/tools` passes normal in 12.480s and
  race in 14.381s. After all overlapping repository gates had exited, one
  isolated foreground `./scripts/test-regression.sh` exited 0: normal and race
  passed, coverage was 85.5%, and zero production functions were uncovered.
