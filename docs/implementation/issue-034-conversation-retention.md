# Issue #34: Conversation Retention Policy / Auto-Cleanup

## Summary

Implemented a retention policy for the conversation persistence layer. Old conversations are automatically deleted based on a configurable age threshold. A `pinned` flag prevents specific conversations from being auto-deleted.

## Changes

### `internal/harness/conversation_store.go`
- Added `Pinned bool` field to `Conversation` struct.
- Added `DeleteOldConversations(ctx, olderThan time.Time) (int, error)` to `ConversationStore` interface.
- Added `PinConversation(ctx, convID string, pin bool) error` to `ConversationStore` interface.

### `internal/harness/conversation_store_sqlite.go`
- Added `pinned INTEGER NOT NULL DEFAULT 0` column to the `conversations` schema.
- Added idempotent migration for the `pinned` column (pre-existing databases are upgraded on startup).
- `ListConversations` now reads and exposes the `pinned` field.
- `SaveConversationWithCost` upsert preserves the `pinned` flag on update (does not reset it).
- Implemented `DeleteOldConversations`: deletes non-pinned rows with `updated_at < threshold`. A zero threshold is a no-op.
- Implemented `PinConversation`: sets/clears `pinned` flag; returns an error if the conversation does not exist.

### `internal/harness/conversation_cleaner.go` (new file)
- `ConversationCleaner` struct wraps a `ConversationStore` and a `retentionDays` value.
- `RunOnce(ctx)`: executes a single sweep. Returns `(0, nil)` when `retentionDays == 0` (disabled).
- `Start(ctx, interval)`: launches a background goroutine that does a startup sweep then ticks every `interval`. Exits when `ctx` is cancelled.

### `cmd/harnessd/main.go`
- Reads `HARNESS_CONVERSATION_RETENTION_DAYS` (default: 30).
- When conversation persistence is enabled and `retentionDays > 0`, creates and starts a `ConversationCleaner` with a 24-hour sweep interval.
- Cancels the cleaner context during graceful shutdown.

### Test mocks updated
- `failingConversationStore` in `runner_test.go` — added stub methods.
- `capturingConversationStore` in `runner_test.go` — added stub methods.
- `mockConversationStore` in `server/http_test.go` — added stub methods.

## Tests (TDD, written first)

9 new tests in `internal/harness/conversation_store_sqlite_test.go`:

| Test | What it verifies |
|------|-----------------|
| `TestConversationStoreDeleteOldConversations_DeletesOld` | Old conversations are deleted; recent ones survive |
| `TestConversationStoreDeleteOldConversations_SparesPinned` | Pinned conversations survive even past the threshold |
| `TestConversationStoreDeleteOldConversations_ZeroThreshold` | Zero `time.Time` threshold is a no-op |
| `TestConversationStoreDeleteOldConversations_NoneOldEnough` | Recent conversations are not deleted |
| `TestConversationStoreDeleteOldConversations_Concurrent` | Concurrent calls correctly delete exactly 10/20 conversations |
| `TestConversationStorePinConversation_TogglePin` | Pin/unpin round-trip and `Pinned` field in `ListConversations` |
| `TestConversationStorePinConversation_NotFound` | Pinning a non-existent conversation returns an error |
| `TestConversationCleanerRunOnce` | `RunOnce` deletes old conversations correctly |
| `TestConversationCleanerRunOnce_ZeroRetentionDisabled` | `retentionDays=0` disables cleanup entirely |

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `HARNESS_CONVERSATION_RETENTION_DAYS` | `30` | Delete conversations older than this many days. Set to `0` to disable. |

## Design decisions

- Zero threshold is always a no-op at both the SQL layer and the cleaner layer, preventing accidental mass deletion.
- The `pinned` column defaults to `0` so existing rows are treated as unpinned.
- The upsert in `SaveConversationWithCost` only updates non-pinned fields so a pin set via `PinConversation` is preserved across conversation saves.
- The sweep interval is 24 hours (matching a daily cadence); startup sweep runs immediately on server start.
