# Grooming: Issue #379 — feat(profiles): expand profile schema with runtime and safety policy

## Already Addressed?

Partial — Some of the target fields already exist in adjacent structs but are absent from `Profile` itself.

What the current `Profile` schema in `internal/profiles/profile.go` contains:
- `ProfileMeta`: name, description, version, created_at, created_by, efficiency_score, review_count, review_eligible.
- `ProfileRunner`: model, max_steps, max_cost_usd, system_prompt.
- `ProfileTools`: allow ([]string tool allowlist).
- `MCPServers`: map of MCP server configs.

What the plan says is missing from the profile schema (new fields to add):
- **Permissions / sandbox / approval** — already modeled in `harness.PermissionConfig` (sandbox scope + approval policy in `internal/harness/types.go` lines 577–606) and in `subagents.Request` (line 54: `Permissions *harness.PermissionConfig`), but not in the profile TOML struct.
- **Isolation mode** — already modeled as `subagents.IsolationMode` (inline/worktree) and in `subagents.Request` (line 55), but absent from `Profile`.
- **Cleanup policy** — already modeled as `subagents.CleanupPolicy` (preserve/destroy_on_success/destroy_on_completion) and in `subagents.Request` (line 56), but absent from `Profile`.
- **Base ref / worktree behavior** — already in `subagents.Request` (lines 57–58: `WorktreeRoot`, `BaseRef`), absent from `Profile`.
- **Reasoning effort** — already in `subagents.Request` (line 51) and in `harness.RunRequest` (line 310), but absent from `Profile`.
- **Output / result mode** — not yet modeled anywhere; this is the most greenfield field.

The `run_agent` handler in `run_agent.go` only extracts `Model`, `MaxSteps`, `MaxCostUSD`, `SystemPrompt`, and `AllowedTools` from the profile via `ApplyValues()`. The richer fields like permissions, isolation, and reasoning effort are not plumbed through even if they were added to `Profile`.

## Clarity

Clear — The plan enumerates the fields to add and names the relevant structs where those concepts already exist. The "optional output/result mode" is the least specified field and will need a design decision (what values are valid, how does it affect the child run). All other fields map to existing enum types or string fields in adjacent structs.

## Acceptance Criteria

Present in the plan file. Explicit criteria:

1. Failing encode/decode tests are written first for each new schema field.
2. New fields are added to `ProfileRunner` or a new `ProfilePolicy` section in `internal/profiles/profile.go`.
3. Existing profiles (all six built-ins) remain loadable without error after the schema expansion (backward compatibility: new fields are optional with TOML zero values).
4. `ApplyValues()` is extended to return the new fields in `ProfileValues`, or a new `ApplyRuntime()` / `ApplyPolicy()` method is added.
5. `run_agent.go` is updated to forward the new fields when building the `SubagentRequest`.
6. At least one built-in profile is updated with non-zero values for one or more new fields as a golden-path test.

## Scope

Medium — Adding schema fields to `Profile`, extending `ProfileValues`, updating `ApplyValues()`, and updating `run_agent.go` to forward the new values. The `subagents.Request` already accepts `Permissions`, `Isolation`, `CleanupPolicy`, `BaseRef`, and `WorktreeRoot`, so the wiring layer already exists. The main work is adding the fields to the profile schema, ensuring backward-compatible TOML parsing, and threading the values through `run_agent`.

## Blockers

Blocked on #378 (per plan dependency graph: "Ticket 4"). #378 must land first because #379 adds new fields that the create/validate surfaces (#378) will need to handle. If #379 lands before #378, the validate endpoint won't know about the new fields.

In practice, #376 (fail-closed) and #377 (discovery) are also recommended first because the expanded schema is most useful when callers can discover, inspect, and create profiles through first-class surfaces.

## Recommended Labels

well-specified, medium, blocked

## Effort

Medium — Schema additions are straightforward but the full plumbing (profile struct → `ProfileValues` → `SubagentRequest` → `RunRequest`) touches four files. The output/result mode field requires additional design time. Estimated 4–8 hours excluding the output mode design.

## Recommendation

well-specified (blocked on #378)

## Notes

- **TOML backward compatibility**: All new fields must be optional (zero values) so existing profile files continue to parse without error. The `BurntSushi/toml` decoder ignores unknown/missing fields by default, so adding new optional fields to the Go struct is safe.
- **`harness.PermissionConfig` vs. inline fields**: The cleanest approach is to embed `PermissionConfig` directly in the profile struct (or reference it by value) rather than duplicating `sandbox` and `approval` as separate string fields. This reuses the existing type and its validation function.
- **Isolation and cleanup policy**: These are subagent-level concepts (which workspace backend to use), not runner-level concepts. They belong in a new `[subagent]` or `[isolation]` TOML section rather than `[runner]`.
- **Reasoning effort**: Belongs in `[runner]` alongside `model` and `max_steps`. Already a string field on `RunRequest`.
- **Output/result mode**: The most open-ended field. A conservative first pass could be a string enum like `"text"` | `"structured"` (matching task_complete return expectations). Deferred design is acceptable; leave this field as a comment/TODO in the struct for now.
- The `tools.SubagentRequest` struct in `internal/harness/tools/types.go` (mirror of `subagents.Request`) will also need updating to add the new fields so `run_agent.go` can forward them without an import cycle.
