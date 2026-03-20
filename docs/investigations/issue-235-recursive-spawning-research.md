# Issue #235 Research: Recursive Agent Spawning with DB-Backed Suspension, Result Pointers, and JSONL Grep

**Date**: 2026-03-18
**Issue**: https://github.com/dennisonbertram/go-agent-harness/issues/235
**Related closed blockers**: #234 (per-run tool filtering), #236 (config propagation to subagents)

---

## 1. What Already Exists

### 1.1 ForkedAgentRunner Interface (P6 from Skills system)

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/types.go` (lines 189–210)

The interface is fully defined and implemented:

```go
// ForkConfig holds configuration for a forked skill execution.
type ForkConfig struct {
    Prompt       string
    SkillName    string
    Agent        string
    AllowedTools []string
    Metadata     map[string]string
}

type ForkResult struct {
    Output  string
    Summary string
    Error   string
}

// ForkedAgentRunner extends AgentRunner with support for forked skill execution.
type ForkedAgentRunner interface {
    AgentRunner
    RunForkedSkill(ctx context.Context, config ForkConfig) (ForkResult, error)
}
```

### 1.2 Runner.RunForkedSkill Implementation

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/runner.go` (lines 1021–1057)

The runner already implements `ForkedAgentRunner`. It:
1. Inherits `SystemPrompt`, `Permissions`, and `ProfileName` from the parent run
2. Forwards `AllowedTools` from `ForkConfig`
3. Calls `StartRun` synchronously and blocks via `waitForTerminalResult`
4. Does NOT record `parent_run_id`, depth, or path

The key limitation is at `internal/harness/tools/core/skill.go` line 122–125:

```go
func handleForkSkill(ctx context.Context, runner tools.AgentRunner, info tools.SkillInfo, content string) (string, error) {
    // Prevent nested forking
    if _, nested := ctx.Value(tools.ContextKeyForkedSkill).(string); nested {
        return "", fmt.Errorf("nested skill forking is not supported")
    }
    // ...
    forkCtx = context.WithValue(forkCtx, tools.ContextKeyForkedSkill, info.Name)
```

This is a binary flag — a single string value in the context. `ContextKeyForkedSkill` is defined as a `contextKey("forked_skill")` constant in `tools/types.go` line 286.

### 1.3 http_agents.go

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/server/http_agents.go`

Handles `POST /v1/agents`. Accepts `prompt` or `skill` (mutually exclusive). Forwards `AllowedTools` to the fork config. Has a 120s default / 600s max timeout. Returns `{output, summary, duration_ms}`.

This is the synchronous agent endpoint — it blocks until the child run completes. No suspension, no result pointers.

### 1.4 http_subagents.go and internal/subagents/manager.go

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/server/http_subagents.go`
**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/subagents/manager.go`

This is a richer, asynchronous subagent manager with:
- `IsolationMode`: `inline` or `worktree` (worktree creates an isolated git worktree)
- `CleanupPolicy`: `preserve`, `destroy_on_success`, `destroy_on_completion`
- A background `monitor` goroutine that subscribes to the child run's SSE stream
- Full `RunRequest` forwarding: model, provider, AllowedTools, ProfileName, MaxCostUSD, Permissions

However, the subagent manager is **not wired to the parent's step loop**. When a parent run calls a tool, it must wait synchronously. The manager is an HTTP-level construct — callers submit `POST /v1/subagents`, the server starts the child run, and callers poll `GET /v1/subagents/{id}` until completion. There is no in-runner "wait and inject result back into parent context".

### 1.5 JSONL Rollout Infrastructure

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/rollout/recorder.go`

A per-run JSONL recorder already writes every run event to:
```
<RolloutDir>/<YYYY-MM-DD>/<run_id>.jsonl
```

Format per line:
```json
{"ts":"...","seq":N,"type":"...","data":{...}}
```

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/forensics/rollout/loader.go`

Loads and validates JSONL rollout files (max 16MiB per line, 100k events, 256MiB total). This is used by the replay/forensics tools (`internal/server/http_replay.go`).

### 1.6 SQLite Store

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/store/sqlite.go`

Current schema has `runs`, `run_messages`, `run_events` tables. The `runs` table has no `parent_run_id`, `depth`, `path`, or `status` columns beyond what lives in the `harness.Run` struct. There is no `run_results` table.

### 1.7 Closed Blockers — What They Delivered

