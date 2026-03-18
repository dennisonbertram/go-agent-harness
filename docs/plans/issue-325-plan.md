# Issue #325: Parallel Tool Execution Plan

## Current State

- `htools.Definition` (internal/harness/tools/types.go) already has `ParallelSafe bool` field.
- Many tools already have this field set (read, glob, grep, git_diff, git_status, fetch, etc.).
- `harness.ToolDefinition` (internal/harness/types.go) does NOT have `ParallelSafe`.
- `registeredTool` in registry.go does NOT store `ParallelSafe`.
- The tool dispatch loop (runner.go ~1765-2129) is fully serial.

## Changes Required

### 1. harness.ToolDefinition (types.go)
Add `ParallelSafe bool` field (json:"-" since it's not wire-format).

### 2. harness.registeredTool (registry.go)
Add `parallelSafe bool` field so it's preserved across registration.

### 3. Registry methods (registry.go)
- Preserve `ParallelSafe` when registering via `Register` and `RegisterWithOptions`.
- Add `IsParallelSafe(name string) bool` method for the runner to query.

### 4. tools_default.go
Pass `ParallelSafe` from `htools.Definition` → `harness.ToolDefinition` when registering core and deferred tools.

### 5. Runner tool dispatch loop (runner.go ~1765)
Strategy: After the serial pre-dispatch section (EventToolCallStarted, causal recording, audit, anti-pattern detection, skill constraint check), group calls into:
- **Serial calls**: calls where `!r.tools.IsParallelSafe(call.Name)` OR calls with special side effects (ask_user, reset_context, skill constraint activation)
- **Parallel-safe calls**: everything else

For a batch of tool calls from a single LLM turn:
1. Emit all EventToolCallStarted events in order (serial - preserves event ordering).
2. Identify which calls are parallel-safe (no special flags, IsParallelSafe=true).
3. Group consecutive parallel-safe runs together; keep serial tools in order.
4. For parallel groups: launch goroutines, collect results into pre-allocated slots (by original index), wait via WaitGroup.
5. Re-attach results in original index order.

**Simpler alternative (chosen for minimal change):**
Since the pre-dispatch logic (EventToolCallStarted, causal, audit, anti-pattern, skill constraint, ask_user check, pre-tool hooks) all runs serially anyway and has mutable side effects, we split the loop into two phases:
1. **Pre-dispatch phase** (serial): For each call, do all the pre-execution work, and either resolve it immediately (blocked by constraint, denied by hook) OR enqueue for execution.
2. **Execution phase** (parallel): For queued calls that are parallel-safe, run concurrently. Collect outputs into indexed slots.
3. **Post-dispatch phase** (serial): In original order, process each result (emit completed event, append to messages, handle meta-messages, handle context reset).

## Key Constraints
- `waitingForUser` (ask_user_question) is always serial - it blocks the run.
- `reset_context` result handling resets `messages` - always serial.
- `compact_history` uses messageReplacer which mutates `messages` - always serial.
- Anti-pattern detection uses `antiPatternCounts` map - needs to stay serial or be protected.
- `r.setMessages` is called after each tool result - must stay in-order in post-dispatch.
- The `messages = r.messagesForStep(latestState)` re-read after tool execution is important for compaction - this needs careful handling in parallel mode (read after all parallel tools complete).

## Race Safety
- Each goroutine gets its own `toolCtx`, `callArgs`, captured loop variables.
- `streamIndex atomic.Int64` is per-tool-call, already safe.
- `messageReplacer` callback captures `messages` by reference - tools using this (compact_history) must be ParallelSafe=false (already true).
- `antiPatternCounts` and `alreadyAlerted` maps stay in the serial pre-dispatch phase.
- Output slots: `[]toolCallResult` pre-allocated by original index, written once per goroutine.

## Implementation Details

### toolCallResult struct (internal to runner.go or new file)
```go
type toolCallResult struct {
    call         ToolCall
    output       string
    err          error
    metaMessages []htools.MetaMessage
    duration     time.Duration
    // pre-computed flags
    waitingForUser bool
}
```

### Parallel execution block
```go
type pendingExecution struct {
    idx            int
    call           ToolCall
    callArgs       json.RawMessage
    toolCtx        context.Context
    waitingForUser bool
}

results := make([]toolCallResult, len(pending))
var wg sync.WaitGroup
for _, pe := range pending {
    pe := pe
    wg.Add(1)
    go func() {
        defer wg.Done()
        start := time.Now()
        out, err := r.tools.Execute(pe.toolCtx, pe.call.Name, pe.callArgs)
        results[pe.idx] = toolCallResult{...}
    }()
}
wg.Wait()
```

## Files Changed
1. `internal/harness/types.go` - Add `ParallelSafe bool` to `ToolDefinition`
2. `internal/harness/registry.go` - Store + expose `ParallelSafe`
3. `internal/harness/tools_default.go` - Pass `ParallelSafe` when building definitions
4. `internal/harness/runner.go` - Parallel dispatch loop
5. `internal/harness/runner_parallel_tools_test.go` - NEW: regression tests

## Test Plan
1. `TestParallelToolsExecuteConcurrently` - two parallel-safe tools run concurrently (timing proof).
2. `TestParallelToolsOrderingDeterministic` - transcript order matches call order regardless of finish order.
3. `TestMixedParallelAndSerialTools` - unsafe tools serialize, safe ones parallelize.
4. `TestParallelToolsRaceDetector` - passes `go test -race`.
