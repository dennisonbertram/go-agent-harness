# Plan: Issue #1006 durable callback retry, idempotency, and run linkage

## Context

- Governing GitHub issue: #1006, depends on merged #1005 (`72fef8ab`).
- Problem: #1005 correctly persists scheduled callbacks, but its `fired`
  transition precedes a best-effort, error-only `RunStarter`. A failed start is
  therefore recorded as fired without a run link, retry state, or safe recovery.
- User impact: a requested one-shot follow-up can disappear instead of visibly
  continuing the same scoped conversation.
- Constraints: retain #1005's persist-before-ack and startup-ready recovery;
  retain #1004 cron semantics; do not add native controls (#1007) or provider
  retry policy. No raw provider error/secret may enter callback persistence.

## Scope

- In scope: durable callback dispatch states, atomic due-work claims and stale
  lease recovery, bounded retry/backoff, deterministic embedded-run identity,
  run/scope link persistence, cancellation winner semantics, lifecycle events,
  list/status serialization, tests, and operator docs.
- Out of scope: callback-create UI, #1007 native controls, cron overlap,
  multi-region coordination, and provider-specific retry classification.

## Documentation Contract

- Feature status: `implemented; independent-review repairs and final full gate verified locally`.
- Public docs affected: callback tool descriptions/status wording only after the
  runtime contract exists and is test-covered.
- Spec docs to update before code: this plan and linked impact map.
- Implementation notes to add after code: engineering/system/observational log
  entries, plan status, indexes, and an operator-facing lifecycle contract.

## Test Plan (TDD)

- First red: an overdue durable callback whose starter returns a retryable
  error must remain queryable as `retry_wait`, retain its stable run identity,
  and be dispatchable once without a duplicate run after its next due time.
- Additional reds: non-retryable and exhausted failure; duplicate workers;
  recovered expired lease; run ID/scope persistence; cancel before claim,
  while leased/dispatching, and after started; event/list serialization.
- Existing tests to update: #1005's `fired`-on-attempt assertions become
  `started` only after durable run admission.
- Required verification: focused normal and race/repetition, harness/server
  callback lifecycle coverage, then `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- `docs/plans/2026-08-03-issue-1006-callback-retry-impact-map.md`

## Implementation Checklist

- [x] Verify structured issue, dependency, exact origin/main base, and current architecture search evidence.
- [x] Document feature status, boundaries, first red, and impact map before code.
- [x] Obtain/reconcile dedicated design review at the embedded Runner durable-identity boundary.
- [x] Add and capture exact-base red tests.
- [x] Implement the smallest state-machine/store/start-boundary changes.
- [x] Prove normal/race lifecycle and full regression.
- [x] Update implementation logs/status/indexes for the local candidate.
- [x] Repair independent-review lifecycle replay, durable-list truthfulness,
  and safe-summary blockers with strict red-green coverage.
- [x] Rerun the full repository gate on the final reviewed candidate.
- [ ] Commit the reviewed slice after the full gate passes.

## Risks and Mitigations

- Risk: retry after a timeout can create a second embedded run. Mitigation:
  reserve one stable `run_` identity at the actual Runner admission boundary,
  not only in callback timer memory.
- Risk: stale owner or cancel races can silently discard work. Mitigation:
  conditional durable transitions with owner/lease fences and explicit winner
  tests.
- Risk: a transient model/start failure leaks sensitive details. Mitigation:
  classified bounded summaries only; no raw error persistence.

## Verification Outcome

- Focused callback/Runner lifecycle tests pass repeatedly under the race
  detector, and complete affected tools/harness/server/harnessd suites pass in
  normal and race modes.
- The first full race gate exposed a local-zone timestamp claim loop; migration
  now parses legacy driver timestamp forms and normalizes rows to UTC. The exact
  full script then passed at 85.5% total coverage with zero uncovered functions.
- Independent-review repairs pass focused and complete affected-package normal
  and race gates. The final retained-tmux repository script passed at 85.5%
  total coverage with zero uncovered functions after removing the superseded
  active-only store helper found by the first coverage gate. This result does
  not claim the live API/TUI/native conversation matrix in #1010.
