# Plan: Profile-Backed Subagent Harness Backlog

## Context

- Problem:
  - The repo now has a partial profile system and two overlapping delegation paths, but not a coherent "sub-harness profile" model.
  - `run_agent` is profile-backed, but `spawn_agent` still behaves like a separate mechanism.
  - Profiles are loadable in code, but there is no first-class CRUD/discovery surface, no reliable context handoff contract, and no full regression suite for profile-backed delegation.
- User impact:
  - Operators cannot safely rely on profiles as reusable worker types.
  - Agents cannot discover, inspect, create, or debug profiles without repo-level knowledge.
  - Small models cannot take this work piecemeal because the current backlog is too umbrella-shaped.
- Constraints:
  - Strict TDD for every implementation ticket.
  - Every behavior fix needs regression coverage.
  - Keep tickets independently executable by smaller models.
  - Preserve existing run/delegation behavior unless a ticket explicitly changes it.
  - Avoid broad rewrites; converge the existing `run_agent`, `spawn_agent`, subagent manager, and profile loader surfaces.

## Scope

- In scope:
  - Break profile-backed delegation into small implementation tickets.
  - Define context handoff, result contracts, profile CRUD, observability, and regression coverage.
  - Create GitHub tickets with enough detail for direct implementation.
- Out of scope:
  - Delivering the implementation in this planning pass.
  - Reworking unrelated provider/model plumbing unless directly required by a ticket.

## Test Plan (TDD)

- New failing tests to add first:
  - Delegation contract tests for `run_agent` and `spawn_agent`.
  - Profile loader/validation/CRUD tests.
  - HTTP + tool-surface tests for profile endpoints/tools.
  - Subagent lifecycle tests for async orchestration.
  - Parent/child context-handoff and result-contract tests.
  - Rollout/efficiency persistence tests.
  - Smoke/integration tests for the supported profile-backed path.
- Existing tests to update:
  - `internal/harness/tools_contract_test.go`
  - `internal/harness/tools/deferred/*_test.go`
  - `internal/profiles/*_test.go`
  - `internal/subagents/*_test.go`
  - `internal/server/http_*_test.go`
  - `cmd/harnesscli/*_test.go` and relevant TUI tests for profile UX
- Regression tests required:
  - Unknown-profile behavior must be pinned.
  - Delegation-path parity between `run_agent` and `spawn_agent` must be pinned.
  - Profile CRUD and schema expansion must be pinned.
  - Context-handoff payload structure and truncation behavior must be pinned.
  - Structured child results and artifact references must be pinned.
  - Per-profile stats/efficiency persistence must be pinned.
  - End-to-end smoke path must be repeatable from one script/entrypoint.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- For this backlog, only the profile recommendation and any model-routing UX tickets should require a dedicated impact map before implementation.
- Most tickets here primarily touch:
  - Config
  - Server API
  - Tool registry
  - Subagent orchestration
  - TUI/CLI profile UX
  - Regression tests

## Context Implementation Approach

Profiles should become execution contracts, not just prompt presets.

### Parent-to-child context rules

- Do not pass the full parent transcript by default.
- Build a compact handoff bundle with:
  - task summary
  - explicit success criteria
  - relevant file paths
  - selected prior findings or snippets
  - profile/runtime constraints
  - optional artifact references
- Keep the handoff deterministic and serializable so it can be:
  - stored on the child run
  - inspected in tests
  - replayed after resume/restart

### Context implementation shape

- Add a typed handoff structure in the harness layer instead of assembling ad hoc prompt strings.
- Render that structure into the child system/user prompt at the last possible boundary.
- Support explicit size limits and truncation markers so a parent cannot accidentally blow out the child context window.
- Make artifact references first-class so large outputs stay out of the immediate prompt.

### Recommended handoff fields

- `task`
- `goal`
- `definition_of_done`
- `constraints`
- `relevant_files`
- `relevant_snippets`
- `prior_findings`
- `artifacts`
- `parent_run_id`
- `profile_name`

## Where To Find Additional Context

- Profile implementation and loader:
  - `internal/profiles/profile.go`
  - `internal/profiles/loader.go`
  - `internal/profiles/efficiency.go`
- Current profile-backed delegation path:
  - `internal/harness/tools/deferred/run_agent.go`
- Current recursive delegation path:
  - `internal/harness/tools/deferred/spawn_agent.go`
  - `internal/harness/tools/deferred/task_complete.go`
  - `internal/subagents/manager.go`
- Default tool registry and deferred-tool behavior:
  - `internal/harness/tools_default.go`
  - `internal/harness/tools_contract_test.go`
- HTTP surfaces:
  - `internal/server/http_agents.go`
  - `internal/server/http_subagents.go`
  - `internal/server/http.go`
- MCP server surface:
  - `internal/mcpserver/mcpserver.go`