#### #234: Per-Run Tool Filtering (CLOSED)

Delivered:
- `AllowedTools []string` added to `RunRequest` (already confirmed present at line 317 of types.go)
- Bootstrap `SkillConstraint` in `StartRun()` pre-registers the allowed tool list before the step loop starts
- `IsBootstrap bool` field on `SkillConstraint` prevents the constraint from being deactivated when a skill completes
- `http_agents.go` and `RunForkedSkill` forward `allowed_tools`, `system_prompt`, `permissions`

This means the #235 prerequisite for controlled child tool sets is fully met.

#### #236: Deterministic Config Propagation to Subagent Workspaces (CLOSED)

Delivered:
- `workspace.Options.ConfigTOML string` field — each `Provision` implementation writes it to `<path>/.harness/config.toml`
- `symphd.Dispatcher` populates `ConfigTOML` from a typed `WorkspaceRunnerConfig` at dispatch time
- All `RunnerConfig` feature flags now have TOML keys
- `build_harness_config` tool for agents to declare subagent config explicitly
- Container workspaces pass API keys via `opts.Env` (not disk)
- `manager.go` already uses `configTOML` in `Options` and `provisionOpts`

This means proper config isolation for worktree-isolated subagents is solved.

### 1.8 RunStatus Constants

**File**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/types.go` (lines 224–233)

Current statuses: `queued`, `running`, `waiting_for_user`, `waiting_for_approval`, `completed`, `failed`, `cancelled`.

Missing for #235: `waiting_for_child` (parent suspended waiting for child), `abandoned` (child terminated without `task_complete`).

---

## 2. Concrete Architecture for Recursive Spawning + DB Suspension

### 2.1 The Core Problem: Synchronous Blocking vs. True Suspension

The current `RunForkedSkill` implementation **blocks the parent goroutine** while the child runs. This means:
- The parent's step loop goroutine is pinned for the entire child duration
- No possibility of server restart resumption (the blocked goroutine is gone after restart)
- Worker pool slots are occupied for the full child run lifetime

True DB-backed suspension requires:
1. Parent calls `spawn_agent` tool
2. Runner stores `continuation_tool_call_id` (the tool call ID the parent is waiting on) and sets `status = waiting_for_child`
3. Parent's step loop goroutine **exits** — the goroutine is not pinned
4. When child completes, a completion hook loads the parent, injects the result as a tool return, and re-dispatches the parent's step loop
5. On server restart, any `waiting_for_child` runs are scanned and re-checked against their children's statuses

### 2.2 Schema Changes Required

```sql
-- Add to runs table
ALTER TABLE runs ADD COLUMN parent_run_id TEXT;                  -- direct parent (null for root)
ALTER TABLE runs ADD COLUMN depth INTEGER NOT NULL DEFAULT 0;    -- distance from root
ALTER TABLE runs ADD COLUMN path TEXT NOT NULL DEFAULT '/';      -- materialized ancestor path
ALTER TABLE runs ADD COLUMN continuation_tool_call_id TEXT;      -- tool call ID to inject result into on resume

-- New table for child results
CREATE TABLE run_results (
    id          TEXT PRIMARY KEY,      -- run_id of the child
    parent_id   TEXT,                  -- parent run_id (null for root)
    depth       INTEGER DEFAULT 0,
    status      TEXT NOT NULL,         -- completed | failed | abandoned
    summary     TEXT,                  -- 1-3 sentence summary
    jsonl       TEXT,                  -- structured JSONL (findings, file_changed, test_result, conclusion)
    full_output TEXT,                  -- complete final answer
    created_at  TEXT NOT NULL,
    finished_at TEXT
);

