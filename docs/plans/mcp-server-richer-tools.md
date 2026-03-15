# MCP Server: Expanded Tool Surface

## Status: PLANNED
## Related Issues: TBD
## Priority: Medium

---

## Problem

The MCP server (`internal/mcpserver/mcpserver.go`) exposes only 3 tools:
- `start_run`
- `get_run_status`
- `list_runs`

The harnessd REST API has significantly more capability that is inaccessible to MCP clients:
- Steering an active run mid-execution (`POST /v1/runs/{id}/steer`)
- Submitting user input when a run pauses at `waiting_for_user` (`POST /v1/runs/{id}/submit-user-input`)
- Listing and searching conversations (`GET /v1/conversations`, `GET /v1/conversations/search`)
- Retrieving conversation message history (`GET /v1/conversations/{id}/messages`)
- Triggering context compaction (`POST /v1/conversations/{id}/compact`)

An MCP client today cannot steer a run, respond to a `waiting_for_user` prompt, or browse past conversations. This limits the usefulness of the MCP interface for any workflow beyond fire-and-forget run execution.

---

## Solution

Add 6 new tools to the MCP server tool registry. Each tool maps directly to an existing harnessd REST endpoint. No new harnessd endpoints are required. Tool input schemas mirror the corresponding REST request bodies, with all optional fields remaining optional.

The existing 3 tools (`start_run`, `get_run_status`, `list_runs`) are unchanged.

---

## Architecture

All changes are confined to `internal/mcpserver/`. No changes to `internal/server/` or any other package.

### File layout changes

```
internal/mcpserver/
  mcpserver.go          ← existing; register 6 new tools in tool map
  tools.go              ← existing or new; add 6 new handler functions
  harness_client.go     ← existing or new; add 6 new client methods
  tools_test.go         ← add unit tests for 6 new tools
  harness_client_test.go ← add unit tests for 6 new client methods
```

If `tools.go` and `harness_client.go` do not yet exist as separate files, extract tool handlers and HTTP client methods from `mcpserver.go` as part of this change. Do not add behavior to the existing file beyond what is needed for registration.

---

## New Tool Definitions

All tool handlers implement:
```go
type Handler func(ctx context.Context, args json.RawMessage) (string, error)
```

(As defined in the existing codebase.)

---

### 1. steer_run

**Description**: Send a steering message to an actively running run. The run must be in `running` state. Use this to redirect the agent's current task or provide additional context mid-execution.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id":  { "type": "string", "description": "ID of the run to steer" },
    "message": { "type": "string", "description": "Steering message to inject" }
  },
  "required": ["run_id", "message"]
}
```

**REST mapping**: `POST /v1/runs/{run_id}/steer` with body `{"message": "<message>"}`

**Success response** (stringified JSON): `{"accepted": true}`

**Error cases**:
- 404: run not found → `isError: true`, message `"run not found: <run_id>"`
- 400: validation error → `isError: true`, message includes REST API error body
- 409 (run not in steerable state): `isError: true`, message from REST API body

**Go signature**:
```go
func handleSteerRun(client MCPHarnessClient) Handler
```

---

### 2. submit_user_input

**Description**: Submit user input to a run that is paused waiting for user input. The run must be in `waiting_for_user` state. Use get_run_status to check if a run needs input before calling this.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id": { "type": "string", "description": "ID of the run awaiting input" },
    "input":  { "type": "string", "description": "The user input to submit" }
  },
  "required": ["run_id", "input"]
}
```

**REST mapping**: `POST /v1/runs/{run_id}/submit-user-input` with body `{"input": "<input>"}`

**Success response**: `{"accepted": true}`

**Error cases**:
- 404: run not found → `isError: true`
- 409: run not in waiting_for_user state → `isError: true`, message `"run is not waiting for user input"`
- 400: validation error → `isError: true`

**Go signature**:
```go
func handleSubmitUserInput(client MCPHarnessClient) Handler
```

---

### 3. list_conversations

**Description**: List recent conversations. Each conversation groups one or more runs under a shared context. Returns conversation IDs, creation timestamps, and message counts.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "limit":  { "type": "integer", "description": "Maximum number of results (default: 20)" },
    "offset": { "type": "integer", "description": "Pagination offset (default: 0)" }
  }
}
```

**REST mapping**: `GET /v1/conversations?limit=<limit>&offset=<offset>`

**Success response**: JSON array stringified:
```json
[
  {
    "conversation_id": "conv-abc123",
    "created_at": "2026-03-14T10:00:00Z",
    "message_count": 12
  }
]
```

If the REST API returns an empty array, return `[]`.

**Error cases**:
- Any non-2xx → `isError: true`, message includes status code

**Go signature**:
```go
func handleListConversations(client MCPHarnessClient) Handler
```

---

### 4. get_conversation

**Description**: Get the full message history for a conversation.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "conversation_id": { "type": "string", "description": "Conversation ID to retrieve" }
  },
  "required": ["conversation_id"]
}
```

