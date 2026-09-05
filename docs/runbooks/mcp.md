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

Per-run server names must not collide with globally registered server names.
`POST /v1/runs` still accepts the request and returns `202` — the collision is
only detected once the run starts executing (`internal/harness/runner.go`
`runPreflight` calling `buildPerRunMCPRegistry` →
`internal/harness/scoped_mcp.go:286-289`), and the run then ends as
`run.failed` with the collision error, not as a rejected `POST`.

---

### How the agent sees MCP tools

MCP tools appear alongside native harness tools in the agent's tool list. The tool name visible to the agent is `mcp_{server_name}_{tool_name}` (single underscore separators; `internal/harness/tools/deferred/mcp.go:109`). For example, a `read_file` tool on the `filesystem` server appears as `mcp_filesystem_read_file`.

This naming is handled by `internal/harness/tools/deferred/mcp.go` (`DynamicMCPTools`) and the `MCPRegistry` interface (`internal/harness/tools/types.go:208`).

---

### Transport details

| Transport | Protocol version | Notes |
|-----------|-----------------|-------|
| stdio     | 2025-11-25 (falls back to 2024-11-05) | subprocess, communicates over stdin/stdout |
| HTTP      | 2025-11-25 (falls back to 2024-11-05) | POST to URL, optional SSE response |

The HTTP transport (`internal/mcp/http_conn.go`) validates that the URL scheme is `http` or `https`. Only these schemes are accepted.

---

## Harness as MCP Server

There are two distinct ways the harness exposes itself as an MCP server, plus
client-management endpoints for the client role above:

| Surface | Transport | Tool set | Who talks to it |
|---|---|---|---|
| `/mcp` (mounted inside `harnessd`) | HTTP, POST-only JSON-RPC | 25 run-delegation tools (`start_run`, `get_run_status`, ...) | The `cmd/harness-mcp` stdio binary, or any MCP host that can reach `harnessd` over HTTP |
| `harnessd --mcp` | stdio JSON-RPC | The full harness tool catalog (`bash`, `read`, `write`, etc.) | An editor/MCP host that wants direct, unrestricted tool access without going through the run API |
| `GET /v1/mcp/servers`, `POST /v1/mcp/servers` | REST | N/A — client-management, not an MCP server itself | Operators inspecting/connecting the harness's own MCP *client* registry |

### `/mcp` — run-delegation HTTP server (`internal/harnessmcp`)

`/mcp` is served by `harnessmcp.NewHTTPHandler` (`cmd/harnessd/runtime_container.go:348-352`),
mounted by default alongside the main HTTP API and behind the same auth
middleware as `/v1` (issue #1328). It is **POST-only**: `internal/harnessmcp/httptransport.go:29-31`
returns `405 Method Not Allowed` (with an `Allow: POST` header) for `GET` or
any other method, so there is no `GET /mcp` SSE stream.

It exposes the same 25 tools as the stdio `harness-mcp` binary described
below — see that section for the full tool list. `harnessd` also exposes MCP
**client management** endpoints (for the client role, not this server role) at:

- `GET /v1/mcp/servers` — list connected MCP client servers
- `POST /v1/mcp/servers` — connect a new MCP client server

Both require auth scopes (`runs:read` for GET, `admin` for POST); any other
method on `/v1/mcp/servers` returns 405.

`internal/mcpserver/` (a different, unmounted package) previously served this
role and is now only used for `harnessd --mcp` stdio mode below — it has no
external callers for HTTP. Do not confuse the two: `internal/harnessmcp` is
what `/mcp` runs today.

### `harnessd --mcp` — stdio tool-catalog server (`internal/mcpserver`)

```bash
harnessd --mcp --mcp-workspace /path/to/project
```

This starts `harnessd` in **stdio MCP mode instead of HTTP server mode**
(`cmd/harnessd/main.go:259-265,284-289`). It builds the same tool catalog the
HTTP runner uses (`harness.NewDefaultRegistryWithOptions`,
`cmd/harnessd/runtime_container.go:67-74`) and serves it over
`internal/mcpserver.NewStdioServer` on stdin/stdout — there is no run
lifecycle here, no `start_run`/`get_run_status`; the agent's own tools (bash,
read, write, edit, grep, etc.) are exposed directly to the connecting MCP
host.

`--mcp-workspace` sets the workspace root (default: current directory) — but
the tool catalog's sandbox scope is deliberately **unrestricted**
(`cmd/harnessd/runtime_container.go:57-66`): bash and write are not
workspace-confined. Anything that can speak to this stdio server has the
process's own filesystem and network authority, the same way an editor
extension would. This mode shuts down on `SIGINT`/`SIGTERM` but deliberately
does not register `SIGHUP`, so a terminal hangup cannot kill it.

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

The stdio binary exposes 25 tools: `start_run`, `get_run_status`, `wait_for_run`,
`continue_run`, `list_runs`, `cancel_run`, `approve_run`, `deny_run`, `steer_run`,
`list_models`, `list_providers`, `list_profiles`, `list_tools`, `list_skills`, `list_conversations`, `get_conversation`,
`search_conversations`, `compact_conversation`, `tail_run_events`, `get_run_input`,
`submit_user_input`, `get_run_todos`, `get_run_summary`, `get_run_context`, and
`compact_run`. `/mcp` (above) serves this same set.

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

`harnessd --mcp` stdio mode (above) is unrelated to this shared implementation —
it is a third surface that skips the run API entirely.

---

## Key packages

| Package | Role |
|---------|------|
| `internal/mcp/` | MCP client: `ClientManager`, stdio+HTTP transports, env config parser |
| `internal/harness/tools/deferred/mcp.go` | Tool layer: wraps `MCPRegistry` into agent-callable tools, `mcp_{server}_{tool}` naming |
| `internal/harness/scoped_mcp.go` | Per-run scoped registry with global shadowing |
| `internal/harnessmcp/` | Run-delegation MCP library (25 tools) shared by `/mcp` and `cmd/harness-mcp` |
| `internal/mcpserver/` | Stdio-only tool-catalog MCP server, used by `harnessd --mcp` |
| `cmd/harness-mcp/` | Thin stdio binary proxying to `harnessd` over HTTP, using `internal/harnessmcp` |
