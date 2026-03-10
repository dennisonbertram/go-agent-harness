# Issue #10: Cost Ceiling and Safety Controls for Unlimited Runs

## Summary

Implemented per-run cost ceiling (`max_cost_usd`) for the agent runner. When a run's cumulative LLM cost reaches or exceeds the configured ceiling, the run is terminated gracefully with a `run.cost_limit_reached` event followed by `run.completed` (not `run.failed`).

## Changes

### `internal/harness/types.go`
- Added `MaxCostUSD float64` field to `RunRequest` with JSON tag `max_cost_usd,omitempty`
- Added `maxCostUSD float64` field to `runState` (unexported; stored from request)

### `internal/harness/events.go`
- Added `EventRunCostLimitReached EventType = "run.cost_limit_reached"` constant in the run lifecycle events block
- Added the new constant to `AllEventTypes()`

### `internal/harness/runner.go`
- Added validation in `StartRun`: negative `MaxCostUSD` is rejected with an error mentioning `max_cost_usd`
- Stored `req.MaxCostUSD` in `runState.maxCostUSD` when initializing the run state
- Added `exceedsCostCeiling(runID string) bool` method — checks if the accumulated cost has reached `maxCostUSD` (returns false when ceiling is 0/unset or cost is unavailable)
- Added cost ceiling check in the main run loop immediately after `recordAccounting` and the `EventUsageDelta` / `EventLLMTurnCompleted` emits; emits `EventRunCostLimitReached` with payload `{step, max_cost_usd, cumulative_cost_usd}` and calls `completeRun` (not `failRun`)

## Design Decisions

- **Complete, not fail**: A run that hits its budget limit did real work and stopped gracefully. Using `run.completed` (not `run.failed`) reflects this — it matches how `max_steps` exhaustion is distinct from an error.
- **Unpriced models are safe**: If `CostStatus != CostStatusAvailable` (e.g., model is unpriced or provider doesn't report costs), the ceiling is never triggered. This prevents false halts on models where cost data is unavailable.
- **Exact boundary triggers**: The comparison is `>=`, so a run with ceiling $0.005 stops after the first turn that brings total to exactly $0.005.
- **Zero means unlimited**: `MaxCostUSD = 0` (the default JSON zero value) means no ceiling, consistent with how `MaxSteps = 0` means unlimited.

## Tests Added

In `internal/harness/runner_test.go` (5 new test functions):
1. `TestCostCeiling_RunCompletesWhenCeilingExceeded` — two turns at $0.002 each, ceiling $0.003; stops after 2nd turn
2. `TestCostCeiling_NegativeIsInvalid` — negative `MaxCostUSD` rejected at `StartRun`
3. `TestCostCeiling_ZeroMeansUnlimited` — $3.00 in turns with no ceiling; all complete normally
4. `TestCostCeiling_UnpricedModelDoesNotTrigger` — unpriced model never triggers ceiling even with tiny limit
5. `TestCostCeiling_CeilingAtExactBoundary` — exactly $0.005 turn with $0.005 ceiling; triggers on first turn

In `internal/harness/events_test.go`:
- Updated `TestAllEventTypes_Count` from 39 to 40
- Added `TestEventRunCostLimitReachedType` verifying string value, non-terminal status, and presence in `AllEventTypes()`

## Test Results

```
go test ./... -race
ok  go-agent-harness/internal/harness  2.085s
(all other packages pass; demo-cli pre-existing build failure unchanged)
```
