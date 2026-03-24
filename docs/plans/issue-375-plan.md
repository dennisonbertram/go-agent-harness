# Plan: Issue #375 — fix(agents): make spawn_agent honor its declared profile/model contract

## Summary

`spawn_agent` declares `profile`, `model`, and `max_steps` parameters in its JSON schema but the handler parses them and then discards them without forwarding to the child execution. The child runs with default model/max_steps from the runner config, violating the declared contract. The fix routes `spawn_agent` through the same profile-aware path as `run_agent`, making both tools consistent and properly model/profile-aware.

## Recommended Option

**Option A: Make spawn_agent fully profile-aware**

**Rationale:**
1. **Consistency with run_agent**: Both tools serve the same use case (spawning a child agent) and should honor the same contract (profile + model + max_steps).
2. **Profile system exists**: The harness already has a full profile system (`internal/profiles/`) with three-tier resolution (user directory, built-in, defaults). Reusing it maintains the principle of least surprise.
3. **MaxSteps propagation gap**: `spawn_agent` declares `max_steps` but currently hardcodes it in the system prompt. Full profile-awareness means it can override via `RunRequest.MaxSteps` (same path as `run_agent`).
4. **Minimal code duplication**: Both tools can share profile-loading and value-application logic.
5. **Future-proof**: Option B (removing fields) would require deprecation/migration; Option A extends capability without breaking anything.

## Files to Change

1. `internal/harness/tools/deferred/spawn_agent.go` — main implementation
2. `internal/harness/tools/deferred/spawn_agent_test.go` — update and add tests
3. `internal/harness/runner.go` — extend `RunForkedSkill` or add profile-aware variant
4. `internal/harness/tools/types.go` — extend `ForkConfig` to carry model/max_steps (optional; or use RunRequest directly)

## Files to Create

None — all changes are in existing files.

## Implementation Steps

### Phase 1: Extend RunForkedSkill to Accept Model & MaxSteps

1. **Update ForkConfig in types.go** (optional approach — cleaner than adding fields):
   - Add optional `Model` and `MaxSteps` fields to `ForkConfig`
   - Document that `Model` overrides the runner's default; empty string = runner default
   - Document that `MaxSteps <= 0` = inherit from parent (current behavior)

2. **Update RunForkedSkill signature in runner.go**:
   - Modify `RunForkedSkill()` to read `config.Model` and `config.MaxSteps`
   - When building the sub-run request (line 1035), set:
     - `req.Model = config.Model` (if non-empty)
     - `req.MaxSteps = config.MaxSteps` (if > 0)
   - If both are empty/zero, inherit from parent as before

### Phase 2: Update spawn_agent Handler to Apply Profile Logic

1. **Load profile** (copy pattern from run_agent.go):
   - Parse `args.Profile` (default to "full" if empty, same as run_agent)
   - Call `profiles.LoadProfile()` or `profiles.LoadProfileFromUserDir()` (if profilesDir available)
   - Extract `model`, `max_steps`, `system_prompt` from profile

2. **Apply overrides** (copy pattern from run_agent.go:86-97):
   - If `args.Model` is non-empty, use it (overrides profile)
   - If `args.MaxSteps > 0`, use it (overrides profile)
   - Otherwise, use profile values or runner defaults

