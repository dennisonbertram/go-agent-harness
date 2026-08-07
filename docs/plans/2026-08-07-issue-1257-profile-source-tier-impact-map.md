# Cross-Surface Impact Map: Issue #1257 profile source tiers

## Ownership and Data Flow

- `internal/profiles/loader.go` owns three-tier lookup and recursive
  inheritance. It is the only suitable owner for a resolver that reports the
  top-level defining tier alongside the effective profile.
- `internal/server/http_profiles.go`, deferred `get_profile`, and
  `internal/harness/profile_tool_manifest.go` currently re-probe the fallback
  loader. They consume the shared result instead.
- Search evidence: `rg 'LoadProfileWithDirs|SourceTier|source_tier' internal`
  identifies these three ad-hoc provenance probes and the existing list path.

## Surfaces

- Config/API/CLI/tools: HTTP profile detail and deferred tool/manifest report
  corrected provenance. List already enumerates concrete files and remains
  unchanged. No CLI wire change.
- Persistence/schema/cache: none. Fresh filesystem reads are retained; no
  cache is added.
- Lifecycle/security: resolution and validation errors retain existing paths;
  no authorization or filesystem trust boundary changes.
- Clients/provider/tool catalogs: existing `source_tier`/`profile_source_tier`
  fields become consistent across consumers. TUI/GUI source is unchanged.
- Deployment/compatibility: no migration or feature flag; rollback is a clean
  code revert.

## Regression Evidence

- Core: empty directories, project/user precedence, inherited-child tier, and
  fresh add/remove override.
- Server/deferred/manifest: built-in parity with empty directories plus
  override/inheritance coverage at their public boundaries.
- Acceptance: a real local API multi-turn run observes list/detail/tool/manifest
  tier parity before and after a project override.
