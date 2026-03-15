# Issue #139 Grooming: Research: OpenAI Responses API support (required for gpt-5.x/codex models)

## Summary
Research the OpenAI Responses API (used by gpt-5.x and codex models) and produce a design doc for implementation. Existing research doc at `docs/investigations/openai-responses-api.md`.

## Already Addressed?
**RESEARCH COMPLETE** — `docs/investigations/openai-responses-api.md` exists with comprehensive research. However, the issue requires Context7 verification of struct field names and streaming event types before implementation begins.

## Clarity Assessment
Clear. Issue explicitly states: "MUST verify struct field names and streaming event names with Context7" before implementation.

## Acceptance Criteria
- Research doc verified with Context7
- Design doc approved before implementation of #140 and #141

## Scope
Research issue.

## Blockers
Blocks #140 and #141 implementation.

## Effort
**Small** (1-2h) — Context7 verification of existing research doc.

## Label Recommendations
Current: `enhancement`. Recommended: `enhancement`, `research`

## Recommendation
**well-specified** — Complete Context7 verification, update research doc, then close and unblock #140/#141.
