# Issue #27 Grooming: Shell escaping breaks curl-based manual testing scripts

## Summary
Special characters in prompts break when sent via curl due to shell escaping issues.

## Already Addressed?
**ALREADY RESOLVED** — Fixed via commit `e87b06c`. Comprehensive integration tests with 13 special character test cases in `internal/server/http_special_chars_test.go`. Round-trip correctness validated.

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