**REST mapping**: `GET /v1/conversations/{conversation_id}/messages`

**Success response**: JSON object stringified:
```json
{
  "conversation_id": "conv-abc123",
  "messages": [
    { "role": "user",      "content": "Write a Go web server" },
    { "role": "assistant", "content": "Here is a simple Go web server..." }
  ]
}
```

**Error cases**:
- 404: conversation not found → `isError: true`, message `"conversation not found: <id>"`
- Other non-2xx → `isError: true`

**Go signature**:
```go
func handleGetConversation(client MCPHarnessClient) Handler
```

---

### 5. search_conversations

**Description**: Search conversations by content. Returns matching conversations with a short snippet showing where the match was found.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "Search query" }
  },
  "required": ["query"]
}
```

**REST mapping**: `GET /v1/conversations/search?q=<query>`

**Success response**: JSON array stringified:
```json
[
  {
    "conversation_id": "conv-abc123",
    "snippet": "...write a Go web server that handles..."
  }
]
```

If no results, return `[]`.

**Error cases**:
- 400: empty query → `isError: true`, message `"query must not be empty"`
- Other non-2xx → `isError: true`

**Go signature**:
```go
func handleSearchConversations(client MCPHarnessClient) Handler
```

---

### 6. compact_conversation

**Description**: Trigger context compaction for a conversation. Compaction summarizes older messages to reduce token usage in future runs within this conversation. This is an async operation; the conversation remains usable during compaction.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "conversation_id": { "type": "string", "description": "Conversation ID to compact" }
  },
  "required": ["conversation_id"]
}
```

**REST mapping**: `POST /v1/conversations/{conversation_id}/compact` with empty body `{}`

**Success response**: `{"ok": true}`

**Error cases**:
- 404: conversation not found → `isError: true`
- 409: compaction already in progress → `isError: true`, message from REST body
- Other non-2xx → `isError: true`

**Go signature**:
```go
func handleCompactConversation(client MCPHarnessClient) Handler
```

---

## MCPHarnessClient Interface

Extend the existing harnessd HTTP client interface with the new methods. Using an interface allows test mocking without changing production HTTP logic.

```go
// MCPHarnessClient is the interface for all harnessd REST calls made by MCP tool handlers.
// All existing tool handlers must be updated to use this interface if they currently use a concrete type.
type MCPHarnessClient interface {
    // Existing methods (unchanged)
    StartRun(ctx context.Context, req StartRunRequest) (StartRunResponse, error)
    GetRun(ctx context.Context, runID string) (RunStatus, error)
    ListRuns(ctx context.Context, params ListRunsParams) ([]RunSummary, error)

    // New methods
    SteerRun(ctx context.Context, runID string, message string) error
    SubmitUserInput(ctx context.Context, runID string, input string) error
    ListConversations(ctx context.Context, params ListConversationsParams) ([]ConversationSummary, error)
    GetConversation(ctx context.Context, conversationID string) (Conversation, error)
    SearchConversations(ctx context.Context, query string) ([]ConversationSearchResult, error)
    CompactConversation(ctx context.Context, conversationID string) error
}
```

### New request/response types

```go
type ListConversationsParams struct {
    Limit  int
    Offset int
}

type ConversationSummary struct {
    ConversationID string    `json:"conversation_id"`
    CreatedAt      time.Time `json:"created_at"`
    MessageCount   int       `json:"message_count"`
}

type Conversation struct {
    ConversationID string    `json:"conversation_id"`
    Messages       []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ConversationSearchResult struct {
    ConversationID string `json:"conversation_id"`
    Snippet        string `json:"snippet"`
}
```

`SteerRun`, `SubmitUserInput`, and `CompactConversation` return only `error`. The HTTP client implementation must return a typed error (`HarnessAPIError`) when the server returns non-2xx:

```go
type HarnessAPIError struct {
    StatusCode int
    Body       string
}

func (e *HarnessAPIError) Error() string {
    return fmt.Sprintf("harnessd returned %d: %s", e.StatusCode, e.Body)
}
```

Tool handlers check `errors.As(err, &apiErr)` to produce correct `isError` messages.

---

## Tool Registration

In `mcpserver.go` (or wherever the tool map is built), add the 6 new tools:

```go
tools := map[string]Tool{
    // existing
    "start_run":      {Handler: handleStartRun(client), Schema: startRunSchema},
    "get_run_status": {Handler: handleGetRunStatus(client), Schema: getRunStatusSchema},
    "list_runs":      {Handler: handleListRuns(client), Schema: listRunsSchema},
    // new
    "steer_run":             {Handler: handleSteerRun(client), Schema: steerRunSchema},
    "submit_user_input":     {Handler: handleSubmitUserInput(client), Schema: submitUserInputSchema},
    "list_conversations":    {Handler: handleListConversations(client), Schema: listConversationsSchema},
    "get_conversation":      {Handler: handleGetConversation(client), Schema: getConversationSchema},
    "search_conversations":  {Handler: handleSearchConversations(client), Schema: searchConversationsSchema},
    "compact_conversation":  {Handler: handleCompactConversation(client), Schema: compactConversationSchema},
}
```

