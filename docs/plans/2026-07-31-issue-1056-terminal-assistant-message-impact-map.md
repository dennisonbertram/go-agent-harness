# Issue #1056 Terminal Assistant Message Impact Map

## Task

- Issue: #1056
- PR: #1059
- Plan: `2026-07-31-issue-1056-terminal-assistant-message-plan.md`
- Status: current-main integration candidate; unmerged.

## Ownership and Data Flow

- Entry: run SSE decoded by `StartSSEBridgeWithOptions`.
- Owner: `cmd/harnesscli/tui.Model.Update` owns visible and transcript
  reduction; `lastAssistantText` is the existing response source of truth.
- Flow: SSE -> `SSEEventMsg` -> assistant bubble -> `SSEDoneMsg` -> transcript
  export entry.
- Search evidence: `rg -n "assistant\\.message(\\.delta)?|run\\.completed" cmd internal macapp scripts`.
- Conclusion: repair the existing reducer; no parallel state owner is needed.

## Config, API, CLI, and Tools

- Config/env/defaults: None; searched TUI and fake-provider configuration.
- API/wire formats: unchanged; consume existing `{content}` on
  `assistant.message`.
- Commands/tools: no flag, slash command, or tool change.
- Validation: malformed/empty terminal payloads remain no-ops.

## Persistence and Compatibility

- Schema/migration/cache: None; HTTP and SQLite transcripts already contain the
  correct assistant content.
- Compatibility: streamed delta providers remain supported; final-only
  providers become visible. Older clients continue to miss this event without
  server coordination.

## Lifecycle, Reliability, and Security

- Lifecycle: preserve polling, Last-Event-ID reconnect, run-active cleanup,
  tool-card offsets, later-run reset, and copy access to the last response.
- Idempotency: identical delta/final and replay are no-ops; a differing final is
  authoritative; repeated completion appends one transcript row.
- Security/auth/privacy: None; only already-authorized response content is
  reduced and no new logging contains prompts or credentials.

## Product Surfaces

- TUI: affected.
- Server, provider, web, macOS, persistence, provider/model/tool catalog: None
  after symbol/caller search; they are compatibility references only.
- UX: the missing assistant bubble appears and the composer remains usable for
  later turns; no keyboard/focus/motion change.

## Deployment and Operations

- Release: ordinary `harnesscli` binary; no migration or feature flag.
- Evidence: pane capture plus raw SSE, run JSON, HTTP/SQLite transcript, and
  daemon logs.
- Rollback trigger: any duplicate, missing, or reordered streamed response or
  tool card. Revert the reducer slice; no data repair.

## Tests

- Red-first: two final-only turns currently produce only user transcript rows.
- New behavior: exact user/assistant/user/assistant viewport and transcript,
  final-only, delta/final, authoritative replacement, mixed tool lifecycle,
  replay, repeated completion, and later-run reset.
- Commands:
  - `go test ./cmd/harnesscli/tui -run 'TestRegression_FinalOnlyAssistantMessagesPreserveTwoTurnConversation|TestSSEEventMsg_AssistantMessage' -count=1`
  - `go test ./cmd/harnesscli/tui -count=1`
  - `go test -race ./cmd/harnesscli/tui -count=1`
  - `go test ./cmd/harnesscli/... -count=1`
  - `./scripts/test-regression.sh`
- Real path: two turns through a tmux-hosted `harnessd` and
  `harnesscli -tui`, correlated with run IDs, SSE, API, and SQLite.

## Documentation

- Update engineering, observational, system, and long-term-thinking logs plus
  plans index and active plan.
- Public docs/runbooks: None; this repairs an existing event contract.
