# Issue #9 Grooming: Client authentication and session management

## Summary
Add API key / JWT authentication to the harness HTTP server, including RBAC scopes, session tracking, and CLI integration.

## Already Addressed?
**NOT ADDRESSED** — No auth middleware exists in `internal/server/http.go`. All endpoints are open.

## Clarity Assessment
Well-specified design. However the scope is very large — API key auth, RBAC scopes, session management, and CLI token handling are all combined.

## Acceptance Criteria
Needs phased breakdown:
- Phase 1: Static API key auth via `Authorization: Bearer` header
- Phase 2: RBAC scope enforcement
- Phase 3: CLI token storage and integration

## Scope
Too large for a single PR. Recommend splitting into 3 sub-issues.

## Blockers
None hard.

## Effort
**Large** (20-30h total, ~5-8h per phase)

## Label Recommendations
Current: none. Recommended: `enhancement`, `security`

## Recommendation
**needs-clarification** — Agree on phased approach and create sub-issues before implementation begins.
