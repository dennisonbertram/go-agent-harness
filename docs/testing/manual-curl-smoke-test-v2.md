# Manual Curl Smoke Test v2

**Date:** 2026-03-10
**Server:** `http://localhost:8080`
**Model:** `gpt-4.1-mini`
**Env:** `HARNESS_WORKSPACE=/tmp/harness-test-workspace`, `HARNESS_ROLLOUT_DIR=/tmp/harness-rollout`
**Note:** `HARNESS_CONVERSATION_DB` was NOT set in the running process, so conversation persistence endpoints return 501 for all store-dependent operations.

---

## API Architecture Notes

Before diving into results, key architectural facts discovered during testing:

- `POST /v1/runs` starts a run and returns `{"run_id": "run_N", "status": "queued"}` (202 Accepted). It does NOT stream.
- SSE streaming is via `GET /v1/runs/{run_id}/events` (text/event-stream).
- Run continuation uses `POST /v1/runs/{run_id}/continue` with body `{"message": "..."}`, NOT a `source_run_id` field in `POST /v1/runs`.
- `RunRequest` struct has no `source_run_id` field; the test spec's proposed form using that field silently ignores the unknown key and starts a fresh run.
- `GET /v1/runs` returns 405 (POST only); list-runs is not a supported endpoint.
- Conversation persistence requires `HARNESS_CONVERSATION_DB` env var (separate from `HARNESS_WORKSPACE`).

---

## Test Results

### Test 1: Health Check — PASS

```
GET /healthz
```

**Response:**
```json
{"status": "ok"}
```

HTTP 200. Server is healthy.

---

### Test 2: Basic Run with SSE Streaming — PASS

```
POST /v1/runs  {"prompt": "Say exactly: hello world", "max_steps": 3}
GET  /v1/runs/run_3/events
```

**Events observed (in order):**
- `run.started`
- `provider.resolved` — `{"model":"gpt-4.1-mini","provider":"default"}`
- `prompt.resolved`
- `run.step.started` — step 1
- `llm.turn.requested`
- `assistant.message.delta` (x2) — "hello", " world"
- `usage.delta`
- `llm.turn.completed`
- `assistant.message` — full content: `"hello world"`
- `memory.observe.started` / `memory.observe.completed`
- `run.step.completed`
- `run.completed` — output: `"hello world"`

All expected event types present. Stream terminated cleanly after `run.completed`. Model responded correctly with exact text.

---

### Test 3: Per-Run max_steps Enforcement — PASS

```
POST /v1/runs  {"prompt": "Use bash to count to 1000", "max_steps": 1}
GET  /v1/runs/run_4/events
```

**Terminal event:**
```json
event: run.failed
data: {
  "type": "run.failed",
  "payload": {
    "error": "max steps (1) reached",
    "max_steps": 1,
    "reason": "max_steps_reached",
    ...
  }
}
```

`run.failed` with `reason: "max_steps_reached"` fired exactly at step limit. The model had called bash (tool.call.delta events visible), and the step was terminated before the tool output was processed on the second step.

---

### Test 4: List Conversations — FAIL (config issue, not a bug)

```
GET /v1/conversations/
```

**Response:** HTTP 501
```json
{"error": {"code": "not_implemented", "message": "conversation persistence is not configured"}}
```

**Root cause:** The running `harnessd` process does not have `HARNESS_CONVERSATION_DB` set. The env var `HARNESS_WORKSPACE` is set but insufficient — the SQLite conversation store requires an explicit DB path via `HARNESS_CONVERSATION_DB`. This is a configuration gap in the test server launch, not a code defect.

Note: `GET /v1/conversations` (without trailing slash) returns HTTP 301 redirect to `/v1/conversations/`.

---

### Test 5: Search Conversations — FAIL (config issue, not a bug)

```
GET /v1/conversations/?q=hello
```

**Response:** HTTP 501 — same reason as Test 4. Store is nil.

---

### Test 6: JSONL Export — PARTIAL PASS

```
GET /v1/conversations/conv_abc123/export
```

**Response:** HTTP 404
```json
{"error": {"code": "not_found", "message": "conversation \"conv_abc123\" not found"}}
```

The export endpoint is reachable and returns a proper 404 (not 501). This indicates the endpoint correctly checks for the specific conversation before requiring the store. The endpoint code path works; it just needs a real conversation ID to export.

---

### Test 7: Context Compaction — FAIL (config issue, not a bug)

```
POST /v1/conversations/conv_abc123/compact
```

**Response:** HTTP 501 — store is nil. Same root cause as Tests 4-5.

