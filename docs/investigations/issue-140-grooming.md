# Issue #140 Grooming: Add catalog flag for models requiring OpenAI Responses API

## Summary
Add an `"api": "responses"` flag to the model catalog for models that require the OpenAI Responses API endpoint (gpt-5.x, codex-mini, etc.) and route requests accordingly.

## Already Addressed?
**NOT ADDRESSED** — No `api` field in model catalog. All models route to the same OpenAI Chat Completions endpoint.

## Clarity Assessment
Clear and atomic.

## Acceptance Criteria
- `api` field added to model catalog entries (value: `"responses"` or `"chat"`)
- ~6 models flagged: gpt-5, gpt-5-mini, codex-mini, etc.
- Routing logic in `internal/provider/openai/client.go` selects endpoint based on flag

## Scope
Atomic — catalog JSON + routing logic change.

## Blockers
Blocked by #139 (research must confirm correct models and endpoint paths).

## Effort
**Small** (2-4h) — JSON edits + routing logic + tests.

## Label Recommendations
Current: `enhancement`. Good.

## Recommendation
**well-specified** — Ready after #139 is verified. Small, clean change.
