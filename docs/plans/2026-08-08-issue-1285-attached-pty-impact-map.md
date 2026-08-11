# Cross-Surface Impact Map: Issue #1285 Attached PTY

## Task

- Task / issue: #1285.
- Plan link: `2026-08-08-issue-1285-attached-pty-plan.md`.
- Owner: acceptance tooling.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry point: a later #1279 scheduled scenario owns `scheduledlifecycle.Lifecycle`, calls `PTY()`, then invokes ptyrunner.
- Source of truth: `scheduledlifecycle.PTYAttachment` supplies the daemon URL and owned workspace/store identity; `freshFrameCollector` owns PTY bytes and immutable frame sealing.
- Consumers: future API/SSE + TUI scheduled evidence only; no production caller exists.
- Similar abstractions searched: all current `Run*` entry points in `internal/acceptance/ptyrunner/runner.go`; each starts a daemon. #1283 introduced the sole `PTYAttachment` at `internal/acceptance/scheduledlifecycle/lifecycle.go`.
- Conclusion: add one attachment-only ptyrunner entry point; do not duplicate lifecycle ownership.

## Config, API, CLI, and Tools

- User-facing config/defaults: None. The internal config accepts only CLI, artifact-root, timeout, and typed lifecycle attachment.
- Environment/API/CLI: no daemon environment writes or production API changes; launches existing `harnesscli -tui -base-url` through a 100x30 PTY.
- Validation: missing URL or non-absolute owned resource paths fail before dispatch; config has no daemon field, so it cannot spawn a second daemon.

## Persistence and Compatibility

- Writes only terminal/frame/identity artifacts below the passed artifact root; lifecycle retains its own temporary databases.
- Additive internal acceptance API; no schema, migration, cache, or mixed-version behavior.

## Lifecycle, Security, and Reliability

- Only lifecycle owns harnessd; attached runner starts/stops only its CLI PTY and leaves lifecycle teardown to the caller.
- No secrets/credentials; reject unsafe paths and persist typed identity plus digest evidence only.
- Missing/mismatched identity fails before PTY launch; an active action token
  rejects stale/double seals before historic terminal bytes are inspected;
  cleanup closes master before waiting for CLI; no listener/PID lookup occurs.

## Product and Integration Surfaces

- Server/runtime, provider/model/tool catalog, deployment: None; no product code change.
- TUI: test-only real rendered 100x30 frames through existing client.
- GUI/macOS: None; native GUI remains separately unproven under #1089.

## Deployment and Operations

- No deployment/configuration change. JSON identity and immutable frame/terminal artifacts support diagnosis; rollback reverts one acceptance-only PR.

## Regression Tests

- First red: malformed/missing attachment rejection and no PTY launch.
- Acceptance: an attached real PTY frame has 100x30 geometry and identity-linked conversation/run metadata supplied by caller.
- Security/containment: stale token A cannot seal historic bytes after B is
  active; traversal/absolute action names and root artifact directories fail
  before artifact creation.
- Negative/lifecycle: invalid attachment cannot start a daemon; context/process cleanup remains bounded.
- Commands: `go test ./internal/acceptance/ptyrunner -run Attached -count=1`, same with `-race`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: None by design. Update plan/log indexes after implementation.
- #1279 later consumes the attachment for cron/callback semantics and records API/SSE identity alongside these frames.
