# Issue #141 Grooming: Implement OpenAI Responses API provider path

## Summary
Add a full provider code path for the OpenAI Responses API, including request/response mapping, streaming event handling, and tool call support.

## Already Addressed?
**NOT ADDRESSED** — All OpenAI provider code in `internal/provider/openai/` uses the Chat Completions API. No Responses API code path exists.

## Clarity Assessment
Excellent — detailed specification with high-risk areas flagged (assistant+tool-calls mapping, streaming event differences). Requires Context7 verification of struct field names before implementation.

## Acceptance Criteria
- `POST /v1/responses` request/response mapping
- Streaming event handlers for Responses API event types
- Tool call support via Responses API format
- Integration with existing runner and pricing

## Scope
Large — new provider code path.

## Blockers
Blocked by #139 (Context7 verification) and #140 (catalog flag).

## Effort
**Large** (3-4 days) — New provider path with streaming + tool calls + tests.

## Label Recommendations
Current: `enhancement`. Good.

## Recommendation
**well-specified** — Excellent spec. Must complete #139 verification first, then #140, then implement this.
