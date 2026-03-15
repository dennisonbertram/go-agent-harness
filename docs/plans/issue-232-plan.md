# Issue #232 Implementation Plan: Fix CompactRun Nondeterministic No-Op

## Root Cause

execute() reads state.messages once at start (~line 913) into a local `messages` variable held for the entire run. When CompactRun() updates state.messages concurrently, execute() overwrites it with its stale local copy on the next setMessages() call.

**Race timeline:**
1. execute() captures `messages = [msg1..msg5]` at line 913
2. CompactRun() compacts to `[summary, msg4, msg5]` and calls setMessages()
3. execute() calls setMessages(runID, messages) with stale 5-message slice
4. Compaction is lost

## Fix

Re-read `state.messages` at the start of each step under `state.compactMu`:

```go
// At top of step loop (line 987):
state.compactMu.Lock()
messages := copyMessages(state.messages)
state.compactMu.Unlock()
```

Remove the one-time initialization at line 913.

## Key Constraints
- Only hold compactMu for the brief slice copy (never for LLM calls/tools)
- CompactRun() also locks compactMu before setMessages() → synchronized
- autoCompactMessages() also uses compactMu → synchronized

## Files to Change
- internal/harness/runner.go only

## Regression Tests (runner_context_compact_test.go)
1. CompactRun while execute() mid-step → final messages reflect compaction
2. Concurrent execute + CompactRun under -race flag
3. CompactRun at step boundary (just before/after setMessages) → correct result

## Commit Plan
- test(#232): add regression tests for concurrent CompactRun + execute
- fix(#232): re-read state.messages each step to prevent CompactRun overwrite
