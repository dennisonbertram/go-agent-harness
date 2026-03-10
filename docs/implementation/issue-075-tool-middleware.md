# Issue #75: Add Tool Middleware / Lifecycle Hooks (PreToolUse, PostToolUse)

## Summary

Implemented `PreToolUseHook` and `PostToolUseHook` interfaces that intercept
individual tool calls before and after execution. These are distinct from the
existing pre/post *message* hooks (which operate at the LLM-turn level).

## Files Changed

- `internal/harness/types.go` — Added `ToolHookDecision`, `PreToolUseEvent`,
  `PreToolUseResult`, `PostToolUseEvent`, `PostToolUseResult`,
  `PreToolUseHook`, `PostToolUseHook` types. Added `PreToolUseHooks` and
  `PostToolUseHooks` slices to `RunnerConfig`.

- `internal/harness/events.go` — Added `EventToolHookStarted`,
  `EventToolHookFailed`, `EventToolHookCompleted` constants and included them
  in `AllEventTypes()`. Updated `TestAllEventTypes_Count` expected count from
  36 to 39.

- `internal/harness/runner.go` — Added `applyPreToolUseHooks()`,
  `applyPostToolUseHooks()`, `safeCallPreToolUseHook()`, and
  `safeCallPostToolUseHook()` methods. Wired them into the tool execution
  loop in `execute()`.

- `internal/harness/events_test.go` — Updated count assertion to 39, added
  `TestEventToolHookTypes` to verify the new event constants.

- `internal/harness/tool_hooks_test.go` (new) — 21 tests covering all
  acceptance criteria from the issue.

## Design Decisions

### Hook Invocation Order (Pre)

Pre-tool-use hooks are called in registration order. The first hook to return
`ToolHookDeny` stops the chain immediately (short-circuits). This mirrors the
behavior of the existing pre-message hooks.

### Arg Modification Chain

Each PreToolUseHook receives the args as modified by any earlier hook in the
chain (not the original LLM args). The last hook's `ModifiedArgs` (if set)
wins.

### Post Hook Result Semantics

`PostToolUseResult.ModifiedResult` is only applied when non-empty. An empty
string means "use original result unchanged". This aligns with the issue design.

For error results (toolErr != nil), the post hook receives `ev.Result = ""`
and `ev.Error != nil`. If the hook returns a non-empty `ModifiedResult`, it
replaces the standard JSON error envelope sent to the LLM; otherwise the
standard `{"error": "..."}` JSON is used.

### Panic Recovery

Both `safeCallPreToolUseHook` and `safeCallPostToolUseHook` use `recover()`
to catch panics and return them as errors. The `HookFailureMode` then
determines whether the panic is ignored (fail_open) or causes the tool to be
denied/return original output (fail_closed).

### Event Naming

New events use `tool_hook.{started,failed,completed}` (underscore separator)
to distinguish them from the existing `hook.{started,failed,completed}` events
(which are message-level hooks). The `stage` payload field is set to
`"pre_tool_use"` or `"post_tool_use"`.

### Zero Overhead

When no hooks are registered (`len(hooks) == 0`), the methods return
immediately without any allocation.

## Test Coverage

21 tests in `tool_hooks_test.go`:

1. Allow passes through
2. Deny blocks tool execution
3. Pre-hook modifies args
4. Post-hook modifies result
5. Post-hook receives error in event
6. Pre-hook receives correct fields (ToolName, CallID, Args, RunID)
7. Post-hook receives correct fields (Duration, RunID)
8. Multiple hooks called in order
9. Deny stops chain (later hooks not called)
10. Pre-hook error fail_open (hook skipped, tool executes)
11. Pre-hook error fail_closed (tool denied)
12. Post-hook error fail_open
13. Empty hook registries (zero overhead)
14. Nil ModifiedArgs uses original
15. Nil ModifiedResult uses original
16. Concurrent pre-tool-use hooks (race-safe)
17. Tool hook events emitted for pre and post stages
18. Hook panic recovery (fail_open → tool still executes)
19. Duration is positive for a slow tool
20. Nil result treated as Allow
21. Concurrent registration safety

All pass with `-race`.
