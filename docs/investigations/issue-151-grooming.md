# Issue #151 Grooming: Add user-facing HTTP endpoint for message summarization

## Summary
Add `POST /v1/summarize` endpoint that summarizes an arbitrary array of messages using the harness's LLM summarizer.

## Already Addressed?
**PARTIALLY ADDRESSED** — `MessageSummarizer` interface and `Runner.SummarizeMessages()` exist. `POST /v1/conversations/{id}/compact` reuses summarization logic. However, no standalone `/v1/summarize` endpoint for arbitrary message arrays exists.

## Clarity Assessment
Excellent — request/response format detailed, error cases explicit.

## Acceptance Criteria
- `POST /v1/summarize` accepts arbitrary message array (not tied to conversations)
- Returns summary text + token counts
- 501 when summarizer not configured
- LLM errors return 500
- Tests including LLM failure paths

## Scope
Atomic — can extract logic from existing compact endpoint.

## Blockers
None.

## Effort
**Small** (1-2 days) — Extract/reuse logic from `handleCompactConversation`.

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement.
