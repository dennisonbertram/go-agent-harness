# Cross-Surface Impact Map: #1148 idle TUI conversation stream

## Task

- Issue: #1148.
- Plan: `2026-08-04-issue-1148-idle-tui-stream.md`.
- Status: locally implemented and focused-test-covered; full regression and
  real PTY scheduled-continuation proof remain pending.

## Ownership and Flow

- Server source: existing `GET /v1/conversations/{id}/events` in `internal/server/http_conversations.go`, durable journal in `internal/harness/runner_event_journal.go`.
- TUI source: `cmd/harnesscli/tui/api.go` has run-only bridge helpers; `Model` starts them on `RunStartedMsg` and clears them at terminal `SSEDoneMsg`.
- Consumers: transcript reducer, selected conversation state, auth API key, reconnect cursor, terminal PTY.
- Search: `rg 'startSSEForRun|conversation.*events|SSEDoneMsg|sseCh' cmd/harnesscli/tui`.

## Impact

- Config/API/tools: no new server contract; reuse base URL/API key and `Last-Event-ID`.
- Persistence/lifecycle: client cursor only; cancel on switch/quit; server journal remains source of truth.
- Security: same authenticated request/tenant isolation as existing bridge.
- Product: TUI only; native app has its own existing conversation-stream client and requires regression protection, not code change.
- Reliability: reconnect bounded and events deduped across two stream sources.
- Operations/docs: TUI reconnect diagnostics and engineering/observational/system logs; no migration.

## Tests

- Red idle transcript event; URL/auth; reconnect/dedupe/switch; `go test ./cmd/harnesscli/tui -race`; full regression; real fake-model PTY cron then callback after #1147 merged.

## Rollout/Rollback

- Additive client consumer. Rollback cancels the idle subscription and restores active-run-only display; no data changes.