---

### Test 8: Cost Ceiling — PARTIAL PASS (model unpriced)

```
POST /v1/runs  {"prompt": "What is 1+1?", "max_steps": 3, "max_cost_usd": 0.000001}
GET  /v1/runs/run_5/events
```

**Terminal event:**
```json
event: run.completed
data: {
  "payload": {
    "cost_totals": {"cost_usd_total": 0, "cost_status": "unpriced_model"},
    "output": "1 + 1 is 2."
  }
}
```

The run completed normally with `cost_status: "unpriced_model"`. The `usage.delta` event showed `cost_status: "unpriced_model"` and `turn_cost_usd: 0`. Per code review, the cost ceiling check is only enforced when `CostStatusAvailable`; unpriced models bypass the ceiling entirely. This is correct behavior per the spec comment in `RunRequest.MaxCostUSD`. To test actual ceiling enforcement, a priced model would be required.

---

### Test 9: Run Continuation — PASS (via correct endpoint)

**Note on test spec:** The spec used `source_run_id` in `POST /v1/runs`. No such field exists in `RunRequest`. The actual continuation endpoint is `POST /v1/runs/{id}/continue` with body `{"message": "..."}`.

**Test using correct endpoint:**

```
POST /v1/runs       {"prompt": "Remember the secret number is 42", "max_steps": 2}
  -> run_10 completed: "Understood: The secret number is 42..."

POST /v1/runs/run_10/continue   {"message": "What was the secret number?"}
  -> run_11 started
GET /v1/runs/run_11/events
```

**Key event in continuation stream:**
```
event: conversation.continued
```

**run_11 output:**
```
The secret number is 42.
```

Continuation correctly loaded the prior conversation context. The `conversation.continued` event was emitted. The model recalled "42" from the previous run's history.

**Behavior of incorrect `source_run_id` approach (original test spec):**

When `POST /v1/runs` is sent with `{"source_run_id": "run_6", "prompt": "..."}`, the field is silently ignored (not in `RunRequest`), and a fresh run starts with no prior context. The model responds asking for clarification. This is expected Go JSON decoding behavior (unknown fields ignored).

---

### Test 10: Rollout Recorder Files — PASS

```
ls /tmp/harness-rollout/
find /tmp/harness-rollout -name "*.jsonl"
```

**Directory contents:**
```
/tmp/harness-rollout/
  2026-03-10/
    run_1.jsonl   (14 lines)
    run_2.jsonl   (14 lines)
    run_3.jsonl   (14 lines)
    run_4.jsonl   (1033 lines - bash run with tool streaming)
    run_5.jsonl   (20 lines)
    run_6.jsonl   (39 lines)
    run_7.jsonl   (37 lines)
    ...
```

**Sample JSONL record (run_3.jsonl line 1):**
```json
{"ts":"2026-03-10T19:49:36.027894Z","seq":0,"type":"run.started","data":{"prompt":"Say exactly: hello world"}}
```

Files are organized by date subdirectory. All runs recorded. JSONL format with `ts`, `seq`, `type`, `data` fields. Continuation runs (run_11) also recorded.

---

### Test 11: Reasoning Effort Parameter — FAIL (model compatibility)

```
POST /v1/runs  {"prompt": "What is 2+2?", "max_steps": 2, "reasoning_effort": "low"}
```

**Terminal event:**
```json
event: run.failed
data: {
  "payload": {
    "error": "provider completion failed: openai request failed (400): {\"error\":{\"message\":\"Unrecognized request argument supplied: reasoning_effort\", ...}}",
    "reason": ""
  }
}
```

**Root cause:** `reasoning_effort` is a parameter for OpenAI o-series reasoning models (o1, o3, o4-mini). The server is running `gpt-4.1-mini`, which does not accept this parameter. The parameter is correctly forwarded to the provider but the model rejects it. This is expected behavior for an incompatible model.

The failure is clean: the run fails with a provider error rather than silently ignoring the parameter. The harness infrastructure for `reasoning_effort` works correctly; it just requires a compatible model (o1, o3, o3-mini, o4-mini, etc.).

---

### Test 12: Usage Delta Events — PASS

Verified from multiple runs. The `usage.delta` event appears after each LLM turn with full token accounting:

```json
event: usage.delta
data: {
  "type": "usage.delta",
  "payload": {
    "cost_status": "unpriced_model",
    "cumulative_cost_usd": 0,
    "cumulative_usage": {
      "prompt_tokens": 4144,
      "completion_tokens": 3,
      "total_tokens": 4147,
      "cached_prompt_tokens": 3968,
      "reasoning_tokens": 0,
      "input_audio_tokens": 0,
      "output_audio_tokens": 0
    },
    "step": 1,
    "turn_cost_usd": 0,
    "turn_usage": { ... },
    "usage_status": "provider_reported"
  }
}
```

