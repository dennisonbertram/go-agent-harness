# Plan: Backend OpenRouter Model Discovery

## Context

- Problem: backend provider/model resolution and `GET /v1/models` still rely mostly on the static model catalog, while OpenRouter exposes a large live slug space that cannot be maintained exhaustively in `catalog/models.json`.
- User impact: dynamic OpenRouter slugs can be selected in some paths but remain brittle in backend routing and model listing, which creates mismatches between startup, runtime, and UI behavior.
- Constraints:
  - Keep the design additive over the existing catalog.
  - Do not block startup on live fetches.
  - Limit provider-specific behavior to OpenRouter for now.
  - Preserve existing static-provider behavior.

## Scope

- In scope:
  - Add a backend discovery subsystem with TTL caching.
  - Fetch and decode OpenRouter live models.
  - Merge live-discovered OpenRouter models with static metadata.
  - Use discovery for runtime provider resolution and `GET /v1/models`.
  - Add focused tests and update docs/logs/indexes.
- Out of scope:
  - Multi-provider live discovery architecture.
  - Reworking TUI model enrichment behavior.
  - Replacing the static catalog as the source of pricing and aliases.

## Test Plan (TDD)

- New failing tests to add first:
  - OpenRouter fetch decode.
  - Discovery TTL cache hit/refresh behavior.
  - `GET /v1/models` merged live + static listing.
  - Dynamic runtime routing through OpenRouter discovery.
  - Safe fallback to cache/static when discovery fails.
- Existing tests to update:
  - Catalog registry tests that currently rely on hardcoded OpenRouter slash heuristics.
  - Server model endpoint tests to validate merged/fallback responses.
- Regression tests required:
  - Provider resolution remains unchanged for static providers.
  - `GET /v1/models` still returns deterministic, non-null JSON when discovery is unavailable.

## Cross-Surface Impact Map

- Required when the task touches provider/model flows, gateway routing, model catalogs, API-key management, or server/TUI provider plumbing.
- Create a one-page impact map from `IMPACT_MAP_TEMPLATE.md` covering:
  - Config
  - Server API
  - TUI state
  - Regression tests
- A blank heading is a warning. Write `None` with rationale when a surface is truly unaffected.
- Impact map for this task: `docs/plans/2026-03-25-openrouter-backend-discovery-impact-map.md`

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] For provider/model flow work, add or update the one-page impact map before implementation.
- [ ] Write failing tests first.
- [ ] Review ownership/copy semantics for exported or state-storing types when mutable fields cross boundaries.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: live discovery introduces request-path instability or latency.
- Mitigation: isolate live fetch behind TTL caching and static/catalog fallback, and do not require discovery at startup.
- Risk: merged live/static data changes alias or pricing semantics unexpectedly.
- Mitigation: preserve static metadata as authoritative overlay and only fill missing dynamic entries from discovery.
