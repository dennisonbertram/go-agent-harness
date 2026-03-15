# Issue #6 Grooming: Add mid-run steering: let users inject guidance while model is executing

## Summary
Allow users to send guidance into running executions via `POST /v1/runs/{runID}/steer`, buffering messages for injection on the next step.

## Evaluation
- **Clarity**: Very clear — endpoint design, implementation, edge cases all detailed
- **Acceptance Criteria**: Explicit — buffering behavior, concurrency, error handling
- **Scope**: Atomic — new endpoint + runner integration
- **Blockers**: None
- **Effort**: small — mostly already implemented

## Recommended Labels
well-specified, small

## Missing Clarifications
None.

## Notes
**APPEARS ALREADY IMPLEMENTED** — codebase has:
- `runner.go`: `steeringCh chan string` in runState (buffered channel), `drainSteering()`, `SteerRun()`, steering drain called at step start
- `http.go`: HTTP routing to `handleRunSteer`
- Error handling for `ErrSteeringBufferFull`

**Action**: Verify with maintainer and close issue if confirmed complete.
