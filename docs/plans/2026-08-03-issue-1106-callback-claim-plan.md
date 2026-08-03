# Plan: Issue #1106 durable callback claim ownership

## Context

- Governing GitHub issue: #1106.
- Problem: competing persistent callback managers can lose a valid lease after a transient SQLite lock, allowing a later expired-lease reclaim to start the same reserved callback run twice.
- User impact: a one-shot callback can advance one conversation more than once, violating its visible automation intent.
- Constraints: preserve the public callback tool/API shape and the reserved run ID; do not claim exactly-once external effects across process crash.

## Scope

- In scope: SQLite pooled-connection setup, token-verified claim/reclaim reads, bounded claim retry, and heartbeat behavior through the last confirmed lease deadline.
- Out of scope: cron behavior, callback tool schemas, distributed coordination, and external provider exactly-once guarantees after process crash.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: None; durable semantics are internal and existing user-visible events remain unchanged.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, and system logs plus indexes.

## Test Plan (TDD)

- New failing tests to add first: two competing managers dispatch one callback once despite a transient heartbeat busy error; a store claim/reclaim cannot report ownership without the caller token.
- Existing tests to update: durable callback store and manager retry/lease tests.
- Regression tests required: repeated busy errors only cancel at the last confirmed deadline and permit later safe takeover; pooled SQLite connections retain WAL/busy configuration.

## Cross-Surface Impact Map

- Required artifact: `2026-08-03-issue-1106-callback-claim-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link the existing issue before implementation.
- [x] Record ownership/search evidence and the impact map.
- [x] Add deterministic failing regressions.
- [x] Implement the minimal ownership repair.
- [x] Run focused, race, and full regression gates.
- [x] Update logs/indexes.
- [ ] Open one closing PR after commit/push.

## Risks and Mitigations

- Risk: backing off a transient heartbeat error past the lease deadline would allow overlapping admission. Mitigation: retain the last confirmed deadline, stop only on a definitive `ok=false` or deadline expiry, and test takeover after expiry.
- Risk: SQLite driver configuration is connection-local. Mitigation: configure every physical connection before query use and test a pooled second connection.
- Rollback: revert this isolated internal state-machine change; callbacks revert to prior behavior without schema changes.
