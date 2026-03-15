# Issue #233 Grooming: bug — deepCloneValue Does Not Clone Struct/Pointer Fields

## Summary
deepCloneValue() (runner.go:3415-3452) only recursively clones maps and slices but returns all other values (including structs with pointer fields) by reference. CompletionUsage structs containing *int pointer fields inserted into event payloads are shared between the stored forensic event and all subscribers.

## Already Addressed?
**No.**
- deepCloneValue() at lines 3415-3452 handles only reflect.Map and reflect.Slice kinds
- Default case (line 3450) returns v as-is for all other types including structs
- CompletionUsage struct has *int pointer fields: CachedPromptTokens, ReasoningTokens, InputAudioTokens, OutputAudioTokens
- recordAccounting() inserts CompletionUsage values into event payloads (line 2440: "cumulative_usage": cumulativeUsage)
- Current tests (runner_forensics_test.go) cover nested maps/slices but NOT struct/pointer fields

## Clarity
**5/5** — Crystal clear with concrete example code demonstrating the problem.

## Acceptance Criteria
**Explicit** — Two proposed solutions:
1. Extend deepCloneValue to handle struct kinds via reflect
2. Convert accounting structs to map[string]any via JSON marshal before inserting into event payloads

## Scope
**Atomic** — Narrowly scoped to deepCloneValue/deepClonePayload and recordAccounting callsites.

## Blockers
None.

## Recommended Labels
bug, correctness, small

## Effort
**Small** — 2–4 hours.
- Recommendation: Option 2 (JSON marshal) — simpler, keeps deepCloneValue focused on map/slice domain

## Recommendation
**well-specified** — Ready to implement immediately. Add regression test verifying struct fields with pointers are cloned, not shared.
