# MCP Client: Native HTTP/SSE Transport in ClientManager

## Status

Draft — not yet scheduled.

## Problem

`ServerConfig.Transport` accepts the value `"http"` and validates it in
`ClientManager.AddServer`, but `dialServer` in `internal/mcp/mcp.go` returns a
hard error for any non-stdio transport:

```go
// dialServer (current)
default:
    return nil, fmt.Errorf("mcp: http transport not yet implemented in ClientManager; use the connect_mcp tool for HTTP servers")
```

Operators who configure HTTP MCP servers in code (e.g. via `ClientManager.AddServer`)
get no tools and no error surface at startup — the error only materialises on first
use at `DiscoverTools` or `ExecuteTool` time, surfaced as a run failure.

The only working path for HTTP servers is the deferred `connect_mcp` tool, which
requires the LLM to issue a tool call mid-run before the tools are available. This:

- Forces every run that needs an HTTP MCP server to spend at least one token-call
  just to connect.
- Prevents pre-configured HTTP servers from appearing in the tool list at run start.
- Makes tenant isolation difficult because connection lifetime is tied to the LLM's
  willingness to invoke the tool.

## Solution

Implement `httpConn`, a new struct in `internal/mcp/mcp.go` that satisfies the
existing `Conn` interface, parallel to `stdioConn`. Plug it into `dialServer` so
that `ClientManager.AddServer` with `Transport: "http"` produces a working connection
on first use.

The transport to implement is **Streamable HTTP** as defined in the MCP 2025-11-25
specification:

- JSON-RPC requests are sent as HTTP POST to the server's `/mcp` endpoint (or the
  configured URL path).
- Each POST carries a single JSON-RPC request or batch in the body
  (`Content-Type: application/json`).
- The server MAY respond with `Content-Type: text/event-stream` (SSE) for
  notifications; for simple request/response the server returns
  `Content-Type: application/json`.
- Notifications arriving on an open SSE stream are delivered to registered listeners
  but do not block the response to an individual request.

The 2024-11-05 transport (HTTP+SSE with a separate GET endpoint for the SSE stream)
does NOT need to be implemented. The 2025-11-25 Streamable HTTP transport supersedes
it and is the current standard.

## Architecture

### `httpConn` struct

```go
// httpConn implements Conn over Streamable HTTP (MCP 2025-11-25).
type httpConn struct {
    name      string
    endpoint  string        // full URL, e.g. "https://host/mcp"
    client    *http.Client
    idCounter atomic.Int64
    mu        sync.Mutex
    closed    bool
}
```

`httpConn` does NOT need a read loop goroutine or a `pending` map because HTTP is
inherently request/response: each `sendRequest` call makes one POST and blocks on
the HTTP response. Concurrency is handled by making multiple independent HTTP requests
(the `http.Client` handles connection pooling internally).

### `Conn` interface compliance

`httpConn` must implement all methods of the `Conn` interface:

```go
func (c *httpConn) Initialize(ctx context.Context) error
func (c *httpConn) ListTools(ctx context.Context) ([]ToolDef, error)
func (c *httpConn) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error)
func (c *httpConn) NextID() int64
func (c *httpConn) Close() error
```

`Initialize` sends the `initialize` JSON-RPC request and handles protocol version
negotiation (see below). `ListTools` and `CallTool` delegate to the shared
`sendRequest` method. `Close` marks the connection closed and returns (no subprocess
or goroutine to tear down).

### `sendRequest` for HTTP

```go
func (c *httpConn) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error)
```

Steps:
1. Check `c.closed`; return error if true.
2. Build JSON-RPC request envelope `{"jsonrpc":"2.0","id":<id>,"method":<m>,"params":<p>}`.
3. POST to `c.endpoint` with `Content-Type: application/json` and `Accept: application/json, text/event-stream`.
4. Read response:
   - If `Content-Type` is `application/json`: unmarshal body as `jsonRPCResponse`.
   - If `Content-Type` is `text/event-stream`: read SSE events until a `data:` line
     containing the JSON-RPC response for this request ID is received; discard
     notification events (no `id` field or `id` != request id).
5. Return `result` or synthesised error from `error` field.
6. Honour `ctx.Done()` by wrapping the HTTP request with the context
   (`http.NewRequestWithContext`).

### `dialHTTP` constructor