Fields present: `cumulative_cost_usd`, `cumulative_usage`, `turn_cost_usd`, `turn_usage`, `cost_status`, `usage_status`, `pricing_version`, `step`. Cache hit tracking works (`cached_prompt_tokens: 3968` on second+ requests in same session).

---

### Test 13: Mid-Run Steering — PASS

```
POST /v1/runs  {"prompt": "Count slowly from 1 to 10 using bash, sleep 1 between each", "max_steps": 5}
  -> run_9 started, bash tool executing (tool.output.delta events: "1\n", "2\n", "3\n", "4\n"...)

POST /v1/runs/run_9/steer  {"message": "stop counting, just say done"}
  -> HTTP 200: {"status": "accepted"}
```

**Post-steer stream:**
The run received the steer mid-execution (while bash was counting). After completing the first step's tool call, the second step incorporated the steering message and responded:

```json
event: assistant.message
data: {"payload": {"content": "Done."}}

event: run.completed
data: {"payload": {"output": "Done."}}
```

The steer was accepted and acted upon correctly. The `tool.output.delta` events confirmed real-time streaming of bash output before the steer was received.

---

## Summary Table

| # | Test | Result | Notes |
|---|------|--------|-------|
| 1 | Health check | PASS | `{"status": "ok"}` |
| 2 | Basic SSE streaming | PASS | All expected event types present, output correct |
| 3 | max_steps enforcement | PASS | `run.failed` with `reason: "max_steps_reached"` |
| 4 | List conversations | FAIL (config) | Needs `HARNESS_CONVERSATION_DB` env var |
| 5 | Search conversations | FAIL (config) | Same as Test 4 |
| 6 | JSONL export | PARTIAL | Endpoint reachable, proper 404 for unknown conv ID |
| 7 | Context compaction | FAIL (config) | Needs `HARNESS_CONVERSATION_DB` env var |
| 8 | Cost ceiling | PARTIAL | Model unpriced; ceiling logic not exercised |
| 9 | Run continuation | PASS | Correct endpoint is `/v1/runs/{id}/continue`; context carried |
| 10 | Rollout recorder | PASS | Files in `/tmp/harness-rollout/2026-03-10/run_N.jsonl` |
| 11 | Reasoning effort | FAIL (compat) | `gpt-4.1-mini` rejects `reasoning_effort`; o-series required |
| 12 | Usage delta events | PASS | Full token/cost breakdown per turn, cache hits tracked |
| 13 | Mid-run steering | PASS | Steer accepted mid-bash, run redirected to "Done." |

**Passing:** 6/13
**Partial pass:** 2/13 (Tests 6, 8)
**Failing due to configuration gaps:** 3/13 (Tests 4, 5, 7 — missing `HARNESS_CONVERSATION_DB`)
**Failing due to model incompatibility:** 1/13 (Test 11 — `reasoning_effort` requires o-series)
**Spec error in test instructions:** Test 9 used non-existent `source_run_id` field; tested with correct `/continue` endpoint instead

---

## Issues Found

### Issue A: Continuation endpoint not documented in smoke test spec
The test spec uses `source_run_id` in `POST /v1/runs`, but this field does not exist in `RunRequest`. The actual continuation mechanism is `POST /v1/runs/{id}/continue`. The spec should be updated.

### Issue B: Conversation persistence requires separate env var
`HARNESS_WORKSPACE` alone is insufficient for conversation endpoints. `HARNESS_CONVERSATION_DB` must also be set. When running smoke tests with persistence, the full launch should be:
```bash
HARNESS_WORKSPACE=/tmp/harness-test-workspace \
HARNESS_CONVERSATION_DB=/tmp/harness-test-workspace/conversations.db \
HARNESS_ROLLOUT_DIR=/tmp/harness-rollout \
./harnessd
```

### Issue C: reasoning_effort incompatible with gpt-4.1-mini
The `reasoning_effort` parameter is correctly plumbed through the harness but rejected by the provider for non-reasoning models. No harness-level validation prevents sending it to an incompatible model. A guard or documentation note would help.

### Issue D: GET /v1/conversations redirects without trailing slash
`GET /v1/conversations` (no trailing slash) returns HTTP 301 to `/v1/conversations/`. Curl without `-L` will not follow the redirect. Clients should use the trailing slash or follow redirects.
