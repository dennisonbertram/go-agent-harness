# MCP Server Research

**Date**: 2026-03-30
**Sources**: MCP specification (2025-11-25), modelcontextprotocol.io, official Go SDK, mcp-go community SDK

---

## 1. MCP Server Specification

### Architecture

MCP follows a client-server architecture with three roles:

- **Hosts**: LLM applications (e.g., Claude Desktop, IDEs) that initiate connections
- **Clients**: Connectors within the host that maintain 1:1 sessions with servers
- **Servers**: Services that provide context and capabilities (tools, resources, prompts)

The protocol uses **JSON-RPC 2.0** as its wire format. All messages MUST be UTF-8 encoded. Connections are **stateful** -- both sides maintain session state after initialization.

### Message Types

MCP defines three JSON-RPC message types:

1. **Requests**: Expect a response (have `id`, `method`, `params`)
2. **Responses**: Reply to requests (have `id`, `result` or `error`)
3. **Notifications**: Fire-and-forget (have `method`, `params`, no `id`)

### Server Capabilities

Servers can declare support for any combination of:

| Capability   | Description                                       |
|-------------|---------------------------------------------------|
| `tools`     | Executable functions the AI model can invoke       |
| `resources` | Contextual data exposed via URI-based access       |
| `prompts`   | Templated messages and workflows for users         |
| `logging`   | Server-side log emission                           |

Each capability object may include a `listChanged` boolean indicating whether the server will emit notifications when the available items change.

---

## 2. Tool Exposure

### Tool Declaration

Tools are declared via the `tools/list` method. Each tool has:

```json
{
  "name": "get_weather",
  "title": "Weather Information Provider",
  "description": "Get current weather information for a location",
  "inputSchema": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "City name or zip code"
      }
    },
    "required": ["location"]
  },
  "outputSchema": {
    "type": "object",
    "properties": { ... },
    "required": ["type"]
  },
  "annotations": { },
  "execution": {
    "taskSupport": "forbidden | optional | required"
  }
}
```

**Key fields**:
- `name` (required): Unique identifier for the tool
- `description` (required): Human-readable description (shown to the LLM)
- `inputSchema` (required): JSON Schema defining expected parameters
- `outputSchema` (optional, draft spec): JSON Schema for structured output
- `annotations` (optional): Metadata about tool behavior (e.g., read-only, destructive)
- `execution.taskSupport` (optional): `"forbidden"` (sync only, default), `"optional"` (sync or async), `"required"` (async only)

### Tool Invocation

