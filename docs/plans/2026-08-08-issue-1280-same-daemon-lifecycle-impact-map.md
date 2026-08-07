# Cross-Surface Impact Map: Issue #1280 shared same-daemon lifecycle

## Task

- Task / issue: #1280; status: implemented pending regression/review.
- Plan: `2026-08-08-issue-1280-same-daemon-lifecycle-plan.md`.

## Current Ownership, Callers, and Data Flow

- Entry point: future #1279 driver calls `scheduledlifecycle.Start`.
- Source: `ptyrunner` starts a private fake daemon; `apisserunner` consumes tool inventories. New package composes neither and leaves them unchanged.
- Flow: `Start` returns one `PublicURL`; API/SSE and `PTYAttachment.BaseURL` derive from it; workspace/stores/provenance remain in `ArtifactRoot`.
- Search evidence: `rg` found ptyrunner's hard-coded fake daemon, harnessd's validated `HARNESS_LISTEN_FD`, `/healthz`, and conversation SSE routes.

## Config, API, CLI, and Tools

- No public config/API/CLI changes. Internal config supplies command, roots, revision, address, and explicit environment.
- Private environment sets `HARNESS_ADDR`, `HARNESS_LISTEN_FD`, workspace/run/conversation paths, and `CRONSD_DB_PATH`.
- Existing server routes only: `/healthz` and `/v1/conversations/{id}/events`.

## Persistence and Compatibility

- All disposable workspace, cron/callback, run, and conversation paths are contained under caller `ArtifactRoot`.
- No schema, migration, public protocol, or compatibility change.

## Lifecycle, Security, and Reliability

- Parent reserves an exact TCP listener and transfers descriptor 3 to one child process group; readiness polls `/healthz`.
- Source mismatch and bind collision fail before command spawn. Close addresses only the recorded child process group, never a port lookup.
- Close first nonblocking-checks the broadcast reap signal before any signal; after SIGKILL it waits boundedly for reaping and leaves the log open if the child cannot be confirmed exited.
- No credentials or production data are created; source SHA/config digest are retained in owned provenance.

## Product and Integration Surfaces

- Server/runtime/TUI/web/macOS/provider/tools: no product changes. Later #1279 may consume the PTY attachment.
- UX/accessibility: None.

## Deployment and Operations

- No deployment. `daemon.log` and `provenance.json` are contained diagnostics. Rollback is removal of internal acceptance tooling.

## Regression Tests

- First red: undefined `Start`/`Config` in focused package command.
- Acceptance: helper daemon proves public health/SSE and PTY base URL equality.
- Negative: mismatch launches nothing; unrelated occupied listener remains healthy.
- Cleanup: a pre-reaped child receives no post-reap process-group signal; a SIGTERM-ignoring child is reaped after escalation before Close returns.
- Commands: focused normal/race package tests, then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update plan, map, active plan, engineering/long-term logs, and indexes.
- #1010 remains pending until API/TUI/native full-conversation matrices pass on merged main.
