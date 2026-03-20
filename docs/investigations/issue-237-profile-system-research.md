# Issue #237 Research: Built-in Profile System with Self-Improving Efficiency Review Loop

**Date:** 2026-03-18
**Status:** Open (blocked, needs-clarification)

---

## Executive Summary

Issue #237 proposes a named profile system for subagent configuration (tool allowlists, model, max_steps, system prompt, cost ceiling) plus a post-run efficiency review loop that improves profiles automatically. The blockers (#234 and #236) are both closed and delivered. The core infrastructure needed to implement this is largely already in place — the profile system would be a relatively thin coordination layer on top of existing machinery. The efficiency review loop is the more novel and speculative piece.

---

## 1. What the Closed Blockers Delivered

### #234 — Per-run tool filtering, system prompt, and permissions forwarding (CLOSED)

**What it delivered:**

- `RunRequest.AllowedTools []string` — when non-empty, only the listed tools (plus `AlwaysAvailableTools`) are visible to the LLM for the full run.
- `runner.filteredToolsForRun(runID)` — a function that applies both the per-run base filter (`runState.allowedTools`) and any active skill constraint on top of it. The per-run filter is applied when no skill constraint is active.
- `RunRequest.SystemPrompt` and `RunRequest.Permissions` forwarded from the HTTP `agents` endpoint and from `ForkConfig`.
- The enforcement path is fully wired: `StartRun` stores `req.AllowedTools` in `runState.allowedTools`, and `filteredToolsForRun` applies it on every step.
- `AlwaysAvailableTools` bypass the filter: `AskUserQuestion`, `find_tool`, `skill`.

**Implication for #237:** The core enforcement mechanism for profile tool allowlists already works. Profile loading just needs to translate `profile.tools.allow` into a `RunRequest.AllowedTools` slice. No new enforcement plumbing is needed.

**Gap from issue spec:** The issue's `IsBootstrap` flag on `SkillConstraint` is NOT in the codebase. The current implementation uses `runState.allowedTools` (a separate per-run field) rather than a persistent bootstrap constraint. The result is equivalent but achieved differently.

### #236 — Deterministic config propagation to subagent workspaces (CLOSED)

**What it delivered:**

- All `RunnerConfig` feature flags now have TOML representations in `internal/config/config.go` (forensics, auto-compact, conclusion watcher, etc.).
- `workspace.Options.ConfigTOML string` — each `Provision` implementation writes it to `<workspacePath>/.harness/config.toml`.
- `symphd.Dispatcher` populates `ConfigTOML` from a typed config at dispatch time.
- `internal/subagents/manager.go` carries `configTOML string` and passes it to worktree provisioning.
- Container workspaces pass API keys via `opts.Env` (not written to disk).

**Implication for #237:** A profile's `[runner]` section can be serialized to TOML and injected as `ConfigTOML` when the subagent runs in a worktree or container workspace. The `DefaultSubagentConfig()` helper mentioned in #236 is the right place to apply safe defaults (capped steps, cost ceiling enabled).

---

## 2. What Exists Today That #237 Builds On

### Profile infrastructure (partial)

The config system at `internal/config/config.go` already has:
- A 6-layer cascade: built-in defaults → `~/.harness/config.toml` → `.harness/config.toml` → named profile `~/.harness/profiles/<name>.toml` → env vars → (cloud stub).
- `LoadOptions.ProfileName` and `LoadOptions.ProfilesDir` fields.
- `ValidateProfileName()` — exported path traversal check, already used by the harness.
- `loadProfileMCPServers()` in `internal/harness/profile_mcp.go` — loads `[mcp_servers]` from a profile TOML and returns `map[string]config.MCPServerConfig`.
- `mergeProfileMCPIntoTOML()` in `internal/symphd/profile_mcp.go` — merges profile MCP servers into an existing TOML string (used for `ConfigTOML` injection).
- A test profile exists at `~/.harness/profiles/test-profile.toml`.

**What the existing profile layer does NOT do:**
- Does not have a `[meta]` section (name, description, efficiency_score, review_count).
- Does not have a `[tools]` section with an `allow` list.
- Does not have a `[runner]` section for model/max_steps/system_prompt/cost ceiling.
- There is no profile registry that knows about built-in embedded profiles.
- There is no `run_agent`, `create_profile`, or `list_profiles` tool.

The existing profile system is MCP-server-scoped: profiles are used only to load MCP server configs. The `[meta]`/`[tools]`/`[runner]` sections proposed in #237 would be entirely new.

