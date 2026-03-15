# Issue #236 Grooming: feat(config) — Deterministic Config Propagation to Subagent Workspaces

## Summary
Spawned subagent harness instances currently start with hardcoded defaults instead of inheriting the parent's configuration (API keys, model, forensics flags, cost settings). This is a silent misconfiguration hazard at scale.

## Already Addressed?
**No.**
- `workspace.Options` has only `[ID, RepoURL, BaseDir, Env]` — no `ConfigTOML` field
- `internal/config/config.go` covers ~5 of ~15 RunnerConfig fields in TOML
- Feature flags (AutoCompactEnabled, TraceToolDecisions, CaptureRequestEnvelope, AuditTrailEnabled, etc.) exist only in RunnerConfig struct — not in TOML
- `symphd`'s `buildWorkspaceFactory()` does not populate `Options.Env` with parent config
- No `WorkspaceRunnerConfig` typed serialization type

## Clarity
**3.5/5** — Problem clearly identified; four design options (A–D) presented but not decided. Secret propagation mechanism unclear.

## Acceptance Criteria
**Explicit:**
- [ ] All RunnerConfig feature flags have a TOML key in `internal/config/config.go`
- [ ] `workspace.Options` has a `ConfigTOML string` field written by each Provision implementation
- [ ] `symphd.Dispatcher` populates `ConfigTOML` from a typed `WorkspaceRunnerConfig` at dispatch time
- [ ] Container workspace passes API keys via `opts.Env` (not written to disk)
- [ ] Unit tests cover config round-trip and per-workspace injection

Note: A comment on the issue proposes replacing `build_harness_config` + `spawn_subagent` with a single `run_agent` tool. Issue description should be updated to reflect this.

## Scope
**Atomic** — Tightly scoped to config propagation. Explicitly excludes full secret management, multi-tenant isolation, and remote reconfiguration.

## Blockers
None hard. Must decide between design options A–D before implementation.

## Recommended Labels
`enhancement`, `well-specified`, `medium`

## Effort
**Medium** (2–3 days)
- Extend config.go to ~15 RunnerConfig fields: 0.5d
- Define WorkspaceRunnerConfig struct + validation: 0.5d
- Add ConfigTOML to workspace.Options + per-type injection: 1d
- Wire symphd.Dispatcher: 0.5d
- Tests: 0.5d

## Recommendation
**well-specified** — Ready for implementation. Before starting: decide design option (B+A hybrid recommended), clarify secret detection strategy, update issue description to reflect `run_agent` tool approach.
