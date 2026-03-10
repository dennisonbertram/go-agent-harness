# Issue #6: Mid-Run Steering

## Summary

Implemented the `/v1/runs/{runID}/steer` endpoint that allows users to inject
guidance into an actively running LLM execution loop. Steering messages are
buffered and injected into the conversation transcript as user messages before
the next LLM call.

## Changes

### `internal/harness/runner.go`

- Added `steeringCh chan string` field to `runState` struct (buffered, capacity 10)
- Initialized `steeringCh` in both `StartRun` and `ContinueRun`
- Added error sentinels: `ErrRunNotActive`, `ErrSteeringBufferFull`
- Added constant `steeringBufferSize = 10`
- Added `SteerRun(runID, message string) error` method:
  - Returns `ErrRunNotFound` if run doesn't exist
  - Returns `ErrRunNotActive` if run is not in Running or WaitingForUser state
  - Returns `ErrSteeringBufferFull` if channel is at capacity (non-blocking send)
  - Returns validation error if message is empty
- Added `drainSteering(runID string, messages *[]Message)` helper:
  - Non-blocking drain of all pending steering messages
  - Appends each message as a `{Role: "user"}` message to the transcript
  - Emits `EventSteeringReceived` for each injected message
- Called `drainSteering` at the top of each step in the `execute` loop

### `internal/harness/events.go`

- Added `EventSteeringReceived EventType = "steering.received"` constant
- Added `EventSteeringReceived` to `AllEventTypes()` slice
- Updated `TestAllEventTypes_Count` expected count from 39 to 40

### `internal/server/http.go`

- Added routing for `steer` sub-resource in `handleRunByID`
- Added `handleRunSteer` handler:
  - Only accepts POST
  - Returns 400 for missing/empty message
  - Returns 404 for unknown run IDs
  - Returns 409 Conflict for inactive (completed/failed) runs
  - Returns 429 Too Many Requests when steering buffer is full
  - Returns 202 Accepted on success

## Tests

### `internal/harness/runner_steer_test.go` (8 tests)

- `TestSteerRun_BasicInjection` - steering message appears in second LLM call transcript
- `TestSteerRun_RunNotFound` - ErrRunNotFound for unknown run ID
- `TestSteerRun_EmptyMessage` - validation error for empty message
- `TestSteerRun_CompletedRun` - ErrRunNotActive for completed run
- `TestSteerRun_BufferFull` - ErrSteeringBufferFull when channel full
- `TestSteerRun_MultipleMessages` - multiple steers injected in order
- `TestSteerRun_SSEEvent` - steering.received SSE event emitted
- `TestSteerRun_ConcurrentSafety` - concurrent steers don't race

### `internal/server/http_steer_test.go` (5 tests)

- `TestHandleSteer_Success` - 202 on active run
- `TestHandleSteer_RunNotFound` - 404 for unknown run
- `TestHandleSteer_EmptyMessage` - 400 for empty message
- `TestHandleSteer_MethodNotAllowed` - 405 for non-POST
- `TestHandleSteer_CompletedRun` - 409 for completed run

## Design Notes

### Injection Point

Steering messages are drained at the TOP of each step, before any LLM call.
This means:
- Steering during a tool execution: message waits in buffer until the tool
  completes and the next step begins
- Multiple steers between steps: all are drained and injected in order
- Steering is non-blocking: never delays the execute loop

### Role Choice

Steering messages are injected as `{Role: "user"}` messages. This is the
cleanest choice since:
- It fits naturally in a conversation transcript
- The LLM treats user messages as guidance/instructions
- No special handling needed in the provider layer

### Thread Safety

`SteerRun` reads state under `r.mu.RLock()` to find the run and check status,
then uses a non-blocking channel send to the `steeringCh`. The channel itself
is goroutine-safe. The `drainSteering` helper only runs in the execute goroutine
so there's no concurrent drain.

### Race Condition in SSE Test

The `TestSteerRun_SSEEvent` test was carefully designed to avoid a send-on-
closed-channel race. The test drains the event stream until a terminal event
(`run.completed` or `run.failed`) before calling `cancel()`. This ensures no
emit goroutine is still trying to send to the channel when it's closed.
