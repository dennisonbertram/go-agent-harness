# MCP Codebase Exploration - Go Agent Harness

**Date**: 2026-03-14  
**Focus**: Tool architecture, runner flow, HTTP endpoints, and existing MCP integration

---

## 1. Tool Structure & Interfaces

### Core Tool Types (internal/harness/tools/types.go)

The tool system uses a simple but powerful interface-based design:

```go
type Tool struct {
    Definition Definition
    Handler    Handler
}

type Definition struct {
    Name         string         // tool name
    Description  string         // loaded via //go:embed from descriptions/*.md
    Parameters   map[string]any // JSON schema
    Action       Action         // read, write, list, execute, fetch, download
    Mutating     bool           // whether tool modifies state
    ParallelSafe bool           // concurrent execution safe
    Tags         []string       // discovery tags
    Tier         ToolTier       // "core" or "deferred"
}

type Handler func(ctx context.Context, args json.RawMessage) (string, error)
```

**Key Design Patterns:**
- Descriptions are embedded markdown files: `internal/harness/tools/descriptions/*.md`
- Handler is a simple function that takes raw JSON args and returns string result or error
- Tools are classified as `TierCore` (always visible) or `TierDeferred` (hidden until find_tool activates)
- Mutating flag indicates if tool modifies state

### MCPRegistry Interface (internal/harness/tools/types.go)

```go
type MCPRegistry interface {
    ListResources(ctx context.Context, server string) ([]MCPResource, error)
    ReadResource(ctx context.Context, server, uri string) (string, error)
    ListTools(ctx context.Context) (map[string][]MCPToolDefinition, error)
    CallTool(ctx context.Context, server, tool string, args json.RawMessage) (string, error)
}
```

This interface abstracts MCP server operations, allowing the tools layer to work with any MCPRegistry implementation.

---

## 2. Tool Execution Flow (LLM → Tool Execution)

### Runner & Registry (internal/harness/)

The `Runner` contains a `Registry` which manages all tools:

```go
type Registry struct {
    mu    sync.RWMutex
    tools map[string]registeredTool
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error)
```

**Execution Flow:**

1. **LLM Response Parsing**: Runner calls provider to get completion with tool calls
2. **Tool Call Extraction**: CompletionResult.ToolCalls contains array of ToolCall structs:
   ```go
   type ToolCall struct {
       ID        string // unique call ID from LLM
       Name      string // tool name
       Arguments string // JSON string of args (note: string not RawMessage)
   }
   ```

3. **Hook System** (Pre & Post tool execution):
   - **PreToolUseHooks**: Called before execution, can:
     - Allow/deny the call
     - Modify arguments
   - **PostToolUseHooks**: Called after execution, can:
     - Modify the result
     - Access execution duration and error

4. **Tool Execution**:
   - Registry.Execute() looks up tool handler by name
   - Handler invoked with parsed arguments
   - Result (string) returned to runner

5. **Message Construction**: Tool result added to messages as:
   ```go
   Message{
       Role:       "tool",
       ToolCallID: originalID,
       Name:       toolName,
       Content:    toolOutput,
   }
   ```

6. **Loop Back**: LLM called again with full message history + tool results

### Tool Result Format

Tools return a simple string. There's also a `ToolResult` struct for metadata wrapping:
```go
type ToolResult struct {
    Output      string
    Error       string
    Metadata    map[string]any
}
```

Wrapped via `WrapToolResult()` / unwrapped via `UnwrapToolResult()` for meta-message injection.

---

## 3. HTTP Server Endpoints (internal/server/http.go)

### Main API Endpoints

**POST /v1/runs** - Start a new run
```go
type RunRequest struct {
    Prompt           string
    Model            string
    ConversationID   string
    Permissions      *PermissionConfig
    AllowedTools     []string // per-run tool filtering
    MaxSteps         int
    MaxCostUSD       float64
    // ... other fields
}

type Run struct {
    ID              string
    Status          RunStatus // running, completed, failed, waiting_for_user
    Messages        []Message
    ToolCalls       []ToolCallSummary
    Usage           *RunUsageTotals
    CostUSD         float64
    Cost            *RunCostTotals
}
```