- Existing docs/research:
  - `docs/implementation/issue-237-profile-system.md`
  - `docs/investigations/issue-237-profile-system-research.md`
  - `docs/investigations/issue-235-recursive-spawning-research.md`
  - `docs/investigations/tool-catalog-review.md`
  - `docs/plans/mcp-server-richer-tools.md`

## Ticket Breakdown

### 1. Ticket #375: Fix `spawn_agent` parameter forwarding and define the canonical delegation split

- Goal:
  - Make `spawn_agent` either honor `profile`, `model`, and `max_steps` fully or explicitly narrow its contract and route profile-backed work through `run_agent`.
- Dependencies:
  - None.
- TDD first:
  - Add failing tests proving `spawn_agent` currently ignores profile/model inputs.
- Regression coverage:
  - Pin the chosen contract so future changes cannot reintroduce dead parameters.
- Implementation notes:
  - Prefer one of two outcomes:
    - `spawn_agent` becomes a thin wrapper over profile-aware delegation, or
    - `spawn_agent` is narrowed to a lower-level primitive and its schema/docs are corrected.
- Additional context:
  - `internal/harness/tools/deferred/spawn_agent.go`
  - `internal/harness/tools/deferred/run_agent.go`
  - `internal/harness/tools/types.go`

### 2. Ticket #376: Make `run_agent` fail closed on unknown profiles

- Goal:
  - Prevent silent fallback to an empty unrestricted profile on typos or missing names.
- Dependencies:
  - None.
- TDD first:
  - Add a failing tool test for unknown profile names.
- Regression coverage:
  - Pin error shape for missing profile, invalid profile name, and unreadable profile file.
- Implementation notes:
  - Return actionable error text.
  - Keep built-in fallback only when the named built-in actually exists.
- Additional context:
  - `internal/harness/tools/deferred/run_agent.go`
  - `internal/profiles/loader.go`

### 3. Ticket #377: Add read-only profile discovery surfaces: `list_profiles` and `get_profile`

- Goal:
  - Expose profile discovery to tools and HTTP callers.
- Dependencies:
  - Ticket 2 recommended first, but not required.
- TDD first:
  - Add failing handler tests for list/get profile operations.
- Regression coverage:
  - Pin project-over-user-over-built-in resolution order in the surfaced payload.
- Implementation notes:
  - Add both tool and HTTP surfaces.
  - Return metadata, runner config, tool allowlist, and source tier.
- Additional context:
  - `internal/profiles/loader.go`
  - `internal/server/http_*.go`
  - `internal/harness/tools_default.go`

### 4. Ticket #378: Add mutating profile management surfaces: create, update, delete, validate

- Goal:
  - Let operators and agents manage profile files without direct filesystem edits.
- Dependencies:
  - Ticket 3.
- TDD first:
  - Add failing tests for create/update/delete/validate operations.
- Regression coverage:
  - Path traversal, invalid names, partial writes, and builtin protection must be pinned.
- Implementation notes:
  - Use atomic writes.
  - Decide whether built-ins are immutable and user/project profiles shadow them.
- Additional context:
  - `internal/profiles/loader.go`
  - `internal/config/config.go`
  - `internal/server/http_*.go`

### 5. Ticket #379: Expand the profile schema to cover runtime and safety policy

- Goal:
  - Make profiles capable of defining real worker contracts.
- Dependencies:
  - Ticket 4.
- TDD first:
  - Add failing encode/decode tests for new schema fields.
- Regression coverage:
  - Ensure old profiles remain loadable.
- Implementation notes:
  - Add fields for:
    - permissions / sandbox / approval
    - isolation mode
    - cleanup policy
    - base ref / worktree behavior
    - reasoning effort
    - optional output/result mode
- Additional context:
  - `internal/profiles/profile.go`
  - `internal/subagents/manager.go`
  - `internal/harness/types.go`

### 6. Ticket #380: Add profile inheritance and composition with `extends`

- Goal:
  - Avoid copy-paste profiles and make small-model customization practical.
- Dependencies:
  - Ticket 5.
- TDD first:
  - Add failing tests for single inheritance, override precedence, and cycle detection.
- Regression coverage:
  - Pin merge semantics for tools, prompts, and runtime fields.
- Implementation notes:
  - Keep inheritance shallow and deterministic.
  - Fail on cycles or unknown bases.
- Additional context:
  - `internal/profiles/loader.go`
  - `internal/profiles/profile.go`

### 7. Ticket #381: Add profile/run tool-manifest introspection

- Goal:
  - Make it easy to inspect the exact tool set a profile-backed run can see.
- Dependencies:
  - Tickets 3 and 5.
- TDD first:
  - Add failing tests for manifest generation under core, deferred, MCP, and script-tool conditions.
- Regression coverage:
  - Pin visible-tool output for at least one built-in profile.
