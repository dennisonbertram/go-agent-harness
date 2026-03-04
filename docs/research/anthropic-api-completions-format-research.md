# Anthropic API Completions & Tooling Research

**Date:** 2026-03-04  
**Scope:** Request/response format, tool calling, streaming, file handling, batches, and migration guidance for Anthropic APIs.

## 1) Completeness check summary

Previous version was directionally correct, but it was missing several operationally useful details.  
Current version adds:

- Concrete API/SDK endpoint-level details from Context7 for message creation, streaming, and batching.
- Explicit stream event semantics and stream lifecycle helpers.
- File API endpoint and operation surface with beta-gated semantics.
- Error-return semantics for tool execution via `ToolError` and `is_error`.
- A stronger implementation playbook for correlation IDs and request tracing.

## 2) Source set (Context7 + official references)

- Anthropic SDK TypeScript API docs: https://github.com/anthropics/anthropic-sdk-typescript/blob/main/api.md  
- Anthropic SDK TypeScript helper/docs: https://github.com/anthropics/anthropic-sdk-typescript/blob/main/README.md, https://github.com/anthropics/anthropic-sdk-typescript/blob/main/helpers.md  
- Context7 Anthropic TypeScript index: https://context7.com/anthropics/anthropic-sdk-typescript/llms.txt  
- Context7 Anthropic API index: https://context7.com/llmstxt/anthropic_llms-full_txt (quality note: low for direct retrieval in this run)
- Anthropic official docs:
  - https://docs.anthropic.com/en/api/overview
  - https://docs.anthropic.com/en/api/messages
  - https://docs.anthropic.com/en/api/tools
  - https://docs.anthropic.com/en/api/files
  - https://docs.anthropic.com/en/api/messages/batches
  - https://docs.anthropic.com/en/api/count-tokens

## 3) API model and version strategy

### 3.1 Canonical vs compatibility API paths

- Canonical: `POST /v1/messages`.
- Compatibility mode: OpenAI-like compatibility adapters are useful for migrations, but should remain adapter-layer only for new work.
- Legacy: `/v1/complete` exists only for compatibility and should not be default for new implementations.

### 3.2 Transport and preview controls

- Direct REST contract requires `x-api-key`, `anthropic-version`, and JSON content type.
- Beta/preview features are commonly passed via versioning mechanism (`anthropic-beta` headers or SDK `betas`/query flags in beta flows).
- File APIs and several new capabilities are beta-gated in current examples and require exact beta tokens.

## 4) `POST /v1/messages` contract

### 4.1 Required request fields

- `model` (string)
- `max_tokens` (integer)
- `messages` (array)

### 4.2 Message content

Each message has:

- `role` (`user` or `assistant`)
- `content` (string or content block array)

### 4.3 Common optional controls

- `system`
- `temperature`, `top_p`, `top_k`
- `stop_sequences`
- `stream`
- `tools`
- `tool_choice`
- `betas` (client-side preview feature flags)
- `metadata` where available in client/sdk

### 4.4 Example

```json
{
  "model": "claude-sonnet-4-5-20250929",
  "max_tokens": 1024,
  "messages": [{ "role": "user", "content": "What is the population of Paris?" }],
  "system": "Answer precisely and cite source links when available.",
  "temperature": 0.2
}
```

### 4.5 Response shape

- `id`, `type`, `role`, `content[]`, `model`, `stop_reason`, `usage`.
- `stop_reason` is a string; examples include `end_turn` and `tool_use` in practice.

## 5) Content blocks

Core blocks for non-legacy flows:

- `text`
- `tool_use`
- `tool_result`
- `image`
- `document`

SDK typing surfaces include broader block families (e.g., thinking/citation/tool-result variants) depending on model, API version, and enabled features. Treat exact supported blocks as versioned.

## 6) Tooling behavior

### 6.1 Tool declaration contract

Tool definitions in `tools[]` use:

- `name` (required) — must match regex `^[a-zA-Z0-9_-]{1,64}$`
- `description` (required) and should be explicit/verbose (Anthropic docs call this a major quality signal)
- `input_schema` (required JSON Schema)
- `input_examples` (optional) — list of schema-valid examples to disambiguate complex tools
- `strict` (optional) — when `true`, requires strict schema adherence and can be combined with `tool_choice: {"type":"any"}` to guarantee tool usage

Client-side tool examples:

- Keep descriptions 3–4+ sentences for non-trivial tools.
- Include action/response shape in tool description.
- Prefer meaningful namespacing for scale (`github_list_prs` vs `list_prs`).
- Provide only high-signal outputs (stable IDs, compact summaries).

### 6.2 Tool selection behavior (`tool_choice`)

`tool_choice` supports these documented forms:

- `{"type":"auto"}` (default when `tools` present): model decides if/when to call.
- `{"type":"none"}`: disallow tool calls (default when no tools are supplied).
- `{"type":"any"}`: force at least one tool call, no specific tool.
- `{"type":"tool","name":"..."}`

Important:

- With `tool_choice` as `any` or `tool`, `tool_use` may be prefilled and natural language text may be skipped before tool blocks.
- With extended thinking, only `auto` and `none` are supported; `any`/`tool` combinations return errors.
- Changing `tool_choice` affects prompt-cached content blocks (tool definitions and system prompts remain cached; message content does not).

### 6.3 Tool-use execution flow (manual orchestration)

