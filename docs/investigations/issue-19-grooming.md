# Issue #19 Grooming: Bidirectional MCP

## Summary
Enhance the harness to fully support MCP (Model Context Protocol) as both a client (consume tools from external MCP servers) and as a server (expose harness itself as an MCP server for IDEs/agents). Currently only basic resource reading is supported.

## Evaluation
- **Clarity**: Clear — Problem statement, design sketches, and use cases are explicit. Architecture clearly separates client vs server concerns.
- **Acceptance Criteria**: Partial — Implementation plan is outlined but acceptance tests are not explicit. Missing: "MCP tools discoverable via tool_search", "MCP server reachable from VS Code extension", etc.
- **Scope**: Too broad — This is really 2-3 issues: (1) Enhance MCP client with tool discovery, (2) Integrate MCP tools with deferred tools system, (3) Implement MCP server mode. Should split.
- **Blockers**: Blocked by #4 (deferred tools system) — MCP tools should integrate as deferred tools, but #4 is already complete.
- **Effort**: Large — Requires: MCPClientManager with lifecycle, tool discovery/execution, deferred tools integration, MCP server implementation, config system, error handling, tests.

## Recommended Labels
needs-clarification, large, blocked-by-4

## Missing Clarifications
1. What are acceptance criteria for "tool discovery works"? (e.g., must discover N tools, must execute successfully, timeout bounds?)
2. Should MCP server be on a separate port (8081 per config example) or same HTTP server with new endpoints?
3. How should MCP server authenticate inbound client connections (VS Code, other agents)?
4. What happens if an MCP server crashes mid-run? Graceful degradation or fail the run?
5. Should discovered MCP tools be cached or re-discovered each run?
6. Configuration format — is TOML + JSON hybrid shown final, or should it be all JSON/TOML?

## Notes
- Related research in `docs/research/codex-cli-architecture.md` provides Codex's bidirectional MCP patterns
- Integration point with deferred tools (#4) is well-identified but should be explicit in implementation plan
- Token overhead of MCP protocol wrapping not mentioned — may matter for cost model
- Security: MCP server needs authentication/authorization model (referenced in #15)
