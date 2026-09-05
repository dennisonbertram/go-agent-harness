---
title: "Exposing harnessd as an MCP Server"
sidebar_label: "harnessd as MCP Server"
sidebar_position: 5
---

import { Callout, Steps, Step, Tabs, TabsList, TabsTrigger, TabsContent, Card, CardHeader, CardTitle, CardContent } from '@site/src/components/ui';

The Model Context Protocol (MCP) is a JSON-RPC 2.0 based standard for connecting AI models to tools and data sources. `harnessd` supports MCP in _both_ directions: it can consume tools from external MCP servers (acting as an MCP client), and it can expose itself as an MCP server so that Claude Desktop and other MCP hosts can drive it directly.

This page covers the second direction — `harnessd` as an MCP server. There are three distinct surfaces, each suited to a different deployment pattern:

| Surface | Transport | Best for |
|---------|-----------|----------|
| **HTTP MCP server (`/mcp`)** | HTTP (JSON-RPC POST + SSE GET) | Programmatic clients, CI integrations, agents calling agents |
| **stdio MCP server (`--mcp`)** | stdio (JSON-RPC over stdin/stdout) | Running `harnessd` directly as an MCP tool inside a host process |
| **`harness-mcp` proxy** | stdio → HTTP proxy | Connecting Claude Desktop to an _already-running_ `harnessd` instance |

<Callout variant="warning">
The HTTP MCP server (`/mcp`) and the `harness-mcp` proxy binary share the **same 25-tool, REST-backed dispatcher** (`internal/harnessmcp`) — the only difference is transport (HTTP POST vs. stdio) and which `harnessd` instance they call (`/mcp` calls back into its own daemon; `harness-mcp` calls whatever `HARNESS_ADDR` names). `harnessd --mcp` (stdio mode) is the one surface with a different tool set: it exposes the full in-process harness tool catalog (core + deferred tools an agent run would use), not the run-management API.
</Callout>

---

## HTTP MCP server (`/mcp`)

When `harnessd` starts in normal HTTP mode, it mounts an MCP server on the same port as the REST API at the path `/mcp`. No extra configuration is needed — the endpoint is always available alongside `/v1/...`.

### Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST /mcp` | JSON-RPC 2.0 | Tool calls, `initialize`, `tools/list` |

<Callout variant="warning">
`GET /mcp` is **not** an SSE stream — the handler only accepts `POST` and returns `405 Method Not Allowed` for anything else (`internal/harnessmcp/httptransport.go:29-31`). There is no `subscribe_run` tool or push-notification mechanism on this surface; poll `tail_run_events` instead (see the tool table below).
</Callout>

The MCP server advertises protocol version `"2025-11-25"` and identifies itself as `name = "harness-mcp"`, `version = "1.0.0"` (`internal/harnessmcp/dispatcher.go:98-99`) — the same identity the `harness-mcp` stdio proxy advertises, because both share the same dispatcher.

<Callout variant="info">
`/mcp` is mounted via `harnessmcp.NewHTTPHandler` (`cmd/harnessd/runtime_container.go:348-352`), which calls back into the daemon's own REST API over HTTP using a self-referential base URL. It is **not** built on `internal/mcpserver.NewServer` — that constructor has no production caller today; `harnessd` only uses `mcpserver.NewStdioServer` for the `--mcp` stdio surface described below. (`internal/mcpserver` is a separate, unmounted package that advertises `name = "go-agent-harness"`, `version = "0.1.0"` — do not confuse the two if you read its source.)
</Callout>

### The 25 tools

Both `/mcp` and the `harness-mcp` proxy (below) expose the same 25 REST-backed tools (`internal/harnessmcp/tools.go`):

<Card>
<CardHeader>
<CardTitle>Tools exposed by the HTTP MCP server and the harness-mcp proxy</CardTitle>
</CardHeader>
<CardContent>

