# Plan: Issue #1135 deterministic cron reconciliation fixture

## Context

- Governing issue: #1135.
- Problem: two recovery tests observed a terminal fixture signal and immediately
  attempted same-scope admission. That signal was not a causal proof that the
  asynchronous scheduler had returned from terminal persistence and released
  its recovered scope lease, so hosted race load could legitimately see denial.
- User impact: cron restart evidence must prove both no-overlap before durable
  terminal persistence and precise eventual re-admission afterwards.
- Constraints: tests and internal documentation only; preserve scheduler,
  persistence, remote/embedded, API, TUI, and native behavior.

## Scope

- In scope: gate terminal `UpdateExecution` in post-bind and remote async
  recovery fixtures; prove denial before its return; release it; join
  `reconcileWG`; prove exact scope/lease cleanup and then admit.
- Out of scope: production scheduler logic, timing, no-overlap rules, remote
  transport, schemas, and client behavior.

## Documentation Contract

- Feature status: implemented and locally verified; independent review, hosted
  checks, and promotion remain.
- Public docs: none; no public contract changes.
- Records: this plan, impact map, durable logs, indexes, and issue/PR evidence.

## Test Plan (TDD)

- Existing expected red: hosted full race reported a duplicate admission still
  denied after the old fixture's terminal notification.
- Updated tests: `TestScheduler_PostBindReconciliationFinalizesRestartedExecution`
  and `TestScheduler_StartRecoversRemoteRunAsynchronouslyRegression`.
- Acceptance: no admission before terminal store return; scheduler reconciliation
  join clears the exact recovered scope/lease; subsequent admission succeeds.
- Verification: focused normal/race stress, complete cron normal/race, and
  `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-03-issue-1135-cron-fixture-impact-map.md`.

## Implementation Checklist

- [x] Verify issue, exact current main, and scheduler ownership boundary.
- [x] Write plan and impact map before fixture changes.
- [x] Replace racy terminal notifications with explicit store/reconciliation gates.
- [x] Run focused normal/race x100, complete cron normal/race, and full
  repository normal/race/coverage gates (85.5%, zero uncovered functions).
- [ ] Update issue evidence, obtain cheap independent review, and publish PR.

## Risks and Mitigations

- Risk: test-only synchronization hides a release-order bug. Mitigation: retain
  explicit pre-return denial and inspect `activeScopes` plus `reconciledLeases`
  only after the scheduler-owned WaitGroup completes.
