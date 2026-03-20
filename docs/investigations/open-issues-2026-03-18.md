# Open Issues — 2026-03-18

Fetched via `gh issue view` against the `upstream` remote.

---

## Issue #313 — TUI: show model availability based on provider configuration

**State:** OPEN
**Labels:** enhancement, medium, needs-clarification, tui
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
Update the TUI model picker so configured models are visually distinct from unavailable ones.

## Desired UX
- Models whose provider is currently configured should render in a strong/emphasized style.
- Models whose provider is not currently configured should render muted/greyed out.
- Availability should be driven by backend provider/model metadata rather than hardcoded assumptions.

## Context
The backend now exposes provider/model configuration state, including whether a provider is currently usable. The TUI should consume that instead of treating the model list as uniformly available.

## Acceptance Criteria
- TUI reads the configured/available state from the backend model/provider endpoints.
- Available models render emphasized.
- Unavailable models render muted and are clearly recognizable before selection.
- Any existing hardcoded model availability assumptions are removed or isolated.

---

## Issue #314 — Feature: add Codex MCP server integration as a future optional capability

**State:** OPEN
**Labels:** deferred, large, needs-clarification
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
Explore `codex mcp-server` as a future integration path distinct from the new `codex app-server` provider.

## Why
We now have a dedicated `codex` provider path backed by `codex app-server`. Separately, `codex mcp-server` is interesting as a way to expose Codex-backed capabilities through the harness MCP surface without replacing the harness runner/provider architecture.

## Scope
- Investigate how `codex mcp-server` should be wired into the harness MCP client/server model.
- Keep this separate from the primary provider routing path.
- Decide whether this should appear as:
  - a globally configured MCP server,
  - a per-run MCP server option, or
  - a delegated tool surface.

## Acceptance Criteria
- Design doc or implementation plan describing the intended UX and architecture.
- Clear boundaries between `codex app-server` provider behavior and `codex mcp-server` behavior.
- Follow-up implementation tasks identified if the design is viable.

## Notes
This is intentionally not part of the current Codex provider work.

---

## Issue #315 — TUI: add provider authentication management for Codex login and API keys

**State:** OPEN
**Labels:** deferred, large, needs-clarification, tui
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
Add a future TUI flow for managing provider authentication instead of requiring users to preconfigure everything out-of-band.

## Scope
- Provide a TUI affordance for initiating Codex login / checking Codex login status.
- Provide a TUI affordance for adding or updating API credentials for providers that use API keys.
- Surface current provider auth state in a user-friendly way.

## Why
The backend can start without a default API key and providers may have different auth modes (`api_key_env`, `codex_login`, etc.). The TUI should eventually help users make providers available directly from the app.

## Acceptance Criteria
- UX/design for provider auth management is defined.
- Codex login status/action path is included.
- API-key-backed providers have a clear configuration/update path.
- Security constraints for storing or forwarding credentials are documented before implementation.

## Notes
This is a future UX/settings feature and is intentionally out of scope for the current provider implementation.

---

## Issue #320 — feat(store): persist run state transitions, messages, and events for durable run history

**State:** OPEN
**Labels:** enhancement, infrastructure, large, well-specified
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
The store interface already supports `UpdateRun`, `AppendMessage`, and `AppendEvent`, but the live backend only persists the initial `CreateRun`. Run status changes, messages, and events still live primarily in runner memory, so historical run recovery is incomplete and `GET /v1/runs/{id}/events` cannot replay from durable storage after restart.

## Impact
- Completed runs do not have a fully durable event/message ledger.
- Restarting the process loses the primary run transcript/event history for inactive runs.
- The store contract is underused, which makes persistence look more complete than it actually is.
- SSE replay and postmortem inspection cannot rely on the database as the source of truth.

## Current Behavior
- `internal/store/store.go:79-110` defines `CreateRun`, `UpdateRun`, `AppendMessage`, `AppendEvent`, and read APIs.
- `internal/server/http.go:433-437` only calls `CreateRun(...)` when a run starts.
- `internal/server/http.go:752-768` falls back to persisted `GetRun(...)` for metadata, but `handleRunEvents(...)` only uses `runner.Subscribe(...)`.
- There is no wiring from runner lifecycle/message/event emission into `Store.AppendMessage(...)` / `Store.AppendEvent(...)` / `Store.UpdateRun(...)`.

## Expected Behavior
With a store configured:
- Run lifecycle transitions are persisted as they happen.
- Final output, error, usage, and cost totals are persisted.
- Message transcript and event stream are durably appended in sequence order.
- Historical `GET /v1/runs/{id}` and `GET /v1/runs/{id}/events` work after restart.
- A late subscriber can replay historical events from the store even if the in-memory runner has already forgotten the run.

## Proposed Fix Direction
1. Decide the source of truth boundary:
   - Runner memory remains the hot-path source for active runs.
   - Store becomes the durable source for historical runs.
