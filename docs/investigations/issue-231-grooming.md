# Issue #231 Grooming: bug — GetRunMessages/ConversationMessages Shallow-Copy ToolCalls

## Summary
`GetRunMessages()`, `ConversationMessages()`, and `completeRun()` shallow-copy `[]Message` slices but share the `ToolCalls` backing array with internal runner state. Callers can silently corrupt runner history by mutating the returned slice.

## Already Addressed?
**No.**
- `runner.go:2625` — `GetRunMessages()` uses `append([]Message(nil), state.messages...)` — shallow copy only
- `runner.go:2760` — `ConversationMessages()` same pattern
- `runner.go:2142` — `completeRun()` same pattern
- `types.go:28` — `Message` struct has `ToolCalls []ToolCall` field (not cloned)
- No tests validate ToolCalls isolation from callers
- `deepCloneValue` utility already exists at `runner.go:3415` — usable for the fix

## Clarity
**4/5** — Clear problem statement, affected methods named, root cause explicit. Missing: which specific ToolCalls operations trigger the bug.

## Acceptance Criteria
**Partial:**
- Deep-copy `ToolCalls []ToolCall` in all three methods
- Regression test: caller mutation of ToolCalls does not affect runner state
- Missing: explicit performance impact criteria for deep-copy

## Scope
**Atomic** — Exactly 3 methods to fix, single concern.

## Blockers
None. `deepCloneValue` utility already exists and can be reused.

## Recommended Labels
`bug`, `correctness`, `small`

## Effort
**Small** — ~40–60 lines code + ~100–150 lines tests.
- Replace shallow `append` with deep-clone loop in 3 methods
- Add 2–3 regression tests (mutation scenarios)

## Recommendation
**well-specified** — Ready to implement immediately.
