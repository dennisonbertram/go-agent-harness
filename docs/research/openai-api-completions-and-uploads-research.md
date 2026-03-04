# OpenAI API Completions and Uploads Research

**Date:** 2026-03-04
**Scope:** Chat Completions contract, streaming/tooling semantics, upload object lifecycle, file APIs, and vector store file operations.

## Completeness review

Compared with the first pass, this version adds missing modern details from Context7-backed OpenAI docs:

- More complete chat message schema (`developer`, `tool`, `function` roles and content-part unions).
- Current completion controls (`max_completion_tokens`, `parallel_tool_calls`, `service_tier`, `metadata`, `prompt_cache_*`, `stream_options`, `safety_identifier`).
- Streaming chunk format (`chat.completion.chunk`) and `finish_reason` options.
- File lifecycle and upload flow detail for `/uploads` + parts + completion + cancel.
- Vector store file management: retrieve/list/delete and status/error fields.

## 1) Sources (Context7-backed)

- OpenAI API Reference (`/v1/chat/completions`): https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
- Chat message and tool schema: https://developers.openai.com/api/reference/resources/chat/subresources/completions
- Tool choice options: https://developers.openai.com/api/reference/typescript/resources/chat/subresources/completions/methods/create
- Files: https://developers.openai.com/api/reference/resources/files/methods/create
- Uploads: https://developers.openai.com/api/reference/resources/uploads/methods/create
- Streaming events: https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events
- Vector stores: https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/create
- OpenAI platform docs (structured outputs and guides): https://platform.openai.com/docs/api-reference/chat/completions

## 2) Chat Completions API

### 2.1 Endpoint

- `POST /v1/chat/completions`
- Base: `https://api.openai.com`

### 2.2 Required request fields

- `model`: string model ID
- `messages`: array of message objects

Current accepted roles (union-based definitions):
- `system`
- `developer`
- `user`
- `assistant`
- `tool`
- `function`

`messages[].content` is now content-type aware:
- plain string for many cases
- array of content-part objects for complex user/tool content (text, image_url, input_audio, etc.)
- role-specific tool/function message fields (`tool_call_id`, `name`, structured fields)

### 2.3 Request options (modern + compatibility)

Sampling / length
- `max_completion_tokens` (preferred where supported)
- `max_tokens` (legacy; deprecated in newer docs)
- `temperature`
- `top_p`
- `n`
- `stop`
- `frequency_penalty`
- `presence_penalty`
- `logit_bias`

Tooling
- `tools` (function/MCP-like tool descriptors depending on SDK/docs variant)
- `tool_choice` (`none`, `auto`, `required`, or explicit tool object)
- `parallel_tool_calls`

Output format / safety
- `response_format` (`json_object`, `json_schema`)
- `safety_identifier`
- `refusal` response handling when safety blocks apply

Routing / infra
- `stream`
- `stream_options`
- `service_tier`
- `metadata`
- `seed`
- `user`
- `prompt_cache_key`
- `prompt_cache_retention`
- `reasoning_effort`
- `verbosity`

## 2.4 Example request

```json
{
  "model": "gpt-5",
  "messages": [
    { "role": "system", "content": "You are a helpful assistant." },
    { "role": "user", "content": "Who won the World Series in 2020?" }
  ],
  "temperature": 0.7,
  "max_completion_tokens": 512,
  "response_format": { "type": "json_object" }
}
```

### Alternative structured output

```json
{
  "model": "gpt-5",
  "messages": [{ "role": "user", "content": "Return name+age from this text." }],
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "person",
      "strict": true,
      "schema": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "age": { "type": "number" }
        },
        "required": ["name", "age"],
        "additionalProperties": false
      }
    }
  }
}
```

### 2.5 Response shape (non-stream)

- `id`
- `object: "chat.completion"`
- `created`
- `model`
- `system_fingerprint`
- `service_tier` (optional)
- `choices[]`
- `usage`

Each choice
- `index`
- `message`
- `finish_reason`
- optional `logprobs`

Message object
- `role`
- `content`
- optional `tool_calls[]`
- optional `refusal`
- optional `annotations`/`audio` in some SDK variants

Usage object now commonly includes
- `prompt_tokens`, `completion_tokens`, `total_tokens`
- `prompt_tokens_details` (`audio_tokens`, `cached_tokens`)
- `completion_tokens_details` (`reasoning_tokens`, `audio_tokens`, `accepted_prediction_tokens`, `rejected_prediction_tokens`)

Example (tool call path):

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4o",
  "service_tier": "auto",
  "system_fingerprint": "fp_123456",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 18,
    "total_tokens": 28
  }
}
```

Tool output message example

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\":18,\"unit\":\"celsius\"}"
}
```

