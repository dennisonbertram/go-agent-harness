# Provider/Model Impact Map

## Task

- Task / issue: Backend OpenRouter model discovery with TTL caching and static-overlay merge behavior.
- Plan link: `docs/plans/2026-03-25-openrouter-backend-discovery-plan.md`
- Owner: Codex
- Status: In progress

## Config

- User-facing config added or changed: None required for the first pass; OpenRouter discovery activates only when the `openrouter` provider already exists in the loaded catalog.
- Defaults / fallbacks: runtime routing and model listing prefer discovered OpenRouter data when available, then cached discovery data, then static catalog data.
- Environment variables, config files, or saved settings touched: existing OpenRouter config continues to rely on catalog provider presence and `OPENROUTER_API_KEY`; no new required env vars in this pass.
- Migration / backward-compatibility notes: existing static-catalog providers remain unchanged; startup still succeeds without network discovery.

## Server API

- Endpoints, request fields, response fields, or server wiring affected: `GET /v1/models` now includes live OpenRouter models merged with static metadata when discovery succeeds.
- Provider/model resolution or registry changes: runtime provider resolution consults OpenRouter discovery instead of only using a slash-based heuristic, while preserving static catalog resolution for all other providers.
- Error states / validation changes: discovery failure should not fail requests when cached or static data exists; live fetch errors remain internal fallback signals, not startup blockers.

## TUI State

- Slash commands, overlays, selection state, routing, or status bar changes: None directly; the TUI already supports live OpenRouter fetching and simply benefits from a richer backend `/v1/models` payload.
- Persisted client state or local config changes: None.
- Keyboard/navigation implications: None.

## Regression Tests

- New acceptance tests required:
  - OpenRouter discovery decode and TTL cache behavior.
  - `/v1/models` merged live + static listing.
  - Dynamic OpenRouter runtime routing without explicit provider name.
  - Safe fallback to cached/static data when discovery fails.
- Existing tests to update:
  - Provider registry tests.
  - Server `/v1/models` tests.
- Cross-surface regressions to guard:
  - Static providers still resolve and list as before.
  - `nil` catalog / no discovery paths still return deterministic empty lists.
