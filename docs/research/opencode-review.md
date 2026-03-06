# OpenCode Research Review

**Date**: 2026-03-05
**Source**: https://github.com/opencode-ai/opencode / https://opencode.ai
**Status**: Active open-source project (formerly "sst/opencode" by Anomaly/SST; v1.0 Go TUI was archived Sep 2025 and the Go TUI portion migrated to Charmbracelet Crush, while the main project continued under opencode-ai)

---

## 1. Language & Runtime

OpenCode is a **hybrid TypeScript + Zig** application:

- **Core agent logic**: TypeScript, running on **Bun** (fast JS runtime).
- **Terminal UI (OpenTUI)**: Originally Go + Bubble Tea in v1.0. Migrated to a custom framework called **OpenTUI** which uses a **Zig** backend for terminal rendering with TypeScript API bindings. The Go TUI was abandoned due to severe performance issues at scale.
- **Single binary**: The TUI binary is either embedded in the Bun executable or built separately.

**Dependencies**:
- Bun runtime
- SQLite (session/conversation persistence)
- Git (snapshots, change tracking)
- LSP servers (optional, per-language: pyright, gopls, ruby-lsp, etc.)
- MCP servers (optional, for extended tool access)

**Startup**: No published startup time benchmarks. Installation via `curl | bash`, npm/bun/pnpm, Homebrew, or Docker. The process launches a Bun HTTP server backend, then starts the TUI which connects via HTTP/SSE.

---

## 2. Architecture

OpenCode uses a **client/server architecture**:

```
TUI (OpenTUI/Zig+TS) <--HTTP/SSE--> Backend (Bun/TS) <--API--> LLM Providers
```

### Agent Loop

The core loop uses Vercel's AI SDK `streamText` function ("LLM in a loop with actions"):

1. System prompt + tool definitions sent to LLM
2. User prompt submitted
3. LLM streams response; may emit `tool_use` calls
4. Runtime executes tool on local machine
5. Tool output fed back into LLM context
6. Loop continues until LLM stops or `stopWhen` triggers (max 1000 steps)

### Event Bus

A strongly-typed global event bus broadcasts all actions (file changes, permission requests, messages, tool calls). Every persisted message part emits an event. Events are exposed via HTTP SSE at `/sse`, enabling multiple clients to subscribe simultaneously.

### Streaming

The `streamText` result provides `fullStream` events with type discrimination:
- `tool-call`, `tool-result`, `tool-error`
- `text-delta` (incremental text generation)
- `start-step`, `finish-step` (step lifecycle)

Results stream to disk via `updatePart`, enabling persistence and real-time TUI updates.

### Persistence

SQLite database in `.opencode/` directory stores sessions, messages, and conversation history.

### Snapshot/Recovery

Git-based snapshotting captures working state without permanent commits (`git add . && write-tree` to track, `git read-tree + checkout-index` to restore). Enables rollback on tool execution failure.

---

## 3. Tool System

### Built-in Tools

- **bash** - Shell command execution
- **read** - File reading
- **write** - File writing
- **edit** - File editing (patch-based)
- **glob** - File pattern matching
- **grep** - Content search
- **list** - Directory listing
- **webfetch** - HTTP fetching
- **todoread** / **todowrite** - Task tracking
- **task** - Subagent invocation

### Tool Definition Pattern

Tools are defined with three components:
```typescript
tool({
  description: "...",
  args: {
    query: tool.schema.string().describe("SQL query to execute"),
  },
  async execute(args, context) {
    const { agent, sessionID, messageID, directory, worktree } = context
    return `Result: ${args.query}`
  },
})
```

Parameters use Zod schemas via `tool.schema`. The tool system is provider-agnostic through the AI SDK.

### Custom Tools

Custom tools are TypeScript/JS files placed in:
- `.opencode/tools/` (per-project)
- `~/.config/opencode/tools/` (global)

Filename becomes tool name. Multiple named exports create `<filename>_<exportname>` tools. Custom tools can override built-in tools with matching names.

### MCP Integration

MCP servers are defined in config files. On startup, OpenCode creates MCP clients that fetch tool lists from configured servers. Supports both stdio and HTTP MCP transports.

**Caveat**: MCP tool definitions consume significant context. Running 7 active MCP servers consumed ~25% of a 200k token window before any user prompt, costing ~$1.25/session.

---

## 4. Context Management

### Auto-Compact

Automatic summarization triggers when token usage exceeds:
```
tokens > Math.max((model.info.limit.context - outputLimit) * 0.9, 0)
```

At ~95% capacity, older conversation is summarized by a smaller model. Summaries preserve task history, file locations, and planned next steps. Creates new session segments to prevent context overflow errors.

### Token Tracking

Token usage (input, output, reasoning) calculated at each `finish-step`. Combined with pricing data from models.dev to compute per-run cost.

---

## 5. Multi-Agent / Parallelism

### Agent Types

**Primary Agents** (user-facing, switchable via Tab):
- **Build** (default) - Full tool access for development
- **Plan** - Read-only for analysis and exploration

**Subagents** (invoked by primary agents or via `@` mention):
- **General** - Full access subagent
- **Explore** - Read-only subagent

### Custom Agents

Defined via `opencode.json` or markdown files in `~/.config/opencode/agents/` (global) or `.opencode/agents/` (per-project). Configuration options:
- `mode`: primary, subagent, or all
- `model`: provider/model-id override
- `prompt`: custom system prompt
- `steps`: max agentic iterations
- `tools`: enable/disable with wildcard patterns
- `permission`: per-tool permission levels

### Subagent Coordination

Primary agents invoke subagents via the **Task** tool. Subagent sessions are child sessions navigable with `<Leader>+Left/Right`. Task permissions controllable per-agent:
```json
"permission": {
  "task": {
    "*": "deny",
    "code-reviewer": "allow"
  }
}
```

