# Hermes Agent -- Comprehensive Architecture Review

**Repository:** https://github.com/NousResearch/hermes-agent
**Version:** 0.6.0
**License:** MIT
**Reviewed:** 2026-03-30

---

## 1. Architecture Overview

Hermes Agent is a **Python 3.11+** monolithic agent system from Nous Research. The codebase is substantial (~17,000 lines across core files alone, with hundreds of supporting files). It is structured as a single-process runtime with optional multi-platform messaging gateway and cron scheduler.

### Directory Layout

```
hermes-agent/
  run_agent.py          # AIAgent class (8,334 lines) -- core conversation loop
  cli.py                # HermesCLI class (7,699 lines) -- interactive TUI
  model_tools.py        # Tool orchestration layer over registry (472 lines)
  toolsets.py           # Toolset groupings/composition (641 lines)
  hermes_state.py       # SQLite session store with FTS5 search
  hermes_constants.py   # Shared constants, provider URLs
  agent/                # Agent internals (prompt builder, compressor, pricing, display)
  tools/                # Tool implementations (40+ tools, each in its own file)
    registry.py         # Central ToolRegistry singleton
    environments/       # 7 execution backends (local, docker, ssh, modal, daytona, singularity, persistent_shell)
  hermes_cli/           # CLI subcommands, setup wizard, plugin system, skin engine
  gateway/              # Multi-platform messaging gateway
    platforms/          # 14 platform adapters (Telegram, Discord, Slack, WhatsApp, Signal, Matrix, etc.)
  skills/               # 75+ bundled skills across 26 categories
  cron/                 # Built-in cron scheduler
  acp_adapter/          # Agent Communication Protocol (VS Code / editor integration)
  tinker-atropos/       # RL training submodule (optional)
  batch_runner.py       # Parallel trajectory generation
  trajectory_compressor.py  # Post-hoc trajectory compression for training
```

### Runtime Model

- **Single Python process** running an event loop for the agent step loop.
- Uses **OpenAI Python SDK** as the primary API client, even for non-OpenAI providers (via OpenRouter or compatible endpoints).
- Has a dedicated **Anthropic Messages API adapter** (`agent/anthropic_adapter.py`) for direct Anthropic access.
- Threading is used for parallel tool execution (ThreadPoolExecutor), background MCP server connections, and subagent delegation.
- **No HTTP server** for the core agent -- the CLI (`cli.py`) is the primary entry point. The gateway (`gateway/run.py`) adds messaging platform integration as a separate process.

---

## 2. Agent Loop

The agent loop lives in `AIAgent.run_conversation()` (starting at line ~6009 of `run_agent.py`). It is a classic **while-loop tool-calling agent**:

```
while api_call_count < max_iterations and iteration_budget.remaining > 0:
    1. Check for interrupt (user sent new message, Ctrl+C)
    2. Consume iteration from budget
    3. Build API messages (inject system prompt, prefill, Honcho context, cache control)
    4. Call LLM (always prefers streaming path for health-check benefits)
    5. Parse response -- extract tool_calls or final text content
    6. If tool_calls: execute tools (parallel when safe), append results, continue loop
    7. If text content only (no tool_calls): break -- conversation complete
    8. Handle errors: retry with backoff, context compression on overflow, auth refresh
```

### Key Loop Features

- **IterationBudget**: Thread-safe counter shared across parent and subagents. Default 90 iterations for parent, 50 per subagent.
- **Budget pressure warnings**: At 70% and 90% of budget, warnings are injected into tool result JSON to nudge the model to wrap up.
- **Parallel tool execution**: Read-only tools (web_search, read_file, etc.) can run concurrently in a ThreadPoolExecutor with up to 8 workers. Path-scoped tools (read_file, write_file, patch) check for path overlap before parallelizing. Interactive tools (clarify) force sequential execution.
- **Interrupt handling**: User can interrupt mid-loop; the agent breaks cleanly and returns partial results.
- **Auto context compression**: When approaching context limits, a ContextCompressor summarizes middle turns while protecting head (system + first exchange) and tail (recent ~20K tokens). Uses a structured summary template.
- **Preflight compression**: Before entering the loop, checks if loaded conversation history already exceeds the model's context threshold.
- **Anthropic prompt caching**: Auto-enabled for Claude models -- injects `cache_control` breakpoints on system prompt and last 3 messages to reduce input costs by ~75%.
- **Reasoning pass-through**: Stores `<think>` tag reasoning in message history for trajectory saving, passes `reasoning_content` back to API for multi-turn reasoning continuity.
- **Honcho integration**: Cross-session user modeling via Honcho. Prefetched context is baked into the system prompt on first turn, injected into user messages on subsequent turns.

