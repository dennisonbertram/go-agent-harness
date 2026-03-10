# Issue #39: Read-Only Agent Access Tools

## Summary

Added two new agent-facing tools that give an LLM read-only access to conversation history:
- `list_conversations` — returns lightweight conversation metadata (ID, title, timestamps, message count)
- `search_conversations` — runs full-text search over stored messages and returns snippets

Both tools are implemented as `TierCore`, `ParallelSafe: true`, `Mutating: false`, and are only activated when a `ConversationStore` is provided to `DefaultRegistryOptions`.

## Files Changed

### New Files

| File | Purpose |
|------|---------|
| `internal/harness/tools/core/conversations.go` | Tool constructors: `ListConversationsTool`, `SearchConversationsTool` |
| `internal/harness/tools/descriptions/list_conversations.md` | Embedded tool description for `list_conversations` |
| `internal/harness/tools/descriptions/search_conversations.md` | Embedded tool description for `search_conversations` |

### Modified Files

| File | Change |
|------|--------|
| `internal/harness/tools/types.go` | Added `ConversationReader` interface, `ConversationSummary`, `ConversationSearchResult` types; added `ConversationStore ConversationReader` and `EnableConversations bool` to `BuildOptions` |
| `internal/harness/tools_default.go` | Added `ConversationStore ConversationStore` to `DefaultRegistryOptions`; added `conversationStoreAdapter` struct; wires tools when store is configured |
| `internal/harness/tools/core/core_test.go` | 14 new tests: definition, parallel-safe, nil-store, success, empty, pagination, error propagation for both tools |
| `internal/harness/tools/descriptions/embed_test.go` | Added `list_conversations` and `search_conversations` to the known-descriptions list |
| `internal/harness/tools_contract_test.go` | Added `contractMockConversationStore` and two new contract tests: with/without store |

## Design Decisions

### ConversationReader interface (tools layer)

Rather than importing `harness.ConversationStore` directly into the tools package (which would create a circular import), a minimal `ConversationReader` interface is defined in `internal/harness/tools/types.go`:

```go
type ConversationReader interface {
    ListConversations(ctx context.Context, limit, offset int) ([]ConversationSummary, error)
    SearchConversations(ctx context.Context, query string, limit int) ([]ConversationSearchResult, error)
}
```

The `conversationStoreAdapter` in `tools_default.go` bridges `harness.ConversationStore` → `tools.ConversationReader`, converting `time.Time` fields to RFC3339 strings and mapping `SearchMessages` → `SearchConversations`.

### No raw transcript loading

Per the issue spec: "No raw transcript loading (context window risk)". Only metadata (`list_conversations`) and short FTS snippets (`search_conversations`) are exposed.

### Opt-in activation

The tools are absent from the registry when `ConversationStore` is nil. This preserves backward compatibility — existing callers of `NewDefaultRegistry` or `NewDefaultRegistryWithOptions` without a store see no change.

## Test Results

```
ok  go-agent-harness/internal/harness                  0.331s
ok  go-agent-harness/internal/harness/tools            9.159s
ok  go-agent-harness/internal/harness/tools/core       1.226s
ok  go-agent-harness/internal/harness/tools/deferred   1.859s
ok  go-agent-harness/internal/harness/tools/descriptions  1.478s
```

Race detector: all pass. Only pre-existing `demo-cli` build failure remains.

## New Tests (14 total)

### `list_conversations` (7 tests)
1. `TestListConversationsTool_Definition` — name, tier, non-nil handler/params
2. `TestListConversationsTool_ParallelSafe` — parallel-safe, non-mutating
3. `TestListConversationsTool_Handler_NilStore` — error when store is nil
4. `TestListConversationsTool_Handler_Success` — returns conversation metadata
5. `TestListConversationsTool_Handler_EmptyStore` — handles empty store
6. `TestListConversationsTool_Handler_CustomLimitOffset` — pagination works
7. `TestListConversationsTool_Handler_StoreError` — propagates store errors

### `search_conversations` (7 tests)
1. `TestSearchConversationsTool_Definition` — name, tier, non-nil handler/params
2. `TestSearchConversationsTool_ParallelSafe` — parallel-safe, non-mutating
3. `TestSearchConversationsTool_Handler_NilStore` — error when store is nil
4. `TestSearchConversationsTool_Handler_MissingQuery` — error for empty query
5. `TestSearchConversationsTool_Handler_Success` — returns snippets
6. `TestSearchConversationsTool_Handler_NoResults` — handles empty results
7. `TestSearchConversationsTool_Handler_StoreError` — propagates store errors

### Contract tests (2 tests)
1. `TestDefaultRegistryToolContractWithConversations` — tools appear when store provided
2. `TestDefaultRegistryToolContractWithoutConversations` — tools absent when store is nil