CREATE INDEX idx_runs_parent ON runs(parent_run_id);
CREATE INDEX idx_runs_path ON runs(path);
CREATE INDEX idx_run_results_parent ON run_results(parent_id);
```

The `path` column enables efficient subtree operations:
- `CancelSubtree(runID)`: `UPDATE runs SET status='cancelled' WHERE path LIKE '{run.path}%'`
- `SubtreeCost(runID)`: `SELECT SUM(cost_usd) FROM runs WHERE path LIKE '{run.path}/%'`
- `SubtreeRuns(runID)`: `SELECT * FROM runs WHERE path LIKE '{run.path}/%'`

Path construction at spawn time:
- Root: `path = '/' + run_id`
- Child: `path = parent.path + '/' + run_id`

### 2.3 The spawn_agent Tool

New tool injected at `TierCore` when `depth > 0` (not available to root agents):

```json
{
    "name": "spawn_agent",
    "task": "Implement the auth module using JWT. Write tests.",
    "allowed_tools": ["bash", "read", "write", "grep"],
    "system_prompt": "You are an implementation agent.",
    "max_cost_usd": 0.50,
    "max_depth": 3
}
```

Handler steps:
1. Validate depth: `currentDepth + 1 <= max_depth` (from `ContextKeyForkDepth` in context)
2. Compute child path: `parentPath + '/' + childRunID`
3. Create child `RunRequest` with `parent_run_id`, `depth+1`, `path`, `AllowedTools`, `MaxCostUSD`
4. Store `continuation_tool_call_id = currentToolCallID` on parent run (from `ContextKeyToolCallID`)
5. Start child run via `StartRun`
6. Set parent status to `RunStatusWaitingForChild` and **return a sentinel** to the step loop that causes it to exit without completing

The tricky part is step 6: the step loop needs to understand that a tool returning a special sentinel means "suspend now, don't call the LLM again, wait for child notification." This requires a new signal path from the tool handler back to the execute loop.

### 2.4 The Completion Hook

When a run completes (in the terminal event path of `execute()`), check if it has a `parent_run_id`:

```go
// In the terminal event emit path:
if state.run.ParentRunID != "" {
    go r.notifyParentOfChildCompletion(state.run.ID, state.run.ParentRunID)
}
```

`notifyParentOfChildCompletion`:
1. Build result: extract summary + JSONL from events/transcript
2. Store `run_results` record
3. Load parent state from `r.runs` (or from DB if server restarted)
4. Inject result as a synthetic tool response for `continuation_tool_call_id`
5. Set parent status back to `running`
6. Re-dispatch parent's step loop via `dispatchRun`

The result injected into the parent's message history looks like:
```json
{
    "result_id": "run_abc123",
    "status": "completed",
    "summary": "Implemented JWT auth. 3 files changed. All tests pass.",
    "full_output_tokens": 14200,
    "jsonl_ref": "run_results:run_abc123",
    "jsonl": [
        {"type": "file_changed", "path": "internal/auth/jwt.go", "lines_added": 87},
        {"type": "warning", "content": "Secret hardcoded in config.go:42"},
        {"type": "test_result", "passed": 14, "failed": 0}
    ]
}
```

### 2.5 Depth Counter Replacement

Current binary flag in `internal/harness/tools/core/skill.go`:
```go
const ContextKeyForkedSkill contextKey = "forked_skill"
if _, nested := ctx.Value(tools.ContextKeyForkedSkill).(string); nested {
    return "", fmt.Errorf("nested skill forking is not supported")
}
```

Replace with:
```go
const ContextKeyForkDepth contextKey = "fork_depth"

