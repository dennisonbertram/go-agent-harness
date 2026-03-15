# Issue #145 Grooming: Add user-facing HTTP endpoints for MCP server management

## Summary
Expose MCP server management (connect, list, inspect resources/tools) via HTTP endpoints.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/mcp/*` endpoints in `internal/server/http.go`. `MCPRegistry` and `MCPConnector` interfaces exist.

## Clarity Assessment
Excellent — 5 endpoints with schemas and error codes (400, 404, 409, 501, 502).

## Acceptance Criteria
- connect, list, resources, tools endpoints
- 502 for unreachable MCP servers
- 409 for duplicate connection attempts
- ~18 test cases

## Scope
Atomic.

## Blockers
None.

## Effort
**Medium** (3-4 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement.
