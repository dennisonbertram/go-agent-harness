# Issue #234 Implementation Plan: Per-Run Tool Filtering, System Prompt, and Permissions Forwarding

## Date
2026-03-13

## Problem Statement
The HTTP endpoint accepts `allowed_tools`, `system_prompt`, and `permissions` in the request body but several forwarding paths are broken:

1. `RunRequest` struct is missing `AllowedTools []string` field — the HTTP JSON decoder can't populate it.
2. Even if the field existed, `filteredToolsForRun()` only checks `SkillConstraintTracker`, not per-run request constraints.
3. `RunForkedSkill()` interface method on `ForkedAgentRunner` is not implemented by `Runner` — the skill tool's fork path falls back to `runner.RunPrompt()` which doesn't forward `AllowedTools`, `SystemPrompt`, or `Permissions`.
4. `SkillConstraint` lacks an `IsBootstrap` flag — all constraints deactivate when `completeRun()` calls `skillConstraints.Cleanup(runID)`.

## What Currently Exists

### Working
- `RunRequest.SystemPrompt` — field exists, forwarded to `runState.staticSystemPrompt`
- `RunRequest.Permissions` — field exists, forwarded to `runState.permissions`
- `filteredToolsForRun()` — correctly applies `SkillConstraint.AllowedTools` when active
- `ForkedAgentRunner` interface in `internal/harness/tools/types.go` — defined but not implemented by `*Runner`
- `SkillConstraint` struct in `internal/harness/skill_constraint.go`

### Missing / Broken
- `RunRequest.AllowedTools []string` — field does not exist (issue #234 point 1)
- Per-run `AllowedTools` enforcement in `filteredToolsForRun()` — only checks skill constraint (issue #234 point 3)
- `(*Runner).RunForkedSkill()` implementation — falls back to `RunPrompt()` via interface check, not forwarding config (issue #234 point 4)
- `SkillConstraint.IsBootstrap` flag (issue #234 point 5)

## Files to Change

### 1. `internal/harness/types.go`
Add `AllowedTools []string` to `RunRequest` struct.

```go
// AllowedTools restricts which tools are available for this run.
// When non-empty, only the listed tool names (plus always-available tools)
// are offered to the LLM. An empty slice means no restriction.
AllowedTools []string `json:"allowed_tools,omitempty"`
```

Place after `Permissions` field.

### 2. `internal/harness/skill_constraint.go`
Add `IsBootstrap bool` to `SkillConstraint` struct:

```go
type SkillConstraint struct {
    SkillName    string   // name of the active skill
    AllowedTools []string // nil = no restriction (all tools allowed)
    // IsBootstrap marks this constraint as coming from a per-run
    // AllowedTools setting. Bootstrap constraints are NOT cleaned up
    // by completeRun() — they persist for the lifetime of the run.
    IsBootstrap bool
}
```

Modify `Cleanup()` / `completeRun()` to skip bootstrap constraints — OR store bootstrap constraints separately. The cleaner approach: modify `SkillConstraintTracker.Cleanup()` to only remove non-bootstrap constraints.

### 3. `internal/harness/runner.go`

**a) StartRun() — activate bootstrap constraint when AllowedTools set:**
After the `runState` is added to `r.runs`, if `req.AllowedTools` is non-empty, call:
```go
r.skillConstraints.Activate(runID, SkillConstraint{
    SkillName:    "__bootstrap__",
    AllowedTools: req.AllowedTools,
    IsBootstrap:  true,
})
```

**b) filteredToolsForRun() — unchanged** (already uses skill constraint tracker which will have bootstrap constraint active)

**c) completeRun() / cleanup — skip bootstrap constraints:**
Modify `skillConstraints.Cleanup(runID)` to preserve bootstrap constraints, OR add a `CleanupSkill(runID)` that only removes non-bootstrap entries. Since `completeRun` currently calls `r.skillConstraints.Cleanup(runID)` which deletes the constraint regardless, we need `Cleanup` to be smarter or have a separate method.

Best approach: add `CleanupUserConstraint(runID string)` to `SkillConstraintTracker` that only removes if `!constraint.IsBootstrap`. In `completeRun` and `failRun`, call `CleanupUserConstraint` instead of `Cleanup`. The full `Cleanup` is still called at the very end of run lifecycle.

Actually simpler: store bootstrap constraints separately, not in the skill constraint tracker. Add `allowedTools []string` directly to `runState` and check it in `filteredToolsForRun`.

**Revised approach — store per-run allowedTools in runState:**
- Add `allowedTools []string` to `runState`
- In `StartRun()`, set `state.allowedTools = req.AllowedTools`
- In `filteredToolsForRun()`, check `runState.allowedTools` first (before skill constraint), apply its filter as a base layer

**d) Implement `RunForkedSkill()` on `*Runner`:**
The runner must implement `ForkedAgentRunner` interface:
```go
func (r *Runner) RunForkedSkill(ctx context.Context, config htools.ForkConfig) (htools.ForkResult, error) {
    req := RunRequest{
        Prompt:       config.Prompt,
        AllowedTools: config.AllowedTools,
        // inherit system prompt and permissions from parent run context
    }
    run, err := r.StartRun(req)
    if err != nil {
        return htools.ForkResult{}, err
    }
    // wait for run completion and collect output
    ...
}
```

To inherit `SystemPrompt` and `Permissions` from the parent run, we need to extract them from the context. The context carries `RunMetadata` (via `RunMetadataFromContext`). We can look up the parent's `runState` from the context's run ID.