- Implementation notes:
  - Expose both "declared allowlist" and "resolved active tools".
  - Include source and deferred/core tier information where available.
- Additional context:
  - `internal/harness/tools_default.go`
  - `internal/harness/registry.go`
  - `internal/harness/tools_contract_test.go`

### 8. Ticket #382: Add async subagent lifecycle surfaces: start, get, wait, cancel

- Goal:
  - Support fan-out/fan-in orchestration instead of sync-only `run_agent`.
- Dependencies:
  - None; can build on existing subagent manager.
- TDD first:
  - Add failing tool and HTTP tests for async lifecycle operations.
- Regression coverage:
  - Pin status transitions and cancellation behavior.
- Implementation notes:
  - Reuse `internal/subagents/manager.go`.
  - Expose handles rather than blocking the parent loop by default.
- Additional context:
  - `internal/subagents/manager.go`
  - `internal/server/http_subagents.go`

### 9. Ticket #383: Converge subagent result handling on a structured completion contract

- Goal:
  - Make child results machine-readable and consistent across delegation paths.
- Dependencies:
  - Ticket 1 and Ticket 8.
- TDD first:
  - Add failing tests for structured result payloads and plain-text fallback behavior.
- Regression coverage:
  - Pin `task_complete` payload shape and parent-visible result normalization.
- Implementation notes:
  - Prefer one result schema across:
    - `spawn_agent`
    - `run_agent`
    - async subagent completion
- Additional context:
  - `internal/harness/tools/deferred/task_complete.go`
  - `internal/harness/tools/deferred/spawn_agent.go`
  - `internal/subagents/manager.go`

### 10. Ticket #384: Implement explicit parent-to-child context handoff bundles

- Goal:
  - Replace ad hoc prompt stuffing with typed, size-bounded context handoff.
- Dependencies:
  - Ticket 9 recommended.
- TDD first:
  - Add failing tests for serialization, truncation, and prompt rendering of context bundles.
- Regression coverage:
  - Pin file/snippet ordering, truncation markers, and omitted-large-artifact behavior.
- Implementation notes:
  - Keep the handoff typed and replayable.
  - Store it with the child run for observability.
- Additional context:
  - `internal/harness/runner.go`
  - `internal/harness/tools/types.go`
  - `internal/subagents/manager.go`
  - `docs/investigations/issue-235-recursive-spawning-research.md`

### 11. Ticket #385: Add artifact exchange and child-result retrieval

- Goal:
  - Let child runs return artifact references instead of bloating parent context.
- Dependencies:
  - Ticket 10.
- TDD first:
  - Add failing tests for artifact reference creation and parent retrieval.
- Regression coverage:
  - Pin handling for missing artifacts, oversized outputs, and preserved worktree paths.
- Implementation notes:
  - Start with lightweight artifact references and metadata.
  - Avoid broad binary-blob handling in the first pass.
- Additional context:
  - `internal/subagents/manager.go`
  - `internal/workspace/*`
  - `internal/store/*`

### 12. Ticket #386: Add profile recommendation / auto-routing

- Goal:
  - Help callers choose a profile instead of forcing exact-name selection.
- Dependencies:
  - Tickets 3, 5, and 7.
- TDD first:
  - Add failing tests for deterministic recommendation based on prompt/task metadata.
- Regression coverage:
  - Pin tie-breaking and fallback to `full`.
- Implementation notes:
  - Start with rules/heuristics before model-based selection.
  - Keep the choice explainable.
- Additional context:
  - `internal/profiles/*`
  - `internal/systemprompt/*`
  - `internal/harness/tools_default.go`

### 13. Ticket #387: Persist per-profile usage history and efficiency stats

- Goal:
  - Replace one-off suggestion events with queryable profile history.
- Dependencies:
  - Ticket 3 and Ticket 9.
- TDD first:
  - Add failing persistence tests for profile-run statistics.
- Regression coverage:
  - Pin rolling-average or last-N aggregation behavior.
- Implementation notes:
  - Track runs, status, steps, cost, tool usage, and timestamps by profile.
  - Do not auto-mutate profiles in this ticket.
- Additional context:
  - `internal/profiles/efficiency.go`
  - `internal/harness/runner.go`
  - `internal/store/*`

### 14. Ticket #388: Add efficiency report retrieval and suggest-only refinement surfaces

- Goal:
  - Make profile tuning inspectable before any auto-application exists.
- Dependencies:
  - Ticket 13.
- TDD first:
  - Add failing tests for report listing/getting and suggestion formatting.
- Regression coverage:
  - Pin no-op behavior for profiles without sufficient history.
- Implementation notes:
  - Expose read-only reports first.
  - Keep this ticket suggest-only.
- Additional context:
  - `internal/profiles/efficiency.go`
  - `internal/harness/runner.go`

