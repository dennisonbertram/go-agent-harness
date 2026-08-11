# Issue #1093 — Deterministic conversation-cleaner shutdown

## Context

- Governing GitHub issue: #1093.
- Problem: `harnessd` cancelled the conversation-retention cleaner without owning or awaiting its exit. Under race load, the server could return or close persistence while that goroutine was still executing.
- User impact: shutdown and startup-failure cleanup can hang or race; retention must retain its existing behavior while becoming lifecycle-safe.
- Constraints: no timeout/sleep increase, no retention-policy change, no cron/callback/UI scope expansion.

## Scope

- In scope: a start-to-completion cleaner lifecycle acknowledgement, idempotent shutdown ownership, and deterministic normal/startup-failure regressions.
- Out of scope: persistence schema, retention scheduling policy, API/CLI/client behavior, and inventory semantics.

## Documentation Contract

- Feature status: implemented; promotion and hosted verification remain.
- Public docs affected: none; this is an internal lifecycle contract.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: long-term-thinking and engineering logs plus indexes.

## Test Plan (TDD)

- New failing tests first: a controlled cleaner proves normal signal shutdown cannot return before the cleaner acknowledges exit; startup failure also waits for that acknowledgement.
- Existing tests to update: cleaner fakes now return a completion channel; direct cleaner tests assert its returned channel closes on cancellation and is already closed when disabled.
- Regression tests required: issue commands under normal/race repetition, package normal/race, and `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-02-issue-1093-cleaner-shutdown-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue and current failure architecture.
- [x] Record ownership/caller search evidence and impact map.
- [x] Capture deterministic red lifecycle tests.
- [x] Return/own cleaner completion acknowledgement.
- [x] Await acknowledgement before closing the conversation store on every exit path.
- [x] Preserve startup-failure cleanup and retention behavior.
- [x] Update logs/indexes and run focused/full verification.

## Risks and Mitigations

- Risk: awaiting a cleaner that never observes cancellation could hang shutdown. Mitigation: production sweeps use the cancellation context; the test controls a cleaner's exit explicitly and proves the daemon waits rather than racing persistence close.
- Risk: double cleanup from normal shutdown plus deferred failure cleanup. Mitigation: an idempotent lifecycle owner performs cancel-and-await once.
