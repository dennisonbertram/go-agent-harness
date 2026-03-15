# MCP Server: stdio Transport Binary (cmd/harness-mcp/)

## Status: PLANNED
## Related Issues: TBD
## Priority: High

---

## Problem

The current MCP server (`internal/mcpserver/mcpserver.go`) only supports HTTP POST to `/mcp`. Claude Desktop and the majority of MCP-compatible clients require stdio transport: a subprocess that communicates via JSON-RPC 2.0 over stdin/stdout. Without a stdio binary, the harness cannot be registered as a tool provider in Claude Desktop or any CLI-based MCP host.

---

## Solution

Add a new binary at `cmd/harness-mcp/main.go`. This binary:

1. Reads newline-delimited JSON-RPC 2.0 messages from stdin.
2. Writes newline-delimited JSON-RPC 2.0 responses to stdout.
3. Proxies all tool calls to a running `harnessd` instance via its REST API.
4. Configures the `harnessd` base URL via `HARNESS_ADDR` env var (default: `http://localhost:8080`).

The binary is a thin stdio-to-HTTP adapter. It does not embed any run logic. All execution remains in `harnessd`.

---

## Protocol

MCP protocol version: `2025-11-25`

Capability negotiation follows the MCP spec:
- Client sends `initialize` request with `protocolVersion` and `capabilities`.
- Server responds with `protocolVersion: "2025-11-25"`, `serverInfo`, and `capabilities`.
- Client sends `initialized` notification (no response expected).
- Client may then send `tools/list` and `tools/call` requests.

---

## Architecture

```
Claude Desktop
     |
  (subprocess)
     |
cmd/harness-mcp/main.go       ← new binary
  - StdioTransport             ← reads stdin, writes stdout
  - Dispatcher                 ← routes JSON-RPC method to handler
  - ToolRegistry               ← 5 tool handlers
  - HarnessClient              ← thin HTTP client wrapping harnessd REST API
     |
  HTTP
     |
harnessd (:8080)               ← existing server, unchanged
```

### Package layout

```
cmd/harness-mcp/
  main.go              ← entry point: wires StdioTransport + Dispatcher
  transport.go         ← StdioTransport: read/write loop, concurrency
  dispatcher.go        ← routes initialize / tools/list / tools/call / notifications
  tools.go             ← 5 tool handler functions
  harness_client.go    ← HTTP client for harnessd REST API
  schema.go            ← JSON-RPC and MCP type definitions (no external SDK)
  transport_test.go
  dispatcher_test.go
  tools_test.go
  harness_client_test.go
  integration_test.go
```

No external MCP SDK. All types defined locally in `schema.go`.

---

## Type Definitions

```go
// schema.go

// JSON-RPC 2.0 envelope types
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      *json.RawMessage `json:"id,omitempty"` // nil for notifications
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// MCP protocol types
type InitializeParams struct {
    ProtocolVersion string             `json:"protocolVersion"`
    Capabilities    ClientCapabilities `json:"capabilities"`
    ClientInfo      struct {
        Name    string `json:"name"`
        Version string `json:"version"`
    } `json:"clientInfo"`
}

type InitializeResult struct {
    ProtocolVersion string             `json:"protocolVersion"`
    Capabilities    ServerCapabilities `json:"capabilities"`
    ServerInfo      struct {
        Name    string `json:"name"`
        Version string `json:"version"`
    } `json:"serverInfo"`
}

type ClientCapabilities struct{}

type ServerCapabilities struct {
    Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}

type Tool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
    Type       string              `json:"type"`
    Properties map[string]Property `json:"properties"`
    Required   []string            `json:"required,omitempty"`
}

type Property struct {
    Type        string `json:"type"`
    Description string `json:"description,omitempty"`
}

type ToolCallParams struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments,omitempty"`
}

type ToolResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
    Type string `json:"type"` // always "text" for these tools
    Text string `json:"text"`
}
```

---

## Tools Exposed

All tool handlers implement this internal interface:

```go
type ToolHandler func(ctx context.Context, args json.RawMessage) (ToolResult, error)
```

### 1. start_run

**Description**: Start a new agent run with the given prompt.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "prompt":          { "type": "string", "description": "The prompt to run" },
    "model":           { "type": "string", "description": "Model override (e.g. gpt-4.1-mini)" },
    "conversation_id": { "type": "string", "description": "Conversation to attach run to" },
    "max_steps":       { "type": "integer", "description": "Maximum steps before stopping" },
    "max_cost_usd":    { "type": "number",  "description": "Cost ceiling in USD" }
  },
  "required": ["prompt"]
}
```

**Behavior**: POST `/v1/runs` with `{prompt, model, conversation_id, max_steps, max_cost_usd}` (omit null fields). Return `{run_id: "<id>"}` as text.

**Go signature**:
```go
func handleStartRun(client *HarnessClient) ToolHandler
```

---

### 2. get_run_status

