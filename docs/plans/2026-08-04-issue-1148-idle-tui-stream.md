# Plan: Render external scheduled continuations in an idle TUI

## Context

- Governing issue: #1148, child of #1000.
- Problem: the TUI owns only a per-run SSE bridge. On terminal cleanup it stops polling; a later cron/callback continuation is persisted by the server but never appears in the idle PTY transcript.
- User impact: a deployment watcher can finish successfully but look dead until the user sends another prompt.
- Constraints: strict TDD, one issue/PR, reuse server conversation SSE and existing bridge/parser; do not substitute task polling for assistant output.

## Scope

- In: selected-conversation SSE lifecycle, Last-Event-ID/reconnect/deduplication/switch cleanup, TUI tests and real PTY proof, logs/docs/indexes.
- Out: server event contract, callback admission (#1147), cron history API (#1149), generic notifications, and macOS UI (#1009).

## Test Plan

- First red: `TestIdleConversationStreamRendersExternalContinuation` opens a selected conversation with no active run, emits a later `assistant.message`, and asserts the transcript renders it.
- Additional: URL/auth header, reconnect cursor, dedupe with active run stream, switch/no-leak, and PTY cron/callback idle proof.

## Checklist

- [x] Issue #1148 and current source seam inspected.
- [x] Impact map created.
- [x] Capture red idle-stream test.
- [x] Add only the existing bridge/parser lifecycle needed.
- [x] Run targeted/race/full regression.
- [ ] Run real PTY cron/callback proof (the #1000 convergence matrix).
- [x] Obtain cheap independent review.
- [ ] Push PR closing #1148.

## Risks

- Duplicate rendering from active-run plus conversation stream: event identity dedupe.
- Event leakage after a session switch: cancel old bridge before accepting new events.
- Reconnect loss: preserve conversation-scoped Last-Event-ID separately from run cursor.

## Implementation Evidence (Local; Not PTY or Production Proof)

- The named red command failed before the fix because no selected-conversation
  stream was started: `go test ./cmd/harnesscli/tui -run
  TestIdleConversationStreamRendersExternalContinuation -count=1` timed out
  waiting for the scheduled marker.
- The bridge now has an explicit conversation mode that retains run terminal
  frames, while the model owns a separate selected-conversation channel,
  cancellation function, replay cursor, reconnect count, and shared durable
  event-ID dedupe set. Switching, `/new`, and idle quit cancel the old stream.
- Focused normal/race tests cover auth and idle rendering, Last-Event-ID
  reconnect, run/conversation overlap dedupe, and switch cancellation. The
  final `./scripts/test-regression.sh` rerun passed normal, race, and coverage
  at 85.5% with zero uncovered functions. Real PTY cron/callback proof remains
  the #1000 convergence requirement; it is deliberately not represented as
  local regression evidence.