No evidence of true parallel subagent execution (multiple subagents running simultaneously). Coordination is sequential: primary agent calls Task, waits for result.

---

## 6. Resource Footprint

No official benchmarks published. Observable characteristics:

- **Process model**: Client/server with Bun runtime + TUI process
- **Storage**: SQLite database grows with conversation history
- **Memory**: Undocumented, but Bun is generally lighter than Node.js
- **Execution time**: In comparative benchmarks, OpenCode took 16m 20s for 4 tasks vs Claude Code's 9m 9s, suggesting thoroughness over speed (ran 94 tests vs Claude's 73)
- **Remote execution**: Docker container support ("Workspaces") for persistent remote sessions

---

## 7. Model Support

Extensive multi-provider support:

| Provider | Models |
|----------|--------|
| **Anthropic** | Claude 3.5/3.7 Sonnet, Claude 4 Sonnet/Opus |
| **OpenAI** | GPT-4.1, GPT-4.5, GPT-4o, O1/O3/O4-mini |
| **Google** | Gemini 2.0/2.5 Flash, Pro |
| **GitHub Copilot** | Access to multiple model families |
| **AWS Bedrock** | Claude models |
| **Google VertexAI** | Gemini models |
| **Groq** | Llama, QWN, DeepSeek |
| **Azure OpenAI** | OpenAI models |
| **OpenRouter** | 75+ providers |
| **Ollama** | Local models (air-gapped mode) |

Per-agent model overrides allow mixing models (e.g., cheap model for title generation, strong model for coding).

---

## 8. Sandboxing

**Current state**: Limited sandboxing.

- **Bash**: Commands execute with user permissions. Path validation against project root prevents accessing files outside working directory.
- **Permissions system**: Per-tool permissions (`ask`, `allow`, `deny`). Supports glob patterns for bash commands:
  ```json
  "bash": { "*": "ask", "git status *": "allow" }
  ```
- **No containerization by default**: Direct shell execution without Docker isolation.
- **Roadmap**: "Workspaces" feature for Docker/cloud sandbox execution (in progress).

---

## 9. Strengths

1. **Model freedom**: Not locked to any provider. Use Claude, GPT, Gemini, or local models via Ollama. Switch models per-agent.
2. **Open source & free**: Zero license cost. Full source available. Active community (112K+ GitHub stars, 700+ contributors).
3. **Beautiful TUI**: Highly polished terminal interface with vim-like keybindings, praised repeatedly in comparisons.
4. **Extensible tool system**: Custom tools via simple TS files, MCP integration, per-agent tool configuration.
5. **Client/server architecture**: Enables remote execution, multiple simultaneous clients, and persistent sessions.
6. **Privacy/air-gapped mode**: Can run entirely with local models via Ollama for sensitive environments (defense, healthcare, fintech).
7. **Auto-compact context management**: Graceful handling of long conversations with automatic summarization.
8. **Custom agents**: Define specialized agents with different models, prompts, tools, and permissions.
9. **LSP integration**: Code intelligence across multiple languages for error detection after edits.
10. **Git-based snapshots**: Rollback capability without polluting git history.

---

## 10. Weaknesses & Limitations

1. **Slower task completion**: 16m vs 9m in comparative benchmarks. Thorough but not fast.
2. **MCP context bloat**: Active MCP servers consume significant context before any user interaction.
3. **No true parallel subagents**: Subagent invocation appears sequential, not parallel.
4. **Beta maturity**: "Occasional bugs" expected. Breaking changes still possible.
5. **Bun dependency**: Requires Bun runtime; not a single static binary. More moving parts than a Go binary.
6. **No native containerized sandboxing**: Bash commands run with user permissions. Docker workspaces are roadmap, not shipped.
7. **Performance degradation**: Quality drops after extended conversations (context window pressure despite auto-compact).
8. **Token cost opacity**: While cheaper than Claude Code subscription, API costs can spike unexpectedly, especially with MCP tool bloat.
9. **Architecture complexity**: Client/server with Zig TUI + Bun backend + SQLite + Git snapshots is many layers vs a simple CLI.
10. **Abandoned Go TUI**: The v1.0 Go implementation was abandoned for performance reasons, suggesting the Go ecosystem's TUI tooling had limits. (The Go TUI portion was spun out as Charmbracelet Crush.)

---

## Relevance to go-agent-harness

| Aspect | OpenCode | go-agent-harness |
|--------|----------|-----------------|
| **Language** | TypeScript (Bun) + Zig | Go |
| **Architecture** | Client/server, event bus, SSE | Client/server, REST + SSE |
| **Agent loop** | AI SDK streamText loop | Deterministic step loop in runner.go |
| **Tools** | ~12 built-in + custom TS + MCP | 30+ built-in Go tools + MCP |
| **Context** | Auto-compact at 95% | Manual (HARNESS_MAX_STEPS) |
| **Persistence** | SQLite | SQLite (observational memory) |
| **Sandboxing** | Permission prompts, path validation | Similar approach |

**Key takeaways for go-agent-harness**:
- OpenCode's auto-compact context management (summarize at 95% capacity) is worth adopting.
- Their per-agent tool/permission configuration is a clean pattern.
- MCP context bloat is a real problem; deferred tool loading (already researched in `deferred-tools-design.md`) is the right answer.
- Their git-based snapshot/rollback system is lightweight and effective.
- The move away from Go for the TUI is notable, but their core agent logic being in TS/Bun doesn't invalidate Go for the backend loop -- different tradeoffs.
- Custom tools via simple file drop-in (`.opencode/tools/`) is a good UX pattern for extensibility.
