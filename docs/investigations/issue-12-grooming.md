# Issue #12 Grooming: High priority: remove premature harnesscli timeout for streamed runs

## Summary
The CLI client has a hardcoded timeout that cuts off long-running streaming runs before completion.

## Already Addressed?
**ALREADY RESOLVED** — Fixed in commits beb766c and 53b65f0. Regression tests in `main_timeout_test.go`. Implementation doc: `docs/implementation/issue-012-streaming-client-timeout.md`.

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