**Description**: Get the current status and output of a run.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id": { "type": "string", "description": "Run ID returned by start_run" }
  },
  "required": ["run_id"]
}
```

**Behavior**: GET `/v1/runs/{run_id}`. Return JSON-encoded `{status, messages, cost_usd, error}`.

Status values: `running`, `completed`, `failed`, `waiting_for_user`.

---

### 3. wait_for_run

**Description**: Wait for a run to reach a terminal state and return the final status. Polls internally; caller does not need to loop.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id":           { "type": "string",  "description": "Run ID to wait for" },
    "timeout_seconds":  { "type": "integer", "description": "Max seconds to wait (default: 300)" }
  },
  "required": ["run_id"]
}
```

**Behavior**: Poll GET `/v1/runs/{run_id}` every 2 seconds until status is `completed`, `failed`, or `waiting_for_user`. If `timeout_seconds` elapses before terminal state, return `isError: true` with message `"timed out waiting for run"`.

Terminal states: `completed`, `failed`, `waiting_for_user`.

**Go signature**:
```go
func handleWaitForRun(client *HarnessClient, clock Clock) ToolHandler

// Clock interface allows test injection
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
}
```

Default polling interval: 2 seconds. Not configurable via tool args (keep implementation simple).

---

### 4. continue_run

**Description**: Continue a conversation by starting a new run that follows from a previous run's conversation.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id":  { "type": "string", "description": "Run ID of the run to continue" },
    "message": { "type": "string", "description": "Follow-up message" }
  },
  "required": ["run_id", "message"]
}
```

**Behavior**: GET `/v1/runs/{run_id}` to retrieve `conversation_id`. POST `/v1/runs` with `{prompt: message, conversation_id}`. Return `{run_id: "<new_id>"}`.

---

### 5. list_runs

**Description**: List recent runs, optionally filtered by conversation.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "conversation_id": { "type": "string",  "description": "Filter by conversation ID" },
    "limit":           { "type": "integer", "description": "Max results (default: 20)" }
  }
}
```

**Behavior**: GET `/v1/runs` with query params `conversation_id` and `limit` if provided. Return JSON array `[{run_id, status, cost_usd}]`.

---

## StdioTransport

```go
type StdioTransport struct {
    in        io.Reader
    out       io.Writer
    mu        sync.Mutex // guards writes to out
    dispatcher *Dispatcher
}

func NewStdioTransport(in io.Reader, out io.Writer, d *Dispatcher) *StdioTransport

// Run reads JSON-RPC messages from in until EOF or context cancellation.
// Each message is dispatched in its own goroutine.
func (t *StdioTransport) Run(ctx context.Context) error

// writeResponse serializes resp as JSON + newline, serialized by mu.
func (t *StdioTransport) writeResponse(resp Response) error
```

Message framing: one JSON object per line (newline-delimited). Each incoming message spawns a goroutine that calls `dispatcher.Dispatch` and writes the response (if the message has an ID). Notifications (no `id` field, or `id` is JSON null) are dispatched but no response is written.

---

## Dispatcher

```go
type Dispatcher struct {
    tools map[string]ToolHandler
}

func NewDispatcher(client *HarnessClient, clock Clock) *Dispatcher

// Dispatch routes a parsed Request to the correct handler.
// Returns (Response, shouldRespond bool).
// shouldRespond is false for notifications.
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Response, bool)
```

Routing table:
- `initialize` → built-in handler, returns InitializeResult
- `initialized` → notification, no response
- `tools/list` → returns tool schemas
- `tools/call` → dispatches to ToolHandler by name
- `$/cancelRequest` → notification, no response (cancel via context if implemented)
- anything else → JSON-RPC error -32601 (Method not found)

---

## HarnessClient

```go
type HarnessClient struct {
    baseURL    string
    httpClient *http.Client
}

func NewHarnessClient(baseURL string) *HarnessClient

func (c *HarnessClient) StartRun(ctx context.Context, req StartRunRequest) (StartRunResponse, error)
func (c *HarnessClient) GetRun(ctx context.Context, runID string) (RunStatus, error)
func (c *HarnessClient) ListRuns(ctx context.Context, params ListRunsParams) ([]RunSummary, error)

type StartRunRequest struct {
    Prompt         string  `json:"prompt"`
    Model          string  `json:"model,omitempty"`
    ConversationID string  `json:"conversation_id,omitempty"`
    MaxSteps       int     `json:"max_steps,omitempty"`
    MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`
}

type StartRunResponse struct {
    RunID string `json:"run_id"`
}

type RunStatus struct {
    RunID          string    `json:"run_id"`
    Status         string    `json:"status"`
    ConversationID string    `json:"conversation_id"`
    Messages       []Message `json:"messages"`
    CostUSD        float64   `json:"cost_usd"`
    Error          string    `json:"error,omitempty"`
}

type RunSummary struct {
    RunID   string  `json:"run_id"`
    Status  string  `json:"status"`
    CostUSD float64 `json:"cost_usd"`
}