### 2.6 Streaming (`stream: true`)

- Streaming returns SSE chunks of type `chat.completion.chunk`.
- The first chunk usually includes `delta.role = assistant` and empty content.
- Final chunk has `finish_reason` and may include usage if `stream_options.include_usage=true`.

`stream_options` can include
- `include_usage` (final chunk includes usage)
- `include_obfuscation` (obfuscation field on chunks for side-channel hardening)

`finish_reason` includes at least `stop`, `length`, `tool_calls`, `content_filter`.

Example last chunk pattern

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "created": 1694268190,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "delta": {},
      "finish_reason": "stop"
    }
  ]
}
```

### Tool-calling in stream deltas

When tools are used with `stream: true`, a `tool_calls` array can appear inside `choices[].delta`. The chunked tool-call form usually emits the full call incrementally by `index`:

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "created": 1700000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{ \"location\": \"Boston\""
            }
          }
        ]
      },
      "finish_reason": null
    }
  ]
}
```

The next chunk typically completes arguments (e.g., JSON suffix), then final `finish_reason: "tool_calls"` appears on completion.

## 3) Tool-calling contract

### 3.1 Modern schema

- Use `tools` + `tool_choice`.
- `tool_choice` controls invocation mode:
  - `none` (never call tools, generate message)
  - `auto` (model may choose content vs tool call)
  - `required` (model must call tool)
- explicit function/MCP tool object to force a specific target
- If no `tools` are provided, `tool_choice` defaults to `none`.
- If `tools` are provided and `tool_choice` omitted, default is typically `auto`.

#### Tool definition (function type)

Use `type: "function"` with a nested `function` object:

```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "description": "Get current weather",
    "parameters": {
      "type": "object",
      "properties": {
        "location": { "type": "string" },
        "unit": { "type": "string", "enum": ["celsius", "fahrenheit"] }
      },
      "required": ["location"]
    }
  }
}
```

#### `tool_choice` forms

String modes:
- `"none"` (never call tools)
- `"auto"` (let model choose)
- `"required"` (must call at least one tool)

Object forms:
- named function tool call:

```json
{
  "type": "function",
  "function": { "name": "get_weather" }
}
```

- named custom tool call (SDK-level support):

```json
{
  "type": "custom",
  "custom": { "name": "data_extractor" }
}
```

- allowed-tools mode (explicit allowlist):

```json
{
  "type": "allowed_tools",
  "allowed_tools": {
    "mode": "auto",
    "tools": [
      { "type": "function", "function": { "name": "get_weather" } }
    ]
  }
}
```

Some SDK/docs variants describe MCP forcing (`type: "mcp", server_label`, optional `name`). Treat that as runtime/SDK-dependent and validate against your selected client binding before use.

### 3.2 Tool message and tool call contract

Tool call payload in assistant output:
- `choices[].message.tool_calls[]` with:
  - `id` (call id)
  - `type` (`function` in most Chat Completions usage; some unions also expose `custom`)
  - `function.name`
  - `function.arguments` (JSON text)
- finish signal for tool route is typically `finish_reason: "tool_calls"` on the same choice.

Tool result message sent back to model:

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\":21,\"unit\":\"celsius\"}"
}
```

Required tool role fields:
- role must be `tool`
- must include `tool_call_id` for the targeted `tool_calls[].id`

Execution pattern:
1. Receive assistant message with `tool_calls`.
2. Execute each call safely and independently.
3. Send exactly one `tool` role message per tool id.
4. Continue loop until model returns non-tool final content.

### 3.3 Streaming tool-calls details

In `stream: true`, tool invocation can arrive as partial deltas:
- `choices[].delta.tool_calls[]` contains partial payloads.
- `tool_calls[].function.arguments` can be chunked across multiple events.
- Track deltas by `(choice_index, tool_call.index, tool_call.id)` and reassemble before dispatch.
- Finalization can still be `finish_reason: "tool_calls"`.

Streaming chunked tool-call example (incremental JSON):

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "choices": [
    {
      "index": 0,
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "id": "call_abc123",
            "type": "function",
            "function": { "name": "get_weather", "arguments": "{ \"location\": \"Boston\"" }
          }
        ]
      }
    }
  ]
}
```

### 3.4 Validation and failure policy

- Validate `function.arguments` before execution:
  - parse JSON string
  - check required parameters
  - type-check against schema
  - reject unknown keys when schema is strict
- Capture and log parse/type failures as hard boundaries (do not execute unsafe calls).
- Keep deprecated fallback path supported:
  - legacy `function_call` / `functions`
  - route both into one internal execution adapter

