# Issue #323: Permissions Approval Broker

## Summary

Implements a pause/approve/deny broker for the `permissions` run mode. When a run's `ApprovalPolicy` is set to `all` or `destructive`, tool calls are paused before execution and the operator must explicitly approve or deny them via HTTP endpoints.

## Problem

The `permissions.approval` field in `RunRequest` accepted values of `none`, `destructive`, and `all` but was never checked during tool execution — calls just proceeded unconditionally. There was no mechanism to pause a run and resume it after operator review.

## Solution

### New Files

- **`internal/harness/approval_broker.go`** — `ApprovalBroker` interface and `InMemoryApprovalBroker` implementation. Modeled after `ask_user_broker.go`. Uses channel-based blocking (`decisionCh chan approvalDecision`) to pause the runner goroutine until the operator responds. Also defines `ApprovalRequest`, `PendingApproval`, `ApprovalTimeoutError`, `ErrNoPendingApproval`.

- **`internal/harness/approval_broker_test.go`** — Unit tests for the broker: lifecycle (approve), lifecycle (deny), timeout, context cancellation, duplicate rejection, approve/deny unknown run, error string formatting.

- **`internal/harness/runner_approval_test.go`** — Integration tests for the runner: `ApprovalPolicyAll` pauses and emits events, denial causes run to continue with error feedback to LLM, `ApprovalPolicyNone` skips the broker entirely, `ApprovalPolicyDestructive` checks only mutating tools.

- **`internal/server/http_approval_test.go`** — HTTP-level tests: 404 for unknown runs, 405 for wrong HTTP method, full integration flows for approve and deny.

### Modified Files

**`internal/harness/types.go`**
- Added `Mutating bool` field to `ToolDefinition` (enables destructive-policy filtering)
- Added `RunStatusWaitingForApproval RunStatus = "waiting_for_approval"`
- Added `ApprovalBroker ApprovalBroker` field to `RunnerConfig`

**`internal/harness/events.go`**
- Added `EventToolApprovalRequired = "tool.approval_required"`
- Added `EventToolApprovalGranted = "tool.approval_granted"`
- Added `EventToolApprovalDenied = "tool.approval_denied"`
- Updated `AllEventTypes()` (count: 64 → 67)

**`internal/harness/events_test.go`**
- Updated `AllEventTypes` count assertion: 64 → 67

**`internal/harness/registry.go`**
- Added `mutating bool` to `registeredTool` struct
- Updated `Register()` and `RegisterWithOptions()` to persist `def.Mutating`
- Added `IsMutating(name string) bool` method

**`internal/harness/tools_default.go`**
- Propagated `Mutating: t.Definition.Mutating` in both core and deferred tool construction

**`internal/harness/runner.go`**
- Snapshot `effectiveApprovalPolicy` from run state at the start of `execute()`
- Added Phase 1 approval check (between pre-tool-use hooks and tool dispatch): emits `tool.approval_required`, calls `broker.Ask()`, handles timeout/ctx-cancel as fatal denial, handles explicit denial as LLM-visible error (run continues), handles approval by falling through to normal tool execution
- Added `ApprovalBroker() ApprovalBroker` accessor method

**`internal/server/http.go`**
- Added `ApprovalBroker harness.ApprovalBroker` to `ServerOptions`
- Added `approvalBroker` field to `Server` struct
- Added routing for `POST /v1/runs/{id}/approve` and `POST /v1/runs/{id}/deny`
- Implemented `handleApproveRun()` and `handleDenyRun()` handlers

## Approval Flow

```
Runner (Phase 1 loop)
  for each tool call:
    1. allowedTools check
    2. ask_user_question special handling
    3. pre-tool-use hooks
    4. [NEW] approval check:
       if broker != nil && policy != none:
         if policy == all || (policy == destructive && IsMutating(tool)):
           emit tool.approval_required
           setStatus(waiting_for_approval)
           approved, err := broker.Ask(ctx, req)  ← BLOCKS HERE
           if err (timeout/cancel): fail or cancel run
           if !approved: emit tool.approval_denied, add error result to messages, continue
           emit tool.approval_granted
           setStatus(running)
    5. build tool execution context
    6. Phase 2: execute
```

## HTTP Endpoints

### `POST /v1/runs/{id}/approve`
Returns `200 {"status":"approved","run_id":"...","call_id":"..."}` or `404 {"error":"no pending approval"}`.

### `POST /v1/runs/{id}/deny`
Returns `200 {"status":"denied","run_id":"...","call_id":"..."}` or `404 {"error":"no pending approval"}`.

Both return `405` for non-POST methods.

## ApprovalPolicy Semantics

| Policy | Behavior |
|--------|----------|
| `none` (or unset) | No approval checks; broker is ignored even if configured |
| `all` | Every tool call requires approval |
| `destructive` | Only tools with `Mutating: true` in their `ToolDefinition` require approval |

## Also Fixed (Pre-existing Bugs)

- `cmd/harnessd/main_test.go`: `runMatrixTest` function body was missing (truncated at variable declarations). Reconstructed from the comment/docstring and usage patterns. `TestRunWithSignalsMCPParseFailureContinues` had stray `runMatrixTest` body fragments mixed in; fixed. `TestMatrix_ProviderAPIKeyCapture` had `TestRunWithSignalsInvalidModelCatalogContinues` embedded in its goroutine closure; split into two separate correct functions.

- `cmd/harnessd/main_test.go`: Added tests for `GetSkillFilePath` and `UpdateSkillVerification` methods on `skillListerAdapter` to satisfy the zero-coverage gate.

- `internal/harness/approval_broker_test.go`: Added `TestApprovalTimeoutError_Error` to exercise the `Error()` method and satisfy the zero-coverage gate.