```go
func dialHTTP(cfg ServerConfig) (Conn, error) {
    if cfg.URL == "" {
        return nil, fmt.Errorf("mcp: http transport requires a URL")
    }
    return &httpConn{
        name:     cfg.Name,
        endpoint: cfg.URL,
        client:   &http.Client{Timeout: 30 * time.Second},
    }, nil
}
```

`dialServer` updated:

```go
func dialServer(cfg ServerConfig) (Conn, error) {
    switch cfg.Transport {
    case "stdio":
        return dialStdio(cfg)
    case "http":
        return dialHTTP(cfg)
    default:
        return nil, fmt.Errorf("mcp: unsupported transport %q", cfg.Transport)
    }
}
```

### Protocol Version Negotiation

The current `stdioConn.Initialize` sends `"protocolVersion": "2024-11-05"` verbatim
and ignores the server's response.

Updated behaviour for both `stdioConn` and `httpConn`:

1. Send `initialize` with `"protocolVersion": "2025-11-25"` (preferred).
2. Parse the `result` from the server's `initialize` response.
3. If the server returns a `protocolVersion` field in the result, record it on the
   conn struct (`negotiatedVersion string`).
4. If the server returns a version the client does not recognise, log a warning but
   do not fail — proceed with the negotiated version.
5. If the server returns an error with code `-32602` (Invalid params) or `-32600`
   (Invalid Request), retry with `"protocolVersion": "2024-11-05"` once.

The `negotiatedVersion` field is informational; it does not gate feature use in this
implementation.

### Connection Pooling and Reuse

`httpConn` uses a single `*http.Client` with default transport settings. The
`http.Client` manages the underlying TCP connection pool automatically. No additional
pooling is required at the `httpConn` level.

### Error Handling

| Condition | Behaviour |
|---|---|
| HTTP non-2xx status | Return `fmt.Errorf("mcp: server %q returned HTTP %d", name, status)` |
| JSON-RPC `error` field set | Return `fmt.Errorf("mcp: server %q returned error: %s (code %d)", ...)` |
| `isError: true` in tool result | `extractToolCallResult` already handles this — reused unchanged |
| Context cancelled | `http.NewRequestWithContext` propagates cancellation |
| `Close()` called mid-request | HTTP request context is from the caller; `Close()` sets `closed=true` but does not cancel in-flight requests (they complete or time out naturally) |

### Reconnection

HTTP is stateless per-request. There is no persistent connection to maintain.
`httpConn` does not implement reconnection logic; each `sendRequest` call opens a new
HTTP request (the `http.Client` reuses keep-alive connections internally). If a
request fails due to a network error, the caller receives an error and may retry.