---

## 3. Tool System

### Registration

Tools self-register at import time via `tools.registry.register()`. Each tool file declares its schema (OpenAI function-calling format), handler function, toolset membership, and optional availability check.

```python
# Example pattern (from tools/*.py):
from tools.registry import registry

registry.register(
    name="terminal",
    toolset="terminal",
    schema={...},           # OpenAI function schema
    handler=terminal_tool,  # The actual function
    check_fn=check_fn,      # Optional availability gate
    is_async=False,
    description="Execute shell commands",
    emoji="terminal",
)
```

### Tool Catalog (~40+ tools)

| Category | Tools |
|----------|-------|
| **Terminal** | `terminal`, `process` (background process management) |
| **Files** | `read_file`, `write_file`, `patch`, `search_files` |
| **Web** | `web_search`, `web_extract` (Exa + Firecrawl + parallel-web) |
| **Browser** | `browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_scroll`, `browser_back`, `browser_press`, `browser_close`, `browser_get_images`, `browser_vision`, `browser_console` |
| **Vision** | `vision_analyze`, `image_generate` (fal.ai) |
| **Planning** | `todo` (persistent todo list) |
| **Memory** | `memory` (MEMORY.md + USER.md persistent stores) |
| **Skills** | `skills_list`, `skill_view`, `skill_manage` |
| **Delegation** | `delegate_task` (subagent spawning), `execute_code` (programmatic tool calling sandbox) |
| **Communication** | `send_message` (cross-platform), `clarify` (ask user questions) |
| **Cron** | `cronjob` (scheduled tasks) |
| **TTS** | `text_to_speech` (Edge TTS free, ElevenLabs premium) |
| **Honcho** | `honcho_context`, `honcho_profile`, `honcho_search`, `honcho_conclude` |
| **Home Assistant** | `ha_list_entities`, `ha_get_state`, `ha_list_services`, `ha_call_service` |
| **MoA** | `mixture_of_agents` (multi-model ensemble reasoning) |
| **MCP** | Dynamically discovered from configured MCP servers |
| **Session** | `session_search` (FTS5 over past conversations) |
| **RL Training** | `rl_training_tool` |

### Toolset System

Tools are grouped into named toolsets (web, terminal, file, browser, vision, etc.) defined in `toolsets.py`. Toolsets can compose other toolsets. Users can enable/disable toolsets via config or CLI flags. A core set (`_HERMES_CORE_TOOLS`) defines the default tool palette shared across CLI and all messaging platforms.

### Tool Dispatch

`model_tools.py` is a thin orchestration layer. `ToolRegistry.dispatch()` handles execution, automatically bridging async handlers via a persistent event loop (avoids "Event loop is closed" errors with cached httpx/AsyncOpenAI clients).

### Dangerous Command Approval

`tools/approval.py` implements pattern-based detection of dangerous commands (rm -r, chmod 777, dd, mkfs, etc.) with per-session approval state. Supports CLI interactive prompts, gateway async approval, and LLM-powered smart auto-approval for low-risk commands.

---

## 4. Provider Support

Hermes is notably **provider-agnostic**. All providers are accessed through the OpenAI SDK's chat completions interface, with adapters for non-OpenAI protocols.

### Supported Providers

| Provider | Access Method |
|----------|--------------|
| **OpenRouter** | Primary path. 200+ models. Default `base_url`. |
| **OpenAI** | Direct via `api.openai.com`. Auto-detects and switches to Responses API for GPT-5.x. |
| **Anthropic** | Direct via `api.anthropic.com`. Full native adapter (`anthropic_adapter.py`) using the Anthropic SDK. Supports extended thinking, prompt caching, OAuth tokens, Claude Code credentials. |
| **Nous Portal** | `portal.nousresearch.com` |
| **z.ai / GLM** | Via compatible endpoint |
| **Kimi / Moonshot** | Via compatible endpoint (special reasoning_content handling) |
| **MiniMax** | Via Anthropic-compatible `/anthropic` suffix endpoint |
| **Any OpenAI-compatible** | Custom `base_url` |

