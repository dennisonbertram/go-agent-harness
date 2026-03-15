# Issue #234 Implementation Summary

## Date
2026-03-13

## Branch
`issue-234-per-run-tool-filtering`

## Commits
- `0a0c528` — `test(#234): regression tests for per-run tool filtering and RunForkedSkill`
- `c52d715` — `feat(#234): wire AllowedTools, SystemPrompt, Permissions forwarding to runner`

## What Was Implemented

### 1. `internal/harness/types.go`
Added `AllowedTools []string` to `RunRequest`:
```go
AllowedTools []string `json:"allowed_tools,omitempty"`
```
HTTP JSON decode auto-populates this field from the request body — no handler changes needed.

### 2. `internal/harness/runner.go`

**`runState` struct**: Added `allowedTools []string` field — stores the per-run base filter for the lifetime of the run.

**`StartRun()`**: Populates `state.allowedTools = req.AllowedTools`.

**`filteredToolsForRun()`**: Two-layer filtering:
1. Skill constraint layer (higher priority, active during skill execution): if a skill constraint is active with non-nil `AllowedTools`, only those tools are offered.
2. Per-run base layer (fallback): if no skill constraint active (or constraint has nil AllowedTools), apply `runState.allowedTools` as base filter.

**`RunPrompt(ctx, prompt)`**: New method satisfying `htools.AgentRunner`. Starts a new run and waits for completion, returning the final output string.

**`RunForkedSkill(ctx, config)`**: New method satisfying `htools.ForkedAgentRunner`. Starts a sub-run with `ForkConfig.AllowedTools`, inheriting parent run's `SystemPrompt` and `Permissions` from the parent run ID in context.

**`forkResultFromRun(runID)`**: Helper that extracts `ForkResult` from a completed run state.

### 3. `internal/harness/runner_tool_filter_test.go`
11 tests covering all new paths:
- `TestAllowedTools_LimitsAvailableTools` — per-run filter restricts tools
- `TestAllowedTools_EmptyMeansNoFilter` — nil/empty = no filter
- `TestAllowedTools_SkillConstraintOverrides` — skill constraint takes precedence
- `TestAllowedTools_BaseFilterAppliesEvenWithSkillNilAllowedTools` — base filter is a security boundary; skill with nil AllowedTools doesn't remove it
- `TestRunForkedSkill_ImplementedByRunner` — compile-time interface check
- `TestRunForkedSkill_ForwardsAllowedTools` — sub-run respects ForkConfig.AllowedTools
- `TestRunForkedSkill_InheritsParentSystemPrompt` — system prompt propagated
- `TestRunForkedSkill_InheritsParentPermissions` — permissions propagated
- `TestAllowedTools_RaceConditionSafe` — concurrent runs don't interfere
- `TestRunPrompt_ReturnsOutput` — RunPrompt returns final output
- `TestRunPrompt_RespectsContextCancellation` — RunPrompt honors ctx cancellation

## Design Decisions

### Two-Layer Filtering (no IsBootstrap needed)
Instead of adding `IsBootstrap bool` to `SkillConstraint`, used a clean two-layer approach:
- Skill constraints live in `SkillConstraintTracker` (cleaned up at run complete)
- Per-run base `AllowedTools` live in `runState.allowedTools` (persists for run lifetime)
- `filteredToolsForRun()` checks skill constraint first (higher priority), falls back to base

### Parent Run Inheritance in RunForkedSkill
Uses `htools.RunMetadataFromContext(ctx)` to get parent run ID, then looks up `runState.staticSystemPrompt` and `runState.permissions` under read lock. No circular imports needed — avoids adding harness types to tools package.

## Test Results
- All 11 new tests pass
- Race detector clean
- `internal/harness` coverage: 85.6% (above 80% threshold)
- Pre-existing 0% functions unchanged (not introduced by this PR):
  - `cmd/forensics/main.go:main`
  - `internal/forensics/redaction/redaction.go:deepTransformValue`
  - `internal/provider/openai/client.go:injectAdditionalPropertiesFalse`
