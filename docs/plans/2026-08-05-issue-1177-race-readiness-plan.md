# Plan: Issue #1177 harnessd race-readiness fixtures

## Context

- Governing GitHub issue: #1177.
- Problem: two parallel `cmd/harnessd` memory-configuration tests reserved a
  free address, closed that listener, and then polled the predicted address
  for three seconds. A competing daemon could rebind that address or startup
  could outlast the diagnostic deadline under hosted race load.
- User impact: unrelated red CI blocks acceptance of real cron/callback and
  transcript proof work.
- Constraints: test-only repair; preserve the two environment/config contracts;
  do not globally increase timeouts or serialize package tests.

## Scope

- In scope: migrate exactly the two named tests to the existing
  listener-aware matrix path, with `HARNESS_ADDR=127.0.0.1:0` and their
  retained three-second health diagnostic deadline.
- Out of scope: production daemon startup, callback/cron semantics, global
  matrix timeouts, and all unrelated test fixtures.

## Documentation Contract

- Feature status: `implemented`.
- Public docs affected: none; this changes only test synchronization.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: durable logs and their index.

## Test Plan (TDD)

- Characterization red: hosted race CI demonstrated the reserve-close-rebind
  fixture can fail both tests with `never became healthy within 3s`; local
  focused race stress characterizes the unchanged configuration contracts.
- Existing tests to update: the two tests retain their environment maps,
  three-second health deadline, and daemon lifecycle assertion, but delegate
  listener identity/readiness and graceful shutdown to the existing helper.
- Regression tests required: focused normal/race stress, complete harnessd
  normal/race, and serial `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- Linked artifact: `2026-08-05-issue-1177-race-readiness-impact-map.md`.

## Implementation Checklist

- [x] Confirm structured issue and current source/CI failure signature.
- [x] Record ownership, impact, and test-first contract.
- [x] Replace each reserve-close-rebind fixture with listener-aware matrix use.
- [x] Run focused normal/race stress and full harnessd suites.
- [x] Run serial repository regression with `TMPDIR=/private/tmp`.
- [ ] Update final log evidence, commit, push, PR, review, and merge gates.

## Risks and Mitigations

- Risk: changing a test's configuration behavior. Mitigation: retain every
  environment/config input other than binding port zero, and keep the real
  `runWithSignalsWithDeps` lifecycle through the helper.
- Risk: masking a startup regression by extending timing. Mitigation: use the
  listener acquired by the daemon as readiness authority; no timeout grows.
