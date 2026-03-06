# Crush: Open-Source Coding Agent Review

**Date**: 2026-03-05
**Repository**: [charmbracelet/crush](https://github.com/charmbracelet/crush)
**Stars**: ~20.9k
**Origin**: Successor to OpenCode (by Kujtim Hoxha), now maintained by [Charm](https://charm.sh)

---

## 1. Language & Runtime

- **Language**: Go
- **Build optimization**: Uses `GOEXPERIMENT=greenteagc` for GC tuning
- **Dependencies**: Bubble Tea v2 (TUI), `mvdan.cc/sh/v3` (POSIX shell parsing), `charm.land/fantasy` (LLM abstraction), Cobra (CLI), SQLite (persistence)
- **Binary distribution**: Single static binary via `go install`, Homebrew, NPM wrapper, Arch AUR, Nix, Winget, Scoop
- **Startup time**: Not benchmarked publicly. Go binary, so sub-second cold start is expected. Some users report hangs on misconfigured providers ([Issue #368](https://github.com/charmbracelet/crush/issues/368))
- **Cross-platform**: macOS, Linux, Windows (PowerShell + WSL), Android (Termux), FreeBSD, OpenBSD, NetBSD

## 2. Architecture

### Agent Loop

Layered event-driven architecture:

1. **CLI entry** (Cobra) -> Config + App orchestrator
2. **Coordinator** manages agent lifecycle, queues prompts per session
3. **SessionAgent.Run()** implements the core loop:
   - User prompt queued -> SessionAgent picks it up
   - Streams LLM response via `fantasy.Agent`
   - On tool call: `OnToolInputStart` -> `OnToolCall` -> `Execute()` -> `OnToolResult`
   - Tool results fed back to provider for next turn
   - Loop continues until LLM produces final text response
4. **Event system** (pub/sub) decouples agent state from TUI rendering
5. **SQLite** persists all messages, tool calls, sessions

### Key Design Choices

- Tool calls execute **synchronously** within the streaming loop
- Message queue per session: if session is busy, new prompts queue and run after current request completes
- Two agent profiles: **Coder** (full tool access) and **Task** (read-only tools)
- Permission system gates destructive operations; `--yolo` flag bypasses all prompts

## 3. Tool System

### Built-in Tools (~20+)

| Category | Tools |
|----------|-------|
| **Shell** | `bash_tool` (POSIX emulation via mvdan/sh), `job_output_tool`, `job_kill_tool` |
| **File Ops** | `view_tool`, `edit_tool`, `multi_edit_tool`, `write_tool`, `download_tool` |
| **Search** | `ls_tool`, `glob_tool`, `grep_tool`, `sourcegraph_tool` |
| **LSP** | `lsp_diagnostics`, `lsp_references`, `lsp_restart` |
| **MCP** | `list_mcp_resources`, `read_mcp_resource` |
| **Network** | `fetch_tool`, `agentic_fetch_tool` |
| **Agent** | `agent_tool` (sub-agent delegation) |
| **Utility** | `todos_tool` (session todo tracking) |

### Tool Definition

Tools implement the `fantasy.AgentTool` interface:
- `Info()` returns metadata (name, description, JSON Schema for parameters)
- `Execute()` implements logic, receives Go `context.Context` with session metadata

### Tool Registration

The coordinator's `buildTools()` method constructs all tools with injected dependencies (permission service, LSP manager, file tracker, history service), then filters based on agent config's `AllowedTools`. MCP tools are additionally filtered via `AllowedMCP` mapping.

### Custom Tools

No first-class custom tool API. Adding tools requires modifying `buildTools()` in the coordinator source. MCP integration provides the primary extensibility path -- users can connect external MCP servers (stdio, HTTP, SSE) to add capabilities without modifying Crush itself.

## 4. Context Management

### Automatic Summarization

Crush auto-summarizes conversations when approaching context window limits:

- **Large models (>200k context)**: Triggers when remaining tokens fall below a fixed 20k buffer
- **Smaller models**: Triggers at 20% remaining context window

### Summarization Process

1. Halts streaming when threshold is hit
2. Creates a new `fantasy.Agent` with a summary prompt
3. Generates summary, stores as `IsSummaryMessage: true` in SQLite
4. Resets token counters, sets `SummaryMessageID`
5. Future message loads start from summary point; earlier messages hidden from LLM but preserved in DB

### Known Issues

- Context limit not always respected; prompts can exceed configured `context_window` ([Issue #824](https://github.com/charmbracelet/crush/issues/824))
- Token counting has had bugs ([Issue #911](https://github.com/charmbracelet/crush/issues/911))
- Users report excessive token consumption even for simple tasks ([Issue #1851](https://github.com/charmbracelet/crush/issues/1851))

## 5. Multi-Agent / Parallelism

### Sub-Agent Delegation

The `agent_tool` enables primary Coder agents to spawn isolated Task sub-agents:

- Registered as `fantasy.NewParallelAgentTool` -- LLM can invoke multiple agent calls in one step, dispatched concurrently
- Each sub-agent gets its own child session (deterministic ID from parent message + tool call ID)
- Sub-agent cost rolls up to parent session
- Sub-agents receive only a `prompt` string -- no parent conversation context beyond what the tool description instructs

### Restrictions

- Task agents are **read-only**: no file writes, no shell execution
- No recursive sub-agent spawning (Task agents don't have access to `agent_tool`)
- Role `enact-all` command processes roles sequentially, not in parallel ([Issue #974](https://github.com/charmbracelet/crush/issues/974))

## 6. Resource Footprint

### Reported Issues

- **CPU**: Users report 30-40% CPU (120-130% per-core) after extended use, causing UI lag ([Issue #1746](https://github.com/charmbracelet/crush/issues/1746))
- **CPU spikes**: Near 100% on lower-end hardware (Raspberry Pi) ([Issue #1027](https://github.com/charmbracelet/crush/issues/1027))
- **MCP process leaks**: Application hangs on exit, leaving MCP processes running ([Issue #970](https://github.com/charmbracelet/crush/issues/970))
- **Memory**: Configurable via `crush config set memory.limit`; SQLite persistence keeps conversation state on disk rather than in memory

### Baseline

Go binary, so inherently lighter than Node.js/Electron alternatives. Single process with goroutines for concurrency. Bubble Tea TUI is efficient but CPU-heavy rendering has been reported during long sessions.

## 7. Strengths

1. **Beautiful TUI**: Charm's Bubble Tea ecosystem produces a polished, visually appealing terminal experience -- consistently praised in reviews
2. **Multi-provider flexibility**: Broad model support (OpenAI, Anthropic, Gemini, Groq, Bedrock, Azure, OpenRouter, local via Ollama) with mid-session model switching while preserving context
3. **LSP integration**: Built-in language server support for diagnostics and references, providing code intelligence beyond raw file reading
4. **Cross-platform breadth**: Runs on more platforms than any competitor (including Android/Termux, BSDs)
5. **Session management**: Multiple concurrent project contexts with SQLite persistence
6. **MCP extensibility**: First-class MCP support (stdio, HTTP, SSE) for connecting external tools and resources
7. **Single binary**: Easy installation, no runtime dependencies
8. **Active community**: ~20.9k stars, frequent releases, responsive maintainers

## 8. Weaknesses

1. **No real sandboxing**: Permission prompts are the only safety layer; no container, VM, or namespace isolation. `--yolo` disables even that
2. **CPU/performance degradation**: Multiple reports of high CPU and UI lag during long sessions
3. **Context window bugs**: Token counting issues, context limits not respected, excessive token consumption
4. **No custom tool API**: Extending tools requires source modification; MCP is the only supported extension path
5. **Sequential role processing**: `enact-all` doesn't parallelize despite documentation suggesting it should
6. **Task agent limitations**: Sub-agents are read-only, limiting delegation to research/exploration tasks
7. **Speed concerns**: At least one comparative report shows OpenCode (SST fork) completing identical tasks ~2x faster with the same model ([Issue #2264](https://github.com/charmbracelet/crush/issues/2264))
8. **MCP cleanup**: MCP processes can leak on exit
9. **License**: FSL-1.1-MIT (Functional Source License) -- not truly open source. Prohibits competing use. Converts to MIT after a specified date
10. **No headless/API mode**: Primarily TUI-focused; `crush run` (non-interactive mode) has reported tool execution failures ([Issue #1322](https://github.com/charmbracelet/crush/issues/1322))

## 9. Model Support

### Providers

| Provider | Notes |
|----------|-------|
| OpenAI | Direct API |
| Anthropic | Direct API |
| Google Gemini | Direct API |
| AWS Bedrock | Via SDK |
| Azure OpenAI | Via SDK |
| Groq | OpenAI-compatible |
| OpenRouter | OpenAI-compatible |
| Vercel AI Gateway | OpenAI-compatible |
| Ollama / local | OpenAI-compatible |
| Any OpenAI-compatible | Generic support |
| Any Anthropic-compatible | Generic support |

### Provider Abstraction

Uses `charm.land/fantasy` library for unified provider interface. Credentials resolved from environment variables or config files. Auto-discovery via Charm's Catwalk service for model metadata.

### Model Switching

Can switch models mid-session while preserving full conversation context. Configured per agent type (Coder vs Task can use different models).

## 10. Sandboxing

**Crush has no execution sandbox.** Safety relies entirely on:

1. **Permission system**: Tools that write files, execute shell commands, or make network requests call `RequestPermission()`, which blocks until user approves via TUI dialog
2. **Tool filtering**: Task agents (sub-agents) get read-only tool access
3. **Disabled tools config**: `options.disabled_tools` can hide specific tools entirely from the agent
4. **YOLO mode**: `--yolo` flag disables all permission prompts -- runs everything without confirmation

There is no process isolation, container, namespace, seccomp, or resource limit enforcement. Shell execution uses `mvdan.cc/sh/v3` for POSIX parsing but runs commands with full user privileges. MCP servers also run with the user's full permissions.

---

## Summary Comparison to go-agent-harness

| Dimension | Crush | go-agent-harness |
|-----------|-------|-----------------|
| Language | Go | Go |
| Architecture | TUI-first, event-driven | HTTP server, REST+SSE |
| Agent loop | Synchronous tool execution in streaming loop | Deterministic step loop |
| Tool count | ~20+ built-in | ~30+ built-in |
| Custom tools | MCP only (no plugin API) | Tool interface in code |
| Context mgmt | Auto-summarization with token thresholds | TBD |
| Sub-agents | Yes (read-only Task agents) | TBD |
| Sandboxing | Permission prompts only | TBD |
| Persistence | SQLite | SQLite |
| Model support | 10+ providers via Fantasy | OpenAI (Anthropic planned) |
| License | FSL-1.1-MIT | -- |
| Headless/API | Limited (`crush run`) | Native (HTTP server) |

---

## Sources

- [charmbracelet/crush GitHub](https://github.com/charmbracelet/crush)
- [DeepWiki: Crush Overview](https://deepwiki.com/charmbracelet/crush)
- [DeepWiki: Tool System](https://deepwiki.com/charmbracelet/crush/6-tool-system)
- [DeepWiki: Agent Delegation](https://deepwiki.com/charmbracelet/crush/6.7-agent-delegation-and-nested-tools)
- [DeepWiki: Streaming and Summarization](https://deepwiki.com/charmbracelet/crush/4.5-streaming-and-summarization)
- [The New Stack: TUI Review of Crush](https://thenewstack.io/terminal-user-interfaces-review-of-crush-ex-opencode-al/)
- [Hacker News Discussion](https://news.ycombinator.com/item?id=44736176)
- [Crush CLI Blog Post](https://atalupadhyay.wordpress.com/2025/08/12/crush-cli-the-next-generation-ai-coding-agent/)
- [Performance Issue #1746](https://github.com/charmbracelet/crush/issues/1746)
- [CPU Issue #1027](https://github.com/charmbracelet/crush/issues/1027)
- [Context Limit Issue #824](https://github.com/charmbracelet/crush/issues/824)
- [Token Consumption Issue #1851](https://github.com/charmbracelet/crush/issues/1851)
- [Speed Issue #2264](https://github.com/charmbracelet/crush/issues/2264)
