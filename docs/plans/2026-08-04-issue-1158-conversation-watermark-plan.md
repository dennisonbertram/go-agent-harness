# Plan: Issue #1158 conversation history watermark foundation

## Context

- Governing GitHub issue: #1158.
- Problem: `GET /v1/conversations/{id}/messages` returns durable transcript
  content without the event identity through which that snapshot is complete.
  A selected-conversation client therefore cannot open conversation SSE at a
  coherent boundary; content-based suppression can also erase a legitimate
  later callback/cron response that happens to repeat historical text.
- User impact: a scheduled continuation can fire successfully in the harness
  yet be absent or duplicated in the TUI transcript, defeating the intended
  conversation-continuation behavior.
- Constraints: the message snapshot and watermark must be paired at the Runner
  ownership boundary under existing per-conversation/event synchronization;
  the wire field is additive; no schema migration or content-derived identity.

## Scope

- In scope: an immutable Runner conversation snapshot containing copied
  messages plus an exact event ID known to precede that snapshot; the additive
  `last_event_id` messages response field; TUI response decoding/carrying;
  tenant/auth and old-server compatibility tests.
- Out of scope: #1148's staged selected-conversation bridge/reducer, removal of
  its provisional content-keyed suppression, and bounded overlap dedupe. #1148
  consumes this foundation after #1158 merges and owns those client lifecycle
  changes. Cron/callback scheduling and native GUI behavior are unchanged.

## Documentation Contract

- Feature status: implemented and focused-verified; coordinated repository
  gate and promotion remain with root.
- Public docs affected: none; the response field is an internal additive wire
  contract consumed by the native TUI.
- Spec docs before code: this plan and its impact map.
- Implementation notes after code: all durable logs and indexes record the
  precise snapshot boundary, compatibility fallback, and #1148 dependency.

## Test Plan (TDD)

- First reds:
  - Runner: completed messages return the exact durable cursor paired with the
    snapshot; an in-flight later run cannot advance that cursor; overlapping
    run lifetimes and restart-after-history-mutation fall back to empty.
  - Server/API: `/messages` returns `last_event_id`, retains scope/tenant
    rejection before snapshot access, and never leaks a foreign watermark.
  - TUI: a new server decodes the cursor into `ConversationHistoryMsg`; an old
    server that omits it decodes successfully with an empty cursor.
- Existing tests: retain messages endpoint 404, auth matrix, conversation SSE
  replay, resume rendering, and message copy-isolation coverage.
- Regression: focused harness/server/TUI normal and race tests. Root coordinates
  the non-concurrent full `./scripts/test-regression.sh` gate.

## Cross-Surface Impact Map

- See `2026-08-04-issue-1158-conversation-watermark-impact-map.md`.

## Implementation Checklist

- [x] Read #1158 and map current message/event ownership.
- [x] Reconcile scope with unmerged #1148 client lifecycle work.
- [x] Write plan and cross-surface impact map before code.
- [x] Add Runner/API/TUI failing tests and retain exact red evidence.
- [x] Implement the minimal snapshot and additive decoding contract.
- [x] Run focused normal/race tests.
- [x] Update durable logs/indexes with verified evidence.
- [x] Complete the coordinated full regression; hand off for commit, PR,
  hosted checks, and merge.

## Risks and Mitigations

- Risk: returning the latest event independently of the messages can skip a
  just-emitted assistant event whose message snapshot is not yet published.
  Mitigation: store and read the pair under the existing conversation sequence
  plus conversation-event locks; any overlapping run lifetime or missing
  process-local pairing returns an empty safe cursor.
- Risk: a mixed-version/old server has no watermark. Mitigation: the omitted
  JSON field decodes to empty and #1148 must use safe full replay rather than
  inventing identity from content.
- Risk: mutable message slices escape the Runner. Mitigation: preserve the
  existing `copyMessages` boundary on every snapshot return.
- Risk: undo/rewind/compaction can durably mutate messages independently of the
  event store. Mitigation: never reconstruct a cursor after restart without a
  future durable snapshot/version marker.
- Rollback: revert the additive field and snapshot method. No schema/data
  migration or repair is required; clients fall back to empty-cursor replay.
