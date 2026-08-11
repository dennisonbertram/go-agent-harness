# MCP Runbook

The harness has two MCP roles:

- **Client** — the harness uses MCP servers as tool providers (the agent calls tools hosted by external MCP servers)
- **Server** — the harness exposes itself as an MCP server so other hosts (Claude Desktop, other agents) can drive it

---

## Harness as MCP Client

### Global servers (all runs)

Set `HARNESS_MCP_SERVERS` to a JSON array before starting `harnessd`. Servers are registered once at startup and available to every run.

**stdio (subprocess)**:
```bash
export HARNESS_MCP_SERVERS='[
  {"name": "filesystem", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]},
  {"name": "fetch",      "command": "uvx", "args": ["mcp-server-fetch"]}
]'
go run ./cmd/harnessd
```

**HTTP/SSE**:
```bash
export HARNESS_MCP_SERVERS='[
  {"name": "my-server", "url": "http://localhost:3001/mcp"}
]'
go run ./cmd/harnessd
```

**Schema** for each entry:

| Field     | Type     | Required | Description |
|-----------|----------|----------|-------------|
| `name`    | string   | yes      | Unique server name (used to route tool calls) |
| `command` | string   | one of   | Executable for stdio transport |
| `args`    | []string | no       | Arguments for stdio command |
| `url`     | string   | one of   | HTTP endpoint for Streamable HTTP transport |

Either `command` or `url` must be set, not both. Duplicate names are skipped (first occurrence wins, logged).

---

### Per-run servers (single run only)

Pass `mcp_servers` in the `POST /v1/runs` body. The server is started when the run begins and torn down when the run completes.

```bash
curl -X POST http://localhost:8080/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "use the sqlite tool to query /tmp/my.db",
    "mcp_servers": [
      {"name": "sqlite", "command": "uvx", "args": ["mcp-server-sqlite", "--db-path", "/tmp/my.db"]}
    ]
  }'
```

Per-run server names must not collide with globally registered server names — the request is rejected with 400 if they do.

---

### How the agent sees MCP tools

MCP tools appear alongside native harness tools in the agent's tool list. The tool name visible to the agent is `{server_name}__{tool_name}` (double underscore). For example, a `read_file` tool on the `filesystem` server appears as `filesystem__read_file`.

This naming is handled by `internal/harness/tools/mcp.go` and the `MCPRegistry` interface.

---

### Transport details

| Transport | Protocol version | Notes |
|-----------|-----------------|-------|
| stdio     | 2025-11-25 (falls back to 2024-11-05) | subprocess, communicates over stdin/stdout |
| HTTP      | 2025-11-25 (falls back to 2024-11-05) | POST to URL, optional SSE response |

The HTTP transport (`internal/mcp/http_conn.go`) validates that the URL scheme is `http` or `https`. Only these schemes are accepted.

---

## Harness as MCP Server

### HTTP MCP server package (`internal/mcpserver`)

`internal/mcpserver` provides an HTTP MCP server handler for `POST /mcp` and `GET /mcp` (SSE). The `/mcp` endpoint is mounted by `harnessd` by default alongside the main HTTP API.

`harnessd` also exposes MCP **client management** endpoints at:

- `GET /v1/mcp/servers`
- `POST /v1/mcp/servers`

**Tools exposed** (10 total):

| Tool | Description |
|------|-------------|
| `start_run` | Submit a new agent run, returns `run_id` |
| `get_run_status` | Poll status and output of a run |
| `list_runs` | List recent runs |
| `steer_run` | Inject a guidance message into an active run |
| `submit_user_input` | Respond to a run paused at `waiting_for_user` |
| `list_conversations` | List recent conversations |
| `get_conversation` | Retrieve full message history for a conversation |
| `search_conversations` | Full-text search across conversations |
| `compact_conversation` | Trigger context compaction for a conversation |
| `subscribe_run` | Register for SSE notifications on a run (`GET /mcp`) |

---

### stdio binary (for Claude Desktop / CLI MCP hosts)

Build:
```bash
go build -o bin/harness-mcp ./cmd/harness-mcp
```

The binary reads JSON-RPC from stdin and writes responses to stdout. It proxies all calls to `harnessd` via HTTP.