| Tool | Key arguments | Description |
|------|---------------|-------------|
| `start_run` | `prompt`, `model`, `conversation_id`, `max_steps`, `max_cost_usd`, `workspace_type`, `extra_dirs`, `allowed_tools`, `denied_tools`, `profile`, `system_prompt`, `provider_name`, `reasoning_effort`, `max_turns`, `plan_mode`, `plan_file`, `agent_intent`, `task_context` | Start a new agent run; returns `run_id` |
| `get_run_status` | `run_id` | Status, messages, cost, and any error |
| `wait_for_run` | `run_id`, `timeout_seconds` (default 300) | Polls until the run reaches a terminal state |
| `continue_run` | `run_id`, `prompt` | Continue an existing conversation with a follow-up prompt |
| `cancel_run` | `run_id` | Cancel an in-flight run |
| `approve_run` | `run_id` | Approve a run paused awaiting tool/plan approval |
| `deny_run` | `run_id` | Deny a run paused awaiting approval |
| `steer_run` | `run_id`, `prompt` | Inject guidance into an in-flight run |
| `tail_run_events` | `run_id`, `after_event_id`, `max_events` (default 100), `wait_seconds` (default 2) | Poll a run's event stream for progress |
| `get_run_input` | `run_id` | Read the pending question when status is `waiting_for_user` |
| `submit_user_input` | `run_id`, `answers` | Answer a run waiting on a question |
| `get_run_todos` | `run_id` | Read a run's todo list |
| `get_run_summary` | `run_id` | Read a run's summary |
| `get_run_context` | `run_id` | Read a run's context-window usage |
| `compact_run` | `run_id` | Compact a run's context |
| `list_runs` | `conversation_id`, `limit` (default 20) | List recent runs |
| `list_profiles` | — | List profiles usable as `start_run`'s `profile` argument |
| `list_tools` | — | List tool names for `allowed_tools` / `denied_tools` |
| `list_conversations` | — | List recent conversations |
| `get_conversation` | `conversation_id` | Full message history |
| `search_conversations` | `query` | Full-text search across conversations |
| `compact_conversation` | `conversation_id` | Compact a conversation's history |
| `list_skills` | — | List skills available to a delegated run |
| `list_models` | — | List models this daemon can route to |
| `list_providers` | — | List providers with configuration/health status |

</CardContent>
</Card>

Every tool is a thin proxy to the corresponding `harnessd` REST route (e.g. `start_run` → `POST /v1/runs`, `get_conversation` → `GET /v1/conversations/{id}`) — there is no separate conversation backend, and no tool is hardcoded to return an unavailable-feature error.

<Callout variant="info">
Neither `/mcp` nor `harness-mcp` expose MCP resources (`resources/list` / `resources/read`) — the dispatcher has no handler for them. This is a separate matter from harnessd's own **outbound** MCP client support: `list_mcp_resources` / `read_mcp_resource` (the in-run tools an agent calls to read resources from an *external* connected MCP server) are implemented (`cmd/harnessd/mcp_setup.go:68-90`, `internal/mcp/mcp.go:231`).
</Callout>

---

## stdio MCP server (`--mcp`)

Running `harnessd --mcp` starts an MCP server over stdin/stdout instead of HTTP. This mode exposes the **full harness tool catalog** — both `TierCore` and `TierDeferred` tools — as MCP tools. Each tool's description includes tier and tag metadata appended in the format `[tier:X tags:Y,Z]`.

This surface is useful when a host process (another agent, an IDE, or a script) wants to launch `harnessd` as a subprocess and interact with it directly through stdio JSON-RPC.

### Workspace resolution order

The workspace root is resolved from these sources, in priority order:

1. `--mcp-workspace` flag
2. `HARNESS_WORKSPACE` environment variable
3. Default: `"."` (the current directory)

### Build and run

```bash
# Build
go build ./cmd/harnessd

# Start in stdio MCP mode (workspace defaults to current directory)
./harnessd --mcp

# Start with an explicit workspace
./harnessd --mcp --mcp-workspace /path/to/workspace
```

In stdio mode `harnessd` does not listen on any TCP port — all communication happens over stdin/stdout. The process exits when the host closes the stdin pipe.

---

## The `harness-mcp` proxy

`harness-mcp` is a standalone binary that bridges a stdio MCP host (such as Claude Desktop) to a `harnessd` instance that is already running over HTTP. The proxy reads JSON-RPC from stdin, translates each call into a REST request against `harnessd`, and writes the JSON-RPC response back to stdout.

This is the recommended path for Claude Desktop integration: keep `harnessd` running as a persistent daemon and configure Claude Desktop to launch `harness-mcp` as a subprocess.

### Architecture

```
Claude Desktop
    │ (stdio JSON-RPC)
    ▼
harness-mcp (StdioTransport → Dispatcher → HarnessClient)
    │ (HTTP REST)
    ▼
harnessd (running at HARNESS_ADDR)
```

