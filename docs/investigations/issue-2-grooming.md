# Issue #2 Grooming: Audit SSE events for completeness and consistency

## Summary
Systematically audit all SSE events across the harness to ensure clients can reconstruct the full run timeline without polling the REST API.

## Evaluation
- **Clarity**: Moderately clear — checklist of gaps is present, but "complete" is ambiguous (depends on use case)
- **Acceptance Criteria**: Partial — defines audit tasks but doesn't specify definition of "complete" or pass/fail criteria
- **Scope**: Too broad — audit + consistency standardization across entire event system
- **Blockers**: None, but depends on understanding of all client use cases
- **Effort**: large — systematic audit across all runner code + event schema definition + client validation

## Recommended Labels
needs-clarification, large

## Missing Clarifications
- What counts as "completeness"? (SSE-only reconstruction vs. allowing one REST call for full state)
- What are the client use cases? (UI dashboards, logging, cost analysis, debugging — each might need different events)
- Should `run.started` include composed system prompt for full transparency?
- How to handle tool execution concurrency in event ordering (are events strictly sequential per step)?
- Token count granularity: per-step, per-tool-call, or per-turn?
- Event versioning strategy (backward compat when schema changes)?

## Notes
- Current runner has ~20+ event types already
- No schema documentation currently exists — this is a documentation + design task
- Consider creating a formal EventSpec schema (OpenAPI/JSON Schema)
