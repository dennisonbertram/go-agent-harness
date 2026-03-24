# Issue #411 Implementation Plan — feat(server): source-agnostic trigger envelope and deterministic external thread routing

**Date**: 2026-03-23
**Branch**: issue-411-source-agnostic-trigger-envelope

## Summary

Add a normalized external-trigger envelope type and deterministic thread-ID routing so GitHub, Slack, and Linear can feed the harness through one shared server endpoint, reusing existing SteerRun/ContinueRun primitives.

## New Package: `internal/trigger/`

### `internal/trigger/types.go`
- `ExternalTriggerEnvelope` struct: Source, SourceID, RepoOwner, RepoName, ThreadID, Action, Message, TenantID, AgentID, Signature, RawBody
- `ExternalThreadID` string type
- `ExternalThreadValidator` interface: `ValidateSignature(ctx, env) error`

### `internal/trigger/thread_id.go`
- `DeriveExternalThreadID(source, repoOwner, repoName, threadID string) ExternalThreadID`
- Algorithm: SHA256(`source\x00repoOwner\x00repoName\x00threadID`) → `source:hexhash`
- If repoOwner/repoName empty: SHA256(`source\x00threadID`)
- Source normalized to lowercase before hashing

### `internal/trigger/validator.go`
- `ValidatorRegistry`: map[string]ExternalThreadValidator
- `NewValidatorRegistry() *ValidatorRegistry`
- `Register(source string, v ExternalThreadValidator)`
- `Get(source string) (ExternalThreadValidator, bool)`
- `GitHubValidator`, `SlackValidator`, `LinearValidator` structs with HMAC-SHA256 validation
- All validators fail closed: no configured secret → error

## New HTTP Handler: `internal/server/http_external_trigger.go`

### Endpoint: `POST /v1/external/trigger`

1. Decode `ExternalTriggerEnvelope` from JSON body
2. Validate signature via registry (fail closed → 401)
3. Derive `ExternalThreadID` from envelope fields
4. Lookup existing run: `ListRuns(RunFilter{ConversationID: threadID.String(), TenantID: ...})`
5. Route:
   - active run + action="steer" → `SteerRun(runID, message)` → 202
   - completed run + action="continue" → `ContinueRun(runID, message)` → 202
   - no run + action="start" → `StartRun(req with ConversationID=threadID)` → 202
   - mismatch → 409 Conflict
   - no run + action≠"start" → 404

### Responses
- 202: `{"status":"accepted"}` or `{"run_id":"...","status":"running"}`
- 401: `{"error":"invalid_signature"}`
- 404: `{"error":"no_thread_found"}`
- 409: `{"error":"run_state_mismatch"}`

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/trigger/types.go` | New — ExternalTriggerEnvelope, ExternalThreadID, interface |
| `internal/trigger/thread_id.go` | New — DeriveExternalThreadID with SHA256 |
| `internal/trigger/thread_id_test.go` | New — determinism, collision, edge cases |
| `internal/trigger/validator.go` | New — ValidatorRegistry + 3 source validators |
| `internal/trigger/validator_test.go` | New — HMAC validation tests |
| `internal/server/http_external_trigger.go` | New — handler + routing logic |
| `internal/server/http_external_trigger_test.go` | New — HTTP handler tests |
| `internal/server/http.go` | Add ValidatorRegistry field + register route |
| `cmd/harnessd/main.go` | Initialize validators from env/config |

## Testing Strategy

**Write tests before implementation**:
- `TestDeriveExternalThreadID_Deterministic` — same inputs same output
- `TestDeriveExternalThreadID_DifferentInputs` — different inputs different output
- `TestDeriveExternalThreadID_EmptyRepo` — stable when repo fields empty
- `TestGitHubValidator_ValidSignature`
- `TestGitHubValidator_InvalidSignature`
- `TestSlackValidator_ValidSignature`
- `TestSlackValidator_ExpiredTimestamp`
- `TestLinearValidator_ValidSignature`
- `TestHandleExternalTrigger_SteerActiveRun`
- `TestHandleExternalTrigger_ContinueCompletedRun`
- `TestHandleExternalTrigger_StartNewRun`
- `TestHandleExternalTrigger_InvalidSignature` → 401
- `TestHandleExternalTrigger_DirectAPIUnaffected` → existing endpoints work

**Regression pin**: Direct `/v1/runs` POST, `/v1/runs/{id}/steer`, `/v1/runs/{id}/continue` all unaffected.

## Risk Areas

- SteerRun/ContinueRun error types: must understand `ErrRunNotActive` / `ErrRunNotCompleted` return values
- Run lookup by ConversationID: verify `ListRuns` supports this filter
- Constant-time HMAC comparison: use `subtle.ConstantTimeCompare`
- Slack timestamp freshness: wall clock dependency (mock in tests)

## Commit Strategy

```
feat(#411): add source-agnostic trigger envelope and external thread routing
```
Single commit covering new package, server handler, and route registration.
