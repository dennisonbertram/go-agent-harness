# Issue #31 Grooming: Delayed callbacks: agent-triggered one-shot continuations

## Summary
Allow agents to schedule a future continuation of a conversation after a delay.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented:
- `internal/harness/tools/delayed_callback.go`: Complete `CallbackManager` with `Set()`, `Cancel()`, `List()` methods
- Tools: `set_delayed_callback`, `cancel_delayed_callback`, `list_delayed_callbacks`
- In-process timers via `time.AfterFunc()`
- `MaxCallbacksPerConv = 10` limit enforced
- Tests in `internal/harness/tools/delayed_callback_test.go`

Note: Callbacks are in-memory (lost on harnessd restart). Persistence across restarts is future work and tracked separately.

## Acceptance Criteria
All core criteria met. Persistence-across-restart is Phase 2 (separate issue).

## Scope
Core feature complete.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close. File new issue for persistence-across-restart if desired.
