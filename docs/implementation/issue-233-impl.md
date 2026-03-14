# Issue #233 Implementation: deepCloneValue struct/pointer field cloning

## Summary

Fixed a bug where `CompletionUsage` structs (which contain `*int` pointer fields) were being
inserted directly into event payloads by `recordAccounting()`. Because `deepCloneValue()` only
handles `reflect.Map` and `reflect.Slice` kinds, the struct would pass through unchanged, and any
`*int` pointer fields inside would be shared across all event payload copies delivered to
subscribers.

## Root Cause

`deepCloneValue()` (runner.go:3414-3453) has a `default` case that returns non-map, non-slice
values as-is. The comment states this is safe for "scalar" types, but `CompletionUsage` is a struct
with pointer fields:

```go
type CompletionUsage struct {
    PromptTokens       int  `json:"prompt_tokens"`
    CompletionTokens   int  `json:"completion_tokens"`
    TotalTokens        int  `json:"total_tokens"`
    CachedPromptTokens *int `json:"cached_prompt_tokens,omitempty"`
    ReasoningTokens    *int `json:"reasoning_tokens,omitempty"`
    InputAudioTokens   *int `json:"input_audio_tokens,omitempty"`
    OutputAudioTokens  *int `json:"output_audio_tokens,omitempty"`
}
```

When `recordAccounting()` put `CompletionUsage` values directly into the payload map, the
`deepClonePayload()` call in `emit()` would not copy the struct's pointer fields — they remained
shared between the stored forensic event and all subscriber copies.

## Fix Approach: Option 2 — JSON marshal/unmarshal at the callsite

Rather than extending `deepCloneValue` with reflect-based struct traversal (complex, fragile for
structs with unexported fields), the fix converts `CompletionUsage` values to `map[string]any` via
JSON marshal+unmarshal before inserting them into the payload.

This approach:
- Is simple and localized to `recordAccounting()`.
- Uses the existing JSON struct tags to produce idiomatic map keys.
- Completely breaks the reference chain — the resulting map contains only scalar `float64`/`bool`
  values from JSON numbers.
- Is consistent with `UsageDeltaPayload` which already declares `TurnUsage` and `CumulativeUsage`
  as `map[string]any`.

## Files Changed

### `internal/harness/runner.go`

1. Added `completionUsageToMap(u CompletionUsage) map[string]any` helper function that
   JSON-marshals the struct and unmarshals to `map[string]any`. Fallback for the impossible marshal
   error case manually constructs the map from the three non-pointer int fields.

2. Updated `recordAccounting()` to call `completionUsageToMap()` on both `turnUsage` and
   `cumulativeUsage` (and the early-exit `CompletionUsage{}` case) before inserting them into the
   returned payload map.

### `internal/harness/runner_forensics_test.go`

Added `TestAccountingStructPointerFieldIsolation` regression test that:
1. Creates a `CompletionResult` whose `Usage` has non-nil pointer fields (`CachedPromptTokens=42`,
   `ReasoningTokens=7`).
2. Runs the harness to completion.
3. Finds the `usage.delta` event and asserts `cumulative_usage` is a `map[string]any` (not a raw
   `CompletionUsage` struct).
4. Asserts `turn_usage` is also a `map[string]any`.
5. Verifies expected values are present in the map (`prompt_tokens=100`, `completion_tokens=50`,
   `cached_prompt_tokens=42`).
6. Mutates the map from the first subscription.
7. Asserts the second subscription's copy is unaffected (isolated).

The test failed before the fix (got `harness.CompletionUsage`, not `map[string]any`) and passes
after.

## Test Results

```
go test ./internal/harness -run TestAccountingStructPointerFieldIsolation -v
--- PASS: TestAccountingStructPointerFieldIsolation (0.01s)
PASS

go test ./internal/harness/... -race
ok  go-agent-harness/internal/harness   2.490s
... (all pass)

go test ./...
... (all 25 packages pass)
```

## Edge Cases Discovered

- The `completionUsageToMap` fallback (manual map construction from non-pointer fields) is
  unreachable in practice since `CompletionUsage` contains only numeric types that `json.Marshal`
  cannot fail on. It is included for defensive completeness.
- The `omitempty` JSON tags on pointer fields mean that nil `*int` fields will be absent from the
  resulting map rather than present as `null`. This matches the existing `UsageDeltaPayload` type
  which uses `map[string]any` for these fields.

## Commits

1. `eb3d35f` — `test(#233): add regression test for struct/pointer field cloning` (failing test)
2. `37db74d` — `fix(#233): marshal accounting structs to map before inserting into event payloads`
