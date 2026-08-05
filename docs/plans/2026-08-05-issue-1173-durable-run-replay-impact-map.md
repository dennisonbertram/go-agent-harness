# Cross-Surface Impact Map: Issue #1173

## Task

- Task / issue: #1173 durable run replay.
- Plan link: `2026-08-05-issue-1173-durable-run-replay-plan.md`.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `handleRunByID`, `handleDurableRunReplay`, and TUI `replayRunCmd`.
- Source of truth: runner memory first, then `store.Store`; `StartRun` persists the distinct replay.
- Consumers: TUI `RunStartedMsg` reuses existing SSE/transcript lifecycle.

## Config, API, CLI, and Tools

- New API: `POST /v1/runs/{id}/replay`, 202 `{run_id,status,replayed_from,conversation_id}`.
- Existing `POST /v1/runs/replay` remains rollout-path simulation; path-shaped TUI input is unchanged.
- Errors: `run_not_found`, `run_not_terminal`; scope and tenant behavior is unchanged.

## Persistence and Compatibility

- No schema or migration. Source lookup is memory then durable store; replay is a normal newly persisted run in the same conversation.

## Lifecycle, Security, and Reliability

- Terminal source only; source is immutable. Existing `runs:write` and opaque cross-tenant 404 gate run before replay.
- Effective model/provider are retained; fallback-list policy is intentionally not invented.

## Product and Integration Surfaces

- Server and TUI changed. GUI: none, unavailable in this task. Provider routing is carried only through the source effective provider.
- External systems, deployment, and automation: none.

## Deployment and Operations

- Rollback: revert the route and TUI bare-ID classifier. Existing rollout simulation remains independently usable.
- Diagnostics: returned `replayed_from`, run-store rows, SSE lifecycle events.

## Regression Tests

- Focused server source/terminal tests and TUI bare-ID/path tests; race and full regression required.

## Documentation and Handoff

- Plan, impact map, logs, and plan index updated. No public docs claim is made before complete verification.
