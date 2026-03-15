# Issue #149 Grooming: Add user-facing HTTP endpoint for provider registry inspection

## Summary
Add `GET /v1/providers` endpoint for inspecting configured providers, available models, and whether env vars are set.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/providers` endpoint. `ProviderRegistry` and `Catalog.ListProviders()` infrastructure exists.

## Clarity Assessment
Excellent — well-specified with response format, env var check behavior, and error cases.

## Acceptance Criteria
- `GET /v1/providers` lists providers with configured status (checks env var presence, not value)
- 501 when no registry configured
- Tests with concurrent access

## Scope
Atomic.

## Blockers
None.

## Effort
**Small** (1-2 days).

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement.