2. Add a runner/store observer or callback path that persists:
   - status transitions
   - assistant/tool/user messages
   - emitted events with stable sequence numbers
3. Update `GET /v1/runs/{id}/events` to fall back to store-backed replay when runner memory no longer has the run.
4. Persist usage/cost totals and final output on completion/failure.
5. Preserve existing in-memory behavior for tests/dev when no store is configured.

## Acceptance Criteria
- `POST /v1/runs` with a configured store results in durable run metadata, messages, and events.
- Restart-style tests can retrieve run metadata/events/messages from the store without runner memory.
- `GET /v1/runs/{id}/events` supports replay from store for inactive runs.
- Sequence ordering remains monotonic and stable.
- The persistence path does not regress current live SSE behavior for active runs.

## Test Plan / Regression Coverage
- Add integration coverage using `MemoryStore` and `SQLiteStore` for:
  - create run -> complete run -> fetch persisted run metadata
  - persisted messages in expected order
  - persisted events replayable after the runner no longer exposes the run in memory
  - failure path persistence (`run.failed`)
  - waiting-for-user + resumed path persistence
- Consider a restart-style test that constructs a new server against the same SQLite DB and reads historical runs/events.

## Related Issues / References
- Broad predecessor: #7
- Related replay work: #212
- Relevant files:
  - `internal/store/store.go`
  - `internal/server/http.go`
  - `internal/harness/runner.go`

---

## Issue #322 — feat(runs): replace cosmetic queued status with a bounded scheduler / worker pool

**State:** OPEN
**Labels:** enhancement, infrastructure, large, reliability, well-specified
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
Runs are marked `queued`, but the backend does not actually maintain a queue or bounded worker pool. `StartRun(...)` returns a queued run and immediately launches `go r.execute(...)`, so the queue status is largely cosmetic and concurrency is effectively unbounded per process.

## Impact
- Burst traffic can spawn an unbounded number of provider/tool execution goroutines.
- There is no fairness or backpressure across runs.
- `queued` does not correspond to a real scheduler state, which makes the API misleading.
- Future features like cancellation, prioritization, and quotas have no central scheduler to hook into.

## Current Behavior
- `internal/harness/runner.go:346-350` initializes runs with `RunStatusQueued`.
- `internal/harness/runner.go:442-446` immediately starts execution with `go r.execute(run.ID, req)`.

## Expected Behavior
- A run can actually remain queued until capacity is available.
- The process enforces a configurable max number of concurrently executing runs.
- Queueing behavior is visible and testable.
- The scheduler becomes the single place to add fairness, cancellation of queued work, and later quota policies.

## Proposed Fix Direction
1. Introduce a runner-owned scheduler/dispatcher with a bounded worker pool.
2. Add a config knob such as `HARNESS_MAX_CONCURRENT_RUNS` (or equivalent config-stack field).
3. Keep `RunStatusQueued` until a worker picks the run up; transition to `running` only when execution actually begins.
4. Preserve current behavior when the limit is unset or set to `0` if the repo prefers `0 = unlimited` semantics.
5. Expose queue depth / running count via logging or status helpers so this is operationally visible.
6. Coordinate with the cancellation issue so queued runs can be removed before they start.

## Acceptance Criteria
- With max concurrency `1`, starting 2 runs leaves the second in `queued` until the first finishes or is cancelled.
- Status transitions are deterministic: `queued -> running -> completed|failed|cancelled`.
- Scheduler shutdown does not orphan queued work.
- Existing single-run behavior remains unchanged when concurrency limit is effectively unlimited.

## Test Plan / Regression Coverage
- Add runner tests covering:
  - max concurrency `1` FIFO behavior
  - queued run remains queued until capacity is free
  - queue + cancel interaction once cancellation exists
  - graceful shutdown / cleanup behavior
- Add any necessary HTTP or status tests that assert the user-visible queued state.

## Related Issues / References
- This complements, but should not be blocked by, run cancellation.
- Relevant files:
  - `internal/harness/runner.go`
  - `internal/harness/types.go`

---

## Issue #323 — feat(permissions): add interactive approval requests and resume/deny workflow

**State:** OPEN
**Labels:** enhancement, infrastructure, large, security, well-specified
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
`permissions` mode currently short-circuits at tool execution time by returning `permission_denied` / `permission_error`. There is no backend approval request workflow, so the system cannot pause, ask an operator for approval, and then resume execution with the original tool call.

## Impact
- The two-axis permission model is static rather than interactive.
- Frontends can render permission UI, but the backend does not expose a durable approval handshake.
- Mutating tools either run immediately (`full_auto`) or fail immediately (`permissions`), with no middle ground.

## Current Behavior
- `internal/harness/tools/policy.go:20-90` enforces approval mode by either allowing the tool or returning an error payload.
- There is no approval broker comparable to the `AskUserQuestion` pause/resume flow.
- No `/v1/runs/...` route exists for approval requests or approval responses.

