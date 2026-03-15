# Issue #75 Grooming: Add tool middleware / lifecycle hooks (PreToolUse, PostToolUse)

## Summary
Add PreToolUse and PostToolUse hook interfaces so external code can observe, modify, or block tool invocations.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented:
- `PreToolUseHook` and `PostToolUseHook` interfaces in `internal/harness/types.go`
- `PreToolUseEvent`, `PostToolUseEvent`, `PreToolUseResult`, `PostToolUseResult` types
- Hook registration via `RunnerConfig.PreToolUseHooks` and `PostToolUseHooks` slices
- `applyPreToolUseHooks()` (line 1136) and `applyPostToolUseHooks()` (line 1231) in `runner.go`
- Panic recovery via safe-call wrappers
- 15+ test cases in `tool_hooks_test.go` covering all scenarios
- Decision priority: Deny > Ask > Allow implemented
- Zero overhead when no hooks registered

Merged commit: `be5f459`

## Clarity Assessment
Clear.

## Acceptance Criteria
All met.

## Scope
Atomic.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close this issue.
