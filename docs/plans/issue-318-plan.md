# Issue #318: Derive Effective Tenant from Auth Context

## Problem

When auth is enabled, the backend validates the API key but still trusts
caller-supplied `tenant_id` values. A client with a valid API key can:

- Create runs under another tenant (POST /v1/runs with mismatched tenant_id)
- List another tenant's runs (GET /v1/runs?tenant_id=other)
- List another tenant's conversations (GET /v1/conversations/?tenant_id=other)

## Root Cause

`authMiddleware` validates the Bearer token and injects the real tenant ID into
the request context (`contextKeyTenantID`). However, the HTTP handlers then read
the tenant ID from the request body or query parameters — bypassing the
validated context value entirely.

Affected handlers:
- `handlePostRun` (http.go ~417): passes `req.TenantID` from JSON body to `runner.StartRun()`
- `handleListRuns` (http.go ~445): reads `?tenant_id=` from query string
- `handleListConversations` (http.go ~1118): reads `?tenant_id=` from query string

## Fix Design

### New Helper: `effectiveTenantID(r *http.Request, requestTenantID string) (string, error)`

Added to `internal/server/auth.go`.

Logic:
1. If auth is disabled (`s.authDisabled`) or no store is configured: return
   requestTenantID as-is (preserves existing no-auth behavior).
2. If auth is enabled: extract `authTenantID` from context via
   `TenantIDFromContext(r.Context())`.
3. If `requestTenantID == ""`: return `authTenantID` (server fills from auth).
4. If `requestTenantID == authTenantID`: return `authTenantID` (matching, allowed).
5. If `requestTenantID != authTenantID`: return error (mismatch, rejected with 400).

The helper is a method on `*Server` so it can check `s.authDisabled` and
`s.runStore`.

### Handler Changes

**handlePostRun**: After JSON decode, call `effectiveTenantID(r, req.TenantID)`.
On error, return 400. On success, overwrite `req.TenantID` with the effective
value before passing to `runner.StartRun()`.

**handleListRuns**: Replace direct query param read with `effectiveTenantID(r,
r.URL.Query().Get("tenant_id"))`. On error, return 400. Use effective tenant in
filter.

**handleListConversations**: Same pattern as handleListRuns, but applied to the
`filter.TenantID` field.

**handleReplayFork** (http_replay.go): Populate TenantID from auth context in the
`StartRun` call (no mismatch possible since there is no user-supplied tenant here;
just fill from context).

**handleRunContinue**: ContinueRun operates on an existing run by ID; the run
already has its tenant baked in. No tenant input from the request. No change
needed.

## Acceptance Criteria

- [ ] POST /v1/runs: authenticated request with no tenant_id -> server fills from auth context
- [ ] POST /v1/runs: authenticated request with matching tenant_id -> allowed
- [ ] POST /v1/runs: authenticated request with mismatching tenant_id -> 400 rejected
- [ ] GET /v1/runs?tenant_id=other -> returns 400 when authenticated as different tenant
- [ ] GET /v1/conversations/?tenant_id=other -> returns 400 when authenticated as different tenant
- [ ] Unauthenticated/dev mode behavior unchanged

## Test Plan

File: `internal/server/http_auth_test.go` (external test package `server_test`)

Table-driven tests:
- `TestEffectiveTenantID_PostRun` -- covers POST /v1/runs cases
- `TestEffectiveTenantID_ListRuns` -- covers GET /v1/runs cases
- `TestEffectiveTenantID_ListConversations` -- covers GET /v1/conversations/ cases
- `TestEffectiveTenantID_AuthDisabled` -- verifies no-auth passthrough

## Security Notes

- The helper is deliberately simple: any mismatch is rejected, not silently
  overwritten, so the client gets clear feedback rather than a silent data leak.
- Auth-disabled path is identical to before (no behavioral change for dev/test).
- The 400 response does not leak which tenant_id the server would have used.
