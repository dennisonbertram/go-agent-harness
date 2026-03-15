# Issue #33 Grooming: conversation-persistence: Add context compaction / summary messages

## Summary
Add context compaction to replace old messages with a summary, preventing context window overflow.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented:
- `POST /v1/conversations/{id}/compact` HTTP endpoint in `internal/server/http.go`
- `compact_history` agent-invokable tool (commit 4d0f99e) with strip/summarize/hybrid modes
- Database schema with `is_compact_summary` flag on messages
- `ConversationStore.CompactConversation()` interface + SQLite implementation
- LLM-based summarization integrated

Auto-compaction on thresholds is not implemented but that is future work beyond this issue's scope.

## Clarity Assessment
Clear.

## Acceptance Criteria
All core criteria met.

## Scope
Core feature complete.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close. File separate issue for auto-compaction on token thresholds if desired.
