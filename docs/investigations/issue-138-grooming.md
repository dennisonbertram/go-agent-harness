# Issue #138 Grooming: Add user-facing HTTP endpoints for mid-run context status and compaction

## Summary
Expose `GET /v1/runs/{id}/context_status` and `POST /v1/runs/{id}/compact` HTTP endpoints so users can inspect and manage context usage mid-run.

## Already Addressed?
**PARTIALLY ADDRESSED** — Agent-invokable tools exist (commit 4d0f99e):
- `internal/harness/tools/context_status.go` — context statistics
- `internal/harness/tools/compact_history.go` — history compaction

However, HTTP endpoints do NOT exist in `internal/server/http.go`:
- `GET /v1/runs/{id}/context_status` — NOT FOUND
- `POST /v1/runs/{id}/compact` — NOT FOUND (existing compact endpoint is on conversations, not runs)

## Clarity Assessment
Clear. Well-specified with request/response formats.

## Acceptance Criteria
- `GET /v1/runs/{id}/context_status` returns token counts, step counts, cost
- `POST /v1/runs/{id}/compact` triggers compaction on an active run
- Both endpoints documented in API reference
- Tests with concurrent run access

## Scope
Atomic — two HTTP endpoints wrapping existing tool logic.

## Blockers
None.

## Effort
**Small** (2-4h) — HTTP handler wiring for two endpoints + tests.

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement. Agent tools exist; just need HTTP exposure.