**GET /v1/runs/{id}** - Poll run status and messages

**POST /v1/runs/{id}/continue** - Continue a completed run

**POST /v1/runs/{id}/steer** - Send user steering message to active run

**POST /v1/runs/{id}/submit-user-input** - Answer ask_user_question prompts

**GET /v1/conversations** - List conversations

**GET /v1/conversations/{id}/messages** - Get conversation message history

**GET /v1/conversations/search?q=...** - Full-text search conversations

**POST /v1/conversations/{id}/compact** - Context auto-compaction

---

## 4. Tool Results Format & Return

### Standard Tool Result
Tool handlers return a simple string via `Handler func(...) (string, error)`.

**Common patterns:**
- Plain text for content tools (read, bash output)
- JSON object as string for structured data (parsed by LLM)
- Error message string (error return triggers error handling)

### MarshalToolResult Helper
```go
func MarshalToolResult(data map[string]any) (string, error)
```
Converts map to JSON string for clean tool output.

### Example from catalog.go (line 38)
```go
return MarshalToolResult(map[string]any{
    "mcp_name": args.Name, 
    "resources": items,
})
```

---

## 5. Existing MCP Integration

### MCP Package (internal/mcp/)

**mcp.go**: Core MCP client manager

- `ClientManager`: Manages connections to multiple MCP servers
- `ServerConfig`: Config for stdio or HTTP transport
  - Stdio: subprocess-based (JSON-RPC over pipes)
  - HTTP: HTTP endpoint (not yet fully integrated)
- `Conn` interface: Abstract connection to MCP server
- `stdioConn`: JSON-RPC 2.0 over stdin/stdout pipes

**Key methods:**
- `AddServer(cfg ServerConfig)`: Register server config (lazy connect)
- `DiscoverTools(ctx context.Context, serverName string) ([]ToolDef, error)`: List tools
- `ExecuteTool(ctx context.Context, serverName, toolName string, args json.RawMessage) (string, error)`: Call tool

**JSON-RPC Protocol:**
- MCP protocol version: `2024-11-05`
- Methods: `initialize`, `tools/list`, `tools/call`
- Concurrent requests supported via unique request IDs

### MCP Tools (internal/harness/tools/mcp.go)

Three main tool types expose MCP to the LLM:

1. **list_mcp_resources(mcp_name)** - List resources from an MCP server
2. **read_mcp_resource(mcp_name, uri)** - Read a resource
3. **dynamicMCPTools(ctx, reg)** - Auto-generated tools from server's tool list
   - Tool names: `mcp_<server>_<tool>` (sanitized)
   - Handler wraps `reg.CallTool()`

### HTTP MCP Integration (internal/server/http_mcp.go)

**GET /v1/mcp/servers** - List connected MCP servers
**POST /v1/mcp/servers** - Connect new MCP server (HTTP endpoint)

```go
type MCPConnector interface {
    Connect(ctx context.Context, serverURL, serverName string) ([]string, error)
}
```

Manages `connectedMCPServer` instances with tool lists.

### Deferred MCP Tools (internal/harness/tools/deferred/)

- `connect_mcp.go`: Tool to establish HTTP MCP connections
- `mcp.go`: Deferred tool infrastructure for MCP

Allows LLM to dynamically connect to MCP servers via tool calls.

---

## 6. Go Modules & Dependencies (go.mod)

### Relevant Dependencies

**Container/VM Support:**
- `github.com/docker/docker v28.5.2`
- `github.com/docker/go-connections`
- `github.com/hetznercloud/hcloud-go/v2 v2.36.0`

