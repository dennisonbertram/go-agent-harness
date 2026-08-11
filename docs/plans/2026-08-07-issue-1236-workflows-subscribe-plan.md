# Plan: Close the plural workflow subscription history/live gap

## Context

- Governing GitHub issue: #1236.
- Problem: `internal/workflows.Engine.Subscribe` reads persisted history before
  registering its live channel, so an emit in that interval can be observed by
  neither source.
- User impact: workflow failure and progress observers can miss terminal state,
  breaking event-driven continuation.
- Constraints: preserve the event wire contract; add only the Store's
  read-only durable high-water primitive; do not hold the global engine lock
  over `GetEvents`, increase timeouts, or alter HTTP SSE terminal-history
  behavior (tracked separately in #1237).

## Scope

- In scope: port the singular engine's per-run sequence watermark plus
  initializing pending-buffer handshake to plural `workflows.Engine.Subscribe`,
  hydrate its durable per-run high-water before both Subscribe and emit, and
  cover exact-once, burst, restart, store-error, and cancellation regressions.
- Out of scope: HTTP history-terminal early return (#1237), event schema,
  Store implementations, workflow execution semantics, and clients.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: None; this is an internal reliability correction.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering log and folder indexes.

## Test Plan (TDD)

- New failing tests first: a controlled Store snapshots history, blocks before
  return, then permits an emit. The test asserts every event appears exactly
  once across returned history plus live channel.
- Existing tests to update: plural workflow engine subscription coverage only.
- Regression tests required: burst larger than the live channel while history
  is blocked, `GetEvents` error deregistration, cancellation cleanup/no leaked
  subscriber, fresh-engine persisted replay, and next durable sequence without
  collision.

## Cross-Surface Impact Map

See `2026-08-07-issue-1236-workflows-subscribe-impact-map.md`.

## Implementation Checklist

- [x] Define acceptance criteria in tests and verify #1236.
- [x] Record current singular/plural ownership and source-of-truth search.
- [x] Complete impact map before production code.
- [ ] Write and preserve deterministic red tests.
- [ ] Port the minimal watermark/pending-buffer handshake and durable high-water
  initialization coordination.
- [ ] Run focused normal, race, stress, and external-cache full regression.
- [ ] Update logs/indexes, PR evidence, and independent review.

## Risks and Mitigations

- Risk: history copied while the subscriber cannot drain a bounded live
  channel can lose a burst. Mitigation: buffer pending events under `e.mu`
  until history handoff finalizes.
- Risk: a Store error leaves a registered channel behind. Mitigation: remove
  the entry under the same mutex before returning the error.
- Risk: broad locking stalls unrelated workflow emits. Mitigation: lock only
  around registration/watermark and pending finalization, never `GetEvents`.
- Risk: a fresh engine starts its in-memory counter at zero and trims durable
  history or reuses a persisted sequence. Mitigation: one per-run initialization
  gate obtains `LastEventSeq` before either subscription registration or emit.