## Expected Behavior
When approval policy requires operator input:
- The runner pauses before executing the gated tool call.
- The frontend/operator can inspect the pending tool call and decide `approve` or `deny`.
- The run resumes after the decision.
- Deny/timeout paths are explicit and deterministic.

## Proposed Fix Direction
1. Add a first-class approval broker in the backend, modeled after the existing wait-for-user broker where appropriate.
2. Add run events for the approval lifecycle (for example `run.approval.requested` / `run.approval.resolved`, or another clearly named pair).
3. Add an HTTP surface for:
   - reading the current pending approval request
   - submitting an approval decision (`approve` or `deny`)
4. On `approve`, execute the original tool call.
5. On `deny`, emit a structured blocked-tool result back into the transcript and continue the run.
6. Decide and document timeout behavior; the preferred behavior is to treat timeout as deny with a machine-readable reason.

## Acceptance Criteria
- A gated tool call pauses the run instead of immediately failing.
- Approve executes the exact pending tool call that was presented.
- Deny/timeout do not execute the tool and resume the run with a structured blocked result.
- Approval state survives SSE reconnects and is queryable over HTTP while pending.
- Existing `full_auto` behavior remains unchanged.

## Test Plan / Regression Coverage
- Add runner tests for approve, deny, and timeout flows.
- Add HTTP tests for reading/submitting approval decisions.
- Add tests for reconnecting to SSE while approval is pending.
- Add integration coverage that combines approval flow with tool filtering / permission config.

## Related Issues / References
- Builds on the closed baseline permission model from #15.
- Existing adjacent UX work on the TUI side already assumes a richer approval flow.
- Relevant files:
  - `internal/harness/tools/policy.go`
  - `internal/harness/runner.go`
  - `internal/server/http.go`

---

## Issue #324 — feat(runs): make workspace backends selectable per run

**State:** OPEN
**Labels:** enhancement, infrastructure, large, well-specified, workspace
**Author:** dennisonbertram
**Comments:** 0

### Body

## Summary
The repo already has a real workspace abstraction (`local`, `worktree`, `container`, `vm`, `pool`), and both subagents and `symphd` use it. Normal runs started through `POST /v1/runs` still execute only in the server's local context. Workspace selection is not a first-class part of `RunRequest`.

## Impact
- The strongest isolation story is limited to subagents/orchestration flows instead of normal runs.
- Clients cannot ask for a run to execute in a worktree, container, VM, or pooled workspace through the main API.
- The backend cannot cleanly expose "same agent runtime, different execution substrate" as a single control plane feature.

## Current Behavior
- `internal/workspace/workspace.go:8-63` defines the abstraction and provisioning options.
- `internal/subagents/manager.go:223-258` provisions worktree-backed isolation for subagents.
- `internal/symphd/orchestrator.go:288-318` selects workspace backends for orchestration.
- `RunRequest` does not expose a workspace-selection block for ordinary runs.

## Expected Behavior
- `POST /v1/runs` can optionally request a workspace backend.
- Default behavior remains the current local process execution when no workspace is specified.
- The backend provisions the requested workspace, scopes tools/workspace root accordingly, and cleans it up on terminal completion per policy.

## Proposed Fix Direction
1. Add an optional `workspace` block to `RunRequest`, for example:
   - `type`: `local | worktree | container | vm | pool`
   - selector fields needed for worktree/pool variants (for example `repo_path`, `worktree_root`, `base_ref`)
2. Keep secrets/config propagation server-controlled; do not expose arbitrary raw secret injection in the public request body as part of this issue.
3. Reuse the existing workspace package and factory wiring instead of inventing a second provisioning system.
4. Ensure cleanup semantics are explicit for normal runs, including failure and cancellation paths.
5. Dependency-gate container/vm/pool modes when the server is not configured to support them.

## Acceptance Criteria
- A run with no `workspace` block behaves exactly as it does today.
- A run with `workspace.type=worktree` provisions an isolated worktree-backed execution context and cleans it up per policy.
- Unsupported workspace types fail fast with a structured error.
- The selected workspace root is reflected in tool behavior (for example file tools stay scoped to the provisioned workspace).
- Provisioning/cleanup failures are surfaced in run status/events.

## Test Plan / Regression Coverage
- Add unit/integration coverage for at least:
  - default local behavior unchanged
  - worktree-backed run provisioning via fake or temp-repo test harness
  - unsupported workspace type error
  - cleanup on success/failure/cancel
- Add config plumbing tests if new server config is needed to expose workspace factories in `harnessd`.

## Related Issues / References
- Existing workspace foundation: #181-#186
- Related orchestrator/subagent plumbing: #203-#206, #234, #236
- Relevant files:
  - `internal/workspace/*`
  - `internal/subagents/manager.go`
  - `internal/symphd/orchestrator.go`
  - `internal/harness/types.go`
