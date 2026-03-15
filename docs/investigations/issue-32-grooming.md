# Issue #32 Grooming: conversation-persistence: Add token/cost tracking per conversation

## Summary
Track prompt tokens, completion tokens, and USD cost per conversation in the persistence layer.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented:
- `internal/harness/conversation_store.go`: `Conversation` struct has `PromptTokens`, `CompletionTokens`, `CostUSD` fields
- SQLite schema includes `prompt_tokens`, `completion_tokens`, `cost_usd` columns with migration
- `SaveConversationWithCost()` method implemented
- Runner accumulates tokens/cost across steps and passes to store

## Clarity Assessment
Clear.

## Acceptance Criteria
All met.

## Scope
Atomic.

## Blockers
None.

## Effort
Done.

## Label Recommendations
Recommended: `already-resolved`

## Recommendation
**already-resolved** — Close this issue.
