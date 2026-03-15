# Swarm: MCP Support (Issues #248, #244, #246, #245, #249, #247)

**Started**: 2026-03-14
**Team**: mcp-swarm
**Scope**: internal/mcp/, internal/mcpserver/, cmd/harness-mcp/, cmd/harnessd/

---

## Issue Sequence
1. #248 — Config-driven MCP server startup (HARNESS_MCP_SERVERS)
2. #244 — Native HTTP/SSE transport in ClientManager
3. #246 — Per-run MCP server configuration and scoped registry
4. #245 — cmd/harness-mcp/ stdio binary (Claude Desktop)
5. #249 — Expanded MCP server tool surface
6. #247 — SSE streaming for run results

---

## Issue #248 — Config-Driven MCP Server Startup

**Status**: In Progress
**Teammate**: builder-248
**Files**: internal/mcp/config.go, internal/mcp/config_test.go, cmd/harnessd/main.go

**Commits**: 332e309, d5d51b9
**Fix applied**: duplicate server name deduplication (found in Pass 2)

### Ralph Loop
- Pass 1 (Adversarial): APPROVED (0C/0H/0M/2L)
- Pass 2 (Skeptical User): APPROVED after fix (0C/0H/1M/2L)
- Pass 3 (Correctness): APPROVED (0C/0H/0M/1L)
**Result**: COMPLETE ✓

---

## Issue #244 — Native HTTP/SSE Transport in ClientManager

**Status**: In Progress
**Files**: internal/mcp/mcp.go, internal/mcp/http_conn.go (new), internal/mcp/http_conn_test.go (new)

### Ralph Loop
- Pass 1 (Adversarial): pending
- Pass 2 (Skeptical User): pending
- Pass 3 (Correctness): pending

---
