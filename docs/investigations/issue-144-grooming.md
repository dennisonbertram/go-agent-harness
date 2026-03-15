# Issue #144 Grooming: Add user-facing HTTP endpoints for skill management

## Summary
Expose skill management (list, get, verify) via HTTP endpoints for pre-session setup and automation.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/skills` endpoints in `internal/server/http.go`. `SkillLister` and `SkillVerifier` interfaces exist (`internal/harness/tools/types.go:214-230`).

## Clarity Assessment
Excellent — 6 endpoints specified with request/response schemas.

## Acceptance Criteria
- 6 skill endpoints: list, get, update verification status, list packs, get pack, install pack
- Concurrent safety
- ~20 test cases

## Scope
Atomic.

## Blockers
None.

## Effort
**Medium** (3-4 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement. All skill infrastructure exists.
