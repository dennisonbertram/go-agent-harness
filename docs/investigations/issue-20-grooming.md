# Issue #20 Grooming: Layered Configuration Cascade

## Summary
Replace flat environment variable configuration with a 6-layer config stack (defaults → user global → project → profile → CLI override → cloud constraints) supporting TOML files, named profiles, and team/enterprise policies.

## Evaluation
- **Clarity**: Clear — Problem statement is concrete (env vars don't scale), design clearly shows 6 layers with priority precedence, examples provided for each layer, Go interface sketch is reasonable.
- **Acceptance Criteria**: Partial — Missing explicit tests. Should specify: "layers merge correctly by priority", "CLI flag overrides profile", "invalid TOML fails gracefully", etc.
- **Scope**: Atomic — Single concern: config resolution from multiple sources. May benefit from extraction of profile-specific work, but core scope is well-bounded.
- **Blockers**: None — Independent of other issues.
- **Effort**: Medium — Requires: TOML loader, layer merging logic with priority, CLI flag integration (--profile), integration into RunRequest defaults, tests. Cloud constraints endpoint is deferred ("future").

## Recommended Labels
well-specified, medium

## Missing Clarifications
1. Should invalid/missing config files error or silently use next layer? (e.g., missing ~/.harness/config.toml OK, but malformed TOML fails?)
2. For "cloud constraints", what is the protocol? HTTP endpoint on server? Baked into binary? How does local config know where to fetch?
3. Config file location — is ~/.harness/ absolute or configurable? What about Windows (AppData)?
4. Should `ConfigStack.Resolve()` support nested keys (e.g., `cost.max_per_run_usd`) or flat only?
5. How to handle secret values (API keys) in config files? .gitignore guidance, env var substitution, encrypted?

## Notes
- Inspired by Codex CLI's ConfigLayerStack — reference doc exists at `docs/research/codex-cli-architecture.md` §6.3
- Integration point unclear: RunRequest defaults need wiring — where does this happen? In server StartRun endpoint?
- Named profiles feature (--profile thorough) is useful but could be follow-up work; core stack is sufficient for MVP
- Test coverage: must test layer priority (higher layers override lower), invalid configs, missing files
