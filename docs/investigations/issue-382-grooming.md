# Grooming: Issue #382 — feat(subagents): add async subagent lifecycle tools and HTTP surfaces

## Already Addressed?
Partial — the synchronous subagent surfaces exist but the async tool layer does not.

What already exists:
- HTTP: `POST /v1/subagents` (create, non-blocking — returns immediately after `Create()`), `GET /v1/subagents` (list), `GET /v1/subagents/{id}` (get/poll), `DELETE /v1/subagents/{id}` (delete, requires terminal status). These are all wired in `internal/server/http_subagents.go`.
- Manager: `subagents.Manager` interface (`internal/subagents/manager.go:77`) has `Create`, `Get`, `List`, `Delete`.
- The `Create` method starts the subagent asynchronously (via a background `monitor` goroutine); `Get` polls status and applies cleanup policy.

What does NOT exist:
- `start_subagent`, `get_subagent`, `wait_subagent`, `cancel_subagent` as named LLM-callable tools in the tools/deferred layer.
- A `wait` endpoint or a `cancel` endpoint in the HTTP layer.
- A `Cancel(ctx, id)` method on the `Manager` interface (only `Delete` exists, and it requires the run to already be in a terminal state).

The `run_agent` tool (`internal/harness/tools/deferred/run_agent.go`) uses `CreateAndWait` (blocking inline) rather than the async `Create/Get` path. `spawn_agent` uses `RunForkedSkill` (also blocking).

## Clarity
Clear for the tool names (`start_subagent`, `get_subagent`, `wait_subagent`, `cancel_subagent`). The issue says "reuse existing subagent manager" which is correct — the `subagents.Manager` interface already provides the primitives. However:
- `cancel_subagent` requires a `Cancel` method on Manager that does not exist (only `Delete` of a stopped agent exists).
- "Wait" semantics (block until terminal vs. long-poll HTTP) need to be specified.
- Whether "start_subagent" creates an HTTP subagent (worktree-capable) or an inline subagent needs clarification.

## Acceptance Criteria
Missing — the issue lists the four tool names and says "add HTTP surfaces" but does not specify:
- Input/output schema for each tool
- Whether `wait_subagent` has a timeout parameter
- Whether `cancel_subagent` requires the run to be cancelable while running (which requires plumbing into `harness.Runner.CancelRun`)
- Which HTTP endpoints are new vs. which already exist (most GET/POST already exist)
- Whether the LLM tools are depth-gated (available to root agents vs. subagents only)

## Scope
Too broad — four new tools plus "HTTP surfaces" is multiple issues bundled together. The HTTP surface for create/get/list/delete already exists. The genuine new work is:
1. The four LLM-callable deferred tools (`start_subagent`, `get_subagent`, `wait_subagent`, `cancel_subagent`)
2. A `Cancel` method on `Manager` + corresponding HTTP `POST /v1/subagents/{id}/cancel`

Recommend splitting: one issue for the four tools, one issue for cancel semantics.

## Blockers
`cancel_subagent` is blocked by the absence of `Cancel(ctx, id)` on the `Manager` interface and the absence of `CancelRun` plumbing in the harness Runner that propagates the cancel signal to the running step loop. This is non-trivial. The other three tools (`start`, `get`, `wait`) are unblocked.

## Recommended Labels
needs-clarification, large, blocked (cancel portion)

## Effort
Large — the four tools + Manager.Cancel + HTTP cancel endpoint together represent substantial work. If scoped to only `start_subagent`, `get_subagent`, `wait_subagent` (no cancel), it drops to medium.

## Recommendation
needs-clarification — the issue should be split into (a) async read-path tools (start/get/wait) and (b) cancel semantics. Cancel requires a new Manager interface method and runner plumbing that is not trivial. The read-path tools alone are well-specified and can proceed.

## Notes
- `internal/subagents/manager.go:77` shows the current Manager interface: `Create`, `Get`, `List`, `Delete`.
- `internal/server/http_subagents.go` already has `POST /v1/subagents` and `GET /v1/subagents/{id}` which cover what `start_subagent` and `get_subagent` would need to wrap.
- `start_subagent` should accept the same `subagents.Request` fields as `POST /v1/subagents` but return the subagent ID immediately (non-blocking).
- `wait_subagent` should accept an ID and optionally a timeout; it should poll `Manager.Get` and block until terminal status.
- `cancel_subagent` needs `Manager.Cancel(ctx, id)` which internally needs to call the runner's cancel path for the associated `RunID`.
- `internal/harness/runner.go` already has a `CancelRun` method; the Manager just needs to call it via `RunEngine`.
