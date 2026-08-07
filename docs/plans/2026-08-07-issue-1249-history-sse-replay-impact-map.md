# Cross-Surface Impact Map: Issue #1249

## Task

- Task / issue: #1249, prevent duplicate historic transcript replay.
- Plan link: `2026-08-07-issue-1249-history-sse-replay-plan.md`.
- Owner: `internal/server/http_conversations.go` and `cmd/harnesscli/tui`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `GET /v1/conversations/{id}/messages`, selected-conversation
  `GET .../events`, and `Model.Update` on `ConversationHistoryMsg`/`SSEEventMsg`.
- Owners: `Runner.SubscribeConversationFrom` atomically registers the live
  subscriber and constructs replay; `handleConversationEvents` serializes SSE;
  the TUI bridge decodes it and Model renders history/events.
- Search evidence: `rg -n "ConversationHistoryMsg|SubscribeConversationFrom|LastEventID|conversation.*events" internal cmd`.
- Conclusion: a marker belongs at the server's existing subscription/replay
  seam; TUI owns buffering and transcript rendering. No second event journal
  or client-side content identity is introduced.

## Config, API, CLI, and Tools

- Config: None.
- API: additive, opt-in request header on the existing conversation SSE route
  and one synthetic `conversation.replay.completed` SSE event. Existing
  callers receive unchanged replay behavior.
- CLI: `harnesscli -tui` uses the opt-in flow only when loading a selected
  conversation snapshot.
- Tools: None; cron/callback use the existing conversation stream later.
- Errors: malformed/unsupported marker use falls back only for legacy callers;
  the new TUI handshake treats missing marker as an ordinary stream and avoids
  falsely claiming reconciliation.

## Persistence and Compatibility

- No schemas, migrations, or stored fields change.
- Empty snapshot cursors remain valid. The marker divides known historical
  replay from later stream delivery when no durable cursor is available.
- Mixed versions are compatible: the header is opt-in and old clients/server
  behavior remains available.

## Lifecycle, Security, and Reliability

- Subscription is registered before marker/headers so live events cannot fall
  into a GET-first gap. TUI buffers rather than drops events until history is
  rendered.
- Existing auth/scope/tenant checks remain before handler entry; no secrets.
- Reconnect retains the last actual event ID. Cancellation still owns channel
  cleanup through the existing bridge cancel function.

## Product and Integration Surfaces

- Server/runtime: conversation SSE only.
- TUI: resumed/selected transcript convergence and later scheduled output.
- GUI/native: None in this slice; no GUI claim.
- Provider/model/tool catalog: None.
- External automation: cron/callback remains a downstream future run; its
  distinct assistant event is a regression postcondition.

## Deployment and Operations

- No migration or flag. Server-first deploy is backward-compatible; matching
  TUI uses the additive opt-in handshake.
- Diagnostics: existing SSE event IDs/cursors remain the causal evidence.
- Rollback: single PR revert, with legacy stream behavior still supported.
- Runbooks: no operator procedure changes.

## Regression Tests

- First expected red: `go test ./cmd/harnesscli/tui -run '^TestResumedConversationEmptyCursorReplayBoundaryRendersHistoricAndFutureOnce$' -count=1`.
- Acceptance: server emits replay-complete only for opt-in subscription after
  historical replay; TUI buffers then renders historic once/future once.
- Edge/lifecycle: empty cursor, nonempty cursor, event during history GET,
  reconnect, and scheduled continuation semantics.
- Real path: unique-port 30x100 PTY selected-session path at exact PR head.
- Commands: focused normal/race then `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1249-cache ./scripts/test-regression.sh`.

## Documentation and Handoff

- No public specification until implementation/test coverage exists.
- Add engineering/observational/system logs and plans index after green code.
- PR records expected-red and exact full/PTY evidence; it closes #1249 only.
