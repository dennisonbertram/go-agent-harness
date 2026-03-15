# Plan: Issue #238 — reset_context tool

## Architecture

```
Runner.execute() step loop
    |
    v
Tool dispatch (for reset_context call)
    |
    +---> obsMemory.AddObservation("context_reset", persist)
    +---> r.emit(ContextReset event)
    +---> reset counter increment on runState.resetIndex
    +---> r.setMessages(runID, []Message{})         -- clear transcript
    +---> r.setMessages with [systemPrompt, openingMsg] -- re-inject
    +---> continue for loop (step counter continues from next iteration)
```

## Interface Changes

### runState (runner.go)
Add:
- `resetIndex int` — increments on each reset (0 = no reset yet)

### events.go
Add:
- `EventTypeContextReset EventType = "context.reset"`

### ContextResetPayload (events.go)
```go
type ContextResetPayload struct {
    ResetIndex int             `json:"reset_index"`
    AtStep     int             `json:"at_step"`
    Persist    json.RawMessage `json:"persist"`
}
```

### tools/reset_context.go
New tool handler. Uses a sentinel return value to signal the runner to perform the reset. The runner checks for a special key in the tool result to intercept before appending the result to the message list.

### ConversationStore (no change needed for MVP)
The spec mentions `run_context_resets` DB table and `segment_index` on conversations, but the harness ConversationStore only stores conversation messages. The DB layer lives in internal/store/. We will add:
- `run_context_resets` table migration in store/sqlite.go
- `RecordContextReset` and `GetContextResets` methods on the store.Store interface
- A new `ContextResetStore` interface in internal/harness/ so runner can call it optionally

Actually, to avoid over-engineering: the store.Store interface is separate from ConversationStore. The runner already uses ConversationStore. We'll add an optional `ContextResetStore` field to RunnerConfig.

### Observational Memory
The Manager interface has `Observe()` which adds observations. We use a direct call to `Observe()` with a custom observation wrapping the persist JSON. The observation type is indicated via content prefix.

### Tool: reset_context
The tool returns a sentinel result (JSON with `__reset_context__: true`) so the runner can detect it in the tool dispatch loop and perform the actual reset operations (memory write, event emit, message clear, re-inject).

## DB Migration Approach

In `internal/store/sqlite.go`, add a new migration:
```sql
CREATE TABLE IF NOT EXISTS run_context_resets (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    reset_index INTEGER NOT NULL,
    at_step     INTEGER NOT NULL,
    persist     TEXT NOT NULL,
    created_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_resets_run ON run_context_resets(run_id);
```

Add `ContextResetStore` interface to harness package:
```go
type ContextResetStore interface {
    RecordContextReset(ctx context.Context, runID string, resetIndex, atStep int, persist json.RawMessage) error
    GetContextResets(ctx context.Context, runID string) ([]ContextReset, error)
}
```

Add `ContextResetStore ContextResetStore` field to `RunnerConfig`.

## Step Loop Integration

After the tool dispatch, detect `reset_context` via the tool result sentinel:
```go
if isContextResetResult(toolOutput) {
    persist := extractPersistPayload(toolOutput)
    // 1. Write to obs memory
    // 2. Emit ContextReset event
    // 3. Record reset index
    // 4. Clear messages
    // 5. Re-inject system prompt + opening message
    // 6. Continue to next step (messages variable is now reset)
    messages = buildResetMessages(systemPrompt, resetIndex, persist)
    r.setMessages(runID, messages)
    continue  // skip appending tool result to messages
}
```

## Post-Reset Opening Message Format

```
[Context Reset — Segment N of this run]

You previously reset your context. Here is what you carried forward:

{persist JSON pretty-printed or key-value formatted}

Continue from here.
```

## Testing Strategy

All tests in `internal/harness/runner_reset_test.go`:

1. `TestResetContext_StepCounterContinues` — step counter doesn't reset
2. `TestResetContext_CostContinues` — cost accumulation continues
3. `TestResetContext_RunIDUnchanged` — same run ID before and after
4. `TestResetContext_MessagesCleared` — messages array is empty after reset then re-injected
5. `TestResetContext_ChainedResets` — can reset multiple times
6. `TestResetContext_DBRecorded` — reset is recorded in run_context_resets table
7. `TestResetContext_ObservationalMemoryWritten` — persist payload written to obs memory
8. `TestResetContext_SegmentIndexIncrements` — resetIndex increments each time

## Risk Areas

- **Concurrency**: reset clears messages under the main loop (no separate goroutine), so no new race condition introduced. setMessages uses the same mutex as all other calls.
- **DB migration ordering**: migrations are append-only, safe to add new table.
- **Tool result sentinel**: using `__reset_context__` JSON key is fragile if LLM outputs that key; mitigation: only check when tool name == "reset_context".
- **Cost accounting**: per spec, cost is NOT restored; accumulated cost remains across resets. This is naturally enforced since costTotals lives in runState, not messages.
