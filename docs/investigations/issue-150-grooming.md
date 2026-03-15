# Issue #150 Grooming: Add user-facing HTTP endpoint for Sourcegraph code search proxy

## Summary
Add `POST /v1/search/code` endpoint that proxies to Sourcegraph for semantic code search, returning results usable by users and automation.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/search/code` endpoint. Deferred `sourcegraph_search` tool exists in `internal/harness/tools/deferred/sourcegraph.go`.

## Clarity Assessment
Excellent — request/response format with examples, timeout handling, error cases.

## Acceptance Criteria
- `POST /v1/search/code` with query, limit, timeout params
- Proxies to `HARNESS_SOURCEGRAPH_ENDPOINT`
- 501 when Sourcegraph not configured
- Empty query returns 400
- Concurrent search safety

## Scope
Atomic.

## Blockers
None (Sourcegraph endpoint is optional config).

## Effort
**Small-Medium** (2-3 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement.
