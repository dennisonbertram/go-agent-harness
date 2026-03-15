# Issue #11 Grooming: Multi-provider support: add Anthropic provider alongside OpenAI

## Summary
Add an Anthropic Claude provider to complement the existing OpenAI provider, enabling model routing across providers.

## Already Addressed?
**NOT ADDRESSED** — Only `internal/provider/openai/` exists. No Anthropic provider package found.

## Clarity Assessment
Well-specified. `ProviderRegistry` and routing infrastructure already exist; just needs the Anthropic client implementation.

## Acceptance Criteria
- Anthropic client implementing the provider interface
- Claude model support (claude-3-5-sonnet, etc.)
- Token usage + cost tracking via pricing registry
- Tool call handling for Anthropic API format
- Phase 2: prompt caching support

## Scope
Medium — one new provider package mirroring the OpenAI provider structure.

## Blockers
None. Provider registry and routing infrastructure are complete.

## Effort
**Medium** (8-12h) — Implement Anthropic client following existing OpenAI pattern.

## Label Recommendations
Current: none. Recommended: `enhancement`, `provider`

## Recommendation
**well-specified** — Infrastructure is ready. Implement Anthropic client following the OpenAI provider as a reference.
