# Issue #28 Grooming: Stream thinking/reasoning content from thinking models

## Summary
Surface reasoning/thinking tokens from o-series models and Claude extended thinking to the CLI and SSE stream.

## Already Addressed?
**ALREADY RESOLVED** — Fully implemented via commits `2ef3349` and `1f481ef`. Evidence:
- `Reasoning` field added to `CompletionDelta` in core types
- `ReasoningEffort` config for o-series models
- Full stack: provider parsing → core types → SSE events → runner → demo-cli rendering
- 48 regression tests

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