Tools are called via `tools/call`:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {
      "location": "San Francisco"
    }
  }
}
```

Response returns a `CallToolResult` with content items (text, image, embedded resources).

### Tool List Changed Notification

If the server declared `tools.listChanged: true`, it can send `notifications/tools/list_changed` to tell clients to re-fetch the tool list.

### Pagination

`tools/list` supports cursor-based pagination via `cursor` request param and `nextCursor` response field.

---

## 3. Resource Exposure

### Resource Types

Resources provide contextual data to the LLM. Two patterns:

**Static Resources** -- fixed URI:
```json
{
  "uri": "docs://readme",
  "name": "Project README",
  "mimeType": "text/markdown"
}
```

**Resource Templates** -- parameterized URI (RFC 6570):
```json
{
  "uriTemplate": "users://{id}/profile",
  "name": "User Profile"
}
```

### Resource Operations

| Method                | Description                          |
|-----------------------|--------------------------------------|
| `resources/list`      | List available static resources      |
| `resources/templates/list` | List available resource templates |
| `resources/read`      | Read a specific resource by URI      |
| `resources/subscribe` | Subscribe to resource change updates |

### Resource Content

Resources return content items with:
- `uri`: The resource URI
- `mimeType`: Content type
- `text` or `blob`: The actual content (text or base64-encoded binary)

---

## 4. Transport Options

### stdio Transport

- Communication over standard input/output streams
- Server launched as a child process by the client
- Messages delimited by newlines
- Best for: local tools, CLI integrations, development
- Simplest to implement; most widely supported

### Streamable HTTP Transport (current standard)

- Single HTTP endpoint (e.g., `/mcp`)
- Client sends JSON-RPC requests via `POST`
- Server can respond with:
  - Single JSON response (simple request/response)
  - SSE stream (for streaming responses, server-to-client notifications)
- Supports optional session management via `Mcp-Session-Id` header
- Best for: remote servers, web deployments, production services
- Initialization: `POST /mcp` with `initialize` method; if 404/405, fall back to legacy SSE

### Legacy SSE Transport (deprecated)

- Two endpoints: `GET /sse` (event stream) + `POST /messages` (client messages)
- Server pushes events to client via SSE
- Being replaced by Streamable HTTP but still supported as fallback
- Best for: backward compatibility only

### Transport Selection Guidance

| Use Case                      | Recommended Transport |
|-------------------------------|----------------------|
| Local CLI tool                | stdio                |
| IDE extension (local)         | stdio                |
| Remote API service            | Streamable HTTP      |
| Browser-based client          | Streamable HTTP      |
| Legacy client compatibility   | SSE (fallback)       |

---

## 5. Server Lifecycle

### Phase 1: Initialization

Client sends `initialize` request with:
- `protocolVersion`: Version the client supports (e.g., `"2025-11-25"`)
- `capabilities`: What the client supports (e.g., `elicitation`, `sampling`, `roots`)
- `clientInfo`: Client name and version

Server responds with:
- `protocolVersion`: Negotiated version (must match or be earlier)
- `capabilities`: What the server supports (`tools`, `resources`, `prompts`, `logging`)
- `serverInfo`: Server name and version

### Phase 2: Initialized Notification

Client sends `notifications/initialized` (no response expected). This signals the server that the client is ready to proceed.

### Phase 3: Operation

Normal request/response flow. Either side can send requests and notifications per their declared capabilities.

### Phase 4: Shutdown

- Client sends a close signal or disconnects
- For stdio: closes stdin/stdout streams or terminates the process
- For HTTP: closes the SSE connection or stops sending requests
- No explicit shutdown handshake in the current spec

### Capability Negotiation Rules

- Server MUST NOT use capabilities the client did not declare
- Client MUST NOT use capabilities the server did not declare
- Both sides should gracefully handle missing optional capabilities
- Protocol version negotiation uses the highest mutually supported version

---

## 6. Go Implementations

### Official Go SDK: `modelcontextprotocol/go-sdk`

**Repository**: https://github.com/modelcontextprotocol/go-sdk
**Maintained by**: MCP project in collaboration with Google
**License**: Apache 2.0 (new contributions), MIT (existing code)

**Packages**:
- `mcp` -- primary APIs for constructing servers and clients
- `jsonrpc` -- custom transport implementations
- `auth` -- OAuth support primitives
- `oauthex` -- OAuth protocol extensions

**Key features**:
- Typed server scaffolding with Go struct tags for JSON Schema generation
- `mcp.AddTool()` for tool registration
- `StdioTransport` for stdin/stdout
- `CommandTransport` for subprocess communication
- Custom transports via `jsonrpc` package
- Spec version compatibility from 2025-06-18 through 2025-11-25

**Server creation pattern**:
```go
import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

server := mcp.NewServer("my-server", "1.0.0", nil)
mcp.AddTool(server, "greet", mcp.ToolHandlerFunc(greetHandler))
// Serve over stdio
transport := mcp.NewStdioTransport()
server.Run(transport)
```

**Strengths**: Spec-compliant, official backing, struct-tag JSON Schema generation
**Gaps**: Fewer built-in HTTP transport options compared to mcp-go

---

### Community SDK: `mark3labs/mcp-go`

**Repository**: https://github.com/mark3labs/mcp-go
**Website**: https://mcp-go.dev
**License**: MIT
**Stars**: Most popular community Go MCP library

**Key features**:
- Four built-in transports: stdio, Streamable HTTP, SSE, in-process
- Fluent tool definition API with `mcp.NewTool()`, `mcp.WithDescription()`, `mcp.WithString()`, etc.
- Resource and resource template support
- Prompt support
- Session management for per-session tools
- Request hooks for intercepting messages
- Tool handler middleware for cross-cutting concerns
- Task support for async long-running operations

**Server creation pattern**:
```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

s := server.NewMCPServer("My Server", "1.0.0")

// Add a tool
tool := mcp.NewTool("calculate",
    mcp.WithDescription("Perform arithmetic"),
    mcp.WithNumber("x", mcp.Required()),
    mcp.WithNumber("y", mcp.Required()),
)
s.AddTool(tool, calculateHandler)

// Add a resource
resource := mcp.NewResource("docs://readme", "Project README",
    mcp.WithMIMEType("text/markdown"))
s.AddResource(resource, readmeHandler)

// Serve over stdio
server.ServeStdio(s)

