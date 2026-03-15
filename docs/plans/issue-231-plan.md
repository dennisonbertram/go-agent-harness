# Issue #231 Implementation Plan: Fix ToolCalls Shallow-Copy in Message Export Methods

## Summary
Three message-export methods shallow-copy Message slices but share ToolCalls backing arrays with internal runner state. Callers can mutate returned ToolCalls and silently corrupt runner history.

## Bug Locations (internal/harness/runner.go)
- GetRunMessages() ~line 2625 — `append([]Message(nil), state.messages...)`
- ConversationMessages() ~line 2760 — same pattern
- completeRun() ~line 2142 — same pattern

## Types
ToolCall: {ID, Name, Arguments string} — no pointers, simple value type
Message.ToolCalls []ToolCall — slice header is copied but backing array is shared

## Fix Approach: Add deepCloneMessage() helper
Add near deepCloneValue (~line 3452):
```go
func deepCloneMessage(m Message) Message {
    if len(m.ToolCalls) > 0 {
        tc := make([]ToolCall, len(m.ToolCalls))
        copy(tc, m.ToolCalls)
        m.ToolCalls = tc
    }
    return m
}
```

Add copyMessages() helper:
```go
func copyMessages(msgs []Message) []Message {
    result := make([]Message, len(msgs))
    for i := range msgs {
        result[i] = deepCloneMessage(msgs[i])
    }
    return result
}
```

Replace all 3 `append([]Message(nil), ...)` sites with `copyMessages(...)`.

## Files to Change
1. internal/harness/runner.go — add helpers, fix 3 sites
2. internal/harness/runner_forensics_test.go — add TestMessageExportMutationIsolation

## Test Strategy
- Create run with messages containing ToolCalls
- Call GetRunMessages() → mutate returned ToolCalls
- Verify runner state unchanged (second GetRunMessages call shows original data)
- Repeat for ConversationMessages()
- Run with -race flag

## Commit Plan
- test(#231): add regression test for ToolCalls mutation isolation
- fix(#231): deep-copy ToolCalls in message export methods
