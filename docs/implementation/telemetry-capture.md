# Telemetry Capture: Phase 1

## Summary

Added a `GET /v1/runs/{id}/summary` endpoint that returns structured telemetry for completed runs, and updated `agent.py` to fetch and persist this data after each bench trial.

## Changes

### Go Backend

**`internal/harness/types.go`**
- Added `RunSummary` struct with fields: `run_id`, `status`, `steps_taken`, `total_prompt_tokens`, `total_completion_tokens`, `total_cost_usd`, `cost_status`, `tool_calls`, `cache_hit_rate`, `error`.
- Added `ToolCallSummary` struct (`tool_name`, `step`).

**`internal/harness/runner.go`**
- Added `GetRunSummary(runID)` method that scans event history to compute:
  - Step count from `EventLLMTurnRequested` events
  - Tool call sequence from `EventToolCallStarted` events
  - Token totals from the `usageTotalsAccumulator`
  - Cost totals from `RunCostTotals`
  - Cache hit rate from cached vs total prompt tokens
- Returns `ErrRunNotFound` if run doesn't exist, error if run is still in progress.

**`internal/server/http.go`**
- Added `summary` sub-route under `/v1/runs/{id}/summary`
- Added `handleRunSummary` handler: GET-only, returns 404 for missing runs, 409 for in-progress runs, 200 with `RunSummary` JSON.

**`internal/server/http_test.go`**
- `TestRunSummaryEndpoint`: Full lifecycle test with a scripted 2-turn provider (one tool call, one final response) verifying all summary fields.
- `TestRunSummaryNotFound`: 404 test for missing run IDs.

### Python Agent Bridge

**`benchmarks/terminal_bench/agent.py`**
- After harnesscli completes, extracts `run_id` from terminal output via regex.
- Fetches `GET /v1/runs/{id}/summary` via curl inside the container.
- Writes `harness_telemetry.json` to the logging directory.
- Passes `total_input_tokens` and `total_output_tokens` to `AgentResult` (with fallback if the framework doesn't support those fields).

## Endpoint Reference

```
GET /v1/runs/{id}/summary

Response 200:
{
  "run_id": "run_1",
  "status": "completed",
  "steps_taken": 3,
  "total_prompt_tokens": 1500,
  "total_completion_tokens": 400,
  "total_cost_usd": 0.012,
  "cost_status": "available",
  "tool_calls": [
    {"tool_name": "bash", "step": 1},
    {"tool_name": "read_file", "step": 2}
  ],
  "cache_hit_rate": 0.15,
  "error": ""
}
```
