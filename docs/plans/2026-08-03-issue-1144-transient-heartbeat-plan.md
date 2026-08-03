# Plan: Issue #1144 transient callback heartbeat fixture

## Context

- Governing GitHub issue: #1144.
- Problem: `TestCallbackManagerTransientHeartbeatBusyRetainsClaim` used a 90 ms
  lease and a raw 150 ms sleep to infer a successful post-busy renewal. Under
  race load the original lease could expire before that inference, allowing a
  legitimate production retry to reach attempt 2 and making the fixture flaky.
- User impact: an unrelated timing fixture makes callback/cron delivery CI red
  without proving a callback product defect.

## Scope

- In scope: test-only notification channels in `transientLeaseStore`, a
  one-second lease, causal failure-and-success waits, durable deadline/attempt
  assertions, cleanup ordering, and evidence documentation.
- Out of scope: callback retry, lease, token, persistence, API, TUI, GUI, or
  production scheduler behavior.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: none; this is an internal regression fixture.
- Spec docs before code: this plan and its impact map.
- Implementation notes after code: engineering, observational, system, and
  long-term-thinking logs plus folder indexes.

## Test Plan (TDD)

- First red: require the transient fixture to await explicit injected-first-
  failure and first-successful-delegated-renewal channels, rather than sleep.
- Acceptance: capture the initial durable token/deadline; after the successful
  delegated renewal, require the same token, attempt `1`, and a later durable
  deadline before releasing the starter.
- Cleanup: an idempotent starter release runs before manager shutdown so failed
  assertions cannot strand a blocked dispatch goroutine.
- Commands: focused normal/race `-count=100`, callback package normal/race,
  and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-03-issue-1144-transient-heartbeat-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue, exact remote base, ownership, and source search.
- [x] Record plan and impact map.
- [x] Capture the required red fixture compile/test failure.
- [x] Add only test-store causal notification behavior.
- [x] Run focused/package/full normal and race gates.
- [x] Update durable logs/indexes and draft-PR evidence.

## Risks and Mitigations

- Risk: a notification channel masks an expired lease rather than proving a
  persisted extension.
- Mitigation: notify only after the real wrapped `ExtendLease` succeeds, then
  re-read the SQLite row and compare the durable token/deadline/attempt.
- Rollback: revert the fixture-only channels/assertions; no migration or
  production state exists.

## Verification Evidence

- Red: `go test ./internal/harness/tools -run
  '^TestCallbackManagerTransientHeartbeatBusyRetainsClaim$' -count=1` failed
  to compile because the pre-fix fixture had no `failed` or `renewedUntil`
  channels.
- Green: focused normal passed; focused race x20 passed in 11.745s; focused
  normal x100 passed in 51.249s; persisted focused race x100 exited `0` in
  53.917s.
- Affected package: `go test ./internal/harness/tools` and `-race` passed in
  13.740s and 15.259s.
- Full gate: `./scripts/test-regression.sh` log ends in coverage-gate PASS at
  85.5% total coverage with zero uncovered functions and `[regression] PASS`.