### 3.5 Parallelism and ordering

- `parallel_tool_calls` controls whether the model may emit multiple tool calls.
- If concurrency is unsafe in your system:
  - disable parallel mode
  - execute serially
  - preserve output ordering per `tool_call.id`
- For multi-choice runs (`n > 1`), keep tool state partitioned by choice index to avoid cross-choice mixing.

### 3.6 Common failure patterns to avoid

1. Executing tool arguments before validation.
2. Treating `"tool_calls"` as completion termination.
3. Emitting tool messages without matching `tool_call_id`.
4. Failing to dedupe/update partially streamed `tool_calls` before execution.
5. Ignoring fallback `function_call` payloads in older integrations.

### 3.7 Tooling quick references

- `tool_choice` and `tool_calls` fields in Chat Completions
- `ChatCompletionToolMessageParam` (`role: "tool"`, required `tool_call_id`)
- legacy compatibility path for `function_call`

---

## 4) Files and upload objects

### 4.1 `/files` endpoints

- `POST /v1/files` (multipart form-data)
  - required: `file`, `purpose`
  - file max: 512 MB
  - project storage cap: 2.5 TB

- `GET /v1/files` list (supports `purpose` filter)

- `GET /v1/files/{file_id}` retrieve

- `DELETE /v1/files/{file_id}` delete (irreversible)

- `GET /v1/files/{file_id}/content` binary content download

File object fields usually include
- `id`, `object: "file"`, `bytes`, `created_at`, `filename`, `purpose`
- `expires_at` when applicable
- status/state fields vary by doc/SDK version (some still show deprecated `status`, `status_details`)

Observed purpose families (current docs vary by endpoint/version)
- `assistants`
- `assistants_output`
- `batch`
- `batch_output`
- `fine-tune`
- `fine-tune-results`
- `vision`
- `user_data`
- `evals` (appears in upload docs)

### 4.2 `/uploads` flow

- `POST /uploads` creates a resumable Upload object.
  - required: `bytes`, `filename`, `mime_type`, `purpose`
  - optional: `expires_after` (anchor `created_at`, 3600–2592000 seconds)
  - upload accepts up to ~8 GB and expires (default behavior in docs: one hour unless purpose overrides)

- `POST /v1/uploads/parts` adds chunked part data
  - required: `upload_id`, `data` (base64), `part_number`

- `POST /uploads/{upload_id}/complete` finalizes with ordered `part_ids` and optional `md5`
  - byte total must match the create `bytes`
  - no parts can be added after completion

- `POST /v1/uploads/{upload_id}` cancels in-flight uploads

Upload object status values observed
- `pending`
- `completed`
- `cancelled`
- `expired`

When complete, nested `file` object becomes available.

## 5) Vector store file operations

### 5.1 Attach files

- `POST /vector_stores/{vector_store_id}/files`
- required: `file_id`
- optional: `attributes`, `chunking_strategy` (e.g., `static` with token overlap/size)
- response status values: `in_progress`, `completed`, `cancelled`, `failed`
- `last_error` object may include `code` (`server_error`, `unsupported_file`, `invalid_file`) and message

### 5.2 Manage attached files

- `GET /vector_stores/{vector_store_id}/files/{file_id}` retrieve status/details
- `DELETE /vector_stores/{vector_store_id}/files/{file_id}` removes file from vector store
  - deleting from vector store does not delete the original file object automatically
- SDK flows often provide bulk file-batch helpers with polling/status counts for ingestion completion.

## 6) Recommended implementation contract for this repo

1. Use modern contracts by default (`tools`, `tool_choice`, `tool_calls`, `response_format`).
2. Centralize parser/validator:
   - parse assistant text responses and tool call JSON defensively
   - support both modern and legacy `function_call` payloads behind feature flag
3. Treat tool argument JSON as untrusted input.
4. Preserve stream handling:
   - accumulate chunks by choice index
   - handle usage chunk only when `stream_options.include_usage=true`
5. Track audit fields (`system_fingerprint`, `service_tier`, `usage`, `finish_reason`) and log `seed`/cache keys when used.
6. Prefer `/uploads` for large files and resumable flows; use `/files` for normal uploads.

## 7) Is research complete now?

More complete than previous version, but open follow-up checks remain worthwhile:

- Model-by-model matrix for `response_format` + `reasoning_effort` + `json_schema` support.
- Exact error schema for completion and upload endpoints (`4xx/5xx` body shapes).
- File extension/mime whitelist per `purpose` for strict validation in your client.

If you want, I can produce a follow-up matrix table for each model family (`gpt-5`, `gpt-4o`, `gpt-4.1`, older `o*`) with supported request fields and failure modes.
