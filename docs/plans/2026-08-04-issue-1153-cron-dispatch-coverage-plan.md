# Plan: Issue #1153 cron dispatch polling coverage

## Context

- Governing GitHub issue: #1153.
- Problem: the cross-server durable cron idempotency poll/cancellation helper is
  uncovered, so the required coverage gate is nondeterministic and the lease
  contention behavior lacks direct higher-level evidence.
- User impact: duplicate cron delivery must either resume the existing reserved
  run or wait/cancel safely; it must never double-admit a run.
- Constraints: strict red-green TDD; no lease-semantic, API, UI, or tenant-scope
  change; one isolated test-focused slice.

## Scope

- In scope: scripted `CronRunStartStore` tests through `getOrStartCronRun`, and
  only a minimal polling seam if deterministic timing requires it.
- Out of scope: production cron lease redesign, changing poll defaults,
  disabling the coverage gate, or changing public endpoints.

## Documentation Contract

- Feature status: implemented only after deterministic tests and all gates pass.
- Public docs affected: none; no public contract changes.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering/observational/system and
  long-term logs plus their index.

## Test Plan (TDD)

- New failing tests first: a first foreign/unaccepted lease followed by a local
  acquisition must retain one reserved ID, make at least two acquire calls, and
  admit exactly one runner; a pre-seeded foreign lease plus cancelled context
  must return the typed unavailable error promptly with no admission/dispatch.
- Existing tests to update: only shared cron test fixtures if a scripted store
  wrapper is needed.
- Regression tests required: focused normal/race stress, server normal/race,
  and `./scripts/test-regression.sh` with zero uncovered functions.

## Cross-Surface Impact Map

See `2026-08-04-issue-1153-cron-dispatch-coverage-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue and current callers.
- [x] Record source-of-truth and impact map.
- [ ] Add and record deterministic red tests.
- [ ] Apply the smallest test seam or production wiring required.
- [ ] Run targeted and full regression evidence.
- [ ] Update durable logs and indexes.

## Risks and Mitigations

- Risk: a test-only shortcut could bypass the actual durable fencing branch.
- Mitigation: exercise `getOrStartCronRun` with the real Runner and a scripted
  `CronRunStartStore`; assert persistent run identity and no provider dispatch
  after cancellation.
