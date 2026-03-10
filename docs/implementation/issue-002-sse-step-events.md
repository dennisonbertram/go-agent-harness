# Issue #2: SSE Step Events and Structured Max-Steps Failure

## Summary

Added `run.step.started` and `run.step.completed` SSE events to the runner loop,
and a structured `reason="max_steps_reached"` field on `run.failed` events so clients
can distinguish step-budget exhaustion from other failures without parsing the error string.

## What Was Implemented

### New Event Types (`internal/harness/events.go`)

- `EventRunStepStarted` (`"run.step.started"`) — emitted at the start of each loop iteration, before `llm.turn.requested`
- `EventRunStepCompleted` (`"run.step.completed"`) — emitted at the end of each loop iteration, after tool calls complete

Both events carry a `"step"` field (1-indexed) and `run.step.completed` additionally carries `"tool_calls"` (count of tool calls made in that step).

### Structured Max-Steps Reason (`internal/harness/runner.go`)

Added `failRunMaxSteps()` — a specialisation of `failRun()` called when the step
loop exhausts its budget.  The `run.failed` event emitted by this path includes:

```json
{
  "error": "max steps (N) reached",
  "reason": "max_steps_reached",
  "max_steps": N,
  "usage_totals": {...},
  "cost_totals": {...}
}
```

Other failure paths (provider errors, hook blocks, tool timeouts) do NOT carry
`reason="max_steps_reached"`, so clients can branch on the `reason` field without
fragile string matching on `error`.

### AllEventTypes() Updated

Both new event types are registered in `AllEventTypes()` (count updated from 36 to 38)
and the `TestAllEventTypes_Count` expectation was updated accordingly.

## Tests Added (`internal/harness/runner_test.go`)

Three new tests:

1. `TestRunnerEmitsStepStartedAndCompletedEvents` — verifies that a 2-step run
   (1 tool call + 1 terminal turn) emits exactly 2 `run.step.started` and 2
   `run.step.completed` events, each with the correct step number, and that
   event ordering is `run.started > run.step.started > llm.turn.requested >
   run.step.completed > run.completed`.

2. `TestRunnerMaxStepsReachedEmitsStructuredReason` — verifies that when the
   step loop exhausts MaxSteps the resulting `run.failed` event carries
   `reason="max_steps_reached"` and `max_steps=N`.

3. `TestRunnerNonMaxStepsFailureHasNoMaxStepsReason` — verifies that a provider
   error (not max-steps) does NOT carry `reason="max_steps_reached"`, so the
   structured reason is exclusive to the budget-exhaustion path.

## Files Changed

| File | Change |
|------|--------|
| `internal/harness/events.go` | Added `EventRunStepStarted`, `EventRunStepCompleted`; registered in `AllEventTypes()` |
| `internal/harness/events_test.go` | Updated `TestAllEventTypes_Count` expectation 36 to 38 |
| `internal/harness/runner.go` | Emit step events in loop; added `failRunMaxSteps()` |
| `internal/harness/runner_test.go` | Added 3 new regression tests |

## TDD Process

1. Wrote failing tests referencing `EventRunStepStarted`/`EventRunStepCompleted` — compile error confirmed
2. Added event constants and `AllEventTypes()` entries
3. Emitted events in runner loop
4. Added `failRunMaxSteps()` with structured payload
5. All 3 new tests pass; full suite passes (only pre-existing `demo-cli` build failure remains)
