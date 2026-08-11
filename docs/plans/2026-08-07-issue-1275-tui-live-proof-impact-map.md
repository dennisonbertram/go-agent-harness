# Cross-Surface Impact Map: Issue #1275

## Task

- Issue: #1275 replay-boundary acceptance proof.
- Plan: `2026-08-07-issue-1275-tui-live-proof-plan.md`.
- Owner: `cmd/harnesscli/tui/idle_conversation_stream_test.go`.
- Status: test-only implementation in progress.

## Current Ownership, Callers, and Data Flow

- Entry: local `httptest` conversation SSE fixture feeds `driveModel`.
- State: `Transcript()` is durable reduced history; `View()` is auto-scrolled
  rendered output. `SSEEventMsg` ID `live:3` identifies the live assistant
  event.
- Evidence: `rg -n "ReplayBoundarySnapshot|live:3|driveModel|Transcript" cmd/harnesscli/tui`.

## Config, API, CLI, and Tools

- None. Test fixture retains the same local server protocol and 6-second bound.

## Persistence and Compatibility

- None. No runtime state/schema change.

## Lifecycle, Security, and Reliability

- Fixture still cancellation-unblocks its handler. Diagnostics report snapshot
  counts, decoded live events, and reduced live events on failure.
- No auth, secret, or permission impact.

## Product and Integration Surfaces

- API/SSE wire semantics, TUI production reducer, web/macOS, cron/callback and
  model/tool surfaces: unchanged. The test proves their existing conversation
  continuation contract more accurately.

## Deployment and Operations

- No rollout. CI becomes less viewport-layout-dependent without hiding bridge
  failures; rollback is reverting this isolated test/docs PR.

## Regression Tests

- Red: short viewport timed out at replay marker with old View predicate.
- New proof: exact-once snapshot transcript releases live; post-Update
  `live:3` decoded assistant event must render live content.
- Commands: focused normal/race x10, `go test -race ./cmd/harnesscli/tui`, full gate.

## Documentation and Handoff

- Plan/map, indexes, active plan, and durable logs updated. PR will `Closes #1275`.