3. **Build ForkConfig with model/max_steps**:
   - Set `config.Model` and `config.MaxSteps` from the resolved values
   - Optionally: inject profile-based system prompt into the prompt (or keep spawn_agent's current prompt logic)

4. **Pass profilesDir to spawn_agent**:
   - Update `SpawnAgentTool()` signature to accept `profilesDir string` (similar to RunAgentTool)
   - Wire it in `BuildTools()` or wherever spawn_agent is registered

### Phase 3: Tests

1. **Failing tests FIRST**:
   - `TestSpawnAgentTool_HonorsModelParameter`: spawn with explicit model, verify child run uses it
   - `TestSpawnAgentTool_HonorsMaxStepsParameter`: spawn with explicit max_steps, verify child run uses it
   - `TestSpawnAgentTool_LoadsProfile`: spawn with profile="fast", verify profile values are applied
   - `TestSpawnAgentTool_OverridesProfileModel`: spawn with profile="fast" + model override, verify override wins

2. **Regression tests** (existing behavior must not break):
   - `TestSpawnAgentTool_DefaultMaxSteps`: spawn with no max_steps → defaults to 30 (from system prompt)
   - `TestSpawnAgentTool_PropagatesDepthToChild`: fork depth still propagated
   - `TestSpawnAgentTool_EnforcesDepthLimit`: max depth still enforced
   - `TestSpawnAgentTool_AllowedToolsForwarded`: allowed_tools still work
   - `TestSpawnAgentTool_WithStructuredTaskCompleteResult`: task_complete parsing still works

## Testing Strategy

### Failing tests to write FIRST (red phase):
- **TestSpawnAgentTool_HonorsDeclaredModel**: Call spawn_agent with `model="gpt-4.1-mini"`, verify the child sub-run request has `Model="gpt-4.1-mini"`
  - Mock forked runner captures `lastConfig`; check model override was passed
  - Or mock `RunForkedSkill` and verify the request used the declared model
- **TestSpawnAgentTool_HonorsDeclaredMaxSteps**: Call spawn_agent with `max_steps=99`, verify child sub-run request has `MaxSteps=99`
- **TestSpawnAgentTool_LoadsProfileAndAppliesValues**: Create temp profile TOML with model/max_steps/allowed_tools, spawn with profile name, verify all values propagated to child request
- **TestSpawnAgentTool_ProfileModelOverridableByParameter**: Load profile with model="gpt-4.1-mini", spawn with model="o3", verify o3 wins

### Tests for new behavior:
- **TestSpawnAgentTool_DefaultProfileToFull**: Spawn with no profile, verify it defaults to "full" (same as run_agent)
- **TestSpawnAgentTool_ProfileNotFound_UsesEmptyDefaults**: Spawn with profile that doesn't exist, verify non-fatal (uses empty Profile{} like run_agent does)

### Regression coverage:
- All existing spawn_agent tests must still pass unchanged:
  - `TestSpawnAgentTool_Definition` — schema, tier, action unchanged
  - `TestSpawnAgentTool_RequiresTask` — validation unchanged
  - `TestSpawnAgentTool_EnforcesDepthLimit` — depth logic unchanged
  - `TestSpawnAgentTool_PropagatesDepthToChild` — depth propagation unchanged
  - `TestSpawnAgentTool_AllowedToolsForwarded` — allowed_tools still forwarded
  - `TestSpawnAgentTool_DefaultMaxSteps` — if no max_steps param → defaults to 30 in system prompt (may change slightly if we use RunRequest.MaxSteps instead; document if so)
  - `TestSpawnAgentTool_WithStructuredTaskCompleteResult` — result parsing unchanged

## Risk Areas / Edge Cases

1. **ProfilesDir availability**: spawn_agent needs to know profilesDir. Currently SpawnAgentTool receives only `runner tools.AgentRunner`. May need to:
   - Add `profilesDir` parameter to `SpawnAgentTool()` function (breaking change in function signature)
   - Or: Make `AgentRunner` optional and require a fuller interface in BuildTools (less clean)
   - Or: Store profilesDir in context (adds complexity)
   - **Decision**: Add `profilesDir string` parameter to `SpawnAgentTool()` (similar to `RunAgentTool`)

2. **Profile system fallback**: run_agent treats missing profiles as non-fatal (uses empty Profile{} with defaults). spawn_agent must do the same to avoid breaking existing code that doesn't use profiles.

3. **System prompt injection**: Current spawn_agent injects a custom system prompt via `buildSubagentSystemPrompt()`. If we also apply profile's system prompt, which wins?
   - **Decision**: spawn_agent's custom prompt takes precedence (the task + step-budget warning is critical). If profile has a custom system prompt, it can be merged or ignored (needs to be decided and documented).

4. **MaxSteps resolution logic**: Currently spawn_agent hardcodes max_steps default to 30 in the system prompt. Once we use RunRequest.MaxSteps:
   - If max_steps param is 0 or omitted → use profile default (or runner default)
   - System prompt must still warn at step (MaxSteps - 3)
   - The prompt text currently says "at most %d steps" — must remain accurate

5. **Model propagation through ForkedAgentRunner**: RunForkedSkill currently doesn't read Model from ForkConfig. Must:
   - Add Model field to ForkConfig
   - Modify RunForkedSkill to use it (lines 1035-1039 in runner.go)
   - Test that the model is actually used (difficult without mocking provider; may just verify req.Model is set)

6. **Backward compatibility**: Existing spawn_agent calls without model/profile/max_steps must work unchanged. This is guaranteed if defaults are applied correctly (profile="full", model="", max_steps=0 → runner defaults).

## Commit Strategy

### Commit 1: Test (Red)
```
test(#375): add failing tests for spawn_agent profile/model/max_steps contract
- TestSpawnAgentTool_HonorsDeclaredModel
- TestSpawnAgentTool_HonorsDeclaredMaxSteps
- TestSpawnAgentTool_LoadsProfileAndAppliesValues
- TestSpawnAgentTool_ProfileModelOverridableByParameter
```

### Commit 2: Extend ForkConfig & RunForkedSkill
```
feat(#375): extend ForkConfig with Model and MaxSteps fields

- Add optional Model and MaxSteps to ForkConfig in types.go
- Update RunForkedSkill in runner.go to read and apply Model/MaxSteps overrides
- If Model is empty, use runner default; if MaxSteps <= 0, use parent default
```

### Commit 3: Update spawn_agent
```
fix(#375): make spawn_agent honor declared profile/model/max_steps contract

- Load profile using three-tier resolution (user, built-in, defaults)
- Apply per-call overrides (model, max_steps) on top of profile values
- Forward resolved model and max_steps to RunForkedSkill via ForkConfig
- Default profile to "full" when not specified
- Gracefully handle missing profiles (use empty defaults, non-fatal)
- Add profilesDir parameter to SpawnAgentTool function signature
```

### Commit 4: Add and verify tests
```
test(#375): add tests for new spawn_agent profile-aware behavior

- TestSpawnAgentTool_DefaultProfileToFull
- TestSpawnAgentTool_ProfileNotFoundUsesDefaults
- Verify all regression tests still pass
```

## Additional Notes

- **Documentation**: Update spawn_agent tool description (currently in code: `spawnAgentDescription`) to mention profile/model/max_steps behavior and match run_agent's documentation style.
- **Wire spawn_agent in BuildTools**: Ensure the updated `SpawnAgentTool(runner, profilesDir)` call is updated wherever it's registered in BuildTools.
- **Shared code opportunity**: Consider extracting `applyProfileValues()` logic to a shared helper so both run_agent and spawn_agent use identical profile resolution. This is optional but cleaner.

