# Cross-Surface Impact Map: Issue #1210

## Ownership and Data Flow

- Entry point: `internal/server.handleRunEvents` writes run-scoped SSE replay
  history and live events from `Runner.Subscribe`.
- Source of race: `Runner.transitionTerminal` journals the terminal event,
  persists the terminal run snapshot, then commits the in-memory status and
  fans out the terminal event. Replay can snapshot the journaled event while
  `UpdateRun` is blocked.
- Authority: the public `Runner.GetRun` read model backs `GET /v1/runs/{id}`;
  the server must not write a terminal SSE frame until that authority reports
  the terminal state represented by the frame.
- Search evidence: `rg -n "transitionTerminal|Subscribe\(|handleRunEvents|UpdateRun" internal/harness internal/server`.

## Affected Surfaces

- HTTP SSE API: direct production fix; terminal replay and live delivery gain
  a matching-status settlement boundary.
- Runner state/event journal: direct support for a context-cancellable
  terminal-status wait; terminal persistence/fanout order is retained.
- `Last-Event-ID`: direct regression coverage; slicing happens before the
  terminal gate and retains exact-once terminal replay.
- TUI/native/API acceptance consumers: improved shared contract only; no
  client behavior or retry added.
- Persistence: existing `UpdateRun` remains the terminal durable commit;
  blocked-store test proves the causal window without schema/migration change.
- Auth/config/CLI/providers/tools/cron/callback/deployment: unaffected. The
  route, scopes, payloads, and execution behavior are unchanged.

## Reliability, Security, and Compatibility

- A cancelled SSE request abandons the settlement wait through its context.
- A terminal status mismatch is not emitted; it remains protected by the same
  request cancellation boundary rather than fabricating a terminal state.
- Existing replay order stays intact because only the terminal write is gated.
- Rollback is a source revert with no retained data conversion.

## Regression Evidence Plan

- Deterministic server table: completed/failed/cancelled terminal `UpdateRun`
  barrier, blocked terminal SSE, released exact-one matching frame, and final
  matching GET state.
- `Last-Event-ID` case ensures prior `run.started` is omitted while the single
  matching terminal remains.
- Commands: focused `go test ./internal/server -run '^TestTerminalSSE...'`,
  same with `-race`, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.
