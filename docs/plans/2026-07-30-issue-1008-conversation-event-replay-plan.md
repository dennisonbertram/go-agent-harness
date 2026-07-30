# Plan: Durable completed-run conversation event replay

## Context

- Governing GitHub issue: `#1008`
- Problem: `GET /v1/conversations/{id}/events` only replays the newest live
  run, and its `Last-Event-ID` logic treats a run-local sequence as though it
  were a conversation-wide cursor. Completed callback/cron runs can therefore
  be absent after reconnect. The macOS Chat view also does not reconcile the
  persisted transcript when it reappears.
- User impact: a monitor can advance the durable conversation while the GUI is
  on Activity, but the completed assistant turn can be missing or disappear
  after returning to Chat even though `GET .../messages` contains it.
- Constraints: preserve existing per-run event IDs and run-stream behavior;
  preserve tenant isolation; keep replay-to-live handoff gap-free; work with
  both SQLite-backed native-app sessions and the no-run-store test/runtime
  fallback; strict TDD and full regression gate before merge.

## Scope

- In scope:
  - Conversation-wide replay across every completed and live run.
  - Exact opaque `Last-Event-ID` resolution across run boundaries.
  - Durable SQLite and in-memory run-store conversation-event queries.
  - A bounded in-process conversation journal fallback.
  - Atomic replay-to-live subscription ordering and persist-before-fanout.
  - Explicit replay-resync/truncation response metadata.
  - macOS Chat re-entry reconciliation from persisted messages.
  - Go, Swift, race, regression, API, and native GUI acceptance coverage.
- Out of scope:
  - Changing the run-scoped SSE cursor format.
  - Combining cron and callback public tools.
  - Adding a new external event broker or cross-host stream.
  - Reworking transcript rendering unrelated to scheduled continuation replay.

## Documentation Contract

- Feature status: `implemented; merge and post-merge native acceptance pending`
- Public docs affected: conversation SSE resume semantics in the API/runtime
  documentation if an existing contract page is found during implementation.
- Spec docs to update before code: this plan and its linked impact map.
- Implementation notes to add after code: engineering, observational, and
  system logs with the replay contract, the GUI reproduction, and verification.

## Test Plan (TDD)

- New failing tests to add first:
  - A completed second run is replayed after the subscriber disconnected.
  - Resume from a full event ID in run A returns only later events from run B.
  - Replay is available after a runner restart with the same SQLite store.
  - Replay/live handoff has neither a missing event nor a duplicate.
  - In-memory fallback retains completed-run history.
  - Chat re-entry fetches persisted scheduled messages.
- Existing tests to update:
  - Store contract tests for conversation-scoped event ordering and tenant
    filtering.
  - Event-journal ordering tests so all events persist before subscriber
    delivery, not only terminal events.
- Regression tests required:
  - Duplicate `Last-Event-ID`, invalid/stale ID, bounded replay continuation,
    cross-conversation and cross-tenant isolation, and Swift stream dedupe.

## Cross-Surface Impact Map

- `docs/plans/2026-07-30-issue-1008-conversation-event-replay-impact-map.md`

## Implementation Checklist

- [x] Define acceptance criteria in tests.
- [x] Link a contract-complete structured GitHub issue before implementation.
- [x] Record current architecture, callers, consumers, and source-of-truth search evidence.
- [x] Document feature status and exact contract before code.
- [x] Complete and reconcile the cross-surface impact map before implementation.
- [x] Add characterization coverage before structural refactors.
- [x] Write failing tests first.
- [x] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [x] Implement minimal code changes.
- [x] Refactor while tests remain green.
- [x] Update docs, status ledgers, and indexes.
- [x] Update engineering/system/observational logs as needed.
- [x] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: replay overlaps live delivery and duplicates an event, or leaves a gap.
- Mitigation: serialize conversation-event persistence/fanout with subscription
  registration and replay snapshot creation; test the boundary concurrently.
- Risk: a new conversation cursor breaks per-run/conversation stream dedupe in
  the macOS app.
- Mitigation: keep origin event IDs as opaque SSE IDs and resolve the complete
  ID through the durable store rather than exposing a second wire identity.
- Risk: unbounded history or a stale cursor causes excessive replay.
- Mitigation: bounded pages, explicit resync/truncation metadata, and reconnect
  continuation using the final delivered event ID.
- Risk: replacing a live transcript races an active user run.
- Mitigation: only reconcile persisted messages on Chat appearance when the
  current user-started run is not active.
