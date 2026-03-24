# Grooming: Issue #377 — feat(profiles): add read-only profile discovery APIs and tools

## Already Addressed?

Partial — The Go-level library functions exist but the external-facing surfaces (tool and HTTP endpoint) do not.

What is already implemented:
- `internal/profiles/loader.go`: `ListProfiles()` and `ListProfilesWithDirs()` return names across all three tiers with deduplication. Tested in `profile_test.go`.
- `internal/profiles/loader.go`: `LoadProfile(name)` and `LoadProfileFromUserDir(name, dir)` return a full `Profile` struct. Tested.
- `internal/profiles/profile.go`: `Profile` struct with `Meta`, `Runner`, `Tools`, and `MCPServers` fields.
- `internal/profiles/loader.go`: `SaveProfile` / `saveProfileToDir` exists (write path for #378).

What is missing:
- No `list_profiles` or `get_profile` tool in the deferred tool registry.
- No `/v1/profiles` or `/v1/profiles/{name}` HTTP endpoint in `internal/server/`.
- The comment in `internal/harness/tools/types.go` line 143 mentions "list_profiles" by name but no such tool is registered in `tools_default.go` — the `ProfilesDir` field is forwarded only to `RunAgentTool`.
- No HTTP handler for profile discovery in any `internal/server/http_*.go` file. A search for `handleProfile`, `list_profiles`, `get_profile`, and `/v1/profiles` across the server package returns no matches.

## Clarity

Clear — The plan specifies both surfaces to add (tool and HTTP), what data to return (metadata, runner config, tool allowlist, source tier), and the resolution order to pin (project over user over built-in). The source tier field is important because it tells callers whether the profile came from a project file, user-global file, or the embedded binary.

## Acceptance Criteria

Present in the plan file. Explicit criteria:

1. Failing handler tests are written first for list and get operations (HTTP and tool surfaces).
2. `list_profiles` tool: returns array of profile name strings, respecting three-tier resolution order.
3. `get_profile` tool: returns full profile fields (meta, runner, tool allowlist, source tier) for a named profile; errors clearly on unknown name.
4. HTTP endpoints `GET /v1/profiles` and `GET /v1/profiles/{name}` returning equivalent payloads.
5. Source tier (project/user/built-in) is included in the per-profile response.
6. Resolution order (project over user over built-in) is pinned in at least one test.
7. Tools are registered as `TierDeferred` in `NewDefaultRegistryWithOptions`.

## Scope

Slightly broad — Two distinct surfaces (tool and HTTP). However, both delegate to the same underlying loader functions, so the implementation is thin wiring. The plan recommends doing both in the same ticket rather than splitting them. This is acceptable given the shared backing logic.

## Blockers

The plan states "Ticket 2 recommended first, but not required" — meaning #376 (fail-closed) is a soft predecessor. Implementing `get_profile` before #376 is fixed means `get_profile` would expose the same ambiguity (does returning an empty profile for an unknown name expose the silent fallback?). However, since `get_profile` calls the loader directly (not through `run_agent`), it should independently error on unknown names. Technically no hard blocker.

## Recommended Labels

well-specified, medium

## Effort

Medium — Two new tool implementations, two new HTTP handlers, and tests for each. Estimated 4–8 hours. The profile struct serialization is already handled by the existing TOML loader; the work is wiring it into the tool/HTTP layers and deciding on the JSON response shape.

## Recommendation

well-specified

## Notes

- The `source` tier field requires a small addition: the loader currently returns a `*Profile` without indicating which tier resolved it. A lightweight approach is to return a `ProfileWithSource` wrapper struct (`Profile` + `Source string`) or add a `Source` field to `ProfileMeta`. The plan does not prescribe the struct shape, leaving this to the implementer.
- Tool registration: `list_profiles` and `get_profile` should be gated on `opts.ProfilesDir != "" || builtins_always_available`. Since built-ins are always embedded, these tools can always be registered.
- HTTP: the server struct in `internal/server/` does not currently hold a profiles loader reference. Wiring it in requires adding a `profilesDir` or `profileLoader` field to the `Server` struct and threading it through `NewServer`.
- The HTTP response for `GET /v1/profiles/{name}` should use the same JSON shape as the tool result for consistency and client reuse.
- Recommended to be implemented after #376 to avoid shipping a `get_profile` endpoint that inconsistently handles the unknown-name case.