The `serverEntry` in `ClientManager` holds the `httpConn` for reuse across calls.
On a transport error the entry's `conn` is NOT cleared — the same `httpConn` is
reused for the next call (the client's connection pool handles TCP reconnect). If an
implementor wants to force reconnect they must call `ClientManager.Close()` and
re-register the server.

## Files Changed

- `internal/mcp/mcp.go` — add `httpConn`, `dialHTTP`, update `dialServer`, update
  `Initialize` with version negotiation.
- `internal/mcp/mcp_test.go` — new test cases (see below).

No changes to `internal/harness/` — `ClientManager` already satisfies `MCPRegistry`
and the tools layer is unchanged.

## Test Requirements

All tests live in `internal/mcp/mcp_test.go` (or a new `internal/mcp/http_test.go`).
All tests must pass with `-race`.

### Unit Tests

**`TestHTTPConn_ImplementsConn`**
Verify at compile time that `*httpConn` satisfies `Conn`:
```go
var _ Conn = (*httpConn)(nil)
```
This is a compile-time assertion, not a runtime test, but it should appear in the
test file so it fails the build if the interface drifts.

**`TestHTTPConn_Initialize_Success`**
Start an `httptest.Server` that responds to POST `/mcp` with a valid `initialize`
result (`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{}}}`).
Call `httpConn.Initialize(ctx)`. Expect no error. Verify the server received a POST
with the correct JSON body including `"method":"initialize"` and
`"protocolVersion":"2025-11-25"`.

**`TestHTTPConn_Initialize_VersionNegotiation_Downgrade`**
Server returns an `initialize` error with code `-32602` on the first request, then
succeeds with `"protocolVersion":"2024-11-05"` on the second. Expect `Initialize`
to succeed and `negotiatedVersion` to equal `"2024-11-05"`. Verify two POSTs were
made to the server.

**`TestHTTPConn_Initialize_VersionNegotiation_OlderVersionAccepted`**
Server responds to `initialize` with `"protocolVersion":"2024-11-05"` on the first
attempt (no error). Expect `Initialize` to succeed — no retry should occur.
Verify exactly one POST was made.

**`TestHTTPConn_ListTools`**
Server responds to `tools/list` with a valid tools array. Call `ListTools`. Assert
returned `[]ToolDef` matches expected names, descriptions, and input schemas.

**`TestHTTPConn_CallTool_Success`**
Server responds to `tools/call` with a valid content array `[{"type":"text","text":"ok"}]`.
Call `CallTool`. Assert result is `"ok"`.

**`TestHTTPConn_CallTool_IsError`**
Server responds with `{"isError":true,"content":[{"type":"text","text":"boom"}]}`.
Assert `CallTool` returns a non-nil error containing `"boom"`.

**`TestHTTPConn_CallTool_JSONRPCError`**
Server responds with `{"error":{"code":-32601,"message":"method not found"}}`.
Assert `CallTool` returns a non-nil error containing `"method not found"`.

**`TestHTTPConn_ContextCancellation`**
Use a slow server that delays 500ms before responding. Create a context with 10ms
deadline. Call `CallTool`. Assert error wraps `context.DeadlineExceeded`.

**`TestHTTPConn_ServerNon2xx`**
Server returns HTTP 503. Assert `sendRequest` returns a non-nil error mentioning the
status code.

**`TestHTTPConn_Close_IdempotentAndConcurrentlySafe`**
Call `Close()` twice concurrently from separate goroutines. Assert no panic and both
return nil (or at most one returns a nil error; the semantics are "idempotent").

**`TestHTTPConn_SSEResponse`**
Server responds with `Content-Type: text/event-stream` and streams one `data:` line
containing the JSON-RPC response. Assert `sendRequest` correctly extracts the result
from the SSE data line.

### Integration Tests

**`TestClientManager_AddServer_HTTP_DiscoverTools`**
Register an HTTP server via `ClientManager.AddServer(ServerConfig{Transport:"http", URL:...})`.
Use a real `httptest.Server`. Call `DiscoverTools`. Assert tools are returned with
correct names.

**`TestClientManager_AddServer_HTTP_ExecuteTool`**
Register an HTTP server. Call `ExecuteTool`. Assert the server received the correct
`tools/call` body and the result is returned correctly.

**`TestClientManager_HTTP_ProtocolVersionNegotiation_Integration`**
Server declares `"protocolVersion":"2024-11-05"` in its initialize response. Assert
`DiscoverTools` succeeds and the negotiated version is recorded.

### Race Tests

**`TestHTTPConn_ConcurrentCallTool_Race`**
Spin up an `httptest.Server`. Create one `httpConn`. Call `CallTool` from 20
goroutines concurrently. Assert all calls succeed and `-race` reports no data races.

**`TestClientManager_HTTP_ConcurrentDiscoverAndExecute_Race`**
Register two HTTP servers via `ClientManager.AddServer`. Concurrently call
`DiscoverTools` on server A and `ExecuteTool` on server B from 10 goroutines each.
Assert no data races.

### Regression Tests (Stdio Must Still Work)

**`TestDialServer_Stdio_NotBroken`**
Existing stdio tests must pass without modification. Run the full `mcp_test.go` suite.
If any existing stdio test fails, the HTTP implementation has introduced a regression.

**`TestClientManager_AddServerWithConn_StillWorks`**
The `AddServerWithConn` factory path (used by tests and by the deferred `connect_mcp`
connector) must still work. Verify with an in-process pipe-based connection.

**`TestConnectMCPDeferredTool_BackwardCompat`**
The `connect_mcp` deferred tool in `internal/harness/tools/deferred/connect_mcp.go`
uses the `MCPConnector` interface and does NOT use `ClientManager` internally — it
creates its own registry. Verify that its test suite still passes with no changes
after this refactor.

## Acceptance Criteria

1. `go test ./internal/mcp/... -race` passes with all new and existing tests green.
2. `go test ./internal/harness/tools/deferred/... -race` passes unchanged.
3. `go test ./... -race` passes.
4. Coverage on `internal/mcp` does not drop below 80%.
5. No 0%-covered functions in `internal/mcp`.
6. An HTTP server configured via `ClientManager.AddServer` with `Transport:"http"`
   has its tools discoverable at startup (verified by integration test).
7. Protocol version `2025-11-25` is sent in `initialize`; fallback to `2024-11-05`
   on negotiation failure.
