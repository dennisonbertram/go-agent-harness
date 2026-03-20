# Issue #235: Phase 1 Recursive Agent Spawning

## Summary

Phase 1 of issue #235 implements recursive agent spawning via two new deferred
tools (`spawn_agent` and `task_complete`), an integer fork-depth counter
replacing the binary forked-skill flag, and step-budget pressure injection for
subagents. No schema migration was needed — Phase 1 reuses the existing
synchronous `RunForkedSkill` wait path.

## Commits

- `d17ab18` feat(#26): spawner + task_complete files initially added
  (spawn_agent.go, task_complete.go, spawn_agent_test.go)
- `4ff6d9c` feat(#235): integrate spawn_agent+task_complete into harness runner

## Files Changed

### New Files

- `internal/harness/tools/deferred/spawn_agent.go` — The `spawn_agent` deferred
  tool. Depth-gates spawning at `DefaultMaxForkDepth = 5`. Propagates `depth+1`
  to child via `WithForkDepth()`. Parses structured `task_complete` JSON from
  child's output via `parseChildResult()`.

- `internal/harness/tools/deferred/task_complete.go` — The `task_complete`
  deferred tool. Depth-gated at call time (error when `depth == 0`). Emits a
  `{"_task_complete": true, "status": ..., "summary": ..., "findings": [...]}`
  JSON payload that `spawn_agent` parses to extract the structured result.

- `internal/harness/tools/deferred/spawn_agent_test.go` — TDD tests covering
  both tools and the fork-depth context helpers.

### Modified Files

**`internal/harness/tools/types.go`**
- Added `ContextKeyForkDepth contextKey = "fork_depth"` (integer depth counter)
- Added `DefaultMaxForkDepth = 5`
- Added `ForkDepthFromContext(ctx) int` helper
- Added `WithForkDepth(ctx, depth) context.Context` helper

**`internal/harness/types.go`**
- Added `ForkDepth int` field to `RunRequest` — propagated by `RunForkedSkill`
  to child runs so they know their nesting level

**`internal/harness/runner.go`**
- Added `forkDepth int` field to `runState` — set from `req.ForkDepth` in
  `StartRun()`
- Captured `runForkDepth := req.ForkDepth` once at start of `execute()` to
  avoid repeated mutex acquisitions in the hot step loop
- Injected fork depth into `toolCtx` via `htools.WithForkDepth(toolCtx, runForkDepth)`
  so deferred tools can read it without acquiring the runner mutex
- Updated `RunForkedSkill()` to read depth from context
  (`htools.ForkDepthFromContext(ctx)`) and pass it as `ForkDepth` in the child
  `RunRequest`, completing the depth propagation chain
- Added step-budget pressure injection: for subagents (`runForkDepth > 0`),
  warning messages are injected at `stepsRemaining == 3` and `stepsRemaining == 1`,
  and `EventStepBudgetPressure` is emitted each time

**`internal/harness/tools/core/skill.go`**
- Replaced binary `ContextKeyForkedSkill` nested-fork check with integer depth
  counter: `currentDepth >= tools.DefaultMaxForkDepth`
- Kept the legacy `ContextKeyForkedSkill` context marker for backward compatibility

**`internal/harness/tools_default.go`**
- Registered `SpawnAgentTool` and `TaskCompleteTool` when
  `EnableAgent && AgentRunner != nil`

**`internal/harness/events.go`**
- Added 4 new event types:
  - `EventSpawnAgentStarted` (`spawn_agent.started`)
  - `EventSpawnAgentCompleted` (`spawn_agent.completed`)
  - `EventTaskCompleted` (`task.completed`)
  - `EventStepBudgetPressure` (`step_budget.pressure`)
- Added all 4 to `AllEventTypes()`

**`internal/harness/events_test.go`**
- Updated `TestAllEventTypes_Count` from 72 → 76

**`internal/harness/tools/core/skill_test.go`**
- Updated `TestSkillTool_Handler_ForkNestedPrevention` to use depth-based
  context (`tools.WithForkDepth(ctx, tools.DefaultMaxForkDepth)`) and expect
  `"max recursion depth"` error

## Architecture

### Depth Counter Flow

```
Root agent (depth=0)
  → calls spawn_agent tool
  → spawn_agent reads depth=0 from toolCtx
  → spawn_agent calls forkedRunner.RunForkedSkill(childCtx, ...)
     where childCtx has depth=1
  → RunForkedSkill passes ForkDepth=1 in child RunRequest
  → child's execute() captures runForkDepth=1
  → child's toolCtx has depth=1 injected
  → task_complete tool sees depth=1 → allowed
  → spawn_agent tool in child sees depth=1 < 5 → can spawn grandchild
```

### task_complete Output Parsing

The `task_complete` tool emits structured JSON with a `_task_complete: true`
sentinel:

```json
{
  "_task_complete": true,
  "status": "completed",
  "summary": "...",
  "findings": [{"type": "finding", "content": "..."}]
}
```

`spawn_agent`'s `parseChildResult()` attempts to unmarshal this from
`ForkResult.Output`. On success, it extracts `status`, `summary`, and
`findings` and returns them as the `spawn_agent` tool result. On failure
(plain-text output), it wraps the text in a generic result.

### Step-Budget Pressure

For subagents (`runForkDepth > 0`) with a finite step budget
(`effectiveMaxSteps > 0`), the runner injects pressure messages at:
- `stepsRemaining == 3`: "SYSTEM: You have 3 steps remaining... Call task_complete soon"
- `stepsRemaining == 1`: "SYSTEM: You have 1 step remaining. You MUST call task_complete now."

Each injection also emits `EventStepBudgetPressure` with `step`, `steps_remaining`,
and `depth` fields.

## Phase 2 Scope (Not Implemented)

Phase 2 would add:
- True async suspension with DB-backed resume (no blocking of parent goroutine)
- Model override (`model` parameter on spawn_agent)
- Profile inheritance (`profile` parameter on spawn_agent)
- Auto-synthetic `task_complete` call when step budget hits 0

## Test Results

```
ok  go-agent-harness/internal/harness          (all events, runner integration)
ok  go-agent-harness/internal/harness/tools    (tool helpers)
ok  go-agent-harness/internal/harness/tools/core  (skill fork depth test)
ok  go-agent-harness/internal/harness/tools/deferred  (spawn_agent + task_complete)
coveragegate: PASS (total=85.1%, min=80.0%, zero-functions=0)
```
