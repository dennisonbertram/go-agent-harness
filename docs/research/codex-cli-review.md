# OpenAI Codex CLI Review

Research date: 2026-03-05
Repository: https://github.com/openai/codex (63k+ stars, Apache-2.0 license)

---

## 1. Language & Runtime

**Primary language: Rust (96.1% of codebase)**. The remaining 4% is TypeScript (SDK), Python, and JavaScript.

Codex CLI was originally written in TypeScript/React/Node.js (Ink-based TUI). In mid-2025, OpenAI announced the "Going Native" rewrite to Rust (see [Discussion #1174](https://github.com/openai/codex/discussions/1174)). The Rust implementation is now the maintained default.

| Aspect | Details |
|--------|---------|
| Language | Rust (96.1%), TypeScript SDK (2.1%) |
| Build system | Cargo workspace, 65+ crates |
| Runtime | Native binary, no garbage collector |
| Install | `npm i -g @openai/codex`, `brew install --cask codex`, or direct binary download |
| Dependencies at runtime | Zero -- single static binary (the npm/brew packages just wrap the binary) |
| Node.js requirement | Eliminated by Rust rewrite (previously required Node v22+) |
| Startup time | Sub-second (native binary), significantly faster than the old Node.js version |

### Repository Structure

```
codex-cli/     # Legacy TypeScript CLI (deprecated)
codex-rs/      # Rust implementation (active)
  ├── cli/           # TUI frontend
  ├── core/          # Agent loop, session, tool dispatch
  ├── exec/          # Sandboxed command execution
  ├── linux-sandbox/ # Landlock/seccomp sandbox binary
  ├── mcp-server/    # MCP server implementation
  └── app-server/    # JSON-RPC protocol server
sdk/typescript/  # TypeScript SDK wrapping the CLI binary
```

---

## 2. Architecture

### Agent Loop

The core loop follows a classical agentic pattern:

```
User Input -> Prompt Construction -> Model Streaming (SSE) -> Tool Dispatch -> Append Results -> Re-query Model -> ... -> Final Assistant Message
```

A single turn can involve hundreds of model-tool iterations. The loop terminates when the model produces a response without requesting any tool calls.

### Event-Driven Protocol

Codex uses an internal `Op`/`EventMsg` protocol that cleanly separates frontends from core logic:

- **Operations (`Op`)**: Submitted asynchronously via a sender channel
- **Events (`EventMsg`)**: Streamed back to any consumer (TUI, CLI, IDE plugin, MCP server)
- This decoupling means all frontends share identical core behavior

The app-server layer translates between JSON-RPC v2 (newline-delimited JSON over stdio) and the internal protocol. The TypeScript SDK spawns the CLI binary and communicates via this JSONL protocol.

### Streaming

All model interactions use SSE streaming. Events like `response.output_text.delta` provide real-time text streaming to the TUI. Tool call results and file change notifications also stream incrementally.

### Session Model

- A `Session` represents an initialized conversation with at most one running task at a time
- Sessions are persisted in `~/.codex/sessions/` as JSONL event logs
- The `resume` subcommand can restore prior sessions with full transcript, plan history, and approvals
- Conversations are stateless per-request: every API call sends the full conversation history (no server-side state dependency)

---

## 3. Tool System

### Built-in Tools

| Tool | Purpose |
|------|---------|
| `shell` / `run_terminal_cmd` | Execute shell commands (Bash/Zsh/PowerShell) |
| `apply_patch` | Create, update, or delete files via structured diffs (the model is specifically trained for this format) |
| `read_file` | Read file contents |
| `list_dir` | List directory contents |
| `glob_file_search` | Search for files by glob pattern |
| `rg` (ripgrep) | Search file contents via regex |
| `git` | All git operations |
| `todo_write` / `update_plan` | Task tracking and planning |
| `js_repl` | JavaScript REPL for computation |
| `web_search` | Web search (cached index by default, live search via `--search` flag) |
| `image_gen` | Image generation |

### Tool Definition

Tools are defined as Rust structs with JSON Schema parameter specifications:

- Factory functions: `create_[tool_name]_tool() -> Tool`
- JSON Schema parameters with types, descriptions, and required fields
- A `build_specs()` function generates the final tool manifest based on model capabilities, feature flags, and session context
- Not all tools are available for all models -- conditional inclusion based on capabilities

### Tool Routing

The `ToolRouter` dispatches based on `ToolPayload` variants:
- `Function` -- standard built-in tool calls
- `Mcp` -- MCP server tools
- `Custom` -- custom tool implementations
- `LocalShell` -- shell execution

### Custom Tools via MCP

Codex supports extending its tool set through the Model Context Protocol (MCP). MCP servers are configured in `~/.codex/config.toml` or `.codex/config.toml`:

```toml
[mcp_servers.my_tool]
command = "npx"
args = ["-y", "my-mcp-server"]
enabled_tools = ["tool_a", "tool_b"]  # optional allowlist
startup_timeout_sec = 30
```

Common MCP integrations: OpenAI Docs, Context7, Figma, Playwright, Chrome DevTools.

MCP tools are **not** sandboxed by Codex -- they must enforce their own guardrails.

---

## 4. Context Management

### Full-History Per Request

Every API request contains the complete conversation history (no server-side state). This is deliberate for:
- Zero Data Retention (ZDR) compliance
- Data privacy guarantees
- No dependency on server-side state

The tradeoff is quadratic JSON overhead as conversations grow.

### Prompt Caching

To mitigate the cost of full-history, Codex relies on prompt caching:
- Cache hits require exact prefix matches
- Static content (system prompt, tool definitions) is placed before dynamic content
- Mid-conversation config changes append new messages rather than modifying earlier ones to preserve the prefix

### Automatic Compaction

When token count exceeds a threshold (approximately 95% of the effective context window):
1. A specialized `/responses/compact` endpoint summarizes the conversation
2. Returns compressed items replacing the original input
3. Includes an `encrypted_content` blob preserving "the model's latent understanding"
4. Converts quadratic growth back to manageable linear growth

### Context Window Limits

- The underlying models (e.g., gpt-5.2-codex) support approximately 400k tokens
- Effective usable context is approximately 272k tokens (400k minus 128k output reservation)
- After applying a 0.95 auto-compaction threshold, users get approximately 258k usable tokens
- There are open issues requesting the CLI expose the full model context capacity

---

## 5. Multi-Agent / Parallelism

### Multi-Agent Support

Codex has experimental multi-agent support, configured via `config.toml`:

```toml
[agents]
max_threads = 4       # concurrent agent threads
max_depth = 2         # maximum nesting depth (root at 0)

[agents.reviewer]
description = "Code review specialist"
```

### Built-in Code Review Agent

The `/review` slash command spawns a separate agent for code review with multiple preset options (security, performance, style, etc.).

### Agents SDK Integration

Codex can be orchestrated programmatically via the OpenAI Agents SDK, enabling:
- Multi-agent workflows where Codex is one participant
- Programmatic task decomposition and coordination
- External orchestration of parallel Codex instances

### Limitations

- Multi-agent is still marked as experimental
- Each agent instance is a separate process with its own context window
- No built-in shared memory between agents beyond filesystem
- `max_depth` controls recursion but not horizontal parallelism well

---

## 6. Resource Footprint

### Performance Characteristics

| Metric | Details |
|--------|---------|
| Binary size | Single static binary (platform-specific) |
| Startup time | Sub-second (native Rust binary) |
| Memory (idle) | Low baseline (no GC, no runtime overhead) |
| Memory (active) | Proportional to conversation history held in memory |
| CPU | Minimal -- most time spent waiting on API responses |

### Known Issues

- **Windows memory leak**: Version 0.104.0 on Windows 10 exhibited unbounded memory growth reaching approximately 90GB when idle, causing system OOM ([Issue #12414](https://github.com/openai/codex/issues/12414))
- **Heavy workload pressure**: Running multiple CLI sessions with heavy workloads (e.g., large Python test suites) can cause severe memory pressure ([Issue #11523](https://github.com/openai/codex/issues/11523))
- Feature requests exist for a global memory budget and proactive OOM protection

### Comparison to Node.js Era

The Rust rewrite eliminated:
- Node.js v22+ runtime dependency
- Garbage collector pauses
- Higher baseline memory consumption from the V8 engine
- Slower startup from module loading

---

## 7. Strengths

1. **Open source (Apache-2.0)**: Full source code available, community contributions welcome, forkable
2. **Zero-dependency install**: Single binary, no runtime requirements
3. **Native OS sandboxing**: Real security via Seatbelt (macOS), Landlock/seccomp (Linux), AppContainer (Windows) -- not just permission prompts
4. **Two-axis security model**: Sandbox scope and approval policy are independent, composable concerns
5. **Extensible via MCP**: Full MCP client and server support, enabling integration with the broader tool ecosystem
6. **Protocol abstraction**: The `Op`/`EventMsg` protocol cleanly decouples frontends from core logic -- TUI, headless, IDE, and SDK all share the same core
7. **Session persistence and resume**: Full conversation replay, branching, and resumption from any point
8. **Strong apply_patch format**: The model is specifically trained to produce well-formed patches, making file editing reliable
9. **Head-tail buffer for process output**: Captures first N and last M lines of command output without unbounded memory
10. **Free with ChatGPT subscription**: No additional API cost for Plus/Pro/Team/Enterprise users
11. **AGENTS.md support**: Project-specific agent instructions baked into the workflow
12. **Multi-model flexibility**: Can switch models mid-session via `/model`

---

## 8. Weaknesses & Limitations

1. **Lower benchmark performance**: Approximately 69.1% on SWE-bench Verified vs. Claude Code's 72.7% -- less effective on complex multi-file refactoring and architectural tasks
2. **OpenAI model lock-in**: Despite config options for `model_provider`, the tool is deeply integrated with OpenAI's Responses API, prompt caching, and compaction endpoints
3. **Context window friction**: The CLI caps usable context below the model's full capacity, causing "context window exceeded" errors on large codebases or CI-like tasks with voluminous output
4. **Code hallucinations**: Users report occasional hallucinated code, especially for less common languages and frameworks
5. **Windows support is experimental**: Memory leaks, WSL2 required for full functionality, AppContainer sandbox less mature
6. **MCP tools are unsandboxed**: External MCP tools bypass Codex's sandbox entirely, creating a security gap
7. **macOS sandbox is deprecated**: Uses `sandbox-exec` which Apple has deprecated, with no announced migration plan
8. **No native IDE integration without SDK**: Must go through the TypeScript SDK or app-server protocol; no direct library embedding
9. **Limited language breadth**: Strongest in Python/JS/TS/Shell, adequate but less reliable for Go, Rust, Java, C++
10. **Multi-agent is experimental**: Limited documentation, no production hardening, basic coordination primitives
11. **Compaction is lossy**: When context is compacted, some information is inevitably lost, which can cause the agent to "forget" earlier context in long sessions

---

## 9. Model Support

### Default Model

The default model is **gpt-5.4** (as of March 2026). Users can switch via:
- `--model` CLI flag
- `/model` slash command mid-session
- `model` key in `config.toml`

### Supported Models

| Model | Notes |
|-------|-------|
| gpt-5.4 | Default, latest |
| gpt-5.3-codex | Previous generation codex-optimized |
| gpt-5.2-codex | Earlier codex model |
| o3 | Reasoning model (69.1% SWE-bench) |
| Other OpenAI models | Configurable via `model` setting |

### Provider Configuration

```toml
model = "gpt-5-codex"
model_provider = "openai"          # currently the only fully supported provider
model_context_window = 400000      # available context tokens
model_reasoning_effort = "medium"  # minimal, low, medium, high, xhigh
model_verbosity = "medium"         # low, medium, high
```

### Third-Party Model Support

While `model_provider` exists as a config key, **OpenAI is the only first-class provider**. The tool's prompt caching, compaction, and streaming protocols are tightly coupled to OpenAI's Responses API. Community forks exist for Anthropic/other providers but are not officially supported.

---

## 10. Sandboxing

### Philosophy

Codex implements defense-in-depth through OS-level process isolation, not application-level permission checks. The sandbox restricts what the child process **can** do at the kernel level.

### Platform Implementations

| Platform | Technology | Mechanism |
|----------|-----------|-----------|
| macOS | Apple Seatbelt | `.sbpl` profiles passed to `sandbox-exec`, restricting filesystem and network |
| Linux | Landlock + seccomp | Kernel-level filesystem restrictions + syscall filtering via dedicated sandbox binary |
| Windows | AppContainer | Windows sandboxing primitive (experimental) |

### Sandbox Modes

| Mode | Behavior |
|------|----------|
| `read-only` | Can read files, no writes, no network |
| `workspace-write` (default) | Read/write within working directory, configurable additional writable paths, configurable network |
| `danger-full-access` | No restrictions |

### Execution Flow

1. Tool call arrives at `ToolOrchestrator`
2. First attempt: execute in sandboxed mode
3. If sandbox denies the operation: request user re-approval, then retry with escalated privileges
4. Approval decisions are cached to avoid repeated prompts

### Configurable Restrictions

```toml
[sandbox_workspace_write]
writable_roots = ["/tmp/build", "/home/user/other-project"]
network_access = true
```

### Admin Enforcement

`requirements.toml` allows administrators to enforce sandbox constraints that users cannot override:

```toml
allowed_sandbox_modes = ["read-only", "workspace-write"]
# Users cannot select danger-full-access
```

### Security Gaps

- MCP server tools run outside the sandbox
- macOS Seatbelt (`sandbox-exec`) is deprecated by Apple
- The `danger-full-access` mode completely disables all protections

---

## Summary Comparison with Claude Code

| Dimension | Codex CLI | Claude Code |
|-----------|-----------|-------------|
| Language | Rust | TypeScript/Node.js |
| License | Apache-2.0 (open source) | Proprietary |
| SWE-bench | ~69.1% | ~72.7% |
| Sandboxing | OS-level (Seatbelt, Landlock) | Application-level permissions |
| Model lock-in | OpenAI only (practical) | Anthropic only |
| MCP | Client + Server | Client + Server |
| Context window | ~258k usable (with compaction) | ~200k |
| Multi-agent | Experimental | Subagent spawning |
| Install | Zero-dependency binary | Requires Node.js |
| Cost | Free with ChatGPT subscription | Token-based pricing |
| Strength | Security model, open source, extensibility | Complex refactoring, architectural reasoning |
| Weakness | Benchmark performance, model lock-in | Cost, closed source |

---

## Sources

- [OpenAI Codex GitHub Repository](https://github.com/openai/codex)
- [Codex CLI Going Native Discussion #1174](https://github.com/openai/codex/discussions/1174)
- [Codex CLI Documentation](https://developers.openai.com/codex/cli/)
- [Codex CLI Features](https://developers.openai.com/codex/cli/features/)
- [Codex Configuration Reference](https://developers.openai.com/codex/config-reference/)
- [InfoQ: Codex CLI Rust Rewrite](https://www.infoq.com/news/2025/06/codex-cli-rust-native-rewrite/)
- [OpenReplay: Codex vs Claude Code Comparison](https://blog.openreplay.com/openai-codex-vs-claude-code-cli-ai-tool/)
- [Context Window Issue #9429](https://github.com/openai/codex/issues/9429)
- [Memory Growth Issue #12414](https://github.com/openai/codex/issues/12414)
- [Memory Budget Feature Request #11523](https://github.com/openai/codex/issues/11523)
