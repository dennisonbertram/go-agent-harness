# Issue #33: Conversation Context Compaction

## Summary

Added context compaction to the conversation persistence layer. Users and clients can now summarize early conversation history into a single "compact summary" message, reducing token usage for long-running conversations.

## Branch

`issue-33-context-compaction`

## Changes

### 1. `internal/harness/types.go`

Added `IsCompactSummary bool` field to the `Message` struct:

```go
type Message struct {
    ...
    IsCompactSummary bool `json:"is_compact_summary,omitempty"`
}
```

### 2. `internal/harness/conversation_store.go`

Added `CompactConversation` to the `ConversationStore` interface:

```go
CompactConversation(ctx context.Context, convID string, keepFromStep int, summary Message) error
```

**Semantics:**
- Messages with `step >= keepFromStep` are retained (renumbered starting at 1)
- The summary message is inserted at step 0
- `keepFromStep=0` retains all messages, prepending the summary
- `keepFromStep > max_step` keeps no existing messages (only summary remains)
- Returns error if conversation does not exist or `keepFromStep < 0`

### 3. `internal/harness/conversation_store_sqlite.go`

- Added `is_compact_summary` column to `conversation_messages` table schema
- Added idempotent migration (`ALTER TABLE ... ADD COLUMN is_compact_summary`) for existing databases
- Updated `SaveConversation` to persist `IsCompactSummary`
- Updated `LoadMessages` to read and populate `IsCompactSummary`
- Implemented `CompactConversation`:
  - Runs in a single transaction
  - Verifies conversation exists (returns error if not)
  - Queries and loads messages with `step >= keepFromStep`
  - Deletes all existing messages
  - Reinserts: summary at step 0, then retained messages renumbered
  - Updates `msg_count` and `updated_at` on the conversations row

### 4. `internal/server/http.go`

Added HTTP endpoint:

```
POST /v1/conversations/{id}/compact
```

Request body:
```json
{
  "keep_from_step": 4,
  "summary": "Summary of the first 4 messages.",
  "role": "system"
}
```

- `keep_from_step`: integer >= 0, messages from this step index onwards are kept
- `summary`: required, non-empty string
- `role`: optional, defaults to `"system"`

Response (200 OK):
```json
{
  "compacted": true,
  "message_count": 3
}
```

Error responses: 400 (bad request), 404 (conversation not found), 501 (no store configured).

### 5. Test files

- `internal/harness/conversation_compact_test.go`: 9 tests covering:
  - Basic compaction (keep steps >= 4 from 6-message conversation)
  - `IsCompactSummary` flag round-trip persistence
  - `keepFromStep` beyond the end (only summary remains)
  - `keepFromStep=0` (all messages kept, summary prepended)
  - Non-existent conversation (error)
  - Negative `keepFromStep` (error)
  - `msg_count` update after compaction
  - Concurrent safety (5 goroutines)
  - Migration idempotency

- `internal/server/http_compact_test.go`: 10 tests covering:
  - Basic HTTP compact (end-to-end with SQLite store)
  - No store configured (501)
  - Invalid JSON (400)
  - Empty summary (400)
  - Negative `keep_from_step` (400)
  - Non-existent conversation (404)
  - Wrong HTTP method (405)
  - Default role ("system")
  - Custom role
  - Concurrent requests

- `internal/harness/runner_test.go`: Added `CompactConversation` stub to `failingConversationStore`

## Test Results

```
ok  go-agent-harness/internal/harness    (19 compaction tests pass)
ok  go-agent-harness/internal/server     (10 HTTP compaction tests pass)
```

All tests pass. Only pre-existing `demo-cli` build failure remains (unrelated to this change).

Race detector: no issues.
