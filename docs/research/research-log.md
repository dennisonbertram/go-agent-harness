# Research Log

Use this file for structured, source-backed research entries.

- Date: 2026-03-04
- Topic: charmbracelet/crush architecture and agentic loop
- Question: How is the core agentic execution loop structured, and what controls prevent unbounded tool-calling behavior?
- Sources:
  - https://github.com/charmbracelet/crush/tree/8bcca78520e5dd082bdf254a4a915a1505bc5c29
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/main.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/app/app.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agent.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/coordinator.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/loop_detection.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agent_tool.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agentic_fetch_tool.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/session/session.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/message/message.go
  - https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/permission/permission.go
- Findings:
  - Core execution flows through `agent.coordinator` and a per-session `sessionAgent.Run`.
  - The loop is centered on `fantasy.Agent` streaming with callbacks for preparation, reasoning/text/tool events, and completion handling.
  - Per-session single-flight is enforced with `activeRequests` plus `messageQueue`; new prompts queue rather than starting parallel sessions.
  - Hard loop-stop controls include context-window summarization and repeated tool-call signature detection (`loop_detection_window_size=10`, `loop_detection_max_repeats=5`).
  - On summarization, the run is interrupted, a continuation prompt is re-queued as needed, and queue ordering is preserved.
  - Errors and cancellation paths close out assistant/tool states with synthetic error tool outputs to keep persistence/UI consistent.
  - Sub-agent (`agent`, `agentic_fetch`) paths spawn child sessions and roll usage back into parent sessions.
- Impact on roadmap/system:
  - Suitable pattern for reliability-focused agent runners: strong per-session sequencing + bounded loop controls + resumable context summarization.
- Open questions:
  - None identified from inspected source paths.

- Date: 2026-03-04
- Topic: Go agent harness implementation patterns
- Question: What patterns are teams using to build Go-based LLM coding harnesses?
- Sources:
  - /Users/dennisonbertram/Develop/go-agent-harness/docs/research/charmbracelet-crush-agentic-loop-research.md
  - /Users/dennisonbertram/Develop/go-agent-harness/docs/research/openai-api-completions-and-uploads-research.md
  - /Users/dennisonbertram/Develop/go-agent-harness/docs/research/anthropic-api-completions-format-research.md
- Findings:
  - Most implementations use a deterministic tool-calling loop with strict tool-call/result correlation.
  - Provider differences are isolated behind adapters while runtime orchestration remains provider-agnostic.
  - Session single-flight + FIFO queueing + explicit cancellation is a common control pattern.
  - Bounded execution controls (summarization, loop detection, max-step/token limits) are used to prevent runaway behavior.
  - Streaming state is treated as a callback/event state machine with persistence at each major transition.
  - Schema-first tool validation and permission gates are standard safety controls.
- Impact on roadmap/system:
  - Confirms a practical architecture for this repo: shared loop core, adapterized providers, durable session state, and safety-first tool execution.
- Open questions:
  - Which durability level is required for queued prompts (in-memory only vs persisted queue) in this repository's first milestone?

- Date: 2026-03-05
- Topic: Crush tool surface and compatibility spec
- Question: What are the exact `crush` tool contracts, and how should we spec similar tools for `go-agent-harness`?
- Sources:
  - https://github.com/charmbracelet/crush/blob/fae0f2e82da57a0e0335d86b417a819121f4e69f/internal/agent/coordinator.go
  - https://github.com/charmbracelet/crush/tree/fae0f2e82da57a0e0335d86b417a819121f4e69f/internal/agent/tools
  - https://github.com/charmbracelet/crush/blob/fae0f2e82da57a0e0335d86b417a819121f4e69f/internal/agent/agent_tool.go
  - https://github.com/charmbracelet/crush/blob/fae0f2e82da57a0e0335d86b417a819121f4e69f/internal/agent/agentic_fetch_tool.go
  - /Users/dennisonbertram/Develop/go-agent-harness/docs/research/crush-tools-inspection-and-harness-spec.md
- Findings:
  - `crush` uses a curated toolset for coding (`bash`, file tools, search tools, background shell lifecycle, and task tracking) with optional LSP and MCP extensions.
  - Permission gating is action-based (`read`, `write`, `list`, `execute`, `fetch`, `download`) and path-scoped.
  - Safety controls are embedded into tool behavior, especially for shell command restrictions and stale-write prevention.
  - Dynamic MCP tools are exposed as runtime tool names (`mcp_<server>_<tool>`) while preserving static MCP resource tools.
  - A phased compatibility spec for this repository is now documented (MVP coding tools -> LSP quality tools -> MCP/orchestration).
- Impact on roadmap/system:
  - Provides a direct implementation contract for building `crush`-like tooling in this repository with clear rollout phases and acceptance criteria.
- Open questions:
  - Should `agent` and `agentic_fetch` be included in first implementation milestone or deferred until core tool reliability SLOs are met?

## Entry Template

- Date:
- Topic:
- Question:
- Sources:
- Findings:
- Impact on roadmap/system:
- Open questions:
