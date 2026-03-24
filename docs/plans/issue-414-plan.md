# Issue #414 Implementation Plan — feat(workspace): add explicit workspace-backend selection

**Date**: 2026-03-23
**Branch**: issue-414-workspace-backend-selection (based on main)

## Summary

Profile.IsolationMode is fully defined but only flows through the `run_agent` tool for subagents — the parent run ignores it completely. RunRequest.WorkspaceType must fall back to Profile.IsolationMode when not explicitly set, and the runner must load the full profile at run time to extract workspace isolation policy.

## Current State vs What's Missing

**EXISTS:**
- `RunRequest.WorkspaceType` (types.go) — supports "local" and "worktree" only
- `Profile.IsolationMode` (profiles/profile.go) — supports "none", "worktree", "container", "vm"
- `provisionRunWorkspace()` in runner.go — uses RunRequest.WorkspaceType
- `validateWorkspaceType()` — rejects unknown types

**MISSING:**
1. **Precedence logic**: RunRequest.WorkspaceType does NOT fall back to Profile.IsolationMode
2. **Profile loading at run start**: Runner doesn't load Profile to extract isolation policy for the parent run
3. **Backend availability validation**: No check before provisioning if backend is actually available
4. **Fallback behavior**: No clear "fail closed" when requested backend is unavailable

## Implementation

### 1. `internal/harness/runner.go` — resolveWorkspaceType()

Add a new function that resolves the effective workspace type with precedence:

```go
func resolveWorkspaceType(req RunRequest, profile *profiles.Profile) string {
    // 1. Explicit RunRequest override wins
    if req.WorkspaceType != "" {
        return req.WorkspaceType
    }
    // 2. Profile isolation mode if profile loaded
    if profile != nil && profile.IsolationMode != "" && profile.IsolationMode != "none" {
        return profile.IsolationMode // "worktree", "container", "vm"
    }
    // 3. Server default (empty = local, no provisioning)
    return ""
}
```

### 2. Load Profile in `startRun` or `execute()`

When `RunRequest.ProfileName != ""`, load the full profile (not just MCP fields) before workspace provisioning, and pass it to `resolveWorkspaceType()`.

### 3. Extend `validateWorkspaceType()` to Check Availability

If WorkspaceType is "container" or "vm", check that the relevant provisioner is available (e.g., Docker daemon reachable for container). Return a clear error if not.

### 4. Extend RunRequest (if not already)

Verify RunRequest.WorkspaceType already covers "container" and "vm" values. If `knownWorkspaceTypes` only has "local" and "worktree", extend it to include "container" and "vm".

### 5. Trigger Adapter Integration

The trigger envelope (from #411) already has TenantID/AgentID. No new fields needed — the trigger adapter sets RunRequest fields normally, and the precedence logic handles backend selection transparently.

## Files to Modify

| File | Change |
|------|--------|
| `internal/harness/runner.go` | Add `resolveWorkspaceType()`, load profile before provisioning, use it in `provisionRunWorkspace()` call |
| `internal/harness/runner_test.go` | Add tests for precedence: explicit override, profile fallback, empty default |
| `internal/harness/types.go` | Extend `knownWorkspaceTypes` to include "container", "vm" if not already |

## Testing Strategy

**Write tests first:**
- `TestResolveWorkspaceType_ExplicitOverride` — RunRequest.WorkspaceType set → uses it regardless of profile
- `TestResolveWorkspaceType_ProfileFallback` — RunRequest empty, profile has IsolationMode="worktree" → uses worktree
- `TestResolveWorkspaceType_EmptyDefault` — both empty → returns ""
- `TestResolveWorkspaceType_ProfileNone` — profile.IsolationMode="none" → returns "" (no provisioning)
- `TestValidateWorkspaceType_ContainerUnavailable` — "container" requested but docker not available → clear error

**Regression:**
- Runs without profile or WorkspaceType → behavior unchanged
- Runs with WorkspaceType="local" → behavior unchanged
- Runs with WorkspaceType="worktree" → behavior unchanged

## Risk Areas

- Profile loading adds latency: profile loader should be fast (file-based, cached)
- Container/VM availability check may not be easy to mock in tests — use interface injection or build tag
- "container" and "vm" backend types may only work with symphd orchestrator config

## Commit Strategy
```
feat(#414): add workspace-backend selection via profile isolation-mode precedence
```