### API Modes

Three distinct API modes, auto-detected from provider/URL:
1. **chat_completions** -- Standard OpenAI chat completions (default)
2. **codex_responses** -- OpenAI Responses API (for GPT-5.x with tool calls + reasoning)
3. **anthropic_messages** -- Native Anthropic Messages API (for direct Anthropic access)

### Model Switching

`hermes model` CLI command or `/model` slash command allows live model switching with no code changes. Model metadata (context length, pricing) is fetched from OpenRouter and cached for 1 hour.

---

## 5. Streaming

Streaming is the **default and preferred path** for all API calls, even when no stream consumer is registered. This is a deliberate design choice:

- Streaming provides fine-grained health checking (90s stale-stream detection, 60s read timeout) that the non-streaming path lacks.
- Without streaming, subagents can hang indefinitely when providers keep connections alive with SSE pings but never deliver responses.
- When no display/TTS consumers are registered, streaming still runs but callbacks are no-ops.
- Streaming is implemented via `_interruptible_streaming_api_call()` which accumulates deltas and supports mid-stream interruption.

### Stream Consumers

Multiple callback hooks for streaming output:
- `stream_delta_callback` -- Per-token text deltas (for TUI display, TTS pipeline)
- `thinking_callback` -- Reasoning/thinking content
- `reasoning_callback` -- Extended thinking deltas
- `tool_gen_callback` -- Tool call generation progress
- `tool_progress_callback` -- Tool execution status
- `status_callback` -- Agent status updates
- `step_callback` -- Per-iteration step events

---

## 6. Configuration

### Config File

Primary config: `~/.hermes/config.yaml` (falls back to `./cli-config.yaml` for development).

Key config sections:
- `model` -- Default model, base_url, provider
- `terminal` -- env_type (local/docker/ssh/modal/daytona/singularity), cwd, timeout, Docker image, volumes
- `browser` -- Inactivity timeout, session recording
- `compression` -- Auto-compression threshold (default 50% of context), summary model
- `smart_model_routing` -- Route simple queries to cheap models
- `agent` -- max_turns (default 90), verbose, system_prompt, personalities, reasoning_effort, memory/skill nudge intervals
- `delegation` -- max_iterations per subagent (default 50), toolsets, blocked tools
- `mcp_servers` -- External MCP server configurations
- `honcho` -- Cross-session user modeling
- `approval` -- Command approval settings, allowlist
- `cron` -- Scheduler settings
- `skin` -- TUI visual customization

### Environment Variables

Extensive env var support for API keys, feature flags, and runtime behavior. Provider credentials are resolved via `hermes_cli/auth.py` with fallback chains.

### Setup Wizard

`hermes setup` runs an interactive wizard that configures provider, model, API keys, terminal backend, and messaging platforms in one pass.

---

## 7. Orchestration / Multi-Agent

### Subagent Delegation

`tools/delegate_tool.py` implements a `delegate_task` tool that spawns child `AIAgent` instances:

- **Isolation**: Each child gets a fresh conversation (no parent history), its own task_id (own terminal session), and restricted toolsets.
- **Batch mode**: Supports parallel delegation of up to 3 concurrent children via ThreadPoolExecutor.
- **Depth limit**: Max depth of 2 (parent -> child -> grandchild rejected). No recursive delegation.
- **Blocked tools**: Children cannot delegate, clarify with users, write to shared memory, send cross-platform messages, or use execute_code.
- **Budget**: Each subagent gets an independent IterationBudget (default 50 iterations).
- **Progress**: Child tool calls are relayed to parent display (tree-view in CLI, batched summaries for gateway).

### Programmatic Tool Calling (PTC)

`tools/code_execution_tool.py` implements `execute_code` -- the agent writes a Python script that calls Hermes tools via RPC over Unix domain sockets. This collapses multi-step tool chains into a single inference turn with zero context cost for intermediate results.

Architecture:
1. Parent generates a `hermes_tools.py` stub module with RPC functions
2. Parent opens a Unix domain socket and starts an RPC listener thread
3. Parent spawns a child process that runs the LLM's script
4. Tool calls travel over UDS back to parent for dispatch
5. Only script stdout is returned to the LLM

### Workspace Isolation