### 4. `internal/harness/tools/types.go` (ForkConfig)
Add `SystemPrompt` and `Permissions` fields to `ForkConfig` so the skill tool can forward them:

```go
type ForkConfig struct {
    Prompt       string
    SkillName    string
    Agent        string
    AllowedTools []string
    Metadata     map[string]string
    SystemPrompt string   // ADDED: forwarded from parent run
    Permissions  *PermissionConfig // ADDED: forwarded from parent run
}
```

Wait — `ForkConfig` is in the `tools` package, and `PermissionConfig` is in the `harness` package. This would create a circular import. Solution: define a `ForkPermissions` in the tools package, or pass permissions as a map.

Better: Keep `ForkConfig` as-is (just `AllowedTools`), and let `RunForkedSkill()` in the runner look up parent run's system prompt and permissions from the context's run ID.

**Revised ForkConfig approach:** The runner's `RunForkedSkill` implementation extracts the parent run ID from context (via `RunMetadataFromContext`), looks up the parent's `runState`, and copies `staticSystemPrompt` and `permissions` to the new `RunRequest`. No changes to `ForkConfig` needed.

## Implementation Steps

### Step 1: Add `AllowedTools` to `RunRequest` (types.go)
Simple field addition.

### Step 2: Add `allowedTools` to `runState` (runner.go)
New field in `runState` struct. Set in `StartRun()`.

### Step 3: Update `filteredToolsForRun()` to apply per-run allowedTools
Before checking skill constraints, apply the per-run filter as a base layer. Or: activate a bootstrap `SkillConstraint` with `IsBootstrap=true` when run starts. Skill constraint replaces/overrides bootstrap. When skill completes, bootstrap re-activates.

**Cleanest design — two-layer filtering:**
1. Per-run base layer: from `req.AllowedTools` (persists whole run)
2. Skill constraint layer: from active skill (overrides base layer during skill execution)

When a skill constraint is active: use skill's `AllowedTools` (intersected with base, or just use skill's list as-is per existing behavior).
When no skill constraint: use per-run base `AllowedTools`.

Implementation in `filteredToolsForRun()`:
```go
func (r *Runner) filteredToolsForRun(runID string) []ToolDefinition {
    defs := r.tools.DefinitionsForRun(runID, r.activations)

    // Check skill constraint first (higher priority during skill execution)
    constraint, active := r.skillConstraints.Active(runID)
    if active && constraint.AllowedTools != nil {
        return applyAllowList(defs, constraint.AllowedTools)
    }

    // Fall back to per-run base allowed tools
    r.mu.RLock()
    state, ok := r.runs[runID]
    baseAllowed := state.allowedTools // safe copy (slice header)
    r.mu.RUnlock()
    if ok && len(baseAllowed) > 0 {
        return applyAllowList(defs, baseAllowed)
    }

    return defs
}
```

### Step 4: Implement `(*Runner).RunForkedSkill()`
- Extract parent run ID from ctx via `RunMetadataFromContext`
- Look up parent's `runState` for `staticSystemPrompt` and `permissions`
- Build `RunRequest` with `ForkConfig.Prompt`, `ForkConfig.AllowedTools`, parent's system prompt and permissions
- Call `r.StartRun(req)` and wait for completion
- Return `ForkResult{Output: final_output}`

### Step 5: Verify `IsBootstrap` is not needed
With the two-layer approach, `IsBootstrap` is not needed because skill constraints and per-run base constraints are separate mechanisms (`skillConstraints` tracker vs `runState.allowedTools`). When `completeRun()` calls `skillConstraints.Cleanup()`, it only removes skill constraints, not the per-run base (which is in `runState.allowedTools`). So `IsBootstrap` on `SkillConstraint` is not needed.

## Testing Strategy

### Test file: `internal/harness/runner_tool_filter_test.go` (new file)

1. **`TestAllowedTools_LimitsAvailableTools`** — `RunRequest.AllowedTools = ["read_file"]` → LLM only sees `read_file` in tool definitions, bash tool not offered
2. **`TestAllowedTools_EmptyMeansNoFilter`** — `RunRequest.AllowedTools = nil` → all tools offered
3. **`TestAllowedTools_SkillConstraintOverrides`** — base `AllowedTools = ["read_file", "bash"]`, then skill activates `AllowedTools = ["grep"]` → during skill, only `grep` available
4. **`TestAllowedTools_SkillConstraintDeactivatesReverts`** — after skill completes, reverts to base `AllowedTools`
5. **`TestRunForkedSkill_ForwardsAllowedTools`** — skill fork with `AllowedTools` constraint, subrun only has those tools
6. **`TestRunForkedSkill_InheritsParentSystemPrompt`** — forked skill run uses parent's system prompt
7. **`TestRunForkedSkill_InheritsParentPermissions`** — forked skill run uses parent's permissions
8. **`TestAllowedTools_RaceConditionSafe`** — concurrent runs with different `AllowedTools` don't interfere

## Acceptance Criteria Mapping
- [x] AllowedTools field added to RunRequest
- [x] HTTP handler auto-forwards (JSON decode handles it)
- [x] filteredToolsForRun() uses AllowedTools when non-empty
- [x] RunForkedSkill() implemented in runner
- [x] System prompt forwarded to forked runs
- [x] Permissions forwarded to forked runs
- [x] Tests for all new paths
- [x] Race detector clean
- [ ] IsBootstrap flag on SkillConstraint — NOT NEEDED with two-layer approach
