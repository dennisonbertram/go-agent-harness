# Grooming: Issue #378 — feat(profiles): add create/update/delete/validate profile management surfaces

## Already Addressed?

Partial — The write primitives exist at the library level but no management surface is exposed.

What is already implemented:
- `internal/profiles/loader.go`: `SaveProfile(p *Profile)` writes atomically to `~/.harness/profiles/` using a temp-then-rename pattern (lines 144–178). Tested in `profile_test.go`.
- `internal/profiles/loader.go`: `saveProfileToDir(p *Profile, dir string)` is the internal implementation, also tested.
- `internal/config/config.go`: `ValidateProfileName(name)` guards against path traversal and empty names (line 507–511). Used by the loader.
- Profile TOML encode/decode is implemented via the BurntSushi/toml library.

What is missing:
- No `create_profile`, `update_profile`, `delete_profile`, or `validate_profile` tool.
- No `POST /v1/profiles`, `PUT /v1/profiles/{name}`, `DELETE /v1/profiles/{name}`, or `POST /v1/profiles/validate` HTTP endpoint.
- No `DeleteProfile` function in `internal/profiles/loader.go` — deleting a profile file requires `os.Remove`, which is not yet wrapped with the appropriate safeguards (built-in protection, path validation).
- No validation function that checks a profile struct for logical correctness beyond name format (e.g., verifying that `allow` tool names are valid, that `max_steps` is non-negative, etc.).

## Clarity

Clear — The plan specifies the four operations, the atomic-write requirement, the built-in-immutability question (to be decided by the implementer), and the security properties to pin in tests. The plan explicitly calls out path traversal, invalid names, partial writes, and built-in protection as required regression targets.

## Acceptance Criteria

Present in the plan file. Explicit criteria:

1. Failing tests are written first for create, update, delete, and validate operations.
2. `create_profile` / `POST /v1/profiles`: creates a user-global profile file; rejects invalid names (path traversal, empty, reserved); uses atomic write.
3. `update_profile` / `PUT /v1/profiles/{name}`: updates a user-global or project-level profile; built-in profiles are either immutable or the policy is documented.
4. `delete_profile` / `DELETE /v1/profiles/{name}`: removes a user-global or project-level profile; refuses to delete built-ins.
5. `validate_profile` / `POST /v1/profiles/validate`: parses and validates a profile body without writing it; returns field-level validation errors.
6. Path traversal attacks are regression-tested and rejected.
7. Partial-write safety (atomic rename) is tested or explicitly justified.
8. Built-in profile immutability decision is documented and pinned in tests.

## Scope

Slightly broad — Four operations across two surfaces. However, all share the same backing library (`profiles.SaveProfile`, `os.Remove` + validation), so the implementations are structurally similar. Splitting further would create too much ceremony. This is acceptable as a single ticket.

## Blockers

Blocked on #377 (read surfaces must exist before write surfaces are added, per plan dependency graph). Implementing create/update without a working `get_profile` means there is no way for callers to verify what was written.

## Recommended Labels

well-specified, medium, blocked

## Effort

Medium — Four operations, two surfaces each, plus security regression tests. The library-level plumbing is already in place (`SaveProfile`, `ValidateProfileName`); the work is wiring into tool/HTTP layers and adding the missing `DeleteProfile` function with built-in protection. Estimated 6–10 hours.

## Recommendation

well-specified (blocked on #377)

## Notes

- A `DeleteProfile(name string, allowedDirs ...string)` function needs to be added to `internal/profiles/loader.go`. It should: (1) validate the name, (2) resolve to user-global or project dir only (never the embedded FS), (3) return an error if the resolved path does not exist, (4) return a distinct error if the name matches a built-in (to enforce immutability).
- The "should built-ins be immutable?" question matters for both update and delete. The safest default is immutable: user-global or project profiles shadow built-ins but do not modify the embedded binary. The plan recommends deciding this explicitly and pinning it.
- The `validate_profile` surface is valuable for agent-authored profiles: it allows a parent agent to verify a profile before saving it, preventing malformed TOML or policy-violating configurations from landing on disk.
- The HTTP server will need the same `profilesDir` wiring as #377.
- `config.go` line 313 already has profile-not-found error handling in `LoadConfig`, which confirms the pattern for how profile load failures should be surfaced to API callers.
