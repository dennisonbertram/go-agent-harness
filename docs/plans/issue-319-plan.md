# Issue #319: API Key Scope Enforcement Plan

## Problem

API keys carry scopes (`runs:read`, `runs:write`, `admin`) but the backend
currently treats successful authentication as full authorization. Scope values
are stored but never consulted by route handlers.

## Authorization Matrix

| Endpoint                                        | Method       | Required Scope |
|-------------------------------------------------|--------------|----------------|
| GET /v1/runs                                    | GET          | runs:read      |
| POST /v1/runs                                   | POST         | runs:write     |
| GET /v1/runs/{id}                               | GET          | runs:read      |
| GET /v1/runs/{id}/events                        | GET          | runs:read      |
| GET /v1/runs/{id}/input                         | GET          | runs:read      |
| POST /v1/runs/{id}/input                        | POST         | runs:write     |
| GET /v1/runs/{id}/summary                       | GET          | runs:read      |
| POST /v1/runs/{id}/continue                     | POST         | runs:write     |
| POST /v1/runs/{id}/steer                        | POST         | runs:write     |
| GET /v1/runs/{id}/context                       | GET          | runs:read      |
| POST /v1/runs/{id}/compact                      | POST         | runs:write     |
| GET /v1/runs/{id}/todos                         | GET          | runs:read      |
| POST /v1/runs/replay                            | POST         | runs:write     |
| GET /v1/conversations/                          | GET          | runs:read      |
| GET /v1/conversations/search                    | GET          | runs:read      |
| DELETE /v1/conversations/{id}                   | DELETE       | runs:write     |
| GET /v1/conversations/{id}/messages             | GET          | runs:read      |
| GET /v1/conversations/{id}/runs                 | GET          | runs:read      |
| GET /v1/conversations/{id}/export               | GET          | runs:read      |
| POST /v1/conversations/{id}/compact             | POST         | runs:write     |
| POST /v1/conversations/cleanup                  | POST         | runs:write     |
| GET /v1/models                                  | GET          | runs:read      |
| GET /v1/agents                                  | GET          | runs:read      |
| GET /v1/subagents                               | GET          | runs:read      |
| POST /v1/subagents                              | POST         | runs:write     |
| DELETE /v1/subagents/{id}                       | DELETE       | runs:write     |
| GET /v1/providers                               | GET          | runs:read      |
| PUT /v1/providers/{name}/key                    | PUT          | admin          |
| POST /v1/summarize                              | POST         | runs:write     |
| GET /v1/cron/jobs                               | GET          | runs:read      |
| POST /v1/cron/jobs                              | POST         | runs:write     |
| GET /v1/cron/jobs/{id}                          | GET          | runs:read      |
| PUT /v1/cron/jobs/{id}                          | PUT          | runs:write     |
| DELETE /v1/cron/jobs/{id}                       | DELETE       | runs:write     |
| POST /v1/cron/jobs/{id}/pause                   | POST         | runs:write     |
| POST /v1/cron/jobs/{id}/resume                  | POST         | runs:write     |
| GET /v1/cron/jobs/{id}/executions               | GET          | runs:read      |
| GET /v1/skills                                  | GET          | runs:read      |
| GET /v1/skills/{name}                           | GET          | runs:read      |
| POST /v1/skills/{name}/verify                   | POST         | runs:write     |
| GET /v1/recipes                                 | GET          | runs:read      |
| GET /v1/search/code                             | GET          | runs:read      |
| GET /v1/mcp/servers                             | GET          | runs:read      |
| POST /v1/mcp/servers                            | POST         | admin          |
| DELETE /v1/mcp/servers/{name}                   | DELETE       | admin          |

## Scope Hierarchy

- `admin` implies `runs:write` and `runs:read` (superscope)
- `runs:write` implies `runs:read`

This means an admin key satisfies any scope check.

## Implementation Plan

### Phase 1: Extend auth context

In `internal/server/auth.go`:
- Add `contextKeyKeyScopes contextKey` constant
- Add `KeyScopesFromContext(ctx) []string` helper
- Inject scopes into context in `authMiddleware` after validation

### Phase 2: Add scope check helper

In `internal/server/auth.go`:
- Add `hasScope(ctx, required string) bool` function
  - If authDisabled, always return true (no-op)
  - `admin` satisfies any scope check
  - `runs:write` satisfies `runs:read`
  - Otherwise exact match
- Add `requireScopeMiddleware(scope string) func(http.Handler) http.Handler`
  - Returns 403 with `{"error": "insufficient_scope", "required": "<scope>"}` when scope check fails

### Phase 3: Apply scope checks in buildMux

Wrap each route with the appropriate scope middleware.

### Phase 4: Tests

In `internal/server/auth_scope_test.go` (new file, external test package):
- Table-driven tests for all scope combinations
- Verify 403 response body structure
- Verify unauthenticated mode bypasses scope checks

## Error Response Format

```json
{
  "error": "insufficient_scope",
  "required": "runs:write"
}
```

Status: 403 Forbidden
