# Grooming: Issue #392 — feat(mcp): add profile and subagent control to the harness MCP server

## Already Addressed?

No — The MCP server currently has no profile or subagent tools.

Current MCP server state (`internal/mcpserver/mcpserver.go`):
- 10 tools implemented: `start_run`, `get_run_status`, `list_runs`, `steer_run`, `submit_user_input`, `list_conversations`, `get_conversation`, `search_conversations`, `compact_conversation`, `subscribe_run`
- No profile-related tools: no `list_profiles`, no `get_profile`, no profile-selection on `start_run`
- No subagent lifecycle tools: no `start_subagent`, no `get_subagent`, no `list_subagents`, no `cancel_subagent`
- The `start_run` tool takes only `prompt` — no `profile` parameter

Notably, `docs/plans/mcp-server-richer-tools.md` was the planning document for the conversation/run tools that have already been implemented (those 6 tools are now live in mcpserver.go). The plan document itself is now stale — it describes a state that has been surpassed. The MCP server is at 10 tools, but the plan document says "3 tools" as the baseline.

## Clarity

Unclear in some areas — The issue mentions two goals:
1. Profile discovery tools
2. Async subagent lifecycle control

But it does not specify:
- Which profile operations: list only? get by name? both?
- Which subagent lifecycle operations: start, get, list, cancel? all four?
- Whether `start_run` gets a `profile` parameter or there is a separate `start_subagent` tool
- Whether async subagent control maps to the existing `internal/subagents/manager.go` CRUD or a new surface
- What the tool schemas look like
- Whether SSE-based subscription (`subscribe_run`) extends to subagents

The `docs/plans/mcp-server-richer-tools.md` is no longer the right reference (it covers tools already implemented). A new design doc may be needed.

## Acceptance Criteria

Missing — No acceptance criteria in the issue body. Suggested criteria:
1. `list_profiles` MCP tool: returns array of profile names and descriptions from the server
2. `get_profile` MCP tool: returns profile schema, runner config, and tool allowlist for a named profile
3. `start_run` or new `start_profiled_run` tool: accepts `profile` parameter alongside `prompt`
4. Async subagent tools (at minimum): `start_subagent`, `get_subagent`, `list_subagents` (cancel is optional for first pass)
5. All tools listed in `tools/list` response
6. Tests for each new tool (schema validation + handler behavior)

## Scope

Too broad as written — "profile discovery" and "async subagent lifecycle control" are two independent feature areas. Each could be a separate issue. Combined, this is a large ticket:
- Profile discovery = 2-3 new tools (list/get/start with profile param)
- Subagent lifecycle = 3-4 new tools (start/get/list/cancel subagent)
- Total: 5-7 new tools + tests

The plan document suggests "profile discovery and async subagent control before mutating profile CRUD" which is a reasonable prioritization, but the scope is still wide for a single ticket.

## Blockers

Blocked on #377 — The MCP `list_profiles` and `get_profile` tools require the server-side HTTP profile discovery endpoints to exist first. The MCP server wraps the harness HTTP API; it cannot expose profiles that are not surfaced via HTTP.

Blocked on #382 — The async subagent lifecycle MCP tools require the server-side async subagent endpoints (`internal/server/http_subagents.go`) to expose stable start/get/list/cancel operations. The current subagent manager (`internal/subagents/manager.go`) has `Create`, `Get`, `List`, `Delete` — these map well to MCP tools, but the HTTP surface must be confirmed stable first.

## Recommended Labels

needs-clarification, blocked, large

## Effort

Large — Estimated 5-8 days. Requires:
- Reading the profile HTTP endpoint specs (from #377) and implementing MCP wrappers
- Reading the subagent HTTP endpoint specs (from #382) and implementing MCP wrappers
- Schema design for 5-7 new tools
- Tests for all new tools
- Updating `docs/plans/mcp-server-richer-tools.md` (currently stale, describes the already-implemented tools as "planned")

## Recommendation

needs-clarification — The issue should be split or clarified:
- Option A: Split into two tickets — "#392a: profile discovery MCP tools" and "#392b: async subagent lifecycle MCP tools"
- Option B: Keep as one ticket but add explicit tool schemas and acceptance criteria

Additionally, `docs/plans/mcp-server-richer-tools.md` should be updated before implementation — it currently describes tools that are already live in mcpserver.go (as if they were planned). The plan doc is the referenced source for this ticket but its baseline is wrong (says 3 tools, current state is 10 tools).

Blocked on #377 and #382.

## Notes

- Current MCP tool count: 10 (not 3 as the plan doc implies — the plan doc was the design for the conversation/run tools now implemented)
- `internal/subagents/manager.go` Manager interface: `Create`, `Get`, `List`, `Delete` — these are the right primitives for MCP subagent tools
- `internal/server/http_subagents.go` exists — checking its state would confirm what HTTP endpoints are available for MCP to proxy
- The `start_run` tool currently only takes `prompt` — adding a `profile` parameter here is lower-effort than a separate `start_profiled_run` tool
- The plan document delivery order places #392 at position 17 (after #377 and #382 are done), confirming the dependencies