### 15. Ticket #389: Implement the real profile efficiency review loop

- Goal:
  - Turn the current event-only suggestion concept into a real reviewer-backed workflow.
- Dependencies:
  - Tickets 9, 13, and 14.
- TDD first:
  - Add failing tests for trigger conditions and persisted review results.
- Regression coverage:
  - Pin that reviews are suggest-only and do not auto-edit profiles.
- Implementation notes:
  - Start async and non-blocking.
  - Reuse existing rollout data where possible.
- Additional context:
  - `internal/profiles/efficiency.go`
  - `internal/harness/runner.go`
  - `docs/implementation/issue-237-profile-system.md`

### 16. Ticket #390: Align tool-registry docs and code, including explicit code-intel policy

- Goal:
  - Eliminate drift between the old catalog story, the current default registry, and what profiles can actually assume.
- Dependencies:
  - None.
- TDD first:
  - Add failing contract/docs tests where possible.
- Regression coverage:
  - Pin the expected registry surface in code-level contract tests.
- Implementation notes:
  - Decide whether LSP is removed, optional, or being reintroduced later.
  - Update docs to match the chosen contract.
- Additional context:
  - `internal/harness/tools_default.go`
  - `internal/harness/tools/catalog.go`
  - `docs/design/tool-roadmap.md`
  - `docs/investigations/tool-catalog-review.md`

### 17. Ticket #391: Add CLI/TUI profile UX

- Goal:
  - Make profile-backed runs operable from first-party clients.
- Dependencies:
  - Tickets 3 and 7.
- TDD first:
  - Add failing CLI request-shape tests and TUI interaction tests.
- Regression coverage:
  - Pin request payloads and selected-profile rendering.
- Implementation notes:
  - Start with profile selection and inspection, not full CRUD.
- Additional context:
  - `cmd/harnesscli/main.go`
  - `cmd/harnesscli/tui/*`

### 18. Ticket #392: Expand the MCP server surface for profile and subagent workflows

- Goal:
  - Let MCP clients participate in profile-backed orchestration instead of only basic run control.
- Dependencies:
  - Tickets 3 and 8.
- TDD first:
  - Add failing MCP server tool-list and tool-call tests.
- Regression coverage:
  - Pin tool schemas for profile/subagent endpoints.
- Implementation notes:
  - Add profile discovery and async subagent control before mutating profile CRUD.
- Additional context:
  - `internal/mcpserver/mcpserver.go`
  - `docs/plans/mcp-server-richer-tools.md`

### 19. Ticket #393: Add a profile/subagent smoke suite and integration harness

- Goal:
  - Give the repo one supported regression path for profile-backed subagents.
- Dependencies:
  - Tickets 2, 8, 9, and 10.
- TDD first:
  - Add a failing smoke entrypoint or integration test path first.
- Regression coverage:
  - Start server, inspect profiles, create child run, observe structured completion, read persistence back.
- Implementation notes:
  - Keep one narrow golden path.
  - Use tmux-backed process management for live smoke scripts.
- Additional context:
  - `scripts/smoke-test.sh`
  - `docs/runbooks/golden-path-deployment.md`
  - `internal/server/http_*`

### 20. Ticket #394: Add docs and runbooks for authoring and operating profiles

- Goal:
  - Make profiles usable without reading code.
- Dependencies:
  - Tickets 3, 4, and 5.
- TDD first:
  - Not applicable beyond doc/link validation.
- Regression coverage:
  - Update indexes and any doc contract tests.
- Implementation notes:
  - Include:
    - how to choose a profile
    - how to author one
    - how context handoff works
    - how to inspect and debug a child run
- Additional context:
  - `README.md`
  - `docs/runbooks/*`
  - `docs/INDEX.md`

## Suggested Delivery Order

1. `#375`
2. `#376`
3. `#377`
4. `#378`
5. `#379`
6. `#381`
7. `#382`
8. `#383`
9. `#384`
10. `#385`
11. `#387`
12. `#388`
13. `#389`
14. `#386`
15. `#390`
16. `#391`
17. `#392`
18. `#393`
19. `#394`
20. `#380`

## Risks and Mitigations

- Risk:
  - Another umbrella issue forms around profile work and smaller models get blocked on ambiguity.
- Mitigation:
  - Keep each ticket narrow, test-first, and explicit about inputs/outputs.

- Risk:
  - Delegation paths continue to diverge.
- Mitigation:
  - Resolve `spawn_agent` vs `run_agent` contract first.

- Risk:
  - Context handoff becomes an unbounded prompt dump.
- Mitigation:
  - Make handoff typed, size-bounded, and regression-tested.

- Risk:
  - Efficiency work adds a speculative subsystem before the basics are solid.
- Mitigation:
  - Land profile correctness, CRUD, context, and lifecycle before reviewer automation.
