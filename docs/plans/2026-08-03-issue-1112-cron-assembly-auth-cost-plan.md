# Plan: Issue #1112 cron assembly authentication cost isolation

## Context

- Governing GitHub issue: #1112.
- Problem: the assembled cron-to-harnessd test stores a production cost-12
  bcrypt API-key hash. Under `-race` and aggregate package load, the real HTTP
  request can spend its entire five-second transport budget in authentication,
  causing an otherwise accepted idempotent start to be recorded as timed out.
- User impact: this is a test-fixture failure that blocks release confidence;
  no live evidence currently shows the scheduler, authenticated correlation,
  durable idempotency, run linkage, or terminal observation contract is wrong.
- Constraints: preserve the #1003 contract that remote retries are classified
  but not implemented. Do not increase request/job deadlines, add sleeps, alter
  production bcrypt cost, or modify scheduler/remote-start behavior.

## Scope

- In scope: the authenticated assembly fixture's API-key hash cost; a
  deterministic invariant that rejects production-cost hashes in this bounded
  timing test; focused normal/race stress and full regression evidence.
- Out of scope: production API-key security, retry policy, remote starter,
  scheduler timeouts, idempotency persistence, callback work (#1106), native
  UI (#1009), and convergence proof (#1010).

## Architecture and search evidence

- `internal/cron/assembly_integration_test.go` calls
  `store.GenerateAPIKey`, which intentionally hashes at production cost 12,
  then sends the key through a real authenticated `httptest` harnessd.
- `internal/store/apikeys.go` owns the production cost; it must remain 12.
- `internal/store/apikey_test_helpers_test.go` and
  `internal/server/http_subagents_tenant_test.go` establish the repository
  pattern of rehashing synthetic test credentials with `bcrypt.MinCost` under
  race instrumentation.
- `internal/cron/remote_run_starter.go` owns the five-second configured request
  bound in this test and preserves the correlation/idempotency key unchanged.
- `docs/plans/2026-07-31-issue-1003-remote-cronsd-plan.md` explicitly keeps
  remote retries beyond classification out of scope.

## Test plan (strict TDD)

1. Add a deterministic pre-dispatch assertion that the assembly credential
   uses `bcrypt.MinCost`; capture the current red (`got 12, want 4`).
2. Rehash only the synthetic test token at `bcrypt.MinCost`; keep real bearer
   authentication, scope enforcement, request timeout, idempotency key,
   scheduler execution, durable run linkage, terminal observation, and
   conversation scope assertions unchanged.
3. Run the focused assembly test repeatedly in normal and race modes, the
   complete cron package in normal/race modes, and `./scripts/test-regression.sh`.

## Implementation checklist

- [x] Verify exact `origin/main` base and structured issue #1112.
- [x] Search authentication, remote-start, scheduler, timeout, idempotency,
  and existing low-cost credential fixture ownership.
- [x] Record plan and cross-surface impact map before test changes.
- [x] Capture deterministic cost-invariant red: cost 12, want 4.
- [x] Implement the test-only low-cost hash and make focused stress green:
  assembly normal x25 and race x10; complete cron normal/race.
- [x] Update durable logs/indexes and run the full regression gate: normal,
  race, 85.5% total coverage, and zero uncovered functions passed.
- [ ] Commit/push and open one PR with `Closes #1112`.

## Risks and rollback

- Risk: lowering production credential security. Mitigation: change only a
  test-local stored hash; `store.GenerateAPIKey` and production cost remain
  unchanged.
- Risk: masking remote timing behavior. Mitigation: retain real HTTP auth and
  the original finite request/job deadlines; remove only unrelated bcrypt CPU
  variability from this assembly contract test.
- Risk: hiding a retry defect. Mitigation: do not change retry semantics; the
  #1003 plan explicitly excludes retries, and this regression continues to
  prove one authenticated start and its durable linked terminal result.
- Rollback: revert this isolated test/docs PR if the fixture invariant weakens
  assembly coverage.
