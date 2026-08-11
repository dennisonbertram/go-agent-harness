# Plan: Issue #1249 history/SSE replay reconciliation

## Context

- Governing GitHub issue: #1249.
- Problem: a resumed TUI renders a durable message snapshot before opening an
  empty-cursor conversation SSE stream. An empty snapshot cursor is a valid
  compatibility fallback, but that stream then replays the same historical
  assistant event and produces a duplicate transcript bubble.
- User impact: a user can see the previous assistant response twice, while a
  later cron/callback continuation must still appear in the selected
  conversation without a new prompt.
- Constraints: preserve empty-cursor replay for old/restarted stores; no
  content dedupe, assistant-finalized shortcut, disabling replay, schema
  migration, or GUI claim. Keep the existing HTTP/SSE contract for legacy
  callers.

## Scope

- In scope: an opt-in conversation SSE replay-boundary protocol, TUI
  snapshot-to-stream ordering/buffering, and focused server/TUI regressions.
- Out of scope: persistent message-version redesign, session-picker routing
  (#1246), normal run SSE, GUI/native evidence, tool behavior, and scheduled
  job execution.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: None; this is an additive internal client/server
  handshake, not a new operator-facing route.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering, observational, and
  system logs plus the plans index.

## Test Plan (TDD)

- New failing test first: selected-conversation snapshot has an empty cursor,
  opt-in SSE replays a historic assistant event before its replay-complete
  marker, then emits a distinct future assistant event. The historic message
  renders once and the future message once.
- Existing tests to update: conversation SSE HTTP tests and TUI idle-stream
  lifecycle tests as required by the additive marker.
- Regression tests required: a race during snapshot fetch after the marker,
  a nonempty cursor reconciliation control, reconnect cursor preservation,
  and scheduled continuation visibility.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1249-history-sse-replay-impact-map.md`.

## Implementation Checklist

- [x] Verify #1249 issue contract and exact base SHA `e6828af7`.
- [x] Record current owners/callers and protocol alternatives.
- [x] Create plan and impact map before code.
- [x] Add and run the expected-red TUI regression.
- [x] Add the minimal opt-in server replay marker.
- [x] Add TUI buffer/reconcile behavior and compatibility tests.
- [x] Run focused normal/race tests and pre-commit real 30x100 PTY acceptance.
- [x] Update logs/indexes and run full regression (`./scripts/test-regression.sh`:
  normal, race, coverage, and coverage gate passed; total coverage 85.1%).
- [ ] Commit, rerun the real 30x100 PTY acceptance at the committed SHA, push,
  and open a reviewable PR.

## Review Repair (2026-08-07)

- Replace the marker-plus-GET handoff with an atomic marker snapshot captured
  with Runner subscription/replay. Pre-marker events are suppressed; marker
  messages render once; post-marker events use the normal reducer.
- Add deterministic A–F ordering/legacy regressions. Full TUI normal/race and
  `./scripts/test-regression.sh` pass; publication remains pending the amended
  commit and committed-SHA PTY rerun.

## Risks and Mitigations

- Risk: a GET-first history fetch leaves a snapshot-to-live gap. Mitigation:
  establish the opt-in SSE subscription and replay-complete boundary first.
- Risk: content equality suppresses a legitimate later reply. Mitigation:
  reconcile only with event order and the durable cursor; never compare text.
- Risk: a future callback/cron message is discarded with historical replay.
  Mitigation: buffer post-marker live events and render events after the
  snapshot cursor (or all post-marker events when the cursor is safely empty).
- Rollback: revert this one additive client/server handshake; old SSE clients
  retain the pre-existing route behavior.
