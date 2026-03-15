# Issue #37 Grooming: conversation-persistence: Add full-text search on message content

## Summary
Add SQLite FTS5-backed full-text search on conversation message content, exposed via `GET /v1/conversations/search?q=...`.

## Already Addressed?
**PARTIALLY ADDRESSED** — The HTTP route `GET /v1/conversations/search?q=...` is already registered in `internal/server/http.go`, but the underlying FTS5 table and `SearchMessages()` implementation are not yet implemented. The endpoint exists as a stub.

## Clarity Assessment
Very clear. Requirements are explicit with request/response format.

## Acceptance Criteria
- FTS5 virtual table `messages_fts` created in schema migration
- `SearchMessages(query, limit)` method on ConversationStore
- HTTP handler wires query → SearchMessages → JSON response
- Returns conversation_id, message snippet, timestamp, role
- Case-insensitive search
- Limit parameter (default 20, max 100)
- Empty query returns 400

## Scope
Atomic.

## Blockers
None.

## Effort
**Small** (1.5-2h) — FTS5 schema + SearchMessages implementation + handler wiring.

## Label Recommendations
Current: `enhancement`. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement. HTTP route already exists; just needs FTS5 table + implementation.
