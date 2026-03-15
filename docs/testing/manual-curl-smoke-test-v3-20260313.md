# Manual Curl Smoke Test v3 — 2026-03-13

## Environment

- Server binary: `/tmp/harnessd-test` (built from `cmd/harnessd/`)
- Port: `:8082` (8080 was occupied by another harnessd instance)
- Config: `HARNESS_MAX_STEPS=3`, `OPENAI_API_KEY` set
- OpenAI model tested: `gpt-4.1-mini`
- Run date: 2026-03-13

---

## Test Results

### Test 1 — Health Check

**Endpoint**: `GET /healthz`

**Result**: PASS

```json
{"status":"ok"}
```

### Test 2 — Create Conversation

**Endpoint**: `POST /v1/conversations`

**Result**: NOTE — There is no standalone "create conversation" endpoint.

- `POST /v1/conversations` (no trailing slash) returns `HTTP 301 Moved Permanently` redirecting to `/v1/conversations/`
- `POST /v1/conversations/` returns `{"error":{"code":"method_not_allowed","message":"method not allowed"}}` — the `/v1/conversations/` handler only supports GET.

**Finding**: Conversations are **auto-created** when a run is started with a new `conversation_id`. If no `conversation_id` is supplied, the server auto-assigns `conversation_id = run_id`. This is by design.

### Test 3 — List Conversations

**Endpoint**: `GET /v1/conversations/`

**Result**: PASS (expected behavior for no-persistence mode)

```json
{"error":{"code":"not_implemented","message":"conversation persistence is not configured"}}
```

The server was started without a database, so the in-memory-only mode returns this. This is correct — `ConversationStore` is nil when no DB path is configured.

### Test 4 — Start a Run

**Endpoint**: `POST /v1/runs`

**Result**: PASS

```json
{"run_id":"run_c989870b-63b7-4fcf-a229-27ac044a62b5","status":"queued"}
```

- Run was queued immediately.
- No `conversation_id` provided — server assigned `conversation_id = run_id`.

### Test 5 — Get Run Status

**Endpoint**: `GET /v1/runs/{id}`

**Result**: PASS

Run completed in ~1.2 seconds. Response:

```json
{
  "id": "run_c989870b-63b7-4fcf-a229-27ac044a62b5",
  "prompt": "Say exactly: HELLO_WORLD",
  "model": "gpt-4.1-mini",
  "provider_name": "default",
  "status": "completed",
  "output": "HELLO_WORLD",
  "usage_totals": {
    "prompt_tokens_total": 4715,
    "completion_tokens_total": 4,
    "total_tokens": 4719,
    "last_turn_tokens": 4719
  },
  "cost_totals": {
    "cost_usd_total": 0,
    "last_turn_cost_usd": 0,
    "cost_status": "unpriced_model"
  },
  "tenant_id": "default",
  "conversation_id": "run_c989870b-63b7-4fcf-a229-27ac044a62b5",
  "agent_id": "default",
  "created_at": "2026-03-13T17:19:18.476883Z",
  "updated_at": "2026-03-13T17:19:19.705975Z"
}
```

State machine: `queued` → `running` → `completed`. Output matched prompt exactly (`HELLO_WORLD`).

### Test 6 — Stream Run Events via SSE

**Endpoint**: `GET /v1/runs/{id}/events`

**Result**: PASS

SSE events observed (in order) for a completed run replayed from in-memory buffer:

| Seq | Event Type |
|-----|-----------|
| 0 | `run.started` |
| 1 | `provider.resolved` |
| 2 | `prompt.resolved` |
| 3 | `run.step.started` |
| 4 | `llm.turn.requested` |
| 5-7 | `assistant.message.delta` (streaming tokens: "HEL", "LO", "_WORLD") |
| 8 | `usage.delta` |
| 9 | `llm.turn.completed` |
| 10 | `assistant.message` (full message) |
| 11 | `memory.observe.started` |
| 12 | `memory.observe.completed` |
| 13 | `run.step.completed` |
| 14 | `run.completed` |

All events include `run_id`, `timestamp`, `schema_version`, and `step` in their payloads. Token streaming via `assistant.message.delta` events works correctly. `memory.observe.completed` shows `observed: false, reflected: false` (no memory configured). `usage.delta` correctly reports `cost_status: "unpriced_model"` and includes `cached_prompt_tokens` tracking.

### Test 7 — Get Conversation Messages

**Endpoint**: `GET /v1/conversations/{id}/messages`

**Result**: PASS

```json
{
  "messages": [
    {"role": "user", "content": "Say exactly: HELLO_WORLD"},
    {"role": "assistant", "content": "HELLO_WORLD"}
  ]
}
```

In-memory conversation history is maintained correctly across runs.

### Test 8 — Compact Endpoint

**Endpoint**: `POST /v1/conversations/{id}/compact`

**Result**: PASS (expected behavior)

- `GET /v1/conversations/{id}/compact` → `method_not_allowed` (correct, only POST)
- `POST /v1/conversations/{id}/compact` → `{"error":{"code":"not_implemented","message":"conversation persistence is not configured"}}`

