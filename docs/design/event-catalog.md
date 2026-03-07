# SSE Event Catalog

All events emitted by the harness runner over the SSE stream. Events use dot-notation naming: `category.action` or `category.subcategory.action`.

## Terminal Events

Only two events signal stream termination. Clients **must** close the connection after receiving either.

## Event Categories

### Run Lifecycle (5 events)

| Event | Terminal | Description |
|-------|----------|-------------|
| `run.started` | No | Run begins execution |
| `run.completed` | **Yes** | Run completed successfully |
| `run.failed` | **Yes** | Run failed with error |
| `run.waiting_for_user` | No | Waiting for user input (ask_user_question) |
| `run.resumed` | No | Run resumed after user answers |

#### `run.started`
```json
{ "prompt": "string" }
```

#### `run.completed`
```json
{
  "output": "string",
  "usage_totals": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cost_totals": { "total_usd": 0.0 }
}
```

#### `run.failed`
```json
{
  "error": "string",
  "usage_totals": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cost_totals": { "total_usd": 0.0 }
}
```

#### `run.waiting_for_user`
```json
{
  "call_id": "string",
  "tool": "ask_user_question",
  "questions": [],
  "deadline_at": "RFC3339 timestamp"
}
```

#### `run.resumed`
```json
{
  "call_id": "string",
  "tool": "string",
  "answered_at": "RFC3339 timestamp"
}
```

### LLM Turn (3 events)

| Event | Description |
|-------|-------------|
| `llm.turn.requested` | LLM call initiated for step N |
| `llm.turn.completed` | LLM response received |
| `assistant.message.delta` | Streaming content chunk |

#### `llm.turn.requested`
```json
{ "step": 1 }
```

#### `llm.turn.completed`
```json
{ "step": 1, "tool_calls": 2 }
```

#### `assistant.message.delta`
```json
{ "step": 1, "content": "string" }
```

### Tool Execution (3 events)

| Event | Description |
|-------|-------------|
| `tool.call.started` | Tool execution begins |
| `tool.call.completed` | Tool execution finished (success or error) |
| `tool.call.delta` | Streaming tool call argument chunk |

#### `tool.call.started`
```json
{ "call_id": "string", "tool": "string", "arguments": "string (JSON)" }
```

#### `tool.call.completed`
```json
{ "call_id": "string", "tool": "string", "output": "string", "error": "string (optional)" }
```

#### `tool.call.delta`
```json
{ "step": 1, "index": 0, "call_id": "string", "tool": "string", "arguments": "string" }
```

### Assistant Completion (1 event)

| Event | Description |
|-------|-------------|
| `assistant.message` | Final assistant message (no tool calls) |

#### `assistant.message`
```json
{ "content": "string" }
```

### Conversation (1 event)

| Event | Description |
|-------|-------------|
| `conversation.continued` | Prior conversation history loaded |

#### `conversation.continued`
```json
{ "conversation_id": "string", "prior_message_count": 5 }
```

### Prompt Resolution (2 events)

| Event | Description |
|-------|-------------|
| `prompt.resolved` | System prompt resolved via prompt engine |
| `prompt.warning` | Warning from prompt resolution |

#### `prompt.resolved`
```json
{
  "intent": "string",
  "model_profile": "string",
  "model_fallback": "string",
  "applied_behaviors": [],
  "applied_talents": [],
  "reserved_skills_ignored": false
}
```

#### `prompt.warning`
```json
{ "code": "string", "message": "string" }
```

### Memory (4 events)

| Event | Description |
|-------|-------------|
| `memory.observe.started` | Memory observation begins |
| `memory.observe.completed` | Memory observation succeeded |
| `memory.observe.failed` | Memory observation failed |
| `memory.reflection.completed` | Memory reflection triggered |

#### `memory.observe.started`
```json
{ "step": 1 }
```

#### `memory.observe.completed`
```json
{ "step": 1, "observed": true, "reflected": false, "observation": 5 }
```

#### `memory.observe.failed`
```json
{ "step": 1, "error": "string" }
```

#### `memory.reflection.completed`
```json
{ "step": 1 }
```

### Accounting (1 event)

| Event | Description |
|-------|-------------|
| `usage.delta` | Token usage and cost for a turn |

#### `usage.delta`
```json
{
  "step": 1,
  "usage_status": "provider_reported",
  "cost_status": "available",
  "turn_usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "turn_cost_usd": 0.001,
  "cumulative_usage": { "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0 },
  "cumulative_cost_usd": 0.003,
  "pricing_version": "string"
}
```

### Hooks (3 events)

| Event | Description |
|-------|-------------|
| `hook.started` | Hook execution begins |
| `hook.failed` | Hook execution failed |
| `hook.completed` | Hook execution completed |

#### `hook.started`
```json
{ "stage": "pre_message|post_message", "hook": "string", "step": 1 }
```

#### `hook.failed`
```json
{
  "stage": "pre_message|post_message",
  "hook": "string",
  "step": 1,
  "error": "string",
  "mode": "fail_closed|fail_open",
  "ignored": false
}
```

#### `hook.completed`
```json
{
  "stage": "pre_message|post_message",
  "hook": "string",
  "step": 1,
  "action": "continue|block",
  "mutated": false,
  "reason": "string (optional)"
}
```

## SSE Wire Format

Each event is sent as an SSE block:

```
id: <runID>:<seq>
retry: 3000
event: <event-type>
data: <JSON-encoded Event object>

```

Event IDs use the format `{runID}:{seq}` where `seq` is a 0-based contiguous index into the run's event history. For example: `run_1:0`, `run_1:1`, `run_1:2`, etc. Each run's sequence starts at 0.

The `Event` JSON envelope:
```json
{
  "id": "run_1:0",
  "run_id": "run_1",
  "type": "event.type",
  "timestamp": "RFC3339",
  "payload": { ... }
}
```

## Reconnection

SSE clients can reconnect by sending the `Last-Event-ID` header with the ID of the last event they received. The server will skip all events up to and including that sequence number, replaying only newer events.

Example: if a client reconnects with `Last-Event-ID: run_1:3`, events `run_1:0` through `run_1:3` are skipped and replay begins at `run_1:4`.

The `retry: 3000` field tells clients to retry after 3 seconds on disconnect.

## Constants

All event types are defined as `EventType` constants in `internal/harness/events.go`. Use `IsTerminalEvent()` to check for stream-ending events. Use `AllEventTypes()` to enumerate all known types.