When `tool_use` is returned, each block contains:

- `id` (tool-use block ID)
- `name`
- `input` (tool arguments, conforms to schema)

Recommended loop:

1. On response `stop_reason = "tool_use"`, iterate all `tool_use` blocks.
2. Execute each tool with `(name,input)` and capture outputs.
3. Send a follow-on `user` message immediately containing matching `tool_result` blocks:
   - `tool_use_id` must match the originating `tool_use.id`.
   - `content` can be text or content blocks (`text`, `image`, `document`).
   - `is_error: true` when tool execution failed.
4. In that `user` message, place all `tool_result` blocks before any `text` blocks.
5. Append no messages between assistant tool-use turn and tool-result turn.
6. Resend full conversation history to `POST /v1/messages`.

API-level required ordering:

- Tool results must be immediate neighbors in history.
- `tool_result` must come FIRST in the `user` content array, then any explanatory text.
- Violating this yields errors like “tool_use ids were found without tool_result blocks immediately after”.

### 6.4 Stop-reason handling for tool workflows

Tool-specific reasons called out in docs:

- `tool_use`: continue the tool execution loop.
- `max_tokens`: may produce truncated/incomplete tool calls; retry with larger `max_tokens` before replaying tool results.
- `pause_turn`: can occur with long-running server tools (example: web search); continue the turn by sending the paused assistant content back and retrying with same tool context.

### 6.5 Parallel and token-efficient tool use

- Default behavior may include multiple tool calls.
- `disable_parallel_tool_use=true` enforces single-tool usage (`auto` => at most one, `any/tool` => exactly one).
- Token-efficient tool use is a beta signal (`token-efficient-tools-2025-02-19`) and can be used to encourage parallel use on capable models; not supported with `disable_parallel_tool_use`.

### 6.6 Tool results as stateful content

- Cache metadata (`cache_control`) can be added to tool results in the SDK/docs guidance to reduce repeated reprocessing for large outputs.
- SDK helper utilities (`ToolError`, beta `toolRunner`) can simplify orchestration:
  - `ToolError` maps failures to tool-result-form output and can return structured blocks.

### 6.7 SDK-specific vs manual control

- Prefer `toolRunner` for simpler loops when acceptable.
- Keep manual flow for custom permissioning, retries, auditing, and strict orchestration.

## 7) Streaming

Anthropic streaming in SDK form emits event lifecycle types and event helpers:

- SSE-style event classes in stream output include `content_block_start`, `content_block_delta`, `content_block_stop`, `message_stop`.
- Stream helpers expose `.on('text'|'message'|'messageStop'|'contentBlock'|'streamEvent'|'error'|'end')` style subscriptions (via SDK helpers; exact event names can vary by wrapper/version).
- Use `finalMessage()` / `done()` / final stream accumulator for complete accounting after stream completion.

Practical implementation note:

- Keep partial state by reading message snapshot events and read final usage from the final message object after completion.

## 8) Count tokens and batching

### 8.1 `POST /v1/messages/count_tokens`

- Request: `model`, `messages`, optional `system`.
- Response: `input_tokens`, `output_tokens`.

### 8.2 `POST /v1/messages/batches`

- Used for fan-out workflows; in practice payload includes `requests[]`.
- Each request item includes `custom_id` and nested `params` (message payload).
- Additional batch operations include:
  - `GET /v1/messages/batches/{message_batch_id}/results`
  - `POST /v1/messages/batches/{message_batch_id}/cancel`
- Use `custom_id` or message IDs for durable result correlation.

## 9) Files and multimodal workflows

### 9.1 Inline image blocks

- `image` blocks with base64 source remain viable for direct, small image payloads.

### 9.2 Beta file API

Current SDK workflow uses beta-gated file operations:

- `client.beta.files.upload(...)`
- `client.beta.files.list(...)`
- `client.beta.files.retrieveMetadata(fileId, ...)`
- `client.beta.files.download(fileId, ...)`
- `client.beta.files.delete(fileId, ...)`

Reference in message content typically uses:

- `{"type":"document","source":{"type":"file","file_id":"file_..."}}`
- `{"type":"image","source":{"type":"file","file_id":"file_..."}}`

Metadata returned by these flows includes ids, filenames, and byte sizes in examples.

## 10) Implementation policy

1. Use canonical Messages API as default.
2. Keep tool orchestration deterministic:
   - `tool_use` -> `tool_result` loop.
3. Treat beta flags and version headers as explicit runtime config.
4. Use file IDs for reusable binaries; avoid repetitive inline media payloads.
5. Keep OpenAI-compatible behavior isolated.

## 11) Migration notes

- Retirement targets:
  - legacy completion-style request wiring
  - implicit/no-schema tool assumptions
- Adoption targets:
  - schema-first tooling (`tools` + `input_schema`)
  - explicit loop semantics
  - beta-gated file APIs + request tracing + model/version pinning

## 12) Monitoring / drift watch list

1. API surface changes around file endpoints and beta tokens.
2. Tooling behavior changes around stop reasons and structured error handling.
3. New stream events and helper API name changes.
4. `model` lineup / token limits and capability flags.

## 13) Recommended maintenance loop

Before any implementation change:

1. Verify docs/source page updated date.
2. Cross-check endpoint fields against the exact SDK version in use.
3. Confirm model-specific behavior and beta requirements before shipping.
