# Issue #148 Grooming: Add user-facing HTTP endpoints for TODO list management

## Summary
Expose run-scoped TODO list (get/update) via HTTP endpoints so users watching via SSE can inspect and modify the agent's task list.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/runs/{id}/todos` endpoints. `todoStore` and `todoItem` types exist in `internal/harness/tools/deferred/todos.go` with mutex protection.

## Clarity Assessment
Excellent — 2 endpoints (GET/PUT) with schemas and error cases (400, 404).

## Acceptance Criteria
- `GET /v1/runs/{id}/todos` returns current TODO list
- `PUT /v1/runs/{id}/todos` replaces TODO list
- Valid statuses: `pending`, `in_progress`, `completed`
- Mutex-safe concurrent access
- ~12 test cases

## Scope
Atomic.

## Blockers
None.

## Effort
**Small** (1-2 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement. Simplest of the HTTP endpoint series.