The proxy advertises `name = "harness-mcp"`, `version = "1.0.0"` and protocol version `"2025-11-25"` — identical to `/mcp` above, since both run the same `internal/harnessmcp` dispatcher.

### Tools

`harness-mcp` exposes the same 25 tools as `/mcp` — see [The 25 tools](#the-25-tools) above. Each tool proxies to the matching `harnessd` REST route (`GET`/`POST /v1/runs...`, `/v1/conversations/...`, `/v1/profiles`, `/v1/tools`, `/v1/skills`, `/v1/models`, `/v1/providers`), the only difference from `/mcp` being that requests go to `HARNESS_ADDR` over the network instead of looping back into the same process.

### Build the proxy

```bash
go build -o bin/harness-mcp ./cmd/harness-mcp
```

### Configuration

| Environment variable | Default | Description |
|---------------------|---------|-------------|
| `HARNESS_ADDR` | `http://localhost:8080` | Base URL of the running `harnessd` instance |

---

## Claude Desktop registration

To register `harness-mcp` with Claude Desktop, edit `~/Library/Application Support/Claude/claude_desktop_config.json` and add an entry under `mcpServers`:

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

Replace `/path/to/bin/harness-mcp` with the absolute path to the binary you built above. Claude Desktop launches `harness-mcp` as a subprocess when it starts; the proxy connects to `harnessd` at the address specified by `HARNESS_ADDR`.

<Steps>
<Step title="Build harness-mcp">
```bash
go build -o bin/harness-mcp ./cmd/harness-mcp
```
Note the absolute path to the resulting binary.
</Step>
<Step title="Start harnessd">
Start `harnessd` in a separate terminal or as a background service. The proxy expects it to be reachable at `HARNESS_ADDR` (default `http://localhost:8080`).

```bash
OPENAI_API_KEY=sk-... ./harnessd
```
</Step>
<Step title="Edit claude_desktop_config.json">
Add the `mcpServers` entry shown above. Use the absolute path from step 1.
</Step>
<Step title="Restart Claude Desktop">
Quit and reopen Claude Desktop. The "harness" MCP server will appear in the tool list. You can ask Claude to start a run, check run status, or wait for a long-running task to complete.
</Step>
</Steps>

<Callout variant="info">
You can also use `harnessd --mcp` (stdio mode) directly as the Claude Desktop command without the proxy. The difference is that `--mcp` mode exposes the full harness tool catalog (core + deferred tools), while `harness-mcp` exposes the 25 run-management tools described above. The proxy also lets you share one persistent `harnessd` daemon across multiple clients simultaneously.
</Callout>

---

## Choosing the right surface

<Tabs defaultValue="http">
<TabsList>
  <TabsTrigger value="http">HTTP MCP (`/mcp`)</TabsTrigger>
  <TabsTrigger value="stdio">stdio (`--mcp`)</TabsTrigger>
  <TabsTrigger value="proxy">harness-mcp proxy</TabsTrigger>
</TabsList>
<TabsContent value="http">

**Use the HTTP MCP endpoint when:**
- You are integrating from another service or agent over a network connection.
- You want a single `harnessd` process to serve many concurrent MCP clients.
- Polling `tail_run_events` is an acceptable substitute for push notifications — there is no SSE or subscription mechanism on this surface.

The endpoint lives at `POST /mcp` on the same port as the REST API (default `127.0.0.1:8080`). No extra build step is required.

</TabsContent>
<TabsContent value="stdio">

**Use `harnessd --mcp` when:**
- An MCP host (a parent agent or IDE) wants to launch `harnessd` as a subprocess.
- You need access to the full harness tool catalog — not just run management — over MCP.
- You are building a single-client, single-process integration.

</TabsContent>
<TabsContent value="proxy">

**Use `harness-mcp` when:**
- You want Claude Desktop (or any stdio MCP host) to drive a persistent, separately-managed `harnessd` daemon over stdio rather than HTTP.
- You want the same 25 run-management tools as `/mcp`, without the client needing to speak HTTP.
- Multiple clients or processes share one `harnessd` instance.

</TabsContent>
</Tabs>

---

## Next steps

- Understand the events that runs emit in [The Event Model](/docs/concepts/events).
- Learn how authentication works for the `harnessd` HTTP server in [Authentication](/docs/server/authentication).
