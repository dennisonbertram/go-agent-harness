# Issue #322 — Bounded Worker Pool with Real RunStatusQueued Backpressure

## Summary

`RunStatusQueued` existed in the type system but was never honoured: every
`StartRun` and `ContinueRun` immediately spawned an unbounded goroutine.
This PR implements a real bounded worker pool so that runs wait in a FIFO
queue when all worker slots are occupied.

## Problem

Before this change:

```go
// StartRun / ContinueRun — both paths
go r.execute(run.ID, req)   // always immediate, no cap
```

Under load this spawns unlimited goroutines, exhausting memory and
goroutine scheduler budget. `RunStatusQueued` was cosmetically set for ~1 µs
before `execute()` immediately overrode it to `running`.

## Solution

### Design

A semaphore-based bounded pool, keeping the implementation simple:

- `Runner.workerSem chan struct{}` — counting semaphore of capacity
  `WorkerPoolSize`. A token is consumed before `execute()` starts and
  released via `defer` when it returns.
- `Runner.runQueue chan queuedRun` — FIFO channel (capacity 4096) feeding
  the long-running `poolDispatcher` goroutine.
- `dispatchRun(runID, req)` — single dispatch site replacing both bare
  `go r.execute(...)` call sites.

When `WorkerPoolSize == 0` (the default), the runner operates in the
legacy unbounded mode — `dispatchRun` immediately calls `go r.execute(...)`.

### New `run.queued` SSE Event

`EventRunQueued` (`"run.queued"`) is emitted when a run enters the queue.
Subscribers see the event immediately so they can display "queued" state
without polling. `AllEventTypes()` and the event count test were updated
from 64 → 65 entries.

### Files Changed

| File | Change |
|------|--------|
| `internal/harness/types.go` | Add `WorkerPoolSize int` to `RunnerConfig` |
| `internal/harness/events.go` | Add `EventRunQueued`, add to `AllEventTypes()` |
| `internal/harness/events_test.go` | Update expected count: 64 → 65 |
| `internal/harness/runner.go` | Add `queuedRun`, pool fields, `poolDispatcher`, `executeWithRelease`, `enqueueRun`, `dispatchRun`; replace both `go r.execute(...)` call sites |
| `internal/harness/runner_worker_pool_test.go` | New test file (6 tests) |

## Tests Written (TDD)

All tests live in `internal/harness/runner_worker_pool_test.go`.

| Test | Assertion |
|------|-----------|
| `TestWorkerPool_QueuedStatusWhenPoolFull` | With pool=2 and 5 runs, exactly 2 are Running and 3 are Queued while workers hold slots |
| `TestWorkerPool_QueuedTransitionsToRunning` | pool=1: run2 is Queued while run1 holds the slot; after run1 completes, run2 transitions to Running then Completed |
| `TestWorkerPool_ConfigurablePoolSize` | Pool sizes 1, 3, 5 each correctly bound concurrency |
| `TestWorkerPool_ZeroMeansUnlimited` | pool=0 starts all runs immediately (legacy behaviour) |
| `TestWorkerPool_PoolSize1Serializes` | pool=1 forces strictly sequential execution (calls complete in order 1, 2, 3) |
| `TestWorkerPool_RunQueuedEventEmitted` | A queued run has `run.queued` in its SSE event history |

## Test Results

```
ok  go-agent-harness/internal/harness    0.47s
ok  go-agent-harness/internal/server     17.3s
```

All 6 new tests pass. All pre-existing tests continue to pass.

## Pool Dispatcher Architecture

```
StartRun / ContinueRun
        │
        ▼
  dispatchRun()
        │
        ├─[workerSem has capacity]──► go executeWithRelease()  ◄─── slot consumed
        │                                      │
        └─[pool full]──► emit run.queued        │
                     ──► runQueue ──► poolDispatcher goroutine
                                            │
                                [workerSem <- struct{}{}  (blocks until free)]
                                            │
                                     go executeWithRelease()
                                            │
                                    execute()  (slot held)
                                            │
                                   defer: <-workerSem  (release slot)
```

## Configuration

```go
runner := NewRunner(provider, tools, RunnerConfig{
    WorkerPoolSize: 10,  // at most 10 concurrent runs; 0 = unlimited (default)
})
```

## Backward Compatibility

`WorkerPoolSize` defaults to `0` (zero value), which preserves the existing
unbounded goroutine behaviour. No existing code needs to change.
