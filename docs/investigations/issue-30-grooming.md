# Issue #30 Grooming: Sub-agent model selection: spawn sub-runs with different providers

## Summary
Allow the `agent` tool to accept an optional `model` parameter so sub-agents can run on different models/providers.

## Already Addressed?
**PARTIALLY ADDRESSED** — `RunRequest` has a `Model` field and `ProviderRegistry` handles routing. However, the `agent` tool in `internal/harness/tools/agent.go` only accepts a `prompt` parameter and does not expose `model`. The infrastructure is complete; only the tool interface is missing.

## Clarity Assessment
Clear and well-motivated.

## Acceptance Criteria
- `agent` tool accepts optional `model` parameter
- Sub-runs inherit or override provider settings
- Cost tracked per sub-run via existing `usage.delta` events
- Test coverage for multi-provider sub-agent calls

## Scope
**Small** — Single tool parameter addition + 3 lines of wiring.

## Blockers
None. All dependencies (ProviderRegistry, model routing) are complete.

## Effort
**Small** (2-4h) — Add `model` param to agent tool schema + wire to RunRequest.

## Label Recommendations
Current: none. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement. Add `model` parameter to the `agent` tool and pass it through to RunRequest.
