# Issue #55 Grooming: Epic: Enable agent to create new tools without recompiling

## Summary
Epic tracking the ability for agents to dynamically create and register new tools at runtime without a Go recompile.

## Already Addressed?
**LARGELY IMPLEMENTED (Tier 0-2 mostly done)** — Significant work already merged:
- Skills system (`internal/skills/`) — agents can write SKILL.md files
- Deferred tool activation via `find_tool` — dynamic tool discovery
- MCP integration — dynamic tool generation from MCP servers
- `create_skill` tool — agent can write skills
- `connect_mcp` tool — runtime MCP server registration
- Script tool loader — shell/Python scripts as tools
- Tool recipe system — compose tool chains
- Skill verification (issue #62)

Remaining gaps: hot-reload file watcher, automated skill verification loop.

## Clarity Assessment
Well-structured epic with 4 tiers. Acceptance criteria need updating to reflect what is already done vs. what's deferred.

## Acceptance Criteria
Need updating. Tier 0-2 is largely complete. Tier 3 (go-plugin/WASM) status is unclear.

## Scope
Epic — should track sub-issues rather than be implemented monolithically.

## Blockers
None for remaining work.

## Effort
Remaining: **Medium** (hot-reload watcher, verification loop, Tier 3 decision).

## Label Recommendations
Current: `infrastructure`, `self-building`, `epic`. Good.

## Recommendation
**needs-clarification** — Update the epic's acceptance criteria to reflect current implementation status. Close completed sub-issues. Determine if Tier 3 (go-plugin/WASM) is in scope. Consider closing this epic and tracking remaining work in specific sub-issues.
