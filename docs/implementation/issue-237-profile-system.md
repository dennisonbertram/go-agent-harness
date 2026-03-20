# Issue #237: Agent Profile System

## Summary

Implements the built-in profile system with suggest-only efficiency review loop (issue #237). Profiles are TOML files that encode agent configuration (model, step budget, cost limit, system prompt, allowed tools) and can be applied to `run_agent` subagent calls. After each profiled run, an efficiency score is computed and if it falls below 0.6 an `EventProfileEfficiencySuggestion` event is emitted — no auto-apply.

## Files Added

### `internal/profiles/`

- **`profile.go`** — Core types: `Profile`, `ProfileMeta`, `ProfileRunner`, `ProfileTools`, `ProfileValues`, `EfficiencyReport`
  - `ApplyValues()` returns a `ProfileValues` copy (safe for callers to mutate)
- **`loader.go`** — Three-tier profile resolution (project-level → user-global → built-in embedded)
  - `LoadProfile(name)` — uses `defaultProjectProfilesDir()` and `defaultUserProfilesDir()`
  - `LoadProfileFromUserDir(name, dir)` — explicit user dir (used by run_agent tool)
  - `ListProfiles()` — deduplicated names across all three tiers
  - `SaveProfile(p)` — atomic write (write to `.tmp`, then `os.Rename`) to user dir
  - `//go:embed builtins/*.toml` for zero-deployment embedded profiles
- **`efficiency.go`** — Scoring and suggestion logic
  - `ScoreEfficiency(steps, costUSD)` → `1/(1+steps*0.1+cost*10)`
  - `ShouldEmitSuggestion(score)` → `score < 0.6`
  - `BuildEfficiencyReport(stats)` → computes unused tools and removal suggestions
- **`profile_test.go`** — 16 tests covering all three resolution tiers, deduplication, save/load round-trip, slice copy safety, invalid names
- **`efficiency_test.go`** — 5 tests covering scoring formula, threshold, report building

### `internal/profiles/builtins/`

Six embedded TOML profiles (all `review_eligible = false`):
- `github.toml` — GitHub automation via gh CLI, gpt-4.1-mini, 20 steps, $0.50
- `file-writer.toml` — File creation/modification, 15 steps, $0.30
- `researcher.toml` — Research/investigation, gpt-4.1, 30 steps, $1.00
- `bash-runner.toml` — Shell execution specialist, 10 steps, $0.20
- `reviewer.toml` — Code review, 25 steps, $0.75
- `full.toml` — General purpose, full tool access, 50 steps, $2.00

### `internal/harness/tools/deferred/run_agent.go`

`RunAgentTool(manager, profilesDir)` — TierDeferred tool:
- Loads the named profile via `LoadProfileFromUserDir` (falls back to built-ins)
- Applies profile values (model, max_steps, max_cost_usd, system_prompt, allowed_tools)
- Allows per-call overrides for `model` and `max_steps`
- Defaults to `"full"` profile when none specified
- Calls `manager.CreateAndWait(ctx, req)` and returns result

### `internal/harness/tools/deferred/run_agent_test.go`

8 tests: basic execution, profile defaults, value application, model/max_steps overrides, nil manager, task/error forwarding.

### `internal/harness/tools/deferred/task_complete_test.go`

10 tests covering `TaskCompleteTool` (from feat#235): basic completion, status defaults, partial/failed status, findings, depth gate, empty summary, invalid status, invalid JSON, marker output.

### `internal/harness/tools/descriptions/run_agent.md`

Tool description for the deferred `run_agent` tool.

### `internal/subagents/inline_manager.go`

`InlineManager` implementing `tools.SubagentManager`:
- `NewInlineManager(m Manager) *InlineManager`
- `CreateAndWait(ctx, req)` — creates inline subagent, polls at 500ms until terminal status

### `internal/subagents/system_prompt_test.go`

6 tests: SystemPrompt forwarding, empty/trimmed SystemPrompt, NewInlineManager construction, CreateAndWait completion, SystemPrompt forwarding through CreateAndWait.

### `internal/harness/runner_profile_efficiency_test.go`

3 tests using `minimalRunner()` (nil provider):
- Emitted when score < 0.6 (50 steps → score ≈ 0.167)
- Not emitted when score >= 0.6 (1 step → score ≈ 0.91)
- Not emitted when no profile name set

## Files Modified

### `internal/harness/events.go`

Added `EventProfileEfficiencySuggestion EventType = "profile.efficiency_suggestion"` and included it in `AllEventTypes()` (count → 76).

### `internal/harness/runner.go`

- Added `"go-agent-harness/internal/profiles"` import
- Added `maybeEmitProfileEfficiencySuggestion(runID, costUSD)` method
- Called before `EventRunCompleted` in `completeRun()`

### `internal/harness/tools/types.go`

Added `SubagentManager` interface, `SubagentRequest`, `SubagentResult` types, and `SubagentManager`/`ProfilesDir` fields to `BuildOptions`.

### `internal/harness/tools_default.go`

Added `SubagentManager` and `ProfilesDir` to `DefaultRegistryOptions`; registers `run_agent` when SubagentManager is non-nil.

### `internal/harness/tools_contract_test.go`

Updated expected tool list to include 5 pre-existing git deep tools (git_blame_context, git_contributor_context, git_diff_range, git_file_history, git_log_search).

### `internal/harness/tools/descriptions/embed_test.go`

Added `"run_agent"` to `TestEmbeddedFSAndKnownListAreInSync`.

### `internal/subagents/manager.go`

- Added `SystemPrompt string` field to `Request` struct
- Forward it in `Create()`: `SystemPrompt: strings.TrimSpace(req.SystemPrompt)`
- Fixed data race in `Create()`: snapshot `managed.Subagent` while holding `m.mu` before launching the monitor goroutine

## Design Decisions

### Import Cycle Prevention

`tools/deferred` cannot import `subagents` (which imports `harness` which imports `tools`). Solution: define `SubagentManager` interface and `SubagentRequest`/`SubagentResult` in `tools/types.go`, with `InlineManager` in `subagents` implementing that interface.

### Suggest-Only Efficiency Review

The efficiency event is emitted but no profile changes are auto-applied. This is intentional — the suggest-only model lets operators review suggestions before updating profiles.

### Atomic Profile Writes

`saveProfileToDir` writes to `path.tmp` then `os.Rename` to prevent corruption during concurrent writes.

### Race Fix in subagents.Manager

The `Create()` method previously had a data race: it launched `go m.monitor(managed)` then returned `managed.Subagent`, but the monitor goroutine could write to `managed` fields via `refresh()` concurrently. Fix: snapshot `managed.Subagent` while holding `m.mu` before launching the goroutine.

## Test Results

```
[regression] go test ./internal/... ./cmd/...
... (all ok)
[regression] go test ./internal/... ./cmd/... -race
... (all ok)
[regression] coverage gate: min total 80.0% + no zero-coverage functions
[regression] PASS
```

Total coverage: 85.0% of statements.
