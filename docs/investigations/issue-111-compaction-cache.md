# Issue #111 / PR #111: In-Memory Conversation Cache Not Invalidated After POST /compact

## Issue Description

PR #111 ("Add conversation context compaction — Issue #33") adds a `POST /v1/conversations/{id}/compact` endpoint that replaces early conversation messages with a summary in the SQLite persistence layer. However, after a successful compaction, the Runner's in-memory conversation cache (`r.conversations[convID]`) is never updated or cleared. This means:

1. Subsequent reads via `ConversationMessages()` or `loadConversationHistory()` return the **pre-compaction** messages from the in-memory cache.
2. New runs that continue a compacted conversation will use the old, full-length history rather than the compacted one.
3. The compaction only becomes visible after a process restart (when the cache is empty and data is loaded fresh from SQLite).

This was identified by the Codex automated review comment on the PR (P1 severity).

## Relevant Files and Line Numbers

### The in-memory cache (the root of the bug)

- **`internal/harness/runner.go`, line 80**: The `Runner` struct holds the cache:
  ```go
  conversations map[string][]Message
  ```

- **`internal/harness/runner.go`, line 117**: Cache initialized in `NewRunner()`:
  ```go
  conversations: make(map[string][]Message),
  ```

### Where the cache is written (populated)

- **`internal/harness/runner.go`, lines 1360-1368**: After a run completes, messages are cached:
  ```go
  r.mu.Lock()
  r.conversations[convID] = msgs
  r.mu.Unlock()
  ```
  This is the only place `r.conversations[convID]` is written to.

### Where the cache is read (stale data returned)

- **`internal/harness/runner.go`, lines 1780-1800** (`ConversationMessages`): Returns cached messages if present, only falls through to the SQLite store on cache miss:
  ```go
  func (r *Runner) ConversationMessages(conversationID string) ([]Message, bool) {
      r.mu.RLock()
      msgs, ok := r.conversations[conversationID]
      if ok {
          r.mu.RUnlock()
          return append([]Message(nil), msgs...), true  // returns stale data
      }
      // ... falls through to store only on cache miss
  }
  ```

- **`internal/harness/runner.go`, lines 1739-1768** (`loadConversationHistory`): Same pattern -- checks cache first, only reads from store on miss:
  ```go
  func (r *Runner) loadConversationHistory(runID string) []Message {
      // ...
      msgs, found := r.conversations[convID]
      if found {
          r.mu.RUnlock()
          return append([]Message(nil), msgs...)  // returns stale data
      }
      // ... falls through to store
  }
  ```

### The compact endpoint (where invalidation is missing)

- **PR #111 diff, `internal/server/http.go`** (`handleCompactConversation`): The handler calls `store.CompactConversation()` to update SQLite, then calls `store.LoadMessages()` to get the new count for the response. **It never touches `r.conversations`**. The handler only has access to the `ConversationStore` via `s.runner.GetConversationStore()`, not to the Runner's internal cache.

### HTTP routing for the compact endpoint

- **PR #111 diff, `internal/server/http.go`** (`handleConversations`): Routes `POST /v1/conversations/{id}/compact` to `handleCompactConversation`.

## Root Cause Analysis

The `Runner.conversations` map serves as a write-through cache: it is populated when a run finishes (line 1368) and read preferentially over the SQLite store in both `ConversationMessages()` and `loadConversationHistory()`. The compact endpoint writes only to the SQLite store via `store.CompactConversation()` but never invalidates or updates the in-memory `r.conversations[convID]` entry.

Because both read paths check the in-memory map first and short-circuit on a cache hit, the compacted data in SQLite is never seen until the process restarts (at which point the map is empty and data is loaded fresh from the store).

The same bug pattern also exists in `handleDeleteConversation` (line 480-491 on main): it calls `store.DeleteConversation()` but does not remove the entry from `r.conversations`, so a deleted conversation's messages would still be served from the cache until restart.

## Which Cache Needs Invalidation and Where

**Cache**: `Runner.conversations map[string][]Message` (line 80 of `internal/harness/runner.go`)

**Where to invalidate**:

1. **After `CompactConversation` succeeds** in `handleCompactConversation`: Either delete the cache entry (forcing a reload from SQLite on next access) or replace it with the freshly loaded messages.

2. **After `DeleteConversation` succeeds** in `handleDeleteConversation`: Delete the cache entry entirely.

## Suggested Fix Approach

### Option A: Add a `InvalidateConversationCache` method to Runner (recommended)

Add a new public method on `Runner`:

```go
// InvalidateConversationCache removes a conversation from the in-memory cache,
// forcing the next read to load from the persistent store.
func (r *Runner) InvalidateConversationCache(conversationID string) {
    r.mu.Lock()
    delete(r.conversations, conversationID)
    r.mu.Unlock()
}
```

Then call it from:
- `handleCompactConversation` after `store.CompactConversation()` succeeds
- `handleDeleteConversation` after `store.DeleteConversation()` succeeds

This is the simplest fix. On the next read, `ConversationMessages()` or `loadConversationHistory()` will miss the cache and load fresh data from SQLite.

### Option B: Replace cache entry with fresh data after compact

Instead of invalidating, reload the compacted messages from the store and update the cache in-place:

```go
// In handleCompactConversation, after store.CompactConversation succeeds:
msgs, err := store.LoadMessages(r.Context(), convID)
if err == nil {
    r.runner.UpdateConversationCache(convID, msgs)
}
```

This avoids the extra SQLite read on the next access but requires a new `UpdateConversationCache` method and is slightly more complex.

### Recommendation

Option A (invalidation via delete) is preferred because:
- It is simpler and less error-prone
- It follows the existing pattern where cache misses fall through to the store
- It avoids race conditions from double-reads
- It fixes both the compact and delete endpoints with the same mechanism

### Tests Needed

1. **HTTP test**: POST /compact, then GET /messages on the same conversation (via `ConversationMessages()`), verify the response reflects the compacted state (not the pre-compaction state).
2. **HTTP test**: DELETE a conversation that was previously cached, then GET /messages, verify 404.
3. **Race test**: Concurrent compact + read to ensure no data races on the cache map.
