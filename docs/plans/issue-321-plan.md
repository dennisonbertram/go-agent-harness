# Issue #321 Implementation Plan: Run Cancellation

## Overview

Add explicit run cancellation: `POST /v1/runs/{id}/cancel` endpoint with cooperative cancellation in the runner.

## Key Findings from Codebase Reading

### Current Context Usage in execute()
- `execute()` uses `context.Background()` for ALL calls:
  - `activeProvider.Complete(context.Background(), completionReq)` (line ~1520)
  - `r.applyPreHooks(context.Background(), ...)` (line ~1474)
  - `r.applyPostHooks(context.Background(), ...)` (line ~1541)
  - `r.applyPreToolUseHooks(context.Background(), ...)` (line ~1865)
  - `r.applyPostToolUseHooks(context.Background(), ...)` (line ~1961)
  - `toolCtx` constructed from `context.Background()` for tool execution (line ~1877)
- The `steeringCh` pattern in `runState` is the model for per-run channels

### AskUserQuestion Broker
- `broker.Ask()` already respects `ctx.Done()` — cancellation will naturally unblock waiting_for_user state

### Run Statuses (internal/harness/types.go)
- RunStatusQueued, RunStatusRunning, RunStatusWaitingForUser, RunStatusCompleted, RunStatusFailed
- Need to add: `RunStatusCancelled`

### Event System
- AllEventTypes() count is currently 63 — must update events_test.go when adding new event types
- IsTerminalEvent() checks for Completed | Failed — needs to also cover Cancelled

### HTTP Routing (internal/server/http.go line ~499-545)
- handleRunByID routes via `parts[1]` switch
- Add `/cancel` case

## Changes Required

### 1. `internal/harness/types.go`
- Add `RunStatusCancelled RunStatus = "cancelled"`

### 2. `internal/harness/events.go`
- Add `EventRunCancelled EventType = "run.cancelled"` in run lifecycle events block
- Add it to `AllEventTypes()` slice (count: 63 → 64, then 65 with the 2 new events)
- Update `IsTerminalEvent()` to include `EventRunCancelled`

Wait — we need TWO things:
1. `RunStatusCancelled` — the terminal status on the Run struct
2. `EventRunCancelled` — the SSE event emitted when a run is cancelled

### 3. `internal/harness/runner.go`
- Add `cancelFuncs sync.Map` to `Runner` struct (maps runID → context.CancelFunc)
- In `execute()`:
  - Create `ctx, cancel := context.WithCancel(context.Background())` at the start
  - Store cancel in `r.cancelFuncs`
  - Replace ALL `context.Background()` calls with `ctx`
  - Defer cleanup: `defer r.cancelFuncs.Delete(runID)` + `defer cancel()`
  - After provider.Complete returns: check if ctx was cancelled, handle gracefully
- Add `CancelRun(runID string) error` method:
  - Look up state under r.mu.RLock
  - If not found: return ErrRunNotFound
  - If already terminal: return nil (idempotent)
  - Load cancel func from cancelFuncs, call it
  - Emit run.cancelled event + set status to cancelled

### 4. `internal/harness/events.go` - `IsTerminalEvent`
- Add `|| et == EventRunCancelled`

### 5. `internal/server/http.go`
- Add `handleCancelRun` handler function
- Wire it: `if len(parts) == 2 && parts[1] == "cancel" { s.handleCancelRun(w, r, runID); return }`

### 6. `internal/harness/events_test.go`
- Update count from 63 to 65 (two new events: EventRunCancelled)
  - Wait: we only add ONE new event type (run.cancelled). So count goes 63 → 64.

## Concurrency Design

The cancel func map uses `sync.Map` — no coarse locking needed.

The key flow:
1. CancelRun loads the cancel func from cancelFuncs, calls it
2. The per-run ctx is cancelled; any blocked provider.Complete or broker.Ask returns with ctx.Err()
3. execute() detects context cancellation and calls a new `cancelledRun()` method instead of failRun
4. cancelledRun emits EventRunCancelled and sets RunStatusCancelled

## CancelRun Idempotency
- Run already terminal (completed/failed/cancelled): return nil
- Run not found: return ErrRunNotFound
- Double cancel: cancel() is safe to call multiple times (no-op after first call)
- cancelFuncs.Delete() is also idempotent

## What Happens to Waiting_for_user
The `ask_user_broker.Ask()` method has:
```go
case <-ctx.Done():
    b.clearPendingIfMatch(req.RunID, entry)
    return nil, time.Time{}, ctx.Err()
```
So when the run ctx is cancelled, Ask() returns ctx.Err() (context.Canceled).
This error flows back to execute() as a tool error. We check for context cancellation there.

## Test Plan

### internal/harness/runner_cancel_test.go
- TestCancelRun_ActiveRun: Cancel a run that's blocked mid-provider-call → reaches RunStatusCancelled
- TestCancelRun_WaitingForUser: Cancel while waiting for user input → unblocks and reaches RunStatusCancelled
- TestCancelRun_DoubleCancelIdempotent: Cancel twice → no panic, no error
- TestCancelRun_NotFound: Cancel non-existent run → ErrRunNotFound

### internal/server/http_cancel_test.go
- TestHandleCancel_Success: POST /cancel on active run → 200
- TestHandleCancel_NotFound: POST /cancel on non-existent run → 404
- TestHandleCancel_TerminalRun: POST /cancel on completed run → 200 (idempotent)
- TestHandleCancel_MethodNotAllowed: GET /cancel → 405
