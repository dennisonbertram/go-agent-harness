# OpenAI Responses API — Research (Issue #139)

## TL;DR

The Responses API (`POST /v1/responses`) is a different wire format from Chat Completions (`POST /v1/chat/completions`). Models like `gpt-5.1-codex-mini`, `gpt-5.2-codex`, and `computer-use-preview` only work on the Responses endpoint. The harness needs a second provider code path to support them. The abstraction boundary (`CompletionRequest` / `CompletionResult`) is close enough that we can add a parallel implementation without changing the public interface.

---

## What needs to change and where

| Layer | File | Change |
|---|---|---|
| Request marshaling | `internal/provider/openai/client.go` | New `responsesRequest` struct, flat tool spec, `input` + `instructions` fields |
| Response parsing | `internal/provider/openai/client.go` | Parse `output[]` items instead of `choices[].message` |
| Streaming | `internal/provider/openai/client.go` | New SSE event names (`response.output_text.delta`, etc.) |
| Usage mapping | `internal/provider/openai/client.go` | `input_tokens`/`output_tokens` instead of `prompt_tokens`/`completion_tokens` |
| Routing | `internal/provider/openai/client.go` | Detect which endpoint to use per model |
| Catalog | `catalog/models.json` | Add `"api": "responses"` flag to affected models |

---

## Request format differences

### Chat Completions (current)

```json
POST /v1/chat/completions
{
  "model": "gpt-4.1-mini",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {"type": "object", "properties": {...}}
      }
    }
  ],
  "tool_choice": "auto",
  "stream": true,
  "stream_options": {"include_usage": true}
}
```

### Responses API (new)

```json
POST /v1/responses
{
  "model": "gpt-5.1-codex-mini",
  "input": [
    {"role": "user", "content": "Hello"}
  ],
  "instructions": "You are a helpful assistant.",
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get weather for a location",
      "parameters": {"type": "object", "properties": {...}},
      "strict": true
    }
  ],
  "stream": true
}
```

**Key request differences:**

| Field | Chat Completions | Responses API |
|---|---|---|
| Endpoint | `/v1/chat/completions` | `/v1/responses` |
| History | `messages[]` | `input[]` (or plain string) |
| System prompt | `messages[{role:"system"}]` | `instructions` top-level field |
| Tool wrapper | `{type, function: {name, desc, params}}` | `{type, name, desc, params}` (flat) |
| Strict mode | opt-in | default (`strict: true`) |
| `tool_choice` | `"auto"` | not used (auto is default) |
| `stream_options` | `{include_usage: true}` | not needed |
| State | stateless | optional: `store: true` + `previous_response_id` |

---

## Response format differences

### Chat Completions (current)

```json
{
  "choices": [{
    "message": {
      "content": "Hello there!",
      "tool_calls": [{
        "id": "call_abc",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\": \"London\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150,
    "prompt_tokens_details": {"cached_tokens": 20},
    "completion_tokens_details": {"reasoning_tokens": 10}
  }
}
```

### Responses API (new)

```json
{
  "id": "resp_abc123",
  "output": [
    {
      "type": "message",
      "content": [
        {"type": "output_text", "text": "Hello there!"}
      ]
    },
    {
      "type": "function_call",
      "id": "fc_abc",
      "call_id": "call_abc",
      "name": "get_weather",
      "arguments": "{\"location\": \"London\"}"
    }
  ],
  "usage": {
    "input_tokens": 100,
    "output_tokens": 50,
    "total_tokens": 150,
    "input_tokens_details": {"cached_tokens": 20},
    "output_tokens_details": {"reasoning_tokens": 10}
  }
}
```

**Key response differences:**

| Field | Chat Completions | Responses API |
|---|---|---|
| Content | `choices[0].message.content` | `output[].content[].text` (where type=output_text) |
| Tool calls | `choices[0].message.tool_calls[]` | `output[]` items where `type=="function_call"` |
| Tool call ID | `tool_calls[].id` | `output[].call_id` |
| Tool call args | `tool_calls[].function.arguments` | `output[].arguments` |
| Input tokens | `usage.prompt_tokens` | `usage.input_tokens` |
| Output tokens | `usage.completion_tokens` | `usage.output_tokens` |
| Cached tokens | `usage.prompt_tokens_details.cached_tokens` | `usage.input_tokens_details.cached_tokens` |
| Reasoning tokens | `usage.completion_tokens_details.reasoning_tokens` | `usage.output_tokens_details.reasoning_tokens` |

---

## Tool result submission (multi-turn)

In Chat Completions, tool results are submitted as messages:

```json
{"role": "tool", "tool_call_id": "call_abc", "content": "72°F, sunny"}
```

In Responses API, tool results are submitted as input items in the next request:

```json
{
  "type": "function_call_output",
  "call_id": "call_abc",
  "output": "72°F, sunny"
}
```

**Impact on harness:** The runner builds up `Messages []harness.Message` and replays the full history on each step. The Responses API path needs to map `harness.Message{Role: "tool"}` → `{type: "function_call_output", call_id: ..., output: ...}` input items. The `call_id` is stored in `harness.Message.ToolCallID`.

---

## Streaming differences

### Chat Completions streaming (current)

SSE stream with `data:` prefix, terminated by `data: [DONE]`:

```
data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":"}}]}}]}
data: {"usage":{"prompt_tokens":100,"completion_tokens":50}}
data: [DONE]
```

### Responses API streaming (new)

SSE stream with typed event names, terminated by `response.completed`:

