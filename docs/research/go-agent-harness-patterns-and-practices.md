# Go Agent Harness Patterns and Practices

**Date:** 2026-03-04  
**Scope:** Consolidated patterns used in current Go-based LLM coding harnesses, based on provider API research and source-level runtime analysis.

## Source Set

- `docs/research/charmbracelet-crush-agentic-loop-research.md`
- `docs/research/openai-api-completions-and-uploads-research.md`
- `docs/research/anthropic-api-completions-format-research.md`

## What People Are Building

Current Go harness implementations converge on a shared architecture:

1. Deterministic agent loop:
   - LLM call -> parse tool intent -> execute tool(s) -> append tool result(s) -> continue.
2. Provider adapter boundary:
   - Shared internal loop, provider-specific request/stream/tool serialization adapters.
3. Session-first runtime:
   - One active run per session, queue additional prompts, support cancellation and replay.
4. Durable state and observability:
   - Persist user/assistant/tool events and usage on each meaningful state transition.

## Common Runtime Patterns

## 1) Deterministic Tool Loop

- Parse and validate model-emitted tool arguments before execution.
- Execute tool calls with strict correlation IDs.
- Append one tool result per tool invocation.
- Continue until non-tool stop reason is returned.

## 2) Concurrency and Admission Control

- Enforce single-flight execution per session.
- Queue inbound prompts in FIFO order while the session is busy.
- Maintain explicit cancel handles for active stream and summarize phases.

## 3) Streaming State Machine

- Track text/reasoning/tool deltas independently.
- Treat stream chunks as partial events and assemble complete tool calls before dispatch.
- Finalize unfinished tool states on cancellation/errors with explicit synthetic tool errors.

## 4) Bounded Autonomy

- Stop or summarize when close to context limits.
- Detect repeated tool-call patterns to prevent runaway loops.
- Keep max step/token controls configurable per run.

## 5) Sub-Agent Composition

- Run delegated tasks in child sessions.
- Carry explicit parent/child linkage for UI and auditability.
- Roll usage and cost from child runs back to parent runs.

## Provider Contract Patterns

## OpenAI-Oriented

- Prefer modern `tools` + `tool_choice` + `tool_calls` contracts.
- Handle streaming `tool_calls` deltas and reassemble arguments incrementally.
- Support `parallel_tool_calls` only when tool concurrency is safe.
- Use `/uploads` for large/resumable file flows and `/files` for normal uploads.

## Anthropic-Oriented

- Use canonical Messages API (`/v1/messages`) for new implementations.
- Respect strict `tool_use` -> immediate `tool_result` adjacency and ordering rules.
- Map `stop_reason` (`tool_use`, `max_tokens`, `pause_turn`) into explicit loop behavior.
- Gate beta features (files/tool optimizations) via explicit runtime configuration.

## Safety and Reliability Practices

- Schema-first tool validation and strict parsing.
- Permission gating for high-risk tools and sensitive actions.
- Stable error surface: convert tool/provider failures into model-visible tool results.
- Request tracing with model/version metadata and usage accounting.

## Practical Blueprint for This Repository

1. Keep one internal run loop and hide provider details behind adapters.
2. Define a strict internal tool-call/result envelope and mapping layer per provider.
3. Implement session single-flight + FIFO queue + cancel semantics first.
4. Add summarization and loop-detection guards before expanding tool surface.
5. Persist event stream state (messages, tool calls, usage, finish reasons) for replay and debugging.
6. Add eval harness hooks early so behavior regressions are measurable.

## Anti-Patterns to Avoid

1. Executing tool arguments before schema validation.
2. Treating tool-stop reasons as final output instead of continuation.
3. Losing tool correlation IDs during streamed partial tool-call assembly.
4. Mixing provider-specific semantics directly into core runtime.
5. Relying on in-memory-only queueing without clear durability expectations.
