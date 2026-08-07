# Issue #1270 Cross-Surface Impact Map

## Task

- Task / issue: #1270 deterministic replay-boundary fixture repair.
- Plan: `2026-08-07-issue-1270-tui-replay-boundary-snapshot-plan.md`.
- Owner: `cmd/harnesscli/tui` test fixture.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestResumedConversationReplayBoundarySnapshotIncludesQueuedFuture`.
- Owner/source of truth: the local `httptest` SSE handler and `driveModel` in
  `cmd/harnesscli/tui/idle_conversation_stream_test.go` /
  `sse_bridge_resilience_test.go`.
- Data flow: fixture sends pre-marker replay → replay-completed snapshot →
  model reducer → fixture sends later live event → normal reducer render.
- Search evidence: `rg` located the focused test, `driveModel`, replay event,
  and replay-boundary header within `cmd/harnesscli/tui`.
- Conclusion: only fixture scheduling is owned here; runtime TUI code is not
  changed.

## Config, API, CLI, and Tools

- Config/defaults/env/API/CLI/tools: None. The fixture continues to use the
  existing replay-boundary request/response header and SSE wire contract.

## Persistence and Compatibility

- Schema/migration/cache/compatibility: None. No persisted runtime behavior
  changes.

## Lifecycle, Security, and Reliability

- Lifecycle: fixture blocks the server's live event until model-visible
  snapshot reduction, while request cancellation can bypass that gate so
  `httptest` cleanup never hangs after an earlier model assertion failure.
- Security/privacy/secrets: None; local test data only.
- Reliability: failure-stage diagnostic names whether the server opened,
  marker flushed, model released, or live event flushed; no timeout increase.

## Product and Integration Surfaces

- Server/runtime/API/TUI/macOS/provider/catalog/external automation: unchanged.
- TUI behavior exercised: existing replay snapshot and post-boundary live
  assistant reducer assertions remain mandatory.
- UX/accessibility: None; test timing only.

## Deployment and Operations

- Deployment/flags/rollback: no runtime deployment. Revert the isolated
  test-only commit if it violates fixture intent.
- Observability: focused failure logs retain causal fixture stage.
- Runbooks: unchanged; required regression gate remains authoritative.

## Regression Tests

- First expected red: gate live event after replay marker without release;
  focused test times out at unchanged six seconds.
- Green acceptance: model-visible snapshot closes the release gate; historic
  and queued entries remain exactly once and live event remains visible.
- Commands: focused normal/race repeats; `go test -race ./cmd/harnesscli/tui`;
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public/spec docs: None, because product behavior is unchanged.
- Durable records: plan, impact map, engineering/long-term logs, and their
  indexes are updated in this PR.
- Release/training: no release-note change required.