```
event: response.output_text.delta
data: {"item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello"}

event: response.function_call_arguments.delta
data: {"item_id":"fc_1","output_index":1,"call_id":"call_abc","delta":"{\"location\":"}

event: response.function_call_arguments.done
data: {"item_id":"fc_1","output_index":1,"call_id":"call_abc","arguments":"{\"location\":\"London\"}"}

event: response.completed
data: {"response":{"id":"resp_abc","output":[...],"usage":{"input_tokens":100,"output_tokens":50}}}
```

**Key streaming differences:**

| Aspect | Chat Completions | Responses API |
|---|---|---|
| Event names | none (all `data:`) | typed (`event:` prefix) |
| Terminator | `data: [DONE]` | `event: response.completed` |
| Content delta | `choices[].delta.content` | `delta` field on `response.output_text.delta` event |
| Tool args delta | `choices[].delta.tool_calls[].function.arguments` | `delta` field on `response.function_call_arguments.delta` |
| Usage | separate `data:` chunk | embedded in `response.completed` payload |

---

## Routing decision

**Option A: Catalog flag** — add `"api": "responses"` to models in `catalog/models.json`. The provider reads this at request time.

**Option B: Runtime retry** — attempt Chat Completions, on 404 retry with Responses API.

**Recommendation: Option A (catalog flag)**

- No extra latency on first call
- Explicit, auditable — we know exactly which models use which endpoint
- Catalog already has per-model metadata
- Option B is fragile: the 404 body must be parsed to distinguish "wrong endpoint" from "model doesn't exist"

**Models known to require Responses API** (from live testing):
- `gpt-5.1-codex`, `gpt-5.1-codex-mini`, `gpt-5.1-codex-max`
- `gpt-5.2-codex`, `gpt-5.3-codex`
- `computer-use-preview`

---

## Current harness code — exact change points in `client.go`

### 1. Routing (line 97)

```go
// current
httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", ...)

// becomes
if usesResponsesAPI(model) {
    return c.completeWithResponsesAPI(ctx, req, model)
}
// existing chat completions path unchanged
```

### 2. New request struct

```go
type responsesRequest struct {
    Model        string          `json:"model"`
    Input        []responsesItem `json:"input"`
    Instructions string          `json:"instructions,omitempty"`
    Tools        []responsesToolSpec `json:"tools,omitempty"`
    Stream       bool            `json:"stream,omitempty"`
}

type responsesItem struct {
    Type    string `json:"type"`
    Role    string `json:"role,omitempty"`
    Content any    `json:"content,omitempty"` // string or []contentBlock
    CallID  string `json:"call_id,omitempty"`
    Output  string `json:"output,omitempty"`
}

type responsesToolSpec struct {
    Type        string         `json:"type"`
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    Parameters  map[string]any `json:"parameters,omitempty"`
    Strict      bool           `json:"strict"`
}
```

### 3. New response struct

```go
type responsesResponse struct {
    ID     string          `json:"id"`
    Output []responsesOutputItem `json:"output"`
    Usage  *responsesUsage `json:"usage,omitempty"`
}

type responsesOutputItem struct {
    Type      string                  `json:"type"`    // "message" or "function_call"
    Content   []responsesContentBlock `json:"content,omitempty"`
    ID        string                  `json:"id,omitempty"`
    CallID    string                  `json:"call_id,omitempty"`
    Name      string                  `json:"name,omitempty"`
    Arguments string                  `json:"arguments,omitempty"`
}

type responsesContentBlock struct {
    Type string `json:"type"` // "output_text"
    Text string `json:"text"`
}

type responsesUsage struct {
    InputTokens         int                      `json:"input_tokens"`
    OutputTokens        int                      `json:"output_tokens"`
    TotalTokens         int                      `json:"total_tokens"`
    InputTokensDetails  *responsesInputDetails   `json:"input_tokens_details,omitempty"`
    OutputTokensDetails *responsesOutputDetails  `json:"output_tokens_details,omitempty"`
}
```

### 4. Message mapping

`harness.Message` → `responsesItem`:
- `role: system` → extract to `instructions` field (not in `input`)
- `role: user` / `role: assistant` → `{type: "message", role, content}`
- `role: tool` → `{type: "function_call_output", call_id: msg.ToolCallID, output: msg.Content}`
- Assistant messages with tool calls → `{type: "message"}` plus preceding `{type: "function_call"}` items for each tool call

### 5. Streaming parser

Replace `processStreamBlock` (which looks for `data: [DONE]`) with a new parser that handles typed SSE events and terminates on `response.completed`.

---

## Scope estimate

| Component | Complexity | Lines |
|---|---|---|
| `responsesRequest` struct + `mapToResponsesRequest()` | Low | ~60 |
| `responsesResponse` struct + `resultFromResponsesResponse()` | Low | ~60 |
| Streaming parser for Responses events | Medium | ~100 |
| `completeWithResponsesAPI()` entry point | Low | ~40 |
| Routing logic + catalog flag | Low | ~20 |
| Tests (non-streaming + streaming + tool calls + cost) | Medium | ~250 |
| **Total** | **Medium** | **~530** |

The existing `CompletionRequest` / `CompletionResult` interfaces don't need to change. The entire Responses API path is additive — a new private code path within the existing `Client`.

---

## References

- [OpenAI Responses API streaming events](https://platform.openai.com/docs/api-reference/responses-streaming)
- [Migrate to the Responses API](https://platform.openai.com/docs/guides/migrate-to-responses)
- [OpenAI Responses API vs Chat Completions (Simon Willison)](https://simonwillison.net/2025/Mar/11/responses-vs-chat-completions/)
