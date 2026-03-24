# Grooming: Issue #376 — fix(profiles): fail closed on unknown run_agent profiles

## Already Addressed?

No — The bug is confirmed present. In `internal/harness/tools/deferred/run_agent.go` lines 78–84:

```go
if loadErr != nil {
    // Non-fatal: if the profile is not found, use empty defaults.
    // This allows run_agent to work even without a profile system.
    p = &profiles.Profile{}
    p.Meta.Name = profileName
}
```

When `profiles.LoadProfile` or `profiles.LoadProfileFromUserDir` returns any error (including "profile not found"), the handler silently falls back to an empty `profiles.Profile{}` with zero-value runner config. This means:

- A caller passing `profile: "reseracher"` (typo) gets an unrestricted, zero-constraint run instead of an actionable error.
- There is no way for the caller to distinguish "profile applied successfully" from "profile silently ignored".
- The test suite in `run_agent_test.go` does not cover the unknown-profile path; `TestRunAgentTool_BasicExecution` and related tests pass `""` or valid built-in names only.

The `profiles.loadProfileWithDirs` function at line 79 of `loader.go` already returns `fmt.Errorf("profile %q not found", name)` for missing profiles, so the profile loader itself is correct. Only the `run_agent` handler swallows the error.

## Clarity

Clear — The goal is precise: return an actionable error when the named profile cannot be resolved from any tier (project, user-global, or built-in). The plan specifies keeping the built-in fallback only when the named built-in actually exists. The error message should be actionable (name the missing profile and suggest checking spelling or running list_profiles).

## Acceptance Criteria

Present in the plan file. Explicit criteria:

1. A failing test is written first: calling `run_agent` with an unknown profile name returns an error instead of succeeding silently.
2. The error shape is pinned (at minimum: error text contains the unknown profile name).
3. Three error paths are regression-tested: missing profile name, invalid profile name (path traversal), and unreadable profile file (parse error).
4. The built-in fallback is removed for the not-found case; a valid built-in or user/project profile is still applied when found.
5. The "no profile system configured" case (nil `profilesDir` and no built-ins matching) produces a clear error, not silent defaults.

## Scope

Atomic — The fix is a single conditional in `run_agent.go` lines 78–84: replace the silent fallback with an error return. The only additional work is adding the error-path tests to `run_agent_test.go`. No profile loader changes, no HTTP handler changes, no schema changes are required.

## Blockers

None — explicitly listed as having no dependencies in the plan. Can be implemented independently of #375.

## Recommended Labels

well-specified, small

## Effort

Small — The production change is 3–5 lines. Adding three error-path test cases adds another 30–50 lines. Total estimated effort is under 2 hours.

## Recommendation

well-specified

## Notes

- The current comment ("Non-fatal: if the profile is not found, use empty defaults. This allows run_agent to work even without a profile system.") reveals the original intent was ergonomic bootstrapping. The fix trades that convenience for safety. The plan explicitly prefers fail-closed behavior.
- `config.ValidateProfileName` is already called inside `loadProfileWithDirs` (loader.go line 44), so path-traversal protection is already in place at the loader level. The `run_agent` handler just needs to stop swallowing the returned error.
- After the fix, the test `TestRunAgentTool_BuiltinProfileGithub` should continue to pass since `github` is a valid built-in.
- A follow-up comment or docstring in the handler should document that profile resolution is now fail-closed and that callers should use `list_profiles` (once #377 exists) to enumerate available names.
