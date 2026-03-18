# Issue #320 — Run History Durability Fix

## Problem

`CreateRun` was called at run start but `AppendMessage`, `AppendEvent`, and `UpdateRun` were never called during runner execution. As a result, after a server restart all run history (status, messages, events) was lost. The `store.Store` interface was fully defined and both `MemoryStore` and `SQLiteStore` implementations existed, but the runner never invoked the persistence methods.

## Solution

Wired `store.Store` persistence calls into the runner at all key lifecycle points.

### Changes Made

#### `internal/harness/types.go`

- Added `store "go-agent-harness/internal/store"` import.
- Added `Store store.Store \`json:"-"\`` field to `RunnerConfig`. When nil (the zero value), all store persistence is silently skipped — fully backward-compatible.

#### `internal/harness/runner.go`

- Added `store "go-agent-harness/internal/store"` import.
- Added `storedMsgCount int` field to `runState` to track how many messages have already been persisted, enabling incremental `AppendMessage` calls.
- Added store persistence helper methods:
  - `storeCreateRun(run Run)` — persists the initial run record via `CreateRun`.
  - `storeUpdateRun(runID string)` — reads current run state and persists via `UpdateRun`.
  - `storeAppendEvent(ev Event, seq uint64)` — serializes event payload to JSON and persists via `AppendEvent`.
  - `storeAppendNewMessages(runID string)` — appends only the tail of `state.messages` not yet stored, using `storedMsgCount` as the watermark. Stops on first error to preserve monotone seq.
  - `runToStoreRun(run Run) *store.Run` — converts `harness.Run` to `store.Run`.
  - `messageToStoreMessage(m Message, runID string, seq int) *store.Message` — converts `harness.Message` to `store.Message`, serializing `ToolCalls` to JSON.
- Wired persistence calls:
  - `StartRun`: calls `storeCreateRun(run)` after run state is registered.
  - `ContinueRun`: calls `storeCreateRun(newRun)` after continuation run state is registered.
  - `setStatus`: refactored from `defer r.mu.Unlock()` to explicit unlock, then calls `storeUpdateRun(runID)` after each status transition (queued → running → completed/failed/cancelled).
  - `setMessages`: refactored from `defer r.mu.Unlock()` to explicit unlock, then calls `storeAppendNewMessages(runID)` after each message batch update.
  - `emit`: calls `storeAppendEvent(event, eventSeq)` after `r.mu.Unlock()` for every emitted event.

### Error Handling

All persistence calls are **non-fatal**. If the store returns an error:
- The error is logged via `r.config.Logger.Error(...)` when a logger is configured.
- The run continues normally.
- This prevents transient database outages from failing otherwise-healthy runs.

### Message Persistence Strategy

Messages are managed as a full-slice replacement in `setMessages`. Rather than comparing slices, we track `state.storedMsgCount` — the number of messages already persisted. Each call to `storeAppendNewMessages` appends the tail `state.messages[storedMsgCount:]` and increments the counter by the number of successfully persisted messages. On error, we stop at the first failure to preserve `seq` monotonicity (the store enforces this as a contract).

### Event Persistence Strategy

Every event emitted via `emit` is immediately persisted to the store after the lock is released. The `Seq` value from `state.nextEventSeq` is used as the store `Seq`, ensuring the store's per-run event sequence matches the in-memory canonical ledger.

## Tests Added

`internal/harness/runner_store_durability_test.go` — 11 test functions covering:

- `TestRunnerStore_CreateRunCalledOnStartRun` — store.GetRun succeeds after StartRun.
- `TestRunnerStore_RunStatusTransitions` — stored status transitions to completed.
- `TestRunnerStore_FailedRunPersisted` — failed runs persist status=failed + error.
- `TestRunnerStore_EventsPersistedAsTheyStream` — all events (including run.started, run.completed) are in the store after completion.
- `TestRunnerStore_MessagesAppended` — messages are persisted with correct seq and roles.
- `TestRunnerStore_NoStoreConfigured` — nil Store runs normally (backward compatibility).
- `TestRunnerStore_RunPersistedAcrossRestartSimulation` — simulates server restart: a new runner backed by the same store can retrieve the completed run, events, and messages.
- `TestRunnerStore_ContinuationRunPersisted` — continuation runs (ContinueRun) are also persisted.
- `TestRunnerStore_ListRunsByConversation` — multiple runs sharing a conversation are queryable via ListRuns.
- `TestRunnerStore_EventSeqMonotonic` — stored event Seq values are strictly monotonic.
- `TestRunnerStore_StoreErrorDoesNotFailRun` — store errors are non-fatal; run completes despite failures.

## Verification

```
go test ./internal/harness/... ./internal/store/... -count=1
```

All tests pass.