Terminal execution supports 7 backends via `tools/environments/`:
- **local** -- Direct host execution (default)
- **docker** -- Isolated Docker containers
- **ssh** -- Remote SSH execution
- **modal** -- Modal cloud sandboxes (serverless, hibernates when idle)
- **daytona** -- Daytona cloud workspaces (serverless persistence)
- **singularity** -- Singularity/Apptainer containers
- **persistent_shell** -- Long-lived shell sessions with state

All backends implement `BaseEnvironment` with `execute()` and `cleanup()` methods.

### ACP (Agent Communication Protocol)

`acp_adapter/` implements an ACP server for editor integration (VS Code, Zed, JetBrains). This allows Hermes to be used as a coding assistant within IDEs.

---

## 8. Skills / Prompt System

### Skills System

Hermes has a sophisticated skills system inspired by Claude Code's skill architecture:

- **Location**: `~/.hermes/skills/` (75+ bundled skills across 26 categories)
- **Format**: Each skill is a directory with a `SKILL.md` file (YAML frontmatter + markdown body) and optional references/templates/assets
- **Progressive disclosure**: `skills_list` returns metadata only (name + description); `skill_view` loads full instructions on demand
- **Frontmatter**: name, description, version, platforms (OS filtering), prerequisites, compatibility, metadata
- **Skills Hub**: Integrates with [agentskills.io](https://agentskills.io) open standard for community skill sharing
- **Self-improvement**: Agent can create skills from experience via `skill_manage` tool, and skills self-improve during use
- **Nudge system**: After sustained tool use without skill creation, the agent is nudged to persist knowledge as skills

### Bundled Skill Categories

apple, autonomous-ai-agents, creative, data-science, devops, diagramming, dogfood, domain, email, feeds, gaming, gifs, github, inference-sh, leisure, mcp, media, mlops, note-taking, productivity, red-teaming, research, smart-home, social-media, software-development

### Context Files

The agent auto-discovers and injects context files into the system prompt:
- `SOUL.md` -- Agent persona/identity (user-customizable)
- `AGENTS.md` -- Project-specific instructions (like CLAUDE.md)
- `.hermes.md` / `HERMES.md` -- Per-project context (searched up to git root)
- `.cursorrules` -- Cursor-compatible project rules

Context files are scanned for **prompt injection** before loading -- patterns like "ignore previous instructions", hidden HTML divs, and exfiltration via curl/wget are blocked.

### Memory System

Two persistent stores injected into system prompt:
- `MEMORY.md` -- Agent's personal notes (environment facts, tool quirks, conventions)
- `USER.md` -- What the agent knows about the user (preferences, workflow habits)

**Frozen snapshot pattern**: Memory is loaded once at session start and injected as a frozen snapshot into the system prompt. Mid-session writes update disk immediately but do NOT change the system prompt (preserves prefix cache). The snapshot refreshes on next session start.

Memory content is also scanned for injection/exfiltration patterns before persistence.

### Personalities

Built-in personality presets (helpful, concise, technical, creative, teacher, kawaii, catgirl, pirate, shakespeare, etc.) switchable via `/personality` command.

---

## 9. Notable Features

### Mixture of Agents (MoA)
A `mixture_of_agents` tool implements multi-model ensemble reasoning: reference models (Claude, Gemini, GPT, DeepSeek) generate diverse responses in parallel, then an aggregator model (Claude) synthesizes them. Based on the "Mixture-of-Agents" research paper.

### Cross-Session Recall
FTS5-powered session search allows the agent to search across all past conversations. Combined with Honcho dialectic user modeling for persistent cross-session understanding.

### Multi-Platform Gateway
A single gateway process serves 14+ messaging platforms simultaneously: Telegram, Discord, Slack, WhatsApp, Signal, Matrix, Mattermost, DingTalk, Feishu, WeChat, Email, SMS, Home Assistant, and a webhook API.

### Cron Scheduler
Built-in cron scheduler with natural language task definitions and delivery to any connected platform.

### RL Training Pipeline
Research-oriented features for training the next generation of tool-calling models:
- `batch_runner.py` -- Parallel trajectory generation across datasets
- `trajectory_compressor.py` -- Post-hoc compression preserving training signal
- `rl_cli.py` -- RL training workflow runner
- Integration with Atropos RL environments and Tinker
- Toolset distributions for varied training data

### Security Features
- Dangerous command detection with pattern matching
- Per-session approval state (CLI interactive + gateway async)
- LLM-powered smart auto-approval for low-risk commands
- Context file injection scanning
- Memory content injection/exfiltration scanning
- Credential stripping in MCP error messages
- Environment variable filtering for subprocess spawning

### Plugin System
`hermes_cli/plugins.py` supports hooks (`on_session_start`, `pre_llm_call`) for extending behavior without modifying core code.

### Smart Model Routing
Optional feature to route simple queries (short messages under character/word thresholds) to cheaper models, saving cost on trivial interactions.

---

## 10. Limitations and Observations

### Monolith Size
`run_agent.py` is 8,334 lines -- a massive single file containing the entire agent loop, API call logic, error handling, streaming, compression, checkpoint management, and more. While the `agent/` package extracts some concerns, the core AIAgent class is extremely large.

### No Typed API / Server
Unlike go-agent-harness, Hermes has no HTTP API server for programmatic access. The core is a Python class (`AIAgent`) consumed directly by `cli.py` and `gateway/run.py`. The ACP adapter exists but is specialized for editor integration.

### Threading Model
Threading is used extensively (parallel tool execution, MCP background loops, subagent delegation) but with careful locking. The codebase is GIL-aware and notes free-threading compatibility for Python 3.13+. However, the lack of async-native design means extensive sync-to-async bridging (`_run_async`, persistent event loops, per-worker-thread loops).

### No Streaming Run API
There is no SSE/WebSocket streaming run API comparable to go-agent-harness's `/v1/runs`. Streaming is internal (callback-based within the process). External consumers (gateway platforms) get tool progress via callbacks, not a standard event stream.

### Provider Abstraction
While supporting many providers, the abstraction is somewhat ad-hoc -- conditional logic based on URL patterns and provider names rather than a clean provider interface. The Anthropic adapter is well-separated, but OpenAI vs. OpenRouter vs. Codex Responses switching involves multiple conditionals in the AIAgent constructor.

### Test Coverage
`pyproject.toml` shows a test directory exists with pytest configuration, but the test suite was not deeply examined. The `-n auto` flag suggests parallel test execution.

### Dependency Weight
The full dependency set is substantial: OpenAI SDK, Anthropic SDK, prompt_toolkit, rich, exa-py, firecrawl-py, fal-client, edge-tts, faster-whisper, PyJWT, plus many optional dependencies for messaging platforms, Modal, Daytona, MCP, RL training, etc.

### Comparison with go-agent-harness

| Aspect | hermes-agent | go-agent-harness |
|--------|-------------|-----------------|
| Language | Python 3.11+ | Go |
| Agent loop | `while` loop in `run_conversation()` | Step loop in `runner.go` |
| Tool registration | `registry.register()` at import time | `//go:embed` descriptions + Go interfaces |
| Tool tiers | Flat (all visible based on toolset) | Core vs. Deferred (progressive disclosure) |
| API surface | Python class + CLI | HTTP SSE streaming API |
| Streaming | Internal callbacks | SSE event stream |
| Providers | OpenRouter + direct adapters | OpenAI primary, Anthropic catalog |
| Skills | 75+ bundled, self-improving, Skills Hub | Loader/resolver with preprocessing |
| Delegation | ThreadPoolExecutor subagents | ForkedAgentRunner interface |
| Workspaces | 7 terminal backends | local, worktree, container, VM, pool |
| Memory | File-backed MEMORY.md + USER.md | N/A (external) |
| Platforms | 14+ messaging platforms | CLI only |
| RL training | Built-in batch runner + trajectory compression | N/A |
| Security | Pattern-based command approval, injection scanning | Conversation scoping, cost/permission propagation |

---

## Summary

Hermes Agent is a feature-rich, production-oriented AI agent with an unusually broad surface area: 40+ tools, 14+ messaging platforms, 7 execution backends, 75+ skills, RL training pipelines, and cross-session memory. Its key differentiators are the self-improving skill system, multi-platform gateway, and research-ready trajectory generation. The main architectural trade-offs are the monolithic Python codebase (no typed API server, callback-based streaming) and the weight of the dependency tree. The codebase is well-commented and clearly maintained by an active team, but the 8K+ line core file suggests growth outpacing modularization.
