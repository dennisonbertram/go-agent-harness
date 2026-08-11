# Plan: deterministic retry-wait recovery fixture (Issue #1124)

## Context and Scope

- Governing issue: #1124, following the hosted race red on callback
  retry-wait recovery after #1107 merged at `46e5c0bc`.
- Problem: `TestCallbackManagerRecoveryHonorsRetryWaitAndReusesIdentity` used
  a 60 ms real retry deadline and a 15 ms sleep. Under aggregate race load,
  the timer could cross that unsynchronised boundary before the assertion.
- User impact: the durable contract remains that a failed callback continues
  the same conversation only at its persisted retry deadline, with the
  originally reserved run identity and no leaked ownership token.
- Constraints: test and documentation only; no callback manager/store/API/TUI,
  native GUI, schema, timer policy, or retry behaviour change.

## Test-First Contract

1. The initial red is a compile failure naming the missing test-only,
   thread-safe fake clock: `undefined: newCallbackFixtureClock`.
2. Recovery persists and publishes the untouched `retry_wait` snapshot with a
   one-hour `next_attempt_at`, stable run ID/attempt, and empty token/lease.
3. Explicit `fire` before fake deadline must not admit or mutate that durable
   snapshot; advancing the fake clock exactly one hour then explicitly firing
   must admit once as attempt two with the same reserved run ID and cleared
   terminal ownership fields.

## Cross-Surface and Rollout

See `2026-08-03-issue-1124-retry-wait-fixture-impact-map.md`. Rollback is a
revert of this isolated test/docs PR. No deployment or data migration occurs.

## Checklist

- [x] Verify the structured #1124 acceptance criteria and exact merged base.
- [x] Record an honest test-first red, then replace only the timing fixture.
- [x] Focused normal/race x100 passed in 0.419s/2.549s; complete tools
  normal/race passed in 13.200s/14.719s.
- [x] Isolated foreground `./scripts/test-regression.sh` passed normal, race,
  85.5% total coverage, and zero uncovered functions in 2m26s.
- [ ] Push one reviewable PR with `Closes #1124`; do not merge it here.
