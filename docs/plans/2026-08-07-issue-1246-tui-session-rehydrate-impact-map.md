# Cross-Surface Impact Map: #1246 selected-session rehydration

## Ownership and Data Flow

- Entry: `/sessions` opens `sessionpicker`; Enter currently reaches global
  `keys.Submit` before generic overlay routing.
- Owner: `cmd/harnesscli/tui/model.go` translates component messages into
  `SessionPickerSelectedMsg`; that message clears state, starts atomic
  conversation replay, and invokes history GET only through the existing
  unsupported-server fallback.
- Consumers: viewport, transcript export, `/search`, and subsequent prompt
  routing via `conversationID`.
- Search evidence: `rg 'SessionPickerSelectedMsg|sessionPicker|Submit|ConversationHistoryMsg|fetchConversationMessagesCmd' cmd/harnesscli/tui`.
- Decision: route the preempted Enter through the existing picker and model
  message; create no parallel fetch, replay, or persistence abstraction.

## Config, API, CLI, Tools

- Config: None — existing base URL/API-key transport is retained.
- API: None — reuses existing atomic conversation SSE and its legacy GET
  fallback.
- CLI: `/sessions` Enter now reaches the established selected-session flow.
- Tools/providers/catalogs: None — searched TUI routing is self-contained.

## Persistence, Lifecycle, Security, Compatibility

- Persistence: None — existing durable messages are read only.
- Lifecycle/concurrency: the supported boundary snapshot is authoritative;
  legacy history replies remain keyed to active conversation and an empty
  legacy cursor stays snapshot-only to prevent duplicate replay.
- Security: existing authenticated request helper remains the only request path.
- Compatibility: no schema/wire change; older clients retain old behavior,
  while the updated client reads the endpoint already present on merged main.

## Product, Deployment, and Operations

- TUI: affected selection/replay/search/continued-message journey.
- Web/macOS/native GUI: None — issue explicitly excludes them.
- Deployment: ordinary binary rollout, no server coordination/migration.
- Observability: visible loading/failure state plus exact 30x100 PTY artifact.
- Rollback: revert a local routing change, with no durable state to unwind.

## Verification and Documentation

- Regression: deterministic keyboard-selection HTTP/render/search test; stale
  history isolation; focused normal/race; full repository gate; real PTY.
- Docs: plan/index before code; engineering, observation, and system logs only
  after implementation and verification evidence exist.
