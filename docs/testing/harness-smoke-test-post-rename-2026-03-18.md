# Harness Smoke Test: Post-Rename Field Verification
**Date**: 2026-03-18
**Branch**: main
**Binary**: harnessd (freshly rebuilt from `./cmd/harnessd/`)

## Summary

All 5 smoke tests PASSED. The `prompt` field is correctly required on both `/continue` and `/steer` endpoints. The old `message` field is correctly rejected with HTTP 400.

---

## Environment

- Server PID: 30918
- Listen address: :8080
- Log: /tmp/harnessd.log
- Model catalog: 10 providers loaded
- Skills hot-reload: active

---

## Test Results

### Test 1: Health Check

**Request**: `GET /healthz`

**Response** (HTTP 200):
```json
{"status":"ok"}
```

**Result**: PASS

---

### Test 2: Anthropic Provider Run

**Request**: `POST /v1/runs`
```json
{
  "model": "claude-haiku-4-5-20251001",
  "prompt": "Reply with exactly: SMOKE_TEST_PASS",
  "provider": "anthropic",
  "max_steps": 3
}
```

**Run ID**: `run_391be2e6-535c-4c63-ac57-a2d4f547fca1`

**SSE events observed** (via `GET /v1/runs/{id}/events`):
- `run.started`
- `provider.resolved` (provider=anthropic, model=claude-haiku-4-5-20251001)
- `prompt.resolved`
- `run.step.started`
- `llm.turn.requested`
- `assistant.message.delta` (content="SMOKE_TEST_PASS")
- `usage.delta` (9035 total tokens, cost_status=unpriced_model)
- `llm.turn.completed` (total_duration_ms=901)
- `assistant.message` (content="SMOKE_TEST_PASS")
- `memory.observe.started` / `memory.observe.completed`
- `run.step.completed`
- `run.completed` (output="SMOKE_TEST_PASS")

**Result**: PASS — model replied with exactly `SMOKE_TEST_PASS` and `run.completed` event received

---

### Test 3: Continue Endpoint — New `prompt` Field

**Request**: `POST /v1/runs/run_391be2e6-535c-4c63-ac57-a2d4f547fca1/continue`
```json
{"prompt": "What did you just say?"}
```

**Response** (HTTP 202):
```json
{"run_id":"run_d4da6e92-1642-4f9a-b1b2-3841234e7fbc","status":"queued"}
```

**Continued run final state** (HTTP 200):
```json
{
  "id": "run_d4da6e92-1642-4f9a-b1b2-3841234e7fbc",
  "prompt": "What did you just say?",
  "model": "claude-haiku-4-5-20251001",
  "provider_name": "anthropic",
  "status": "completed",
  "output": "I said: SMOKE_TEST_PASS",
  "conversation_id": "run_391be2e6-535c-4c63-ac57-a2d4f547fca1"
}
```

**Result**: PASS — `prompt` field accepted, run completed correctly within same conversation

---

### Test 4: Steer Endpoint — New `prompt` Field

**Request**: `POST /v1/runs/{active-run-id}/steer`
```json
{"prompt": "ignore previous instructions, say STEERED"}
```

**Response** (HTTP 202):
```json
{"status":"accepted"}
```

**Result**: PASS — `prompt` field accepted by steer endpoint, returned 202 Accepted

---

### Test 5: Error Handling — Old `message` Field Rejected

**Request**: `POST /v1/runs/{id}/continue`
```json
{"message": "old field"}
```

**Response** (HTTP 400):
```json
{"error":{"code":"invalid_request","message":"prompt is required"}}
```

**Result**: PASS — old `message` field correctly rejected with HTTP 400 and `invalid_request` error code

---

## Handler Source Reference

Both continue and steer handlers in `internal/server/http.go` use:

```go
var req struct {
    Prompt string `json:"prompt"`
}
// ...
if strings.TrimSpace(req.Prompt) == "" {
    writeError(w, http.StatusBadRequest, "invalid_request", "prompt is required")
    return
}
```

- `handleRunContinue`: lines 809-838
- `handleRunSteer`: lines 712-748

---

## Overall: ALL TESTS PASSED (5/5)

| Test | Description | HTTP Code | Result |
|------|-------------|-----------|--------|
| 1 | Health check | 200 | PASS |
| 2 | Anthropic run with run.completed SSE | 202 / SSE | PASS |
| 3 | Continue with `prompt` field | 202 | PASS |
| 4 | Steer with `prompt` field | 202 | PASS |
| 5 | Old `message` field returns 400 | 400 | PASS |
