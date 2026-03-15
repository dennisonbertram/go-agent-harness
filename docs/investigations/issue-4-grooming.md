# Issue #4 Grooming: Implement deferred (lazy-loaded) tools via ToolSearch meta-tool

## Summary
Tools (~6000 tokens) are sent every request regardless of use. Split into core (always visible) and deferred (load on-demand via `find_tool` / `tool_search`) to save tokens per call.

## Evaluation
- **Clarity**: Very clear — design is comprehensive with implementation phases, registry methods, token savings estimates
- **Acceptance Criteria**: Explicit — specific registry methods needed, runner integration, observability events
- **Scope**: Large — significant registry refactor + runner changes + multiple tool tier management
- **Blockers**: None
- **Effort**: large — 3 implementation phases, registry redesign, testing of deferred tool activation

## Recommended Labels
well-specified, large

## Missing Clarifications
None — design doc is thorough.

## Notes
- Codebase already has tier infrastructure: `registry.go` has `DefinitionsForRun()`, `tools/types.go` defines `TierCore` and `TierDeferred`, `ActivationTrackerInterface` exists, `find_tool.go` exists
- Key work: complete the deferred tools plumbing in runner to use `DefinitionsForRun()` instead of `Definitions()`
- Partially implemented — need to verify current state