Configure via environment:

| Variable | Meaning |
|---|---|
| `HARNESS_ADDR` | harnessd base URL (default `http://localhost:8080`) |
| `HARNESS_API_KEY` | Bearer token, when the daemon requires auth |

`harnessd` enforces Bearer authentication unless it was started with
`HARNESS_AUTH_DISABLED=true`. Without `HARNESS_API_KEY` the stdio server can only
reach an auth-disabled daemon. The token is read from the environment only — it is
never a tool argument, so a model cannot set or read it.

**Register with Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "harness": {
      "command": "/path/to/bin/harness-mcp",
      "env": {
        "HARNESS_ADDR": "http://localhost:8080"
      }
    }
  }
}
```

The stdio binary exposes 24 tools: `start_run`, `get_run_status`, `wait_for_run`,
`continue_run`, `list_runs`, `cancel_run`, `approve_run`, `deny_run`, `steer_run`,
`list_models`, `list_providers`, `list_profiles`, `list_tools`, `list_conversations`, `get_conversation`,
`search_conversations`, `compact_conversation`, `tail_run_events`, `get_run_input`,
`submit_user_input`, `get_run_todos`, `get_run_summary`, `get_run_context`, and
`compact_run`.

**Watching a run.** An MCP tool call is request/response, so a tool cannot stream
into an in-flight call. Poll `tail_run_events` instead: it returns the events since
your cursor plus a `last_event_id` to pass back as `after_event_id` next time, and
returns promptly when the run is quiet. `get_run_todos` shows what the run has done
and plans to do next.

**Answering a run.** When `get_run_status` reports `waiting_for_user`, the run has
asked a question. Read it with `get_run_input` and answer with `submit_user_input`
so the run resumes — cancelling it throws the work away.

`start_run` forwards the full run surface — `model`, `workspace_type`,
`extra_dirs`, `allowed_tools`, `denied_tools`, `profile`, `system_prompt`,
`provider_name`, `reasoning_effort`, `max_steps`, `max_turns`, `max_cost_usd`,
`plan_mode`, `plan_file`, `agent_intent`, `task_context`, `conversation_id`. Every
field except `prompt` is optional and omitted when unset, so a prompt-only call
behaves as it always did.

Use `workspace_type: "worktree"` for anything that writes: the run is provisioned a
git worktree and the caller's checkout is untouched.

---

### One implementation, two transports

`/mcp` and the stdio `harness-mcp` binary serve the **same** tool definitions and
handlers (`internal/harnessmcp`). Previously `/mcp` was a second, independent
delegation API whose `start_run` accepted a prompt and nothing else — it could not
select a model, isolate a workspace, or restrict tools. The two had already
drifted (issue #1317).

`/mcp` runs inside `harnessd` and reaches the REST API over loopback, forwarding
the caller's own bearer token, so an authenticated daemon stays authenticated end
to end. It is mounted behind the same auth middleware as `/v1` (issue #1328).

### SSE streaming (`GET /mcp`)

Subscribe to live run events:

1. Call `subscribe_run` via `POST /mcp` with `{"run_id": "<id>"}`.
2. Open a persistent `GET /mcp` connection — server sends SSE events.

These steps work out of the box with `harnessd` — the `/mcp` endpoint is mounted by default.

Event format:
```
data: {"jsonrpc":"2.0","method":"run/event","params":{"run_id":"...","event_type":"status_changed","status":"running"}}\n\n
data: {"jsonrpc":"2.0","method":"run/completed","params":{"run_id":"...","status":"completed","cost_usd":0.004}}\n\n
```

---

## Key packages

| Package | Role |
|---------|------|
| `internal/mcp/` | MCP client: `ClientManager`, stdio+HTTP transports, env config parser |
| `internal/harness/tools/mcp.go` | Tool layer: wraps `MCPRegistry` into agent-callable tools |
| `internal/harness/scoped_mcp.go` | Per-run scoped registry with global shadowing |
| `internal/mcpserver/` | MCP HTTP server (broker, poller, SSE, 10 tools) |
| `cmd/harness-mcp/` | Thin stdio binary proxying to `harnessd` |
| `internal/harnessmcp/` | Library used by the stdio binary |
