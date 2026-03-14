# Issue #233 Implementation Plan: deepCloneValue Does Not Clone Struct/Pointer Fields

## Root Cause Analysis

`deepCloneValue()` in `internal/harness/runner.go` (lines 3414-3453) handles map and slice kinds
recursively, but its default case returns every other value type (including structs) as-is:

```go
default:
    // Scalars (string, bool, int*, uint*, float*) are value types —
    // no aliasing is possible through an interface{}.
    return v
```

This comment is correct for Go primitive scalars but misleading: structs that contain pointer fields
are NOT value-safe. Specifically, `CompletionUsage` has four `*int` pointer fields:

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

`recordAccounting()` inserts a `CompletionUsage` directly into the event payload:

```go
return map[string]any{
    ...
    "turn_usage":       turnUsage,        // CompletionUsage struct value
    "cumulative_usage": cumulativeUsage,  // CompletionUsage struct value
    ...
}
```

When `deepClonePayload()` processes this map, it calls `deepCloneValue()` on each value. The
`CompletionUsage` struct passes through the default case unchanged. The struct's value fields
(`PromptTokens`, `CompletionTokens`, `TotalTokens`) are copied by value into the interface box, so
they are safe. However the `*int` pointer fields (`CachedPromptTokens`, `ReasoningTokens`, etc.)
inside the struct are shared between the event stored in forensic history and any subscriber copies.

If anything mutates the pointed-to int via the pointer, or replaces the pointer itself by
reassigning the outer struct, all consumers of the event payload that hold a reference to the same
struct would be affected.

## Bug Scenario

1. `completionUsage()` creates a `CompletionUsage` with `CachedPromptTokens = &n` (pointer to a
   local int).
2. `recordAccounting()` puts the struct into the payload map.
3. `emit()` calls `deepClonePayload()`, which calls `deepCloneValue(cumulativeUsage)`.
4. The struct value is returned as-is. The `*int` pointer is shared with the stored forensic event.
5. Any subsequent mutation of the pointed-to integer (or replacement of the struct in the original
   accumulator) would be visible to existing event payload holders.

## Chosen Fix: Option 2 — JSON Marshal/Unmarshal in recordAccounting()

Rather than extending `deepCloneValue` to handle arbitrary structs via reflect (which would be
complex and fragile), we convert `CompletionUsage` values to `map[string]any` at the callsite
before inserting them into the event payload.

This approach:
- Is simple and localized to `recordAccounting()`.
- Uses the existing JSON struct tags on `CompletionUsage` to produce idiomatic map keys.
- Completely breaks the reference chain — the resulting `map[string]any` contains only scalar values
  (JSON numbers map to `float64`, JSON booleans to `bool`).
- Keeps `deepCloneValue` focused on its current domain (maps and slices).
- Is consistent with how JSON-serialized events are eventually transmitted anyway.

### Helper Function

We will add a `completionUsageToMap(u CompletionUsage) map[string]any` helper that JSON-marshals
the struct and unmarshals to `map[string]any`. On marshal error (which cannot happen for a plain
numeric struct), it falls back to a manually constructed map.

### Alternative Considered: Extend deepCloneValue with reflect

Option 1 (extending `deepCloneValue` to handle `reflect.Struct` and `reflect.Ptr` kinds) was
considered but rejected because:
- Requires recursive traversal of arbitrary struct fields.
- Would need to handle unexported fields (which reflect cannot set).
- `encoding/json` already provides a correct, tested implementation of this via marshal+unmarshal.
- The accounting struct is the only struct type known to be inserted into event payloads.

## Files to Change

1. `internal/harness/runner.go` — modify `recordAccounting()` to marshal `turnUsage` and
   `cumulativeUsage` before inserting them into the payload.
2. `internal/harness/runner_forensics_test.go` — add regression test.

## Testing Strategy

### Regression Test (to write FIRST — must FAIL before fix)

`TestAccountingStructPointerFieldIsolation` in `runner_forensics_test.go`:

1. Use a `stubProvider` that produces a `CompletionResult` with a `Usage` containing non-nil
   pointer fields (`CachedPromptTokens`, `ReasoningTokens`).
2. Run the harness to completion.
3. Collect two separate Subscribe batches.
4. In each batch, find the `usage.delta` event and extract `cumulative_usage` from the payload.
5. Assert the value is a `map[string]any` (not a `CompletionUsage` struct) — this demonstrates
   JSON serialization happened.
6. Alternatively, assert that two separate Subscribe calls yield independent copies of the
   pointer-fielded data (mutation of one does not affect the other).

### Why It Fails Before Fix

Before the fix, `cumulative_usage` in the payload is a `CompletionUsage` struct value. When
`deepClonePayload` processes the event map, the struct passes through unchanged. The `*int` pointer
fields inside the struct are shared across all event payload copies served by `Subscribe`.

Mutating the `int` pointed to by `CachedPromptTokens` through one subscriber's payload would
corrupt the stored forensic event and all other subscribers' copies.

The test verifies:
- `cumulative_usage` in the stored event is a `map[string]any` (after the fix), not the raw struct.
- The `turn_usage` field similarly becomes a `map[string]any`.

## Exact Code Changes

### runner.go — recordAccounting()

Change:
```go
return map[string]any{
    "step":                step,
    "usage_status":        usageStatus,
    "cost_status":         costStatus,
    "turn_usage":          turnUsage,
    "turn_cost_usd":       turnCostUSD,
    "cumulative_usage":    cumulativeUsage,
    "cumulative_cost_usd": costTotals.CostUSDTotal,
    "pricing_version":     costTotals.PricingVersion,
}
```

To:
```go
return map[string]any{
    "step":                step,
    "usage_status":        usageStatus,
    "cost_status":         costStatus,
    "turn_usage":          completionUsageToMap(turnUsage),
    "turn_cost_usd":       turnCostUSD,
    "cumulative_usage":    completionUsageToMap(cumulativeUsage),
    "cumulative_cost_usd": costTotals.CostUSDTotal,
    "pricing_version":     costTotals.PricingVersion,
}
```

And the early-exit case:
```go
return map[string]any{
    "step":                step,
    "usage_status":        usageStatus,
    "cost_status":         costStatus,
    "turn_usage":          completionUsageToMap(turnUsage),
    "turn_cost_usd":       turnCostUSD,
    "cumulative_usage":    completionUsageToMap(CompletionUsage{}),
    "cumulative_cost_usd": 0.0,
    "pricing_version":     pricingVersion,
}
```

### runner.go — new helper

```go
// completionUsageToMap converts a CompletionUsage struct into a map[string]any
// using its JSON representation. This breaks all pointer aliases: the returned
// map contains only scalar values (float64 for numbers) safe for insertion into
// event payloads that will be deep-cloned and distributed to subscribers.
func completionUsageToMap(u CompletionUsage) map[string]any {
    b, err := json.Marshal(u)
    if err != nil {
        // CompletionUsage contains only numeric types; marshal cannot fail.
        return map[string]any{
            "prompt_tokens":     u.PromptTokens,
            "completion_tokens": u.CompletionTokens,
            "total_tokens":      u.TotalTokens,
        }
    }
    var m map[string]any
    if err := json.Unmarshal(b, &m); err != nil {
        return map[string]any{
            "prompt_tokens":     u.PromptTokens,
            "completion_tokens": u.CompletionTokens,
            "total_tokens":      u.TotalTokens,
        }
    }
    return m
}
```
