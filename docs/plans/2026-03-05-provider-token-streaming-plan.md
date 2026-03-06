# Plan: Provider Token Streaming

## Context

- Problem: The harness exposes SSE run events, but model responses are still delivered only after the provider finishes a turn.
- User impact: Clients cannot render token-by-token assistant output or watch tool-call arguments assemble in real time.
- Constraints:
  - Keep the existing REST/SSE API stable for current clients.
  - Preserve deterministic tool execution after streamed tool calls are fully assembled.
  - Maintain strict TDD and regression coverage.

## Scope

- In scope:
  - Add provider-to-runner streaming callback support for incremental assistant text/tool-call deltas.
  - Implement OpenAI chat completions streaming assembly.
  - Emit new SSE events for streamed assistant/tool-call deltas while preserving existing terminal events.
  - Add tests for provider streaming assembly and runner event emission.
- Out of scope:
  - Incremental stdout/stderr streaming from long-running tools.
  - Provider streaming support for non-OpenAI backends.
  - New client UI behaviors beyond existing SSE consumption.

## Test Plan (TDD)

- New failing tests to add first:
  - OpenAI provider streams assistant content/tool-call deltas and still returns assembled final result.
  - Runner emits assistant delta events during a text turn.
  - Runner emits tool-call delta events before executing the tool.
- Existing tests to update:
  - Provider tests affected by request payload changes.
  - Runner stubs to support optional streaming callbacks.
- Regression tests required:
  - Existing non-stream completion behavior remains green.
  - Existing SSE run/event tests continue to pass without contract regressions.

## Implementation Checklist

- [ ] Define acceptance criteria in tests.
- [ ] Write failing tests first.
- [ ] Implement minimal code changes.
- [ ] Refactor while tests remain green.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run full test suite.
- [ ] Merge branch back to `main` after tests pass.

## Risks and Mitigations

- Risk: Partial tool-call chunks are assembled incorrectly, causing malformed tool execution.
- Mitigation: Keep index-based accumulation logic covered by dedicated provider tests.
