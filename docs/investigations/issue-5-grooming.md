# Issue #5 Grooming: Add run continuation for multi-turn conversations

## Summary
Enable follow-up messages to completed runs via `POST /v1/runs/{runID}/continue`, maintaining conversation history and cost tracking.

## Evaluation
- **Clarity**: Very clear — proposed endpoint, behavior, and edge cases documented
- **Acceptance Criteria**: Explicit — endpoint behavior, transcript accumulation, status transitions
- **Scope**: Atomic — new endpoint + runner method + state persistence
- **Blockers**: None
- **Effort**: small — mostly already implemented

## Recommended Labels
well-specified, small

## Missing Clarifications
None.

## Notes
**APPEARS ALREADY IMPLEMENTED** — codebase has:
- `runner.go`: `ContinueRun()` method fully implemented
- `http.go`: HTTP routing to `handleRunContinue`
- `http_continuation_test.go`: Comprehensive test suite
- Conversation state preservation, status transitions, SSE resume event

**Action**: Verify with maintainer and close issue if confirmed complete.
