# Issue #1 Grooming: Stream tool output incrementally during execution

## Summary
Tools currently complete fully before results return to the client. Need to stream partial output via SSE events (tool.output.delta) as it is produced.

## Evaluation
- **Clarity**: Clear — problem statement and proposed solution are well-articulated
- **Acceptance Criteria**: Partial — design proposes new SSE event (`tool.output.delta`) but doesn't specify exact payload schema or format
- **Scope**: Atomic — focused on tool output streaming
- **Blockers**: None
- **Effort**: medium — requires plumbing through bash stdout/stderr, agent event forwarding, and fetch progress handling

## Recommended Labels
well-specified, medium

## Missing Clarifications
- Exact JSON schema for `tool.output.delta` payload (stream_index semantics, content encoding)
- Behavior when tool produces binary/non-text output (fetch, download)
- Buffering strategy if client can't consume fast enough (drop oldest, block, return backpressure)
- Should structured-output tools (JSON tools) opt-out of streaming?

## Notes
- Bash tool already has `bash_manager.go` that manages processes — integration point exists
- Agent tool needs to forward child events transparently
- Consider interaction with tool error handling — should partial results include error context?
- Will interact with #4 (deferred tools)
