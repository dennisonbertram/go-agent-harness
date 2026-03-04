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

## Entry Template

- Date:
- Topic:
- Question:
- Sources:
- Findings:
- Impact on roadmap/system:
- Open questions:
