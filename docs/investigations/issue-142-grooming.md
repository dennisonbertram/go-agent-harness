# Issue #142 Grooming: Add GET /v1/models endpoint for model and provider discovery

## Summary
Add a `GET /v1/models` HTTP endpoint that lists all available models with provider, capabilities, and pricing info. Supports filtering by provider.

## Already Addressed?
**NOT ADDRESSED** — No `/v1/models` endpoint in `internal/server/http.go`. Model catalog infrastructure exists but is not exposed via HTTP.

## Clarity Assessment
Excellent — well-specified with request/response format, filtering, and error cases.

## Acceptance Criteria
- `GET /v1/models` returns list of models with provider, capabilities, pricing
- `?provider=openai` filter supported
- 501 when no provider registry configured
- Includes which models are "default"
- Tests with concurrent access

## Scope
Atomic. Independent of other issues.

## Blockers
None.

## Effort
**Medium** (1-2 days) — HTTP handler wrapping existing catalog/registry APIs.

## Label Recommendations
Current: none. Recommended: `enhancement`, `medium`

## Recommendation
**well-specified** — Ready to implement. High priority as it unblocks #137 (demo-cli /models command).