// OR serve over Streamable HTTP
server.ServeStreamableHTTP(s, ":8080")
```

**Strengths**: More transport options (especially HTTP), simpler API, active community, faster to get started
**Gaps**: Not official, API may diverge from spec changes

---

### Comparison Matrix

| Feature                    | `go-sdk` (official)        | `mcp-go` (mark3labs)       |
|---------------------------|---------------------------|---------------------------|
| Maintainer                | MCP project + Google      | Community (Ed Zynda)      |
| Spec compliance           | Canonical                 | High (tracks spec)        |
| stdio transport           | Yes                       | Yes                       |
| Streamable HTTP           | Via jsonrpc package       | Built-in                  |
| SSE transport             | Manual                    | Built-in                  |
| In-process transport      | No                        | Yes                       |
| Tool registration         | `mcp.AddTool()`           | Fluent builder API        |
| Resource support          | Yes                       | Yes                       |
| JSON Schema generation    | Struct tags               | Builder functions         |
| Middleware/hooks           | Limited                   | Yes                       |
| Session management        | Basic                     | Per-session tools         |
| Async task support        | No                        | Yes                       |
| OAuth support             | Yes (`auth` package)      | No                        |

**Recommendation**: For this harness project, `mark3labs/mcp-go` is the better fit due to built-in HTTP transport support, simpler API, and middleware/hooks. The official SDK is preferable if OAuth or strict spec compliance is critical.

---

## 7. Best Practices

### Server Design

1. **Declare minimal capabilities** -- only advertise what you actually implement
2. **Support `listChanged` notifications** -- emit them when tools/resources change dynamically
3. **Use descriptive tool names and descriptions** -- the LLM relies on these to decide when to call tools
4. **Provide JSON Schema for all tool inputs** -- enables client-side validation and LLM parameter generation
5. **Support pagination** for large tool/resource lists

### Security

1. **Validate all tool inputs** -- never trust client-provided arguments
2. **Implement access controls** -- scope tools and resources to authorized operations
3. **Rate limit tool invocations** -- prevent abuse
4. **Sanitize tool outputs** -- prevent injection of malicious content
5. **Treat tool annotations as untrusted** unless the server is trusted

### Transport

1. **Support stdio as the baseline** -- it is the most universally supported transport
2. **Use Streamable HTTP for remote deployments** -- it handles sessions, streaming, and works with load balancers
3. **Implement graceful shutdown** -- clean up resources on disconnect
4. **Set appropriate timeouts** for tool execution

### Error Handling

1. **Use standard JSON-RPC error codes** (-32700 parse error, -32600 invalid request, -32601 method not found, -32602 invalid params, -32603 internal error)
2. **Return meaningful error messages** -- clients may surface these to users
3. **Handle unknown methods gracefully** -- return method-not-found rather than crashing
4. **Implement progress reporting** for long-running operations

### Testing

1. **Use the MCP Inspector** (official debugging tool) to validate your server
2. **Test with multiple clients** -- Claude Desktop, VS Code, etc. may behave differently
3. **Test both transports** if you support stdio and HTTP
4. **Validate JSON Schema** output matches what your tools actually accept

---

## 8. Relevance to go-agent-harness

The harness already has an MCP config design doc (`docs/investigations/mcp-config-design.md`). Key integration points:

- **Exposing harness tools as MCP tools**: The 30+ tools in `internal/harness/tools/` could be exposed via an MCP server interface, making the harness accessible to any MCP-compatible client
- **MCP client for tool consumption**: The harness could connect to external MCP servers to gain additional tools (file systems, databases, APIs)
- **Transport choice**: stdio for local CLI usage, Streamable HTTP for remote/symphd deployments
- **Library choice**: `mark3labs/mcp-go` recommended for its HTTP transport support and middleware hooks, which align with the harness's existing middleware patterns

---

## Sources

- [MCP Specification (2025-11-25)](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP GitHub Organization](https://github.com/modelcontextprotocol)
- [Official Go SDK (`modelcontextprotocol/go-sdk`)](https://github.com/modelcontextprotocol/go-sdk)
- [mcp-go (`mark3labs/mcp-go`)](https://github.com/mark3labs/mcp-go)
- [mcp-go Documentation](https://mcp-go.dev/getting-started/)
- [Go SDK on pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)
- [mcp-go on pkg.go.dev](https://pkg.go.dev/github.com/mark3labs/mcp-go/mcp)
- [Building MCP Server in Go (blog)](https://navendu.me/posts/mcp-server-go/)
- [MCP Server Go Guide (fast.io)](https://fast.io/resources/mcp-server-golang/)
- [2026 MCP Roadmap](http://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
