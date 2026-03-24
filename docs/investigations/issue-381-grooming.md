# Grooming: Issue #381 — feat(profiles): expose resolved tool manifests for profile-backed runs

## Already Addressed?
Partial — the registry has `DefinitionsForRun` and `DeferredDefinitions` methods that return tool definition slices, but there is no API endpoint or tool that exposes a "resolved active manifest" (the per-run union of core + activated deferred tools, tagged with their tier source).

Specifically:
- `Registry.DefinitionsForRun(runID, tracker)` (`internal/harness/registry.go:145`) returns core tools plus activated deferred tools for a given run — this is the resolved manifest logic.
- `Registry.DeferredDefinitions()` (`internal/harness/registry.go:165`) returns all deferred tools regardless of activation state.
- Neither is exposed as an HTTP endpoint or as a callable tool.
- `tools_contract_test.go` tests tool presence but not a "manifest inspection" surface.
- No `GET /v1/runs/{id}/tools` or `GET /v1/profiles/{name}/tools` endpoint exists in `internal/server/http.go`.

## Clarity
Unclear in one dimension — the issue says "tool/API to return the resolved active manifest for a profile or run, including deferred/core source information." This could mean:
1. An HTTP endpoint (e.g., `GET /v1/runs/{id}/tools`) returning the live per-run manifest
2. A static introspection endpoint for a profile name (e.g., `GET /v1/profiles/{name}/tools`) independent of a run
3. A new agent-callable tool (e.g., `list_active_tools`) so an LLM can inspect its own manifest

All three are plausible. The issue references both `tools_default.go` (the registry builder) and `tools_contract_test.go` (the test contract), but the desired user-facing surface is not stated.

## Acceptance Criteria
Missing — the issue body does not specify:
- Whether this is an HTTP endpoint, a new tool, or both
- What the response schema looks like (tool name, description, tier, tags, source profile?)
- Whether "for a profile" means statically (from profile config) or dynamically (from an active run using that profile)
- Whether tier/source annotation (core vs deferred, which profile tier it came from) is required in the response

## Scope
Too broad as written — mixing "for a profile" (static) and "for a run" (live) into a single issue makes it harder to implement atomically. These are distinct surfaces:
- Static profile manifest: requires profile loading + tool registry introspection (no run needed)
- Live run manifest: requires run state access from the runner

Recommend splitting into two issues or picking one surface to deliver first.

## Blockers
None at the infrastructure level — `DefinitionsForRun` already computes the correct set. The work is wiring it to an HTTP surface.

## Recommended Labels
needs-clarification, medium, well-specified (once split)

## Effort
Medium — adding an HTTP endpoint is straightforward (`handleRunByID` in `http.go` could handle `GET /v1/runs/{id}/tools`). The harder part is deciding the schema and whether tier/source annotation needs a new registry method (currently `registeredTool` has `tier` and `tags` but no public accessor).

## Recommendation
needs-clarification — the desired surface (HTTP endpoint vs tool, static vs live) must be specified before any implementation starts. Once clarified it is a medium-sized addition.

## Notes
- `internal/harness/registry.go:14` shows `registeredTool` has `tier` and `tags` fields, but these are unexported. A new `DefinitionsWithMeta()` method returning a richer struct would be needed to expose tier/source information.
- `internal/harness/tools_contract_test.go` already has patterns for validating tool presence; the new endpoint test can follow the same pattern.
- The `DefinitionsForRun` method at line 145 is the correct computation; it just needs to be wired to a public HTTP surface.
- If the intent is only a static profile manifest (no run), the implementation does not require a live runner at all — just profile loading + `NewDefaultRegistryWithOptions` with profile-specified tool allow-list applied.
