# Cross-Surface Impact Map: Issue #1183

## Task

- Task / issue: #1183 replay command SSE fixture.
- Plan link: `2026-08-05-issue-1183-replay-sse-fixture-plan.md`.
- Owner: `cmd/harnesscli/tui` test suite.
- Status: implemented locally; canonical regression, review, and CI pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `TestRunControl_ReplayCommandCallsReplayEndpoint` drives `executeReplayCommand`, `replayRunCmd`, and `Model.Update(RunStartedMsg)`.
- Source of truth: `Model.Update(RunStartedMsg)` intentionally calls `startSSEForRun`; `SSEBridgeOptions` sends `Accept: text/event-stream` and polls the returned run until terminal.
- Consumers: the Bubble Tea lifecycle and package-local `httptest` fixture only.
- Search evidence: `rg -n "replayRunCmd|RunStartedMsg|startSSEForRun|ReplayRollout" cmd/harnesscli/tui`.
- Conclusion: only the fixture omitted the post-replay stream contract; production subscription is correct.

## Config, API, CLI, and Tools

- User config/API/CLI/tool changes: none.
- Existing paths covered: durable `POST /v1/runs/run_*/replay` then returned `GET /v1/runs/<new>/events`; rollout simulation remains `POST /v1/runs/replay` only.
- Errors/validation: fixture rejects unexpected method/path/header deterministically.

## Persistence and Compatibility

- Schemas, migration, cache, and compatibility: none; all payloads remain existing wire shapes.

## Lifecycle, Security, and Reliability

- Lifecycle: fixture synchronizes the exact stream request, emits a terminal `run.completed` frame, and closes the HTTP handler rather than relying on cancellation or sleep.
- Security: validate existing `Accept: text/event-stream`; no credentials or auth behavior changes.
- Reliability: no test-specific production seam, timeout extension, or cancellation interception.

## Product and Integration Surfaces

- Server/runtime: no production server modification; `httptest` mirrors its existing replay/SSE contract.
- TUI: test now drives visible returned-run lifecycle through terminal completion.
- GUI/macOS/providers/tools/cron/callback/external systems: none; source search shows no touched ownership.

## Deployment and Operations

- Deployment/flags/metrics/runbooks: none.
- Diagnostics: unexpected fixture requests remain explicit test failures.
- Rollback: revert test/docs only.

## Regression Tests

- Characterization/red: #1183 hosted race reported `unexpected request: GET /v1/runs/run_replayed_1/events`.
- Acceptance: exact durable POST, returned stream GET/Accept, terminal close, inactive terminal state, and zero simulation events requests.
- Exact commands: focused `go test ./cmd/harnesscli/tui -run '^TestRunControl_Replay' -count=20`; same with `-race`; complete normal/race TUI package; canonical-temp `TMPDIR=/private/tmp ./scripts/test-regression.sh` serially.

## Documentation and Handoff

- Public docs: none.
- Internal docs: plan/index plus engineering, observational, system, and long-term logs.
- PR contract: one fixture/doc-only PR with `Closes #1183` and recorded red/green evidence.
