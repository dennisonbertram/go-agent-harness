# Issue #17 Grooming: Conversation compaction for long-running unlimited-step sessions

## Summary
Add context compaction to prevent context window overflow in long-running sessions.

## Already Addressed?
**ALREADY RESOLVED** — Implemented as part of issue #33. Evidence:
- Merged via commit `120c1df`
- `POST /v1/conversations/{id}/compact` HTTP endpoint in `internal/server/http.go`
- `compact_history` agent-invokable tool (commit 4d0f99e)
- Database schema with `is_compact_summary` flag
- LLM-based summarization with memory system integration

## Clarity Assessment
Clear.

## Acceptance Criteria
Met by the #33 implementation.

## Scope
Effectively the same issue as #33.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close as duplicate of #33's implementation.
