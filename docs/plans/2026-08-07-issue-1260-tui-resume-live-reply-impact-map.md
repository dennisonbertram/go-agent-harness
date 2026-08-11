# Impact Map: Issue #1260

- Ownership/callers: `bridge.go` decodes terminal SSE envelopes and
  `model.go` owns selected-conversation plus run-stream reconciliation;
  `api.go` only tags the selected-conversation stream. Search evidence:
  `rg 'SSEDoneMsg|seenSSEEvent|assistantTranscriptFinalized' cmd/harnesscli/tui`.
- Data flow/lifecycle: harness event envelope `run_id` now reaches
  `SSEDoneMsg`; terminal ownership and pre-start assistant text are keyed by
  that already-existing server identity. Conversation replay and Last-Event-ID
  cursor semantics are unchanged.
- API/CLI/config: None. HTTP routes, payloads, slash commands, config, and
  command invocation are unchanged; only an in-process TUI message gains a
  field already present on the SSE envelope.
- Persistence/schema: None. No store writes, schema, migration, or durable
  data contract changes.
- Security/tenancy: None. Run identity remains server supplied through the
  authenticated existing SSE endpoint; no authorization decision changes.
- Product clients: TUI affected: resumed visible transcript correctness.
  Native macOS GUI and web GUI: none; they do not consume this Bubble Tea
  reducer (separate native proof remains required by the parent epic).
- Provider/model/tool catalog/deployment: None; no tool implementation,
  provider selection, model flow, catalog, environment, or deployment change.
- Compatibility: ownerless `bridge.closed`/`bridge.fatal` retain existing
  behavior; only real terminal events have a `RunID` and stale terminals are
  ignored.
- Tests/docs/rollout: deterministic reducer tests cover required orderings;
  normal/race/full gates plus PTY/API evidence are required. Rollback is one
  isolated revert, with no state repair.
