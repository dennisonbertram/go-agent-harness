# Issue #1: Stream Tool Output Incrementally During Execution

## Summary

Implemented incremental tool output streaming via SSE events. Long-running tools (e.g., bash commands) now emit `tool.output.delta` events as output is produced, rather than waiting until the tool completes.

## Changes

### `internal/harness/events.go`
- Added `EventToolOutputDelta EventType = "tool.output.delta"` constant
- Added `EventToolOutputDelta` to `AllEventTypes()` list (count: 40 → 41)

### `internal/harness/tools/types.go`
- Added `ContextKeyOutputStreamer contextKey = "output_streamer"` context key
- Added `OutputStreamerFromContext(ctx context.Context) (func(chunk string), bool)` helper

### `internal/harness/tools/bash_manager.go`
- Added `bufio` and `io` imports
- Modified `runForeground` to check for an output streamer in context
- When a streamer is present: uses `io.Pipe` + `io.MultiWriter` to stream stdout line-by-line in a goroutine while also capturing the full output for the final result
- When no streamer: original behavior unchanged

### `internal/harness/runner.go`
- In the tool execution loop, creates an `outputStreamer` closure that calls `r.emit(runID, EventToolOutputDelta, ...)` with `call_id`, `tool`, and `content` fields
- Injects the streamer into the tool context via `ContextKeyOutputStreamer`

## Event Schema

```json
{
  "type": "tool.output.delta",
  "payload": {
    "call_id": "call-123",
    "tool": "bash",
    "content": "line of output\n"
  }
}
```

## Design Decisions

1. **Context-based callback**: The streamer is passed through context so tools opt in without changing the `Handler func(ctx, args) (string, error)` signature. Non-streaming tools ignore it transparently.

2. **stdout only, stderr buffered**: Stderr is always buffered and appended to the final output. This avoids interleaved stderr in the delta stream, which would be confusing for clients.

3. **Full output still returned**: The complete output is always returned in `tool.call.completed`. The delta events are supplementary — clients that don't handle `tool.output.delta` still get the full result.

4. **Line-by-line granularity**: Used `bufio.Scanner` for line-by-line delivery, which is the most useful granularity for shell output. Partial lines are flushed at command completion.

5. **No backpressure**: Streaming chunks are emitted with the same fire-and-forget model as all other events (dropped if subscriber is too slow). This matches the existing event system behavior.

## Tests Added

- `internal/harness/tools/bash_manager_test.go` (new file):
  - `TestJobManagerRunForegroundStreaming`: verifies chunks are received and full output is correct
  - `TestJobManagerRunForegroundNoStreamer`: verifies backward-compatible behavior without streamer
  - `TestJobManagerRunForegroundStreamingCapturesFull`: verifies full output captured alongside streaming
  - `TestJobManagerRunForegroundStreamingConcurrency`: concurrent execution with race detector
  - `TestOutputStreamerFromContext`: nil/empty/populated context cases

- `internal/harness/runner_test.go` (additions):
  - `TestToolOutputDeltaEvents`: end-to-end test verifying event ordering and payload fields
  - `TestToolOutputDeltaAbsentWhenToolDoesNotStream`: verifies no spurious events for silent tools
  - `TestAllEventTypesIncludesToolOutputDelta`: registry completeness check

- `internal/harness/events_test.go`: updated `TestAllEventTypes_Count` from 40 → 41

## Test Results

```
ok  go-agent-harness/internal/harness          2.4s
ok  go-agent-harness/internal/harness/tools    9.5s
```

All tests pass with `-race`. Only pre-existing `demo-cli` build failure remains (unrelated to this change).