The compaction endpoint requires a persisted conversation store. Without a DB configured, it correctly returns `not_implemented`. No in-memory compaction is triggered without persistence.

**Note**: There is also `POST /v1/runs/{id}/compact` (run-level compaction). For a completed run this returns `run_not_active` which is correct — compaction can only be triggered during an active run.

### Test 9 — Multi-Turn Continuation (same conversation_id)

**Endpoint**: `POST /v1/runs` with explicit `conversation_id`

**Result**: PASS — conversation tracking works correctly.

Run 2 on same conversation:
```json
{"prompt":"What did I just ask you to say?","conversation_id":"run_c989870b-63b7-4fcf-a229-27ac044a62b5","model":"gpt-4.1-mini","max_steps":2}
```

Response: `"output": "You asked me to say exactly: HELLO_WORLD"`

Conversation messages after run 2:
```json
{
  "messages": [
    {"role": "user", "content": "Say exactly: HELLO_WORLD"},
    {"role": "assistant", "content": "HELLO_WORLD"},
    {"role": "user", "content": "What did I just ask you to say?"},
    {"role": "assistant", "content": "You asked me to say exactly: HELLO_WORLD"}
  ]
}
```

The model correctly recalled the prior exchange. Multi-turn history is threaded correctly through the conversation.

### Test 10 — Replay Endpoint

**Endpoint**: `POST /v1/runs/replay`

**Result**: PASS (all error cases handled correctly)

| Request | Expected | Actual |
|---------|----------|--------|
| Missing `rollout_path` | `invalid_request` | PASS |
| `GET /v1/runs/replay` | `method_not_allowed` | PASS |
| `mode: "invalid"` | `invalid_request` with clear message | PASS |
| Nonexistent rollout file | `rollout_not_found` with path in message | PASS |

No existing rollout files found in `.harness/` (only db files). The endpoint routing correctly intercepts `"replay"` before treating it as a run ID.

### Test 11 — Error Paths

**Endpoint**: Various

**Result**: PASS — all error responses are well-structured.

| Scenario | HTTP Code | Error Code | Message |
|----------|-----------|------------|---------|
| Missing `prompt` | 400 | `invalid_request` | "prompt is required" |
| Nonexistent `run_id` | 404 | `not_found` | `run "nonexistent-run-id" not found` |
| Nonexistent `conversation_id` messages | 404 | `not_found` | `conversation "nonexistent-conv-id" not found` |
| Invalid JSON body | 400 | `invalid_json` | exact parse error |
| `max_steps: -1` | 400 | `invalid_request` | "max_steps must be >= 0 (0 means use runner default)" |

All errors return `{"error":{"code":"...","message":"..."}}` envelope. No panics. No 500s.

### Test 12 — Conversation Export (JSONL)

**Endpoint**: `GET /v1/conversations/{id}/export`

**Result**: PASS

```
{"role":"user","content":"Say exactly: HELLO_WORLD"}
{"role":"assistant","content":"HELLO_WORLD"}
{"role":"user","content":"What did I just ask you to say?"}
{"role":"assistant","content":"You asked me to say exactly: HELLO_WORLD"}
```

Returns newline-delimited JSON (JSONL format). Includes full multi-turn history. No extra headers needed for content negotiation.

### Test 13 — Live SSE Stream During Active Run

**Result**: PASS

A live run for "Count from 1 to 5" was observed in real-time via SSE. 21 events emitted total including:
- Streaming deltas (each digit streamed individually: "1", "\n", "2", "\n", "3", "\n", "4", "\n", "5")
- `usage.delta` showing `cached_prompt_tokens: 3840` (caching working)
- `run.completed` event with full output and token totals
- TTFT: 1220ms, total_duration_ms: 1303ms

No compaction events observed (conversation is very short, no pressure).

---

## Additional Endpoint Tests

### /v1/runs/{id}/context

**Result**: PASS

```json
{
  "message_count": 2,
  "estimated_tokens": 9,
  "context_pressure": "low"
}
```

### /v1/runs/{id}/summary

**Result**: PASS

Returns step count, token totals, cost, tool_calls array, cache hit rate.

### /v1/runs/{id}/todos

**Result**: PASS

```json
{"run_id":"...","todos":[]}
```

### /v1/runs/{id}/steer (on completed run)

**Result**: PASS

Returns `{"error":{"code":"run_not_active","message":"run is not active"}}` — correct, steering only works on live runs.

### /v1/runs/{id}/continue

**Result**: PASS

Continued a completed run with `"Now say GOODBYE_WORLD"`. Server created a new run (`run_ab58374d-...`) on the same conversation. Model responded `"GOODBYE_WORLD"`. Conversation then had 6 messages total (full history preserved). The `continue` endpoint creates a new run linked to the same `conversation_id`.

### /v1/models

**Result**: PASS (no catalog configured)

```json
{"models":[]}
```

### /v1/providers

**Result**: PASS (no catalog configured)

```json
{"providers":[]}
```

