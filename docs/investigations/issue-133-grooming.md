# Issue #133 Grooming: Inject model name and per-token cost into agent system prompt context

## Summary
Make the agent aware of what model it is running on and the per-token cost by injecting this data into the runtime system prompt context.

## Already Addressed?
**PARTIALLY ADDRESSED (~90%)** — Runtime context injection infrastructure exists (`internal/systemprompt/runtime_context.go`, commit 265807d). `RuntimeContextInput` struct has cost fields (`cost_usd_total`, `last_turn_cost_usd`). Pricing data is available in `internal/provider/pricing/`.

However, model name and per-token pricing are NOT yet injected into the rendered context. `EnvironmentInfo` struct lacks model and pricing fields.

## Clarity Assessment
Clear and well-specified.

## Acceptance Criteria
- Agent correctly answers "what model are you?" (model name in context)
- Agent correctly reports per-token cost (pricing in context)
- Pricing data matches registry values
- System prompt injection tested

## Scope
Small — add model + pricing fields to `EnvironmentInfo`, update `BuildRuntimeContext()`, pass from runner.

## Blockers
None. Infrastructure is 90% done.

## Effort
**Small** (1-2h) — ~50 lines of Go + tests.

## Label Recommendations
Current: `enhancement`. Good.

## Recommendation
**well-specified** — Ready to implement. Add model name and per-token cost to `EnvironmentInfo` and ensure they are rendered in the runtime context output.