Total tool count after this change: 9.

---

## Error Handling Reference

All tool handlers follow this pattern:

```go
func handleSteerRun(client MCPHarnessClient) Handler {
    return func(ctx context.Context, args json.RawMessage) (string, error) {
        var params struct {
            RunID   string `json:"run_id"`
            Message string `json:"message"`
        }
        if err := json.Unmarshal(args, &params); err != nil {
            return `{"error":"invalid arguments"}`, nil // isError handled by caller
        }
        if params.RunID == "" || params.Message == "" {
            return errorResult("run_id and message are required"), nil
        }

        if err := client.SteerRun(ctx, params.RunID, params.Message); err != nil {
            var apiErr *HarnessAPIError
            if errors.As(err, &apiErr) {
                if apiErr.StatusCode == 404 {
                    return errorResult("run not found: " + params.RunID), nil
                }
                return errorResult(apiErr.Body), nil
            }
            return errorResult("upstream error: " + err.Error()), nil
        }
        return `{"accepted":true}`, nil
    }
}

func errorResult(msg string) string {
    b, _ := json.Marshal(map[string]any{"error": msg})
    return string(b)
}
```

The `isError: true` flag in the MCP ToolResult is set by the `tools/call` dispatcher when the handler returns a non-nil error. For application-level errors (wrong run state, not found), return `nil` for the Go error and encode the error in the result string. The MCP caller then sees `isError: true` on errors where the dispatcher sets it, or reads `result.content[0].text` for structured errors.

Clarification: the existing `Handler` type is `func(ctx context.Context, args json.RawMessage) (string, error)`. The dispatcher wraps `isError: true` only when the Go error is non-nil. For semantic errors (404, 409), return `(errorJSON, nil)` — callers read the JSON. For infrastructure errors (network down, context cancelled), return `("", err)` — dispatcher sets `isError: true`.

---

## Test Requirements

### Unit Tests (`internal/mcpserver/tools_test.go`)

All tests use `httptest.NewServer` as a mock harnessd. The mock is configured per test to return specific status codes and bodies.

**T1**: `tools/list` response includes all 9 tools. Verify by unmarshaling the result and checking `len(tools) == 9`. Check that all 6 new tool names are present.

**T2**: `steer_run` makes `POST /v1/runs/{id}/steer` with body `{"message":"<msg>"}`. Mock returns 200. Result is `{"accepted":true}`.

**T3**: `submit_user_input` makes `POST /v1/runs/{id}/submit-user-input` with body `{"input":"<input>"}`. Mock returns 200. Result is `{"accepted":true}`.

**T4**: `list_conversations` makes `GET /v1/conversations?limit=10&offset=0`. Mock returns a JSON array. Result matches the array.

**T5**: `list_conversations` with no params uses default limit. Verify query param `limit` is present in the request URL.

**T6**: `get_conversation` makes `GET /v1/conversations/{id}/messages`. Mock returns messages array. Result includes `conversation_id` and `messages`.

**T7**: `search_conversations` makes `GET /v1/conversations/search?q=<query>`. Verify URL encoding of query string (e.g. query with spaces).

**T8**: `compact_conversation` makes `POST /v1/conversations/{id}/compact`. Mock returns 200. Result is `{"ok":true}`.

**T9**: `steer_run` with mock returning 404. Tool result text contains "not found" and the handler returns `nil` Go error (not setting `isError` via Go error path).

**T10**: `get_conversation` with mock returning 400 and body `{"error":"bad request"}`. Tool result text includes the body text.

### Integration Test (`internal/mcpserver/integration_test.go`)

**T11**: Full `tools/call` flow for each of the 6 new tools via a single mock harnessd server. Each call verified to make exactly one HTTP request to the correct path with the correct method and body.

### Race Tests

**T12**: 6 goroutines each call a different new tool concurrently (one goroutine per tool). All 6 use the same mock server. Run with `-race`. No data races.

---

## Regression Tests

**R1**: `start_run`, `get_run_status`, `list_runs` — all existing tests pass without modification. No change to their handler functions or client methods.

**R2**: `tools/list` for the original 3 tools: their schemas are byte-for-byte identical to the pre-change schemas. Verified by snapshot test: marshal the schema, compare to a golden string.

**R3**: `go test ./internal/mcpserver/... -race` passes with 0 data races.

**R4**: `go test ./... -race` passes. The new code does not introduce races in any dependent package.

---

## Out of Scope

- New harnessd REST endpoints (all new tools map to existing endpoints)
- Pagination cursors for `list_conversations` (limit/offset is sufficient for v1)
- Streaming results for `get_conversation` (message history is returned in one response)
- Authentication or per-tool authorization
- Changes to `cmd/harnesscli` or `cmd/harnessd`