currentDepth, _ := ctx.Value(ContextKeyForkDepth).(int)
if currentDepth >= maxDepth {
    return "", fmt.Errorf("max recursion depth %d reached", maxDepth)
}
childCtx := context.WithValue(ctx, ContextKeyForkDepth, currentDepth+1)
```

The `ContextKeyForkedSkill` binary flag can be removed once the depth counter replaces it.

---

## 3. What "Result Pointers" Mean Concretely

A **result pointer** is the return value that `spawn_agent` delivers to the parent run — not the full child output, but a compact reference to it. Concretely:

```json
{
    "result_id": "run_abc123",
    "status": "completed",
    "summary": "Implemented JWT auth. 3 files changed. All tests pass.",
    "full_output_tokens": 14200,
    "jsonl_ref": "run_results:run_abc123",
    "jsonl": [
        {"type": "file_changed", "path": "internal/auth/jwt.go", "lines_added": 87},
        {"type": "warning", "content": "Secret hardcoded in config.go:42"},
        {"type": "test_result", "passed": 14, "failed": 0},
        {"type": "conclusion", "content": "Auth module complete."}
    ]
}
```

The parent sees `full_output_tokens: 14200` and decides:
- **Work from inline JSONL**: sufficient for most cases — structured summary of what the child did
- **Load full output**: only if the parent needs verbatim content (e.g., to re-read generated code). Uses `jsonl_ref` as a key to a DB read tool call

**Why this matters**: A child run processing a 100-file codebase might produce 14,000 tokens of output. Injecting that wholesale into the parent context would consume the parent's entire context window for one tool call. The pointer + JSONL summary keeps the parent's context lean — typically 200–500 tokens per child result regardless of child output size.

**JSONL line types**:
- `{"type": "finding", "content": "..."}` — notable discovery or observation
- `{"type": "file_changed", "path": "...", "lines_added": N, "lines_removed": N}` — file mutation
- `{"type": "tool_call", "tool": "...", "summary": "..."}` — summarized tool usage
- `{"type": "test_result", "passed": N, "failed": N, "details": "..."}` — test outcomes
- `{"type": "error", "content": "..."}` — errors encountered
- `{"type": "warning", "content": "..."}` — non-fatal issues
- `{"type": "conclusion", "content": "..."}` — final answer / result summary

---

## 4. What "JSONL Grep" Means Concretely

Two distinct concepts both called "JSONL grep" in the issue:

### 4.1 Inline JSONL Grep (Parent Searching Child Result)

The `jsonl` array in the result pointer is already in the parent's context. The parent can use its existing `grep` tool or just reason over the structured lines directly. No new tooling needed for this — it's just structured data in the tool result.

Example parent reasoning: "The child result shows `warning: Secret hardcoded in config.go:42`. I need to investigate that before proceeding."

### 4.2 Cross-Run JSONL Grep (Searching Stored Run Results)

A new tool (or extension of the existing `bash`/`grep` tools) that searches across the `run_results` table or the on-disk JSONL rollout files:

```json
{
    "tool": "grep_run_results",
    "parent_run_id": "run_parent_xyz",
    "query": "type=file_changed AND path LIKE 'internal/auth/%'"
}
```

This lets a parent agent find patterns across all its children's results — e.g., "which of my child agents touched the auth module?" — without loading each child's full output.

The existing JSONL rollout files at `<RolloutDir>/<date>/<run_id>.jsonl` are already greppable with standard tools. The `forensics/rollout` loader provides the parsing infrastructure. What's missing is a tool that a running agent can call to search them.

---

## 5. The task_complete Tool

The issue specifies a `task_complete` tool as the **only valid return path** for subagents:

```json
{
    "tool": "task_complete",
    "summary": "Implemented JWT auth. 3 files changed. All tests pass.",
    "status": "completed | partial | failed",
    "findings": [...],
    "artifacts": ["internal/auth/jwt.go"]
}
```

Key design choices from the issue:
- Only injected into tool list when `depth > 0` (top-level agents never see it)
- Calling it triggers: save to `run_results` → generate JSONL → notify parent → terminate child
- If child hits step limit without calling it, runner force-synthesizes a partial result
- System prompt for all subagents must include "You MUST call `task_complete` when done"

This is cleaner than having the completion hook synthesize from the transcript because:
1. The child explicitly declares what it found (no heuristic extraction needed)
2. The JSONL schema is agent-controlled, not reconstructed from events
3. The child can signal partial success vs full failure

---

## 6. Recommended Phase 1 Scope

Given the 31 acceptance criteria spanning schema, tooling, suspension, budget, check-ins, and tree operations, a minimal coherent Phase 1 should deliver:

**Phase 1: Depth Counter + spawn_agent + In-Memory Suspension (no server restart recovery)**

This is the smallest slice that proves the concept end-to-end while being shippable:

### Phase 1 Scope

1. **Replace binary flag with depth counter** (`internal/harness/tools/core/skill.go` + `tools/types.go`)
   - Remove `ContextKeyForkedSkill`
   - Add `ContextKeyForkDepth int`
   - Configurable `max_depth` (default 5) enforced in `handleForkSkill`

2. **New `spawn_agent` tool** (`internal/harness/tools/spawn_agent.go`)
   - Accepts: `task`, `allowed_tools`, `system_prompt`, `max_cost_usd`, `max_steps`
   - Injects `task_complete` into child's tool list (gated by `depth > 0`)
   - Sets child's system prompt to include mandatory `task_complete` instruction
   - Synchronous wait (same as current `RunForkedSkill`) for Phase 1 — no true suspension yet
   - Returns result pointer with inline JSONL (no DB storage in Phase 1)

3. **New `task_complete` tool** (`internal/harness/tools/task_complete.go`)
   - Depth-gated: only visible when `depth > 0`
   - Accepts: `summary`, `status`, `findings`, `artifacts`
   - Terminates the child run with the provided structured result
   - Emits a new `child.completed` event type

4. **JSONL extraction from findings** (inline, no separate `run_results` table)
   - `spawn_agent` returns the `findings` array as the inline `jsonl` field in the result pointer
   - No DB storage — just pass-through from `task_complete` to `spawn_agent` return value

5. **Step-budget pressure messages** (`internal/harness/runner.go`)
   - At `max_steps - 3`: inject "You have 3 steps remaining, begin wrapping up and call task_complete"
   - At `max_steps - 1`: inject "You have 1 step remaining, call task_complete now"
   - At `max_steps` without `task_complete`: force-synthesize partial result

6. **Tests**
   - Depth limit enforcement (depth 0 → 1 → 2, reject at max)
   - `task_complete` only visible to depth > 0
   - Step-budget pressure injection
   - End-to-end: parent spawns child, child calls `task_complete`, parent receives pointer

### Phase 1 Acceptance Criteria (8 of 31)

- [ ] Depth counter replaces binary `ContextKeyForkedSkill` flag
- [ ] Configurable `max_depth` (default: 5) enforced at spawn time
- [ ] `task_complete` tool is injected into tool list only when `depth > 0`
- [ ] Subagent system prompt includes mandatory `task_complete` instruction
- [ ] `task_complete` call terminates child run with structured result
- [ ] Result pointer returned to parent (not full content)
- [ ] Step-budget pressure messages injected at `max_steps - 3` and `max_steps - 1`
- [ ] Force-synthetic `task_complete` fires if child hits step limit without calling it

### What Phase 1 Defers

- DB-backed suspension and server-restart recovery (needs `run_results` table, new RunStatus, completion hook)
- True parent goroutine suspension (Phase 1 uses synchronous wait, same as current `RunForkedSkill`)
- Budget propagation from parent to child subtree
- Concurrent sibling agents (same parent, different children in parallel)
- Check-in mechanism and watcher goroutine
- Materialized path column and subtree operations (`CancelSubtree`, `SubtreeCost`)
- Cross-run JSONL grep tool

### Phase 2: DB-Backed Suspension and Server Restart Recovery

- Schema migration: `parent_run_id`, `depth`, `path`, `continuation_tool_call_id` on `runs` table
- New `run_results` table
- New `RunStatusWaitingForChild` status
- Completion hook: when child completes, inject result into parent and re-dispatch
- Server startup scan: find `waiting_for_child` runs and re-check their children
- Budget propagation via subtree cost queries

### Phase 3: Concurrent Siblings + Subtree Operations

- Concurrent sibling agents don't interfere (path-based budget isolation)
- `CancelSubtree`, `SubtreeCost`, `SubtreeRuns` store methods
- `idx_runs_path` index

### Phase 4: Check-In / Oversight

- `check_in_every_steps` spawn parameter
- `ChildCheckIn` SSE event type
- Watcher goroutine, `continue`/`redirect`/`force_complete` signals
- `check_in_policy: require_approval` suspension

---

## 7. Open Questions

### Q1: Goroutine Suspension Signal Path

In Phase 2, the `spawn_agent` tool handler needs to suspend the parent step loop without terminating the run. Current architecture: `execute()` calls LLM → dispatches tool calls → collects results → loops. There is no existing mechanism for a tool call to signal "stop the loop and put this run in waiting state."

Two options:
- **Return a sentinel error**: Define a special error type `ErrSuspendForChild` that `execute()` recognizes and handles specially (don't fail, don't loop, set status to `waiting_for_child`)
- **Use a channel**: The `spawn_agent` tool writes a "suspend" message to the run's `steeringCh` or a new `suspendCh`. The execute loop checks for this after tool dispatch.

The sentinel error approach is simpler and avoids adding a new channel to `runState`. The `execute()` loop already does `switch err { case ErrCostLimitReached: ... }` patterns.

### Q2: Re-dispatching a Suspended Run

After injecting the child result, `notifyParentOfChildCompletion` needs to re-dispatch the parent's step loop. This is essentially calling `dispatchRun(parentRunID, originalReq)` — but the original `RunRequest` is not stored anywhere accessible after `execute()` starts. The parent state has `staticSystemPrompt`, `maxCostUSD`, `permissions`, `allowedTools` — enough to reconstruct a minimal `RunRequest`. The `messages` slice already contains the full conversation history including the injected tool result.

The reconstruction needs to be explicit and documented — it is not a free `StartRun` call.

### Q3: JSONL Extraction Strategy

The issue proposes both:
- Agent-produced JSONL (via `task_complete` `findings` array) — clean, structured
- System-extracted JSONL (heuristic from events/transcript) — fallback for abandoned runs

For abandoned runs (child hits step/context limit without calling `task_complete`), the system needs to synthesize a partial result. What heuristics should be used? Candidates:
- Last assistant message content → `conclusion`
- All file_write tool calls → `file_changed` entries
- All bash tool results containing "PASS"/"FAIL" → `test_result`
- Any tool call errors → `error` entries

This extraction logic belongs in a new `internal/harness/jsonl_extractor.go` as the issue specifies.

### Q4: Parent Context Blowout with Many Children

If a parent spawns 10 sequential children, each returning a result pointer with 20 JSONL lines, the parent's context grows by ~200 lines of structured JSON across all tool results. At scale, even result pointers accumulate. Should the auto-compact logic recognize `spawn_agent` result messages as compaction candidates? The inline JSONL could be stripped to just summary + conclusion after the parent has processed it.

### Q5: The `task_complete` Tool vs. Skill Fork

Skills with `context: fork` currently use `RunForkedSkill` and return plain text via `handleForkSkill`. After Phase 1, should forked skills also use `task_complete` as their return path, or only the new `spawn_agent` tool? The issue implies `task_complete` is for `spawn_agent`-spawned children, not skill forks. This distinction should be documented to avoid confusion.

### Q6: Worker Pool Interaction

When the worker pool is bounded and a parent is `waiting_for_child`, should the parent's worker slot be released? Currently `execute()` holds a worker slot for its entire lifetime. A parent waiting for a child that's also in the pool could deadlock if the pool is small (parent holds slot 1, child can't get slot 2). Phase 2 must release the parent's worker slot when it enters `waiting_for_child` and re-acquire one when it resumes.

---

## 8. Key File Inventory

| File | Role | Phase |
|---|---|---|
| `internal/harness/tools/types.go` | Add `ContextKeyForkDepth`, remove `ContextKeyForkedSkill` | P1 |
| `internal/harness/tools/core/skill.go` | Replace binary flag with depth counter | P1 |
| `internal/harness/tools/spawn_agent.go` | New spawn_agent tool | P1 |
| `internal/harness/tools/task_complete.go` | New task_complete tool (depth-gated) | P1 |
| `internal/harness/runner.go` | Step-budget pressure injection, suspension signal, re-dispatch | P1/P2 |
| `internal/harness/types.go` | `RunRequest` depth/parent fields; new `RunStatusWaitingForChild` | P2 |
| `internal/harness/events.go` | New `child.completed`, `child.check_in` event types | P1/P4 |
| `internal/store/sqlite.go` | Schema migration: parent_run_id, depth, path, continuation_tool_call_id, run_results | P2 |
| `internal/store/store.go` | `CancelSubtree`, `SubtreeCost`, `SubtreeRuns` methods | P3 |
| `internal/harness/jsonl_extractor.go` | Extract JSONL from run events/transcript for abandoned runs | P2 |
| `internal/server/http_agents.go` | Wire depth + parent_run_id through fork path | P2 |

---

## 9. Summary

**What exists and is ready to build on**:
- `ForkedAgentRunner` interface + `Runner.RunForkedSkill` — synchronous forking works
- `ContextKeyForkedSkill` binary flag — needs replacement with depth counter
- `AllowedTools` forwarding (#234) — fully working
- Config propagation to worktree subagents (#236) — fully working
- JSONL rollout recorder — per-run JSONL files already exist on disk
- SQLite store — schema extensible with migrations
- `subagents.Manager` — async subagent management with worktree isolation works

**What is genuinely new in #235**:
- `task_complete` tool (depth-gated, structured return path)
- `spawn_agent` tool (depth counter, tree construction)
- True goroutine suspension + DB-backed resume
- `run_results` table with result pointers
- Step-budget pressure and force-complete
- Materialized path + subtree operations
- Check-in / watcher oversight

**Recommended Phase 1**: Replace the binary fork flag with a depth counter, add `spawn_agent` and `task_complete` tools with synchronous wait (no true suspension), add step-budget pressure. This delivers a usable recursive spawning primitive in approximately 3–4 days without touching the DB schema.

**Remaining gap before Phase 2**: The goroutine suspension signal path needs design decision (sentinel error vs. channel). This is the highest-risk design question — everything else in Phase 2 follows from it.
