# Cross-Surface Impact Map: #1237 workflow terminal-history SSE completion

## Task

- Task / issue: #1237
- Plan link: `2026-08-07-issue-1237-workflow-sse-terminal-plan.md`
- Owner: workflow HTTP server
- Status: implemented and locally verified; PR/hosted review pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `Server.handleWorkflowRunByID`, `GET /v1/workflow-runs/{id}/events`.
- Source of truth: `workflowManager.Subscribe` returns persisted `workflows.Event`
  history, live channel, and cancellation callback.
- Consumers: API SSE clients; indirect TUI/GUI workflow observers consume the
  same HTTP stream.
- Search evidence: `rg` found the history write at
  `internal/server/http_workflows.go:186-195`, the unconditional live loop at
  `:196-212`, terminal producers in `internal/workflows/engine.go:257,459`,
  and a separate script-workflow handler that is explicitly out of scope.
- Conclusion: one HTTP handler owns the completion decision; #1236 owns only
  the engine history-to-live registration gap.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment: None.
- API/wire: existing SSE `id`, `event`, and JSON `data` fields remain unchanged;
  an already terminal replay now completes the response promptly.
- CLI/tools: no command, tool, or integration change.
- Error/validation: unchanged.

## Persistence and Compatibility

- Persistence/schema/cache: None; existing event history remains authoritative.
- Compatibility: terminal replay remains byte-shape compatible; clients that
  awaited EOF now receive it rather than hanging.
- Mixed rollout: server-only behavior is safe against existing persisted events.

## Lifecycle, Security, and Reliability

- Lifecycle: terminal history writes then returns, triggering the existing
  deferred subscription cancellation. Nonterminal history still waits for live
  events. No retry or timeout change.
- Security/auth/privacy: existing `runs:read` authorization remains before
  subscription; payload handling is unchanged.
- Recovery/idempotency: reconnect still replays history; terminal repetition is
  prevented by ending after the first terminal event.

## Product and Integration Surfaces

- Server/runtime: narrow `http_workflows.go` control-flow repair.
- TUI/GUI: indirect late-observer completion signal only; no client rendering
  code changes.
- Provider/model/catalog/external automation: None.
- UX/accessibility: no visual change; correctly completed stream lets clients
  settle status rather than spin.

## Deployment and Operations

- Deployment/flags: ordinary server rollout; no migration or flag.
- Diagnostics: existing workflow event/replay logs and client EOF behavior.
- Rollback: revert the terminal-history return branch.
- Runbooks: no operator change required.

## Regression Tests

- First red: an open live channel plus terminal replay must not require request
  cancellation for handler return.
- Acceptance: both completed/failed terminal types output once and return;
  nonterminal history admits a live event before live terminal completion.
- Negative/lifecycle: cancellation remains a cleanup fallback for the red test;
  no terminal duplicate from history/live transition.
- Commands: `go test ./internal/server -run WorkflowSSE -count=1`, race variant,
  `go test ./internal/server`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: no public-doc change.
- Implementation notes: engineering log and plan index in this PR.
- Training/release notes: None.