### RunRequest and RunnerConfig

`RunRequest` already has:
- `Model` — model override per run.
- `MaxSteps` — step cap per run.
- `MaxCostUSD` — cost ceiling per run.
- `SystemPrompt` — system prompt override.
- `AllowedTools []string` — tool allowlist (from #234).
- `ProfileName string` — currently used only for MCP server loading; would be extended to load the full profile.
- `Permissions *PermissionConfig` — sandbox + approval policy.

`RunnerConfig` already has all the field representations needed to apply a profile's full configuration.

### Rollout system

`internal/rollout/recorder.go` writes `<RolloutDir>/<YYYY-MM-DD>/<run_id>.jsonl` for every run when `RunnerConfig.RolloutDir` is set.

`internal/forensics/rollout/loader.go` provides `LoadFile(path) ([]RolloutEvent, error)` with:
- Up to 100,000 events per file.
- Full integrity validation: monotonic steps, no post-terminal events, nesting depth caps, element count caps.
- `RolloutEvent` has `ID`, `Type`, `Step`, `Payload`, `Timestamp`.

The event types the efficiency reviewer needs already exist in rollouts:
- `llm.turn.completed` — marks an LLM turn with step number.
- `tool.call.started` / `tool.call.completed` — tool invocations with names and results.
- `run.started` / `run.completed` / `run.failed` — run boundaries.
- `tool.antipattern` — already emitted by the runner when `DetectAntiPatterns` is enabled.

### Training/scoring infrastructure

`internal/training/scorer.go` already computes an `Efficiency` score from a `TraceBundle`:

```go
efficiency := 1.0 / (1.0 + steps*0.1 + bundle.CostUSD*10.0)
```

`internal/training/storage.go` has an SQLite schema with `efficiency_score REAL` per run. This is the training/fine-tuning subsystem, not the profile efficiency loop — but the scoring formula is ready to reuse.

`internal/training/types.go` has `TraceBundle` with `ToolCalls []ToolCallTrace`, `AntiPatterns []AntiPatternAlert`, `EfficiencyScore`, etc. The profile efficiency report would be a subset of this.

### Subagent manager

`internal/subagents/manager.go` is the subagent execution engine. `subagents.Request` already has `ProfileName string` (passed through to `RunRequest.ProfileName`). The manager handles inline and worktree isolation, cleanup policies, and async monitoring via `go m.monitor(managed)`.

The `run_agent` tool proposed in #237 is essentially a wrapper that: resolves a profile name → builds a `subagents.Request` → calls `manager.Create()`. This is the correct integration point.

---

## 3. Concrete Design

### Profile schema (what a profile TOML looks like)

The issue's proposed TOML is sound. The `[meta]` section is the key extension over what exists today:

```toml
[meta]
name = "github"
description = "GitHub automation: issues, PRs, repo management"
version = 1
created_at = "2026-03-13"
created_by = "built-in"         # or "agent" for auto-created profiles
efficiency_score = 0.0          # updated by efficiency review loop
review_count = 0

[runner]
model = "gpt-4.1-mini"
max_steps = 20
max_cost_usd = 0.50
system_prompt = "You are a GitHub automation specialist."
auto_compact = true
auto_compact_mode = "hybrid"

[tools]
allow = ["bash", "read"]
```

The `[meta]` and `[tools]` sections are new keys that do NOT conflict with the existing `config.Config` TOML structure (which uses `[model]`, `[max_steps]`, `[cost]`, `[memory]`, `[auto_compact]`, `[forensics]`, `[mcp_servers]`). Adding a `[meta]` and `[tools]` section to the existing `rawLayer` struct is additive and backward compatible.

**Alternative:** Keep profiles as a completely separate struct in `internal/profiles/`, loaded independently from the harness config layer. This avoids polluting `config.Config` with agent-profile concerns. This is the recommended approach.

### Where profiles are stored

Three-tier resolution (matches the issue):
1. **Embedded built-ins** — `//go:embed` in `internal/profiles/builtins/*.toml`. Compiled into the binary, always available.
2. **User-global** — `~/.harness/profiles/<name>.toml`. Override or extend built-ins per user.
3. **Project-level** — `.harness/profiles/<name>.toml` in the workspace root. Override per-project.

Resolution order: project-level wins over user-global wins over built-ins. This mirrors how the config cascade works.

The existing `ValidateProfileName()` from `config.go` is already suitable for validating profile names before constructing file paths.

### How a profile is selected

**Current flow (what happens today when `RunRequest.ProfileName` is set):**

1. `runner.go` stores `req.ProfileName` in `runState.profileName`.
2. `runner.go` calls `loadProfileMCPServers(profilesDir, req.ProfileName)` to load MCP servers from the profile TOML.
3. MCP servers from the profile are added to a `ScopedMCPRegistry` for the run duration.

**Extended flow for #237:**

1. The `run_agent` tool (or `subagents.Manager.Create()`) resolves the profile name via `ProfileRegistry.Resolve(name)`.
2. `Resolve()` checks project-level → user-global → built-ins, returns a `Profile` struct.
3. The `Profile` is translated into `subagents.Request` fields:
   - `Model` ← `profile.Runner.Model`
   - `MaxSteps` ← `profile.Runner.MaxSteps`
   - `MaxCostUSD` ← `profile.Runner.MaxCostUSD`
   - `AllowedTools` ← `profile.Tools.Allow`
   - `ProfileName` ← profile name (for MCP server loading in runner.go)
4. `subagents.Manager.Create()` starts the run with these constraints.
5. `runState.allowedTools` is populated from `AllowedTools`, enforced by `filteredToolsForRun()`.

The `SystemPrompt` from the profile needs to be forwarded. The field already exists in `RunRequest`; the subagent manager already passes it through. The gap: `subagents.Request` does not currently have a `SystemPrompt` field. It needs to be added.

### New package: `internal/profiles/`

```
internal/profiles/
  profile.go          # Profile, ProfileMeta, ProfileRunner, ProfileTools types
  registry.go         # ProfileRegistry: resolve, list, save, embed-lookup
  embed.go            # //go:embed builtins/*.toml + loadEmbedded()
  builtins/
    github.toml
    file-writer.toml
    researcher.toml
    bash-runner.toml
    reviewer.toml
    full.toml
```

The `Profile` type:
```go
type Profile struct {
    Meta    ProfileMeta    `toml:"meta"`
    Runner  ProfileRunner  `toml:"runner"`
    Tools   ProfileTools   `toml:"tools"`
    MCP     config.MCPServerConfig `toml:"mcp_servers,omitempty"`
}

type ProfileMeta struct {
    Name            string    `toml:"name"`
    Description     string    `toml:"description"`
    Version         int       `toml:"version"`
    CreatedAt       string    `toml:"created_at"`
    CreatedBy       string    `toml:"created_by"` // "built-in" | "agent" | "user"
    EfficiencyScore float64   `toml:"efficiency_score"`
    ReviewCount     int       `toml:"review_count"`
}

type ProfileRunner struct {
    Model           string  `toml:"model"`
    MaxSteps        int     `toml:"max_steps"`
    MaxCostUSD      float64 `toml:"max_cost_usd"`
    SystemPrompt    string  `toml:"system_prompt"`
    AutoCompact     bool    `toml:"auto_compact"`
    AutoCompactMode string  `toml:"auto_compact_mode"`
}

type ProfileTools struct {
    Allow []string `toml:"allow"`
}
```

---

## 4. How the Efficiency Review Loop Would Work

### Concrete mechanism

After a subagent run using a profile completes, an optional post-run hook fires. The hook:

1. Checks trigger conditions:
   - First use of an agent-created profile (`profile.Meta.CreatedBy == "agent"` and `profile.Meta.ReviewCount == 0`).
   - Profile `EfficiencyScore` < 0.70 across last N uses (requires tracking per-run scores).
   - Explicit `review_efficiency: true` in the `run_agent` call.
   - Periodic: every 10th use of an agent-created profile.

2. If triggered, loads the rollout JSONL via `rollout.LoadFile(path)`.

3. Spawns a `reviewer` profile subagent with the rollout content (or a summarized version) as context. The reviewer subagent has `AllowedTools: ["read", "glob"]` and a system prompt instructing it to:
   - Identify tools in the allowlist that were never called.
   - Identify repeated calls to the same tool with near-identical arguments.
   - Identify tools the LLM attempted to call but were not available (these appear as error results in tool.call.completed events).
   - Compute a simple efficiency score.
   - Return structured JSON as its output.

4. The reviewer's JSON output is parsed as `EfficiencyReport`.

5. `EfficiencyReport` is persisted to `.harness/profiles/<name>.efficiency/<run_id>.json`.

6. `profile.Meta.EfficiencyScore` is updated (rolling average) and `profile.Meta.ReviewCount` incremented. The profile TOML is rewritten.

7. If `auto_apply` is configured and the score is below threshold, `suggested_refinements.remove_tools` entries are applied to `profile.Tools.Allow`.

### Where the hook lives

The hook belongs in the `subagents.Manager.monitor()` goroutine, which already listens for terminal events. After `IsTerminalEvent(ev.Type)` is detected and cleanup is applied, the efficiency review hook can fire asynchronously (it does not block the original `run_agent` return).

This avoids touching `runner.go` directly — the review loop is a subagent manager concern, not a runner concern.

### Rollout path availability

The `EfficiencyReport` needs `rollout_path`. This is the `RolloutDir/<YYYY-MM-DD>/<run_id>.jsonl` path. The runner knows `RolloutDir` (from `RunnerConfig`); the issue proposes returning `rollout_path` in the `RunResult` from `run_agent`. Since `subagents.Subagent` already has the `RunID`, the rollout path can be reconstructed as `filepath.Join(RolloutDir, date, runID+".jsonl")`.

For the efficiency reviewer to read the rollout, `RolloutDir` must be set in `RunnerConfig` (i.e., forensics/rollout recording must be enabled). If it is not set, the efficiency review is skipped silently.

### Efficiency scoring formula

The existing `training.Scorer.Score()` formula can be used as a starting point:

```go
efficiency := 1.0 / (1.0 + steps*0.1 + costUSD*10.0)
```

The LLM reviewer's role is to provide qualitative observations (tool redundancy, missing tools, system prompt suggestions) rather than to compute the score itself. The structural score can be computed deterministically from the rollout events without an LLM call. The LLM adds value for the qualitative suggestions only.

---

## 5. The `run_agent` Tool

This is a new tool in `internal/harness/tools/` that wraps the subagent manager. It is NOT a new execution path — it calls the same `subagents.Manager.Create()` that exists today.

**Proposed signature:**
```json
{
  "profile": "github",
  "prompt": "Close all stale issues",
  "workspace": "local",
  "wait": true,
  "review_efficiency": false,
  "overrides": {
    "max_steps": 25,
    "max_cost_usd": 1.00
  }
}
```

**Differences from existing `agent` tool:**
- The existing `agent` tool calls `AgentRunner.RunPrompt()` — a synchronous call that returns the output directly, with no profile, isolation, or workspace control.
- `run_agent` calls `subagents.Manager.Create()` + waits for completion (when `wait: true`) — with profile resolution, isolation selection, and workspace provisioning.

The `run_agent` tool needs access to:
- The `ProfileRegistry` (to resolve profile names).
- The `subagents.Manager` (to create/poll subagents).
- The `RolloutDir` string (to construct rollout paths for efficiency review).

These would be injected into `BuildOptions` as new fields.

---

## 6. The `create_profile` and `list_profiles` Tools

### `create_profile`

Validates inputs, optionally persists to `.harness/profiles/<name>.toml`. Key validation:
- `ValidateProfileName()` from `config.go` (already exists).
- Tool names in `allow` must be from the registered tool catalog (requires `BuildOptions.ToolNames []string` injection or a `ToolRegistry` interface).
- No profile with the same name already exists (unless `overwrite: true`).

When `save: false`, the profile is one-shot and never written to disk. The `run_agent` call would need to accept an inline profile definition as an alternative to a name.

### `list_profiles`

Returns name + description + efficiency_score + created_by for all available profiles. Queries the `ProfileRegistry` which walks: embedded built-ins + `~/.harness/profiles/` + `.harness/profiles/`.

---

## 7. Storage: TOML Files vs SQLite

**Issue recommendation:** TOML files at `~/.harness/profiles/<name>.toml` and `.harness/profiles/<name>.toml`.

**Analysis:**

TOML files are the right choice for the profile definitions themselves:
- Human-readable and directly editable.
- Version-controllable.
- Matches the existing config cascade pattern.
- Embedded built-ins use `//go:embed` (already established pattern in tool descriptions).

SQLite is the right choice for efficiency tracking history:
- Multiple `EfficiencyReport` records per profile (one per reviewed run).
- Aggregation queries (rolling average score, last N scores).
- Append-only writes.

Proposed split:
- Profile definition: TOML file at `~/.harness/profiles/<name>.toml`.
- Efficiency reports: JSON files at `~/.harness/profiles/<name>.efficiency/<run_id>.json` (per-issue recommendation, matches forensics pattern).
- Efficiency metadata (score + review_count): written back into the TOML `[meta]` section on each review completion.

This avoids adding a new SQLite database dependency for the profile system (SQLite is already used for conversation store, cron, and training data).

---

## 8. Recommended Implementation Phases

### Phase 1: Profile registry + built-in profiles (no tools)
**Scope:** `internal/profiles/` package only.

Deliverables:
- `Profile`, `ProfileMeta`, `ProfileRunner`, `ProfileTools` types.
- `ProfileRegistry` with `Resolve(name)`, `List()`, `Save(profile)` methods.
- `embed.go` with `//go:embed builtins/*.toml` and embedded lookup.
- Built-in TOML files for `github`, `file-writer`, `researcher`, `bash-runner`, `reviewer`, `full`.
- Unit tests: resolve order (project > user > built-in), `ValidateProfileName` reuse, TOML round-trip.

**No runner changes.** This phase is pure data model + file I/O.

### Phase 2: `run_agent` tool — local/inline workspace
**Scope:** New tool in `internal/harness/tools/` + subagent manager extension.

Deliverables:
- `run_agent` tool: resolves profile → builds `subagents.Request` → calls `manager.Create()` → polls for completion (when `wait: true`).
- Add `SystemPrompt string` to `subagents.Request` (gap identified above).
- Add `ProfileRegistry` and `SubagentManager` to `BuildOptions`.
- Tool description at `internal/harness/tools/descriptions/run_agent.md`.
- Tests: profile resolution integration, tool allowlist enforcement, wait vs fire-and-forget.

### Phase 3: `create_profile` and `list_profiles` tools
**Scope:** Two new tools using `ProfileRegistry`.

Deliverables:
- `create_profile` tool: validates, optionally persists.
- `list_profiles` tool: returns name + description + score.
- Tool descriptions at `descriptions/create_profile.md` and `descriptions/list_profiles.md`.
- Tests: name validation, persistence, list ordering.

### Phase 4: Efficiency review loop
**Scope:** Post-run hook in `subagents.Manager.monitor()`.

Deliverables:
- `EfficiencyReport` and `ProfileRefinements` types in `internal/profiles/`.
- Trigger condition evaluation (first use, score < 0.70, explicit flag, periodic).
- Structural scorer (reuse/adapt `training.Scorer` formula).
- Async reviewer subagent spawn via `manager.Create()` with the `reviewer` profile.
- Report persistence to `.harness/profiles/<name>.efficiency/<run_id>.json`.
- Profile TOML update (score + review_count).
- Tests: trigger conditions, report parsing, score update.

**Dependency:** Phase 4 requires `RunnerConfig.RolloutDir` to be set (rollout recording must be enabled). If not set, the review is skipped.

### Phase 5: `run_agent` with container/VM workspace
**Scope:** Extend Phase 2 to non-local workspace types.

This is where #236's `ConfigTOML` injection and `workspace.Options.ConfigTOML` are used to carry the profile's `[runner]` section into a spawned harnessd process. This phase adds profile TOML serialization into `ConfigTOML` via `mergeProfileMCPIntoTOML()` (already in symphd) plus a new `serializeProfileToRunnerTOML()` function.

---

## 9. Open Questions That Need User Decisions

1. **Profile versioning**: Should profile files be versioned? When the efficiency loop applies refinements, should the old version be preserved as `<name>.v1.toml`? Without versioning, there is no rollback path if an auto-applied refinement regresses performance. **Recommendation: keep old version as `<name>.prev.toml` on auto-apply; manual applies have no auto-save of old version.**

2. **Auto-apply vs suggest-only**: The issue acknowledges the risk. Auto-applying removes tools from the allowlist without explicit human approval. **Recommendation: default `auto_apply = false`; reviewer writes report + pending refinements; applying requires explicit `create_profile` call with `apply_refinements: true`.**

3. **Score visibility to calling agent**: Should the `run_agent` result include `efficiency_score` from the profile metadata? The calling agent could use this to prefer higher-scoring profiles. **Recommendation: include it in `list_profiles` output; including it in `run_agent` result is optional.**

4. **Cross-project profile scores**: User-global profiles (`~/.harness/profiles/`) are shared across projects. An efficiency score for a `researcher` profile measured on a documentation project may not generalize to a code-analysis project. **Recommendation: store per-project efficiency reports under `.harness/profiles/<name>.efficiency/` and keep the user-global TOML's score as an aggregate. Per-project `list_profiles` should show the project-local score when available.**

5. **Tool name validation in `create_profile`**: Validating that `allow` tool names exist requires access to the registered tool catalog at tool-construction time. This is a chicken-and-egg problem — the tool catalog is built in `BuildCatalog()` which runs after `create_profile` is constructed. **Recommendation: validate at run time (when `create_profile` is called), by passing a `ToolNameSet` into the tool handler via closure at `BuildCatalog` time.**

6. **Profile discovery / fuzzy matching**: Should `run_agent` support fuzzy profile selection ("find me the best profile for GitHub operations")? The issue mentions this as a question. **Recommendation: defer to a future issue; MVP is exact name match only.**

7. **`reviewer` profile bootstrapping problem**: The efficiency review loop spawns a subagent using the `reviewer` profile. If the `reviewer` profile itself has never been reviewed, does it trigger a review of the review? **Recommendation: built-in profiles are never subject to efficiency review (only agent-created profiles are reviewed). Add `review_eligible = false` to built-in TOML meta sections.**

8. **`run_agent` in the tool tier system**: Should `run_agent` be `TierCore` or `TierDeferred`? Given that it is a high-power orchestration tool, `TierDeferred` (hidden until `find_tool` discovers it) would reduce token overhead for runs that don't need subagent spawning. **Recommendation: `TierDeferred`.**

---

## 10. Architecture Decision: `run_agent` Is Not a New Code Path

A key clarification from reading the codebase: `run_agent` does NOT need new execution infrastructure. The subagent manager (`internal/subagents/manager.go`) already provides:
- Inline and worktree isolation.
- Async monitoring (via goroutine).
- Cleanup policies.
- `RunRequest` forwarding with all required fields (model, max_steps, max_cost_usd, allowed_tools, system_prompt, permissions).

`run_agent` is a tool that wraps the subagent manager with profile resolution on top. The profile resolution layer maps a profile name to the fields already supported by `subagents.Request`. No new runner machinery is needed.

The only gap: `subagents.Request` does not have a `SystemPrompt` field. Adding it is a one-line change to `internal/subagents/manager.go`.

---

## 11. Files That Need to Change

| File | Change |
|---|---|
| `internal/profiles/` (new) | New package: Profile types, registry, embed, built-in TOMLs |
| `internal/subagents/manager.go` | Add `SystemPrompt string` to `Request` and forward to `RunRequest` |
| `internal/harness/tools/catalog.go` | Add `run_agent`, `create_profile`, `list_profiles` to `BuildOptions` + `BuildCatalog` |
| `internal/harness/tools/types.go` | Add `ProfileRegistry`, `SubagentManager`, `RolloutDir` to `BuildOptions` |
| `internal/harness/tools/run_agent.go` (new) | `run_agent` tool implementation |
| `internal/harness/tools/profile_tools.go` (new) | `create_profile` and `list_profiles` tools |
| `internal/harness/tools/descriptions/run_agent.md` (new) | Tool description |
| `internal/harness/tools/descriptions/create_profile.md` (new) | Tool description |
| `internal/harness/tools/descriptions/list_profiles.md` (new) | Tool description |
| `cmd/harnessd/main.go` | Wire `ProfileRegistry` and pass it into `BuildOptions` |

The efficiency review loop additions go in `internal/subagents/manager.go` (post-terminal hook) and `internal/profiles/efficiency.go` (new: `EfficiencyReport` type + file I/O).

---

## 12. Risk Areas

1. **Rollout path construction**: The reviewer needs the rollout path. If `RolloutDir` is not configured, the review is silently skipped. The `run_agent` tool needs to handle this gracefully and document it clearly.

2. **Reviewer subagent spawning loops**: The efficiency review spawns a subagent using the `reviewer` built-in profile. That subagent uses the inline runner (no worktree). If the reviewer's run itself fails (e.g., LLM error), the parent's `EfficiencyReport` is missing but the original run is already complete — this is safe.

3. **Profile file writes during concurrent runs**: If two runs using the same profile complete simultaneously and both trigger a review, both would try to update `profile.Meta.EfficiencyScore` and rewrite the TOML file. This requires file-level locking or atomic write (write to temp + rename). **Recommendation: use `os.Rename` for atomic TOML updates.**

4. **Built-in profile `[tools].allow` vs 30+ tool names**: The `researcher` profile's `allow = ["read", "grep", "glob", "ls", "web_search", "web_fetch"]` must match the exact tool names registered in the catalog. Tool names that change break built-in profiles silently. A startup validation step (comparing built-in profiles against registered tool names) would catch this early.