type ListRunsParams struct {
    ConversationID string
    Limit          int
}
```

---

## main.go

```go
func main() {
    addr := os.Getenv("HARNESS_ADDR")
    if addr == "" {
        addr = "http://localhost:8080"
    }

    client := harnessmcp.NewHarnessClient(addr)
    dispatcher := harnessmcp.NewDispatcher(client, harnessmcp.RealClock{})
    transport := harnessmcp.NewStdioTransport(os.Stdin, os.Stdout, dispatcher)

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer cancel()

    if err := transport.Run(ctx); err != nil && !errors.Is(err, io.EOF) {
        fmt.Fprintf(os.Stderr, "harness-mcp: %v\n", err)
        os.Exit(1)
    }
}
```

Harnessd availability is NOT checked on startup. Errors communicating with harnessd during a tool call are returned as `isError: true` in the tool result, not as process exit. This matches MCP client expectations: the subprocess should stay alive even if the backend is temporarily unavailable.

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Malformed JSON from stdin | Write JSON-RPC error response (code -32700, "Parse error"); continue |
| Request missing `method` | Write JSON-RPC error response (code -32600, "Invalid Request") |
| Unknown method | Write JSON-RPC error response (code -32601, "Method not found") |
| Unknown tool name in tools/call | Return ToolResult with isError:true, message "unknown tool: <name>" |
| harnessd returns non-2xx | Return ToolResult with isError:true, message includes status code and body |
| harnessd unreachable (connection refused) | Return ToolResult with isError:true, message includes error string |
| wait_for_run timeout | Return ToolResult with isError:true, message "timed out waiting for run <id>" |
| Context cancelled mid-poll (wait_for_run) | Return ToolResult with isError:true, message "cancelled" |

---

## Concurrency Model

The StdioTransport read loop is single-threaded (one goroutine reads stdin sequentially). Each parsed message is dispatched in a new goroutine. Responses are written under a mutex so they don't interleave on stdout. Request IDs are preserved per goroutine, so out-of-order responses are valid (MCP clients must handle this per spec).

---

## Test Requirements

### Unit Tests (`cmd/harness-mcp/*_test.go`)

All HTTP calls mocked via `httptest.NewServer`.

**T1**: `initialize` request returns correct `protocolVersion: "2025-11-25"`, `serverInfo.name`, and `capabilities.tools`.

**T2**: `tools/list` returns exactly 5 tools: `start_run`, `get_run_status`, `wait_for_run`, `continue_run`, `list_runs`. Each has non-empty `description` and a valid `inputSchema`.

**T3**: `tools/call` with `start_run` POSTs to `harnessd /v1/runs` with correct JSON body. Response contains `run_id`.

**T4**: `tools/call` with `get_run_status` GETs `harnessd /v1/runs/{id}`. Response includes status field.

**T5**: `tools/call` with `wait_for_run` polls harnessd. Mock returns `running` twice, then `completed`. Verifies 3 HTTP calls total. Returns final status.

**T6**: `wait_for_run` with `timeout_seconds: 1` and mock always returning `running`. Returns `isError: true` with timeout message before real time passes (use injected Clock).

**T7**: `tools/call` with unknown tool name returns response with `isError: true`.

**T8**: Sending malformed JSON (not a JSON object) produces a JSON-RPC parse error response on stdout.

**T9**: A notification message (no `id` field) does not produce any output on stdout. Verified by checking that stdout remains empty after dispatch.

**T10**: `continue_run` fetches the run to get `conversation_id`, then starts a new run with that conversation ID. Verifies two HTTP calls in order.

### Integration Test (`cmd/harness-mcp/integration_test.go`)

**T11**: Full flow via `httptest.Server` mock for harnessd. Pipe stdin/stdout into `StdioTransport.Run`. Send: `initialize` → `initialized` → `tools/list` → `tools/call start_run`. Assert all responses are valid JSON-RPC with correct structure.

### Race Test

**T12**: 10 concurrent `tools/call` requests with different IDs written to stdin simultaneously (via goroutines writing to a pipe). Run with `-race`. Assert all 10 responses are written and each response ID matches its request ID.

---

## Regression Tests

**R1**: Existing HTTP MCP server (`internal/mcpserver/mcpserver.go`) tests pass unchanged. The new binary does not modify `internal/mcpserver`.

**R2**: Existing REST API tests (`internal/server/http_test.go`) pass unchanged.

**R3**: `go test ./...` with `-race` passes with no data races in the new package.

---

## Build

Add to repository Makefile (or build script):

```makefile
build-mcp:
	go build -o bin/harness-mcp ./cmd/harness-mcp
```

The binary is standalone. It requires only `HARNESS_ADDR` to be set (or defaults to `http://localhost:8080`).

### Claude Desktop Registration

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

---

## Out of Scope

- OAuth / authentication for harnessd (separate issue)
- stdio transport for the existing `internal/mcpserver` package (that stays HTTP-only)
- Streaming tool results via stdio (separate spec: mcp-server-sse-streaming.md)
- Packaging / distribution (separate issue)
