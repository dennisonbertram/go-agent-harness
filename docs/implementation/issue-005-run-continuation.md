# Issue #5: Run Continuation for Multi-Turn Conversations

## Summary

Implemented `POST /v1/runs/{runID}/continue` endpoint that allows clients to
send follow-up messages to completed runs, enabling true multi-turn conversation
loops without requiring a separate conversation persistence layer.

## Changes

### `internal/harness/runner.go`

- Added `ErrRunNotCompleted` sentinel error (alongside existing `ErrRunNotFound`).
- Added `Runner.ContinueRun(runID, message string) (Run, error)` method.

### `internal/server/http.go`

- Added route: `POST /v1/runs/{id}/continue` → `handleRunContinue`.
- Added `handleRunContinue` handler with full error mapping.

### `internal/harness/runner_continuation_test.go` (new)

Seven tests covering the harness layer:

| Test | What it validates |
|---|---|
| `TestContinueRunBasic` | Happy path: new run_id, shared conversation_id, correct output |
| `TestContinueRunNotFound` | `ErrRunNotFound` for nonexistent run |
| `TestContinueRunWhileRunning` | `ErrRunNotCompleted` when run is still in progress |
| `TestContinueRunEmptyMessage` | Validation error for empty message |
| `TestContinueRunCarriesConversationHistory` | Prior user+assistant messages appear in second provider request |
| `TestContinueRunConcurrencyRace` | Exactly 1 of N concurrent ContinueRun calls succeeds (race-safe) |
| `TestContinueRunFailedRun` | `ErrRunNotCompleted` for failed runs |

### `internal/server/http_continuation_test.go` (new)

Seven tests covering the HTTP layer:

| Test | What it validates |
|---|---|
| `TestContinueRunEndpointBasic` | 202 response, new run_id, run completes |
| `TestContinueRunEndpointNotFound` | 404 for nonexistent run |
| `TestContinueRunEndpointInvalidJSON` | 400 for malformed JSON body |
| `TestContinueRunEndpointEmptyMessage` | 400 for empty message field |
| `TestContinueRunEndpointMethodNotAllowed` | 405 for non-POST methods |
| `TestContinueRunEndpointRunningConflict` | 409 when run is still running |
| `TestContinueRunEndpointSSEResumedEvent` | Continuation run emits run.completed via SSE |

## Design Decisions

### Concurrency Safety

The key invariant is: exactly one goroutine can continue a given completed run.

The naive approach of "check status, unlock, create run, re-lock, re-check" has a
TOCTOU window where N goroutines can all see `RunStatusCompleted` before any of
them mutate it. The fix: within a single `mu.Lock()` acquisition:

1. Check run exists and status == completed.
2. Immediately stamp `state.run.Status = RunStatusRunning`.
3. Create the new run state.
4. Unlock.

Any concurrent `ContinueRun` that arrives after step 2 sees status != completed
and returns `ErrRunNotCompleted`.

### Source Run Status After Continuation

The source run's status is changed from `completed` to `running` during the
lock. This is intentional: it prevents double-continuation and provides an
accurate signal to callers. The source run is NOT further updated after this
point — it stays at `running`. This is acceptable for the MVP; future work
could add `RunStatusContinued` if needed.

### Conversation History

The continuation re-uses the same `conversation_id`. `loadConversationHistory`
looks up prior messages by conversation_id from `r.conversations` (in-memory)
or the SQLite store (if configured), so the full transcript is automatically
included in the new run's LLM requests.

### System Prompt Preservation

The source run's `staticSystemPrompt` is snapshotted and passed into the new
run's `runState`. If a prompt engine was used, the `promptResolved` pointer is
also carried over (though the engine is not re-invoked; the resolved static
prompt is used). This ensures the assistant's persona is consistent across turns.

## API Contract

```
POST /v1/runs/{runID}/continue
Content-Type: application/json

{ "message": "What did you mean by that?" }
```

**Success (202)**:
```json
{ "run_id": "run_2", "status": "queued" }
```

**Error responses**:
- `404 not_found` — run does not exist
- `409 run_not_completed` — run is still running, queued, or failed
- `400 invalid_request` — empty message or malformed JSON

## Test Results

```
ok  go-agent-harness/internal/harness   (all continuation tests pass, -race)
ok  go-agent-harness/internal/server    (all continuation tests pass, -race)
```

Full suite: all packages pass except the pre-existing `demo-cli` build failure.
