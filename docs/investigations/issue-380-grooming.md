# Grooming: Issue #380 — feat(profiles): add profile inheritance and extends semantics

## Already Addressed?
No — no inheritance or `extends` field exists anywhere in the profiles package.

The `Profile` struct (`internal/profiles/profile.go`) has `Meta`, `Runner`, `Tools`, and `MCPServers` sections but no `extends` or parent reference field. The loader (`internal/profiles/loader.go`) performs three-tier resolution (project > user > built-in) but does no merging — it returns the first matching profile as-is. No cycle detection, no override logic, no base resolution exists.

Searching the entire `internal/profiles/` tree for `extends`, `inherit`, or `parent` returns zero hits.

## Clarity
Clear — the issue specifies single inheritance, multi-field override precedence, cycle detection, and missing-base-profile detection. The semantics are unambiguous: a child profile inherits all fields from its base and can selectively override them.

One detail that would benefit from clarification: does `extends` stack (child inherits from parent, parent inherits from grandparent), or is it strictly one level deep? The issue says "single inheritance" which implies linear chains are allowed, but the phrasing could mean only one level deep. This should be nailed down before implementation.

## Acceptance Criteria
Partial — the issue body gives the high-level requirements (extends field, single inheritance, multi-field override precedence, cycle detection, missing base detection) but lacks:
- Explicit test cases for each AC (e.g., "cycle A→B→A returns ErrCyclicInheritance")
- Behavior when only some fields are overridden (are zero-value fields in the child treated as "not overriding" or "override to empty"?)
- Whether `AllowedTools` merges (union) or replaces (override wins)
- Whether `MCPServers` maps merge by key or replace entirely

## Scope
Atomic — this is confined to the profiles package (`loader.go`, `profile.go`). No runner, server, or tool changes required. The scope is well-bounded.

## Blockers
None — the profiles package is independent. No other in-flight issues appear to touch this area.

## Recommended Labels
well-specified, medium, needs-clarification (for zero-value override ambiguity and multi-level chain behavior)

## Effort
Medium — requires:
1. Adding `Extends string` to `ProfileMeta`
2. Post-load merge pass in `loadProfileWithDirs` (or a new `ResolveInheritance` function)
3. Cycle detection (hash-set or path tracking)
4. Field-by-field merge logic across `ProfileRunner`, `ProfileTools`, `MCPServers`
5. Tests for all edge cases

## Recommendation
needs-clarification — two ambiguities (zero-value override semantics, multi-level chain depth limit) must be resolved before implementation starts. Once clarified, this is straightforwardly implementable.

## Notes
- `internal/profiles/loader.go:43` (`loadProfileWithDirs`) is the ideal place to add the inheritance resolution pass — after the initial profile is loaded but before returning it.
- `internal/profiles/profile.go:20` (`Profile` struct) needs a new `Meta.Extends string` field.
- The existing `saveProfileToDir` will round-trip the `extends` field automatically since TOML encoding is struct-driven.
- Built-in profiles should be documented as valid inheritance bases (they already load via `loadBuiltinProfile`).
- The `ProfileValues` struct that `ApplyValues()` returns should not change — inheritance is a loading concern, not a runtime concern.
