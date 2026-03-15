# Issue #11 Implementation Plan: Multi-Provider Support — Add Anthropic Provider

## Summary

Add Anthropic Claude provider alongside OpenAI. The harness already has a provider interface and catalog system. Anthropic requires a new provider package because its API differs from OpenAI's format (different message structure, content blocks, tool call format, headers).

## Files to Create

- `internal/provider/anthropic/client.go` (~650 lines) — full provider implementation
- `internal/provider/anthropic/client_test.go` (~500 lines) — comprehensive tests
- `internal/provider/anthropic/types.go` (~150 lines) — type definitions

## Files to Modify

- `catalog/models.json` — add Anthropic provider entry with 3 models
- `cmd/harnessd/main.go` — provider factory switch for "anthropic" protocol

## Key API Differences (Anthropic vs OpenAI)

| Aspect | OpenAI | Anthropic |
|--------|--------|-----------|
| Endpoint | `POST /v1/chat/completions` | `POST /v1/messages` |
| Auth | `Authorization: Bearer <key>` | `x-api-key: <key>` + `anthropic-version: 2023-06-01` |
| Tool calls in response | `message.tool_calls[]` | `content[]` where type="tool_use" |
| Tool results in request | Separate role="tool" message | `tool_result` content block in user message |
| Max tokens | Optional | Required |
| Streaming sentinel | `data: [DONE]` | No sentinel; `message_stop` event |

### Anthropic Message Format (multi-turn with tools)
```json
// Request
{"messages": [
  {"role": "user", "content": [{"type": "text", "text": "List files"}]},
  {"role": "assistant", "content": [{"type": "tool_use", "id": "1", "name": "list_files", "input": {}}]},
  {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "1", "content": "file1.txt"}]}
]}
// Response stop_reason: "tool_use" (not "tool_calls")
```

## Testing Strategy

### Unit Tests (17 tests)
- Text response parsing
- Single and multiple tool calls
- Message format conversion (OpenAI harness format → Anthropic API format)
- Tool definition conversion
- Cost calculation with pricing resolver
- Error responses (4xx/5xx)
- Streaming: text, tool calls, event reconstruction
- Edge cases: empty tool defs, nested tool inputs, missing usage

### Regression
Run `./scripts/test-regression.sh` — must pass all tests, 80% coverage, no 0% functions, clean `-race`.

## Risk Areas

1. **Message format conversion** — union content block type (text | tool_use | tool_result). Use tagged union with `type` field.
2. **Streaming event order** — must match Anthropic SSE sequence: message_start → content_block_start → content_block_delta → content_block_stop → message_stop.
3. **Tool input serialization** — Anthropic sends `input` as object; must re-serialize to JSON string for ToolCall.Arguments.
4. **Max tokens required** — must always set; use 4096 default.

## Commit Strategy

1. `feat(#11): Add Anthropic provider client with tool call support` — client.go + types.go + basic tests
2. `feat(#11): Add Anthropic streaming support` — streaming in client.go + streaming tests
3. `feat(#11): Register Anthropic in catalog and provider factory` — models.json + main.go

## No New Go Dependencies

Use `net/http` + `encoding/json` directly (same pattern as OpenAI provider). No `anthropic-sdk-go`.

## Catalog Models to Add

- `claude-opus-4-5`: $3.00/$15.00 per 1M tokens, 200K context
- `claude-sonnet-4-5`: $0.75/$3.00 per 1M tokens, 200K context
- `claude-haiku-4-5-20251001`: $0.08/$0.24 per 1M tokens, 200K context
