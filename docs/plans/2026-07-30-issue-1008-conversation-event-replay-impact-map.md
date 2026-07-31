# Cross-Surface Impact Map: Issue #1008 conversation event replay

## Task

- Task / issue: durable completed-run conversation replay, GitHub `#1008`
- Plan link: `2026-07-30-issue-1008-conversation-event-replay-plan.md`
- Owner: Codex
- Status: implemented; merge and post-merge native acceptance pending

## Current Ownership, Callers, and Data Flow

- Entry points: `GET /v1/conversations/{id}/events`,
  `HarnessClient.conversationEvents`, `RunSession.streamConversation`, and
  Chat/Activity navigation in `AppShell`.
- Owning packages/types/functions and source of truth:
  `Runner.emit`/`eventJournal` own canonical run events;
  `store.Store.AppendEvent` owns durable event rows;
  `Runner.SubscribeConversation` owns replay/live subscription;
  `RunSession.transcript` owns rendered GUI state.
- Callers, consumers, events, and downstream data: user runs, cron runs,
  delayed-callback runs, job bridge events, SSE clients, TUI/API consumers, and
  the native macOS transcript.
- Similar abstractions searched: run-scoped `Subscribe`/`handleRunEvents`,
  SQLite `run_events`, in-memory store events, conversation message persistence,
  Swift run/conversation SSE streams, session-open and undo/rewind reload paths.
- Search commands/evidence:
  `rg -n 'SubscribeConversation|storeAppendEvent|AppendEvent|GetEvents'`;
  `rg -n 'conversationEvents|messages\\(conversation|openConversation' macapp`.
- Duplication/ownership conclusion: do not invent a parallel event store.
  Extend the existing run-event store with a conversation query, retain an
  in-process bounded journal only as the no-store fallback, and keep persisted
  messages as the GUI's transcript reconciliation source.

## Config, API, CLI, and Tools

- User-facing config added or changed: none.
- Defaults / fallbacks: SQLite replay when `HARNESS_RUN_DB` is configured;
  bounded process-local replay otherwise.
- Environment variables, config files, or saved settings touched:
  existing `HARNESS_RUN_DB` and `HARNESS_CONVERSATION_DB` behavior only.
- Endpoints, request fields, response fields, or server wiring affected:
  conversation SSE honors the complete opaque `Last-Event-ID`; additive
  response headers report full-resync or paged replay.
- CLI commands, tools, wire formats, or integrations affected: no command or
  tool schema change; origin event IDs and SSE frames remain compatible.
- Error states / validation changes: stale/unknown cursors explicitly request
  resync instead of being parsed as an unrelated run-local index.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: no SQLite
  table rewrite; existing `run_events.id` supplies durable global ordering and
  an additive conversation/tenant query joins through `runs`. An additive index
  may be introduced if the query plan needs it.
- Backward/forward compatibility and versioning: existing `<run-id>:<seq>` IDs
  remain on the wire and become opaque conversation cursors resolved by exact
  ID. Legacy clients continue to send the same header and dedupe the same ID.
- Partial rollout and mixed-version behavior: new clients tolerate old servers;
  new servers improve replay without requiring a client migration.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership:
  serialize conversation event persistence/fanout with subscription snapshot
  creation; cancellation remains idempotent; bounded replay pages reconnect.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  existing `runs:read` and conversation owner gate remain; durable queries add
  tenant and conversation filters for defense in depth. No secret content is
  added to events.
- Failure modes, recovery, idempotency, and data repair: store-query failures
  fall back to the bounded in-process journal; duplicate reconnect is harmless;
  stale cursor is explicit and GUI transcript reconciliation restores durable
  message content.

## Product and Integration Surfaces

- Server/runtime: runner event journal, run store, conversation SSE handler.
- TUI/web/macOS/other clients: TUI/run SSE unchanged; macOS Chat re-entry
  reconciles stored messages; other conversation SSE clients receive better
  replay.
- Provider/model/tool catalog and routing: None; search found no provider,
  model, or tool-schema ownership in this path.
- External systems and automation: cron and delayed callback run starters are
  consumers; their public contracts do not change.
- UX states, keyboard/focus/accessibility/motion: transcript contents change
  only by restoring missed completed turns; no keyboard, focus, accessibility,
  or motion changes.

## Deployment and Operations

- Deployment/migration order and feature flags: server and native app ship
  together; no flag or destructive migration.
- Logs, metrics, traces, alerts, and support diagnostics: additive structured
  logging or response metadata records stale/truncated replay; existing run IDs,
  conversation IDs, and event IDs remain correlation keys.
- Rollback triggers and recovery steps: rollback if race tests find ordering
  regressions or GUI creates duplicate turns; reverting code leaves existing DB
  rows compatible.
- Runbooks and operator docs: engineering/system logs capture the contract;
  no operator action required.

## Regression Tests

- Characterization and first expected red test: reconnect after a completed
  second run currently omits that run because only a live run is replayed.
- New acceptance tests required: completed-run replay, cross-run resume,
  restart replay, replay/live handoff, GUI re-entry reconciliation.
- Edge, negative, failure, lifecycle, and security tests: invalid/stale cursor,
  page boundary, no-store fallback, store failure, duplicate ID, cancellation,
  tenant and conversation isolation.
- Integration/e2e/real-path proof: harness API reconnect plus a native app test
  that leaves Chat, allows a scheduled run to complete, and returns to Chat
  before another event occurs.
- Cross-surface regressions to guard: per-run stream termination/resume,
  dual-stream Swift dedupe, Activity task status, callback and cron continuations.
- Exact targeted and full commands:
  `go test ./internal/store ./internal/harness ./internal/server -run
  'Conversation.*Event|EventJournal|LastEvent' -count=1`;
  `go test ./internal/harness ./internal/server -race -count=1`;
  `swift test --package-path macapp --filter Conversation`;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs before code: this plan and impact map.
- Implementation notes/logs/indexes after code: plans index, engineering log,
  observational log, system log, and this checklist.
- Training/onboarding/release notes: None; behavior is a correctness repair to
  an existing API and GUI promise, not a new user workflow.

## Warning Check

- Every surface is addressed. `None` entries include search-backed rationale.