### /v1/agents

**Result**: NOTE — `GET /v1/agents` returns `method_not_allowed`. Only `POST` is accepted (for agent invocation). This is intentional per the route definition.

### /v1/skills

**Result**: PASS (skills not configured)

```json
{"error":{"code":"not_configured","message":"skills not configured"}}
```

### /v1/recipes

**Result**: PASS

```json
{"count":0,"recipes":[]}
```

### /v1/search/code (Sourcegraph proxy)

**Result**: PASS (not configured)

```json
{"error":{"code":"not_configured","message":"sourcegraph not configured"}}
```

### /v1/mcp/servers

**Result**: PASS (no MCP servers registered)

```json
{"servers":[]}
```

### /v1/cron/jobs

**Result**: PASS (cron not configured via HTTP, though embedded scheduler started)

```json
{"error":{"code":"not_configured","message":"cron not configured"}}
```

### max_cost_usd Feature

Started a run with `max_cost_usd: 0.00001`. Since `gpt-4.1-mini` is an unpriced model (`cost_status: "unpriced_model"`), the cost ceiling was not enforced (by design — cost ceiling only enforces when pricing is available). Run completed normally. This is correct behavior per the type definition.

---

## Key Findings

### Compaction and Conversation Tracking

1. **In-memory conversation history** is maintained correctly across multiple runs with the same `conversation_id`. Messages from all runs accumulate in order.

2. **Conversation persistence** (`/v1/conversations/` list, search, export to DB) is not active in the default startup config. This requires a DB path to be configured (e.g., via `HARNESS_DB` or similar). The in-memory fallback works for single-session use.

3. **Context compaction** (`/v1/conversations/{id}/compact`, `/v1/runs/{id}/compact`) both require either:
   - An active run (for run-level compaction)
   - A configured conversation store (for conversation-level compaction)
   Neither was available in this test configuration, so both return appropriate "not configured" / "not active" errors. No automatic proactive compaction was observed (context pressure was "low" throughout).

4. **`/v1/runs/{id}/context`** reports context pressure as "low" with accurate `message_count` and `estimated_tokens`. The token estimate appears to be a rough count (reported 9 tokens for a 4-word exchange).

5. **`/v1/runs/{id}/continue`** works correctly — creates a new run on the same conversation. The multi-turn history is fully injected into the new run's context.

### SSE Streaming

- Events stream correctly from the in-memory buffer for completed runs.
- The `retry: 3000` reconnect hint is present on all events.
- Event IDs use `{run_id}:{sequence}` format for position tracking.
- All 14+ event types in the happy path were observed.
- `assistant.message.delta` streaming works at sub-token granularity (individual characters/substrings).

### State Machine

Run state transitions observed: `queued` → (implicit `running` during execution) → `completed`. The `GET /v1/runs/{id}` endpoint shows `"status": "running"` when polled mid-execution (observed once for the long count run).

### Error Handling

All error responses are consistently structured as `{"error":{"code":"...","message":"..."}}`. No 500 errors, no panics, no unstructured error responses encountered.

### Server Log

Server log was clean — 4 startup lines only, no warnings or errors during the entire test run. The embedded cron scheduler, hot-reload watcher, and delayed callbacks all started successfully.

---

## Anomalies / Notes

1. **POST /v1/conversations does 301 redirect** to `/v1/conversations/` (trailing slash). This is Go's standard `http.ServeMux` behavior for path normalization. Clients should use the trailing slash form or follow redirects.

2. **`/v1/runs/{id}/context` token estimate** reported 9 tokens for a 2-message conversation ("Say exactly: HELLO_WORLD" / "HELLO_WORLD"). The estimate is intentionally approximate.

3. **cost_status: "unpriced_model"** for `gpt-4.1-mini` — the pricing table does not include this model. The `max_cost_usd` ceiling is silently bypassed for unpriced models (correct per design, documented in the `RunRequest` struct comment).

4. **Conversation runs list** (`GET /v1/conversations/{id}/runs`) requires `runStore` to be configured — returns `not_implemented` without it. This means conversation-level run history is ephemeral in default config.

5. **`GET /v1/agents` returns method_not_allowed** — the agents endpoint only supports `POST`. This may be surprising but is consistent with its role as an agent invocation endpoint.

---

## Overall Assessment

**The server is fully functional** for core workflows in the default (no-persistence) configuration:

- Run creation, execution, and status polling all work.
- Multi-turn conversation threading is correct.
- SSE event streaming is well-structured and complete.
- Error handling is consistent and informative.
- The `continue` endpoint correctly chains runs on the same conversation.
- No crashes, panics, or 500 errors observed.

The persistence-dependent features (conversation listing, compact, run history) correctly degrade to `not_implemented` errors rather than silently failing — this is good defensive behavior.

**Compaction was not exercised** because: (a) no DB was configured so `POST /v1/conversations/{id}/compact` returns `not_implemented`; (b) no runs were long enough to trigger automatic compaction (context pressure stayed "low"). A full compaction test would require a SQLite-backed run with a long conversation history.