**Process Management:**
- `github.com/robfig/cron/v3 v3.0.1` - Cron scheduler

**Core Infrastructure:**
- `github.com/google/uuid` - Run/message IDs
- `modernc.org/sqlite v1.33.1` - SQLite for conversations/memory
- `gopkg.in/yaml.v3` - System prompt & config YAML

**No explicit MCP SDK dependency** - MCP is implemented natively using stdlib `encoding/json` and pipes/HTTP.

---

## 7. Tool Registration & Activation

### Static Tool Catalog (internal/harness/tools/catalog.go)

`BuildCatalog(opts BuildOptions)` builds the complete tool set:

```go
tools := []Tool{
    askUserQuestionTool(...),
    readTool(...),
    writeTool(...),
    editTool(...),
    bashTool(...),
    // ... 30+ tools
}

if opts.EnableMCP && opts.MCPRegistry != nil {
    tools = append(tools,
        listMCPResourcesTool(opts.MCPRegistry),
        readMCPResourceTool(opts.MCPRegistry),
    )
    dynamic, _ := dynamicMCPTools(context.Background(), opts.MCPRegistry)
    tools = append(tools, dynamic...)
}
```

### Activation & Visibility

- **TierCore** tools always sent to LLM in tool list
- **TierDeferred** tools hidden initially, activated by `find_tool` command
- `ActivationTracker` tracks which deferred tools have been activated per run

### Tool Permissions & Filtering

Per-run tool filtering via `RunRequest.AllowedTools[]`:
- If non-empty, only listed tools (+ `AlwaysAvailableTools`) offered
- Policy-based approval via `PreToolUseHooks`
- Sandbox scope: workspace, local, unrestricted

---

## 8. Tool Descriptions & Embedded Files

**Convention:** One `.md` file per tool in `internal/harness/tools/descriptions/`

```go
//go:embed descriptions/*.md
var descFS embed.FS

func Load(name string) string {
    // Returns tool description from file
}
```

Used in all tool definitions:
```go
Description: descriptions.Load("read")
```

---

## 9. Tool Hook System

### PreToolUseHook
- Called before execution
- Can Allow/Deny/ModifyArgs
- Returns `PreToolUseResult` or error

### PostToolUseHook
- Called after execution (even on error)
- Can ModifyResult
- Receives duration and error info

Located in `internal/harness/types.go` and implemented in `runner.go`:
- `applyPreToolUseHooks()` (line 2027)
- `applyPostToolUseHooks()` (line 2170)

---

## 10. Key Insights for MCP Enhancement

### Current State
✓ MCP client manager (`internal/mcp/mcp.go`) fully functional  
✓ Tool registration and execution flow mature  
✓ HTTP server with `/v1/mcp/servers` endpoint ready  
✓ Deferred tool system allows dynamic MCP connection  

### Integration Points
1. **MCPRegistry** injected into BuildOptions
2. **dynamicMCPTools()** auto-generates tools from MCP servers
3. **Tool naming**: `mcp_<server>_<tool>` pattern (sanitized)
4. **Hook system** can intercept and modify MCP tool calls

### Design Strengths
- Clean Handler interface: `func(ctx, args) (string, error)`
- Tier-based visibility (core vs deferred)
- Hook system for pre/post tool processing
- Built-in concurrency support via Registry mutex
- No external MCP SDK required

---

## Summary

The go-agent-harness has a well-architected tool system with native MCP support already integrated:

- **Tools** are simple handlers + metadata definitions
- **Runner** executes tools via Registry after LLM calls them
- **HTTP server** exposes REST API with `/v1/mcp/servers` endpoint
- **MCP client** handles stdio and HTTP connections natively
- **Tool filtering** per-run with hook-based approval system
- **Deferred tools** enable dynamic MCP server discovery and connection

MCP is a first-class citizen in the tool ecosystem, with static and dynamic tool exposure, resource access, and complete JSON-RPC 2.0 support.
