# OpenAI Codex CLI Architecture Research

Research date: 2026-03-05
Source repository: https://github.com/openai/codex (63k+ stars, Apache-2.0)

## Overview

Codex CLI is OpenAI's local coding agent, rewritten in **Rust** (96% of codebase). It runs locally, interfaces with OpenAI's Responses API, and provides sandboxed tool execution. The codebase spans 65+ Rust crates organized into user-facing binaries (CLI/TUI, headless exec, app-server, MCP server), core logic, and tool systems.

---

## 1. Agent Loop Architecture

### Core Loop Pattern

The agent loop follows a classical agentic cycle:

```
User Input -> Prompt Construction -> Model Streaming (SSE) -> Tool Dispatch -> Append Results -> Re-query Model -> ... -> Final Assistant Message
```

A single "turn" can involve hundreds of model-tool iterations. The loop terminates when the model produces an assistant message without requesting any tool calls.

### Key Implementation Details

- **Session-scoped execution**: A `Session` represents an initialized conversation with at most one running task at a time.
- **Queue-based concurrency**: Operations (`Op`) are submitted asynchronously via a sender channel (`tx_sub`), and results stream back as `EventMsg` via a receiver (`rx_event`). This decouples submission from execution, enabling interrupts and async workflows.
- **Task types within a turn**:
  - `RegularTask` -- standard agent turns with model interaction
  - `ReviewTask` -- review-specific workflows
  - `GhostSnapshotTask` -- internal snapshots
- **Event streaming**: Long-running operations emit incremental events (`response.output_text.delta` for UI streaming, `response.output_item.added` for state objects), enabling real-time TUI updates.

### Go Harness Takeaway

The `Op`/`EventMsg` protocol pattern is powerful -- it creates a clean boundary between frontends and the core agent. Any frontend (TUI, CLI, IDE plugin, MCP server) can drive the same core via this protocol. Consider defining a similar `Submission`/`Event` interface in Go using channels.

---

## 2. Sandboxing and Tool Execution

### Platform-Specific Sandbox Backends

Codex uses OS-level sandboxing, not containers:

| Platform | Technology | Details |
|----------|-----------|---------|
| Linux    | Bubblewrap + Landlock + seccomp | Process isolation with filesystem and syscall filtering |
| macOS    | Apple Seatbelt profiles (`.sbpl` files) | Sandbox profiles restrict filesystem/network access |
| Windows  | AppContainer | Windows sandboxing primitive |

### Sandbox Modes

Sandbox enforcement is orthogonal to approval policy:

- **`workspace-write`** (default in `--full-auto`): Can read/write in the working directory, but nothing outside it
- **`danger-full-access`**: Disables sandbox restrictions entirely
- Custom sandbox profiles can restrict network, filesystem, and process capabilities independently

### Execution Flow

The `ToolOrchestrator` manages a two-attempt execution pattern:

1. **First attempt**: Run tool in sandboxed mode
2. **If sandbox denies**: Request re-approval from user, then retry with escalated (unrestricted) privileges
3. Approval decisions are cached to avoid re-prompting for the same operation type

### Scope Limitation

Only Codex-provided tools (shell, apply_patch, etc.) are sandboxed. MCP tools from external servers are **not** sandboxed by Codex and must enforce their own guardrails.

### Go Harness Takeaway

The two-attempt pattern (sandboxed first, escalate on denial) is elegant. For Go, consider using `exec.Cmd` with platform-specific wrappers: on macOS use `sandbox-exec` with Seatbelt profiles, on Linux use Bubblewrap or namespaces. The key insight is that sandbox enforcement and approval policy are separate concerns that compose independently.

---

## 3. Multi-Turn / Conversation Continuity

### Stateless Request Architecture

Codex deliberately avoids using `previous_response_id` (server-side conversation state). Instead, every request contains the **full conversation history**. This is a deliberate design choice for:

- Zero Data Retention (ZDR) compliance
- Data privacy guarantees
- No server-side state dependency

The tradeoff: quadratic JSON overhead as conversations grow.

### Prompt Caching

To mitigate the quadratic cost, Codex relies heavily on **prompt caching**:

- Cache hits require **exact prefix matches** in the prompt
- Static content (system instructions, tool definitions) is placed **before** variable content
- Configuration changes mid-conversation (tools, models, working directory) cause expensive cache misses
- Mid-conversation config changes append new messages rather than modifying earlier ones, preserving the prefix

### Automatic Compaction

When token count exceeds a threshold:

1. A specialized `/responses/compact` endpoint summarizes the conversation
2. Returns a list of items replacing the original input
3. Includes a special `type=compaction` item with `encrypted_content` that preserves "the model's latent understanding of the original conversation"
4. This converts quadratic overhead back to manageable linear growth

### Go Harness Takeaway

For a Go harness using any LLM API:
- **Always send full history** rather than relying on server-side state (more portable, more debuggable)
- **Implement compaction** as a separate concern -- when context exceeds N tokens, summarize older messages
- **Order prompt components carefully**: static prefix (system prompt, tool defs) then dynamic content (conversation history) to maximize cache hits
- The `encrypted_content` approach is OpenAI-specific, but the general pattern of summarization + a compressed state blob is replicable

---

## 4. Tool Catalog and Tool Definition

### Built-in Tools

The default solver tool catalog:

| Tool | Purpose |
|------|---------|
| `shell` / `run_terminal_cmd` | Execute shell commands (Bash/Zsh/PowerShell) |
| `apply_patch` | Create, update, or delete files via structured diffs |
| `read_file` | Read file contents |
| `list_dir` | List directory contents |
| `glob_file_search` | Search for files by glob pattern |
| `rg` (ripgrep) | Search file contents |
| `git` | All git operations |
| `todo_write` / `update_plan` | Task tracking and planning |
| `js_repl` | JavaScript REPL for computation |
| `web_search` | Web search capability |
| `image_gen` | Image generation |

### Tool Definition Schema

Tools are defined as Rust structs with JSON Schema parameter specifications:

```
ToolsConfig {
    shell_backend: ShellBackend,   // Classic, ZshFork, Direct, UnifiedExec
    feature_flags: {               // web_search, image_gen, js_repl, etc.
        collab_tools: bool,
        artifact_tools: bool,
        agent_jobs_tools: bool,
    }
}
```

Each tool has:
- A factory function: `create_[tool_name]_tool() -> Tool`
- JSON Schema parameters with types, descriptions, required fields
- A `build_specs()` function generates the final tool manifest based on model capabilities, feature flags, session context, and MCP server availability

### Tool Routing

The `ToolRouter` dispatches calls based on `ToolPayload` variants:
- `Function` -- standard function calls
- `Mcp` -- MCP server tools
- `Custom` -- custom tool implementations
- `LocalShell` -- shell execution

A security gate (`js_repl_tools_only` mode) can restrict direct tool calls, forcing everything through the JS REPL as an intermediary.

### Go Harness Takeaway

The factory pattern (`create_X_tool()`) with JSON Schema specs is clean and portable to Go. Consider defining tools as:
```go
type Tool struct {
    Name        string
    Description string
    Parameters  jsonschema.Schema
    Handler     func(ctx context.Context, params json.RawMessage) (ToolResult, error)
}
```
The conditional tool inclusion via `build_specs()` based on model capabilities is worth replicating -- not all models support all tools.

---

## 5. Approval / Permission Model

### Three Approval Modes

| Mode | Flag | Behavior |
|------|------|----------|
| **Suggest** | `--suggest` | Agent proposes actions, user must approve everything |
| **Auto-edit** | `--auto-edit` | File edits auto-approved, commands need approval |
| **Full-auto** | `--full-auto` | Everything auto-approved within sandbox constraints |

`--full-auto` is an alias for `--sandbox workspace-write --ask-for-approval on-request`.

### Approval Policy Architecture

The `ToolOrchestrator` evaluates approval requirements:

1. `exec_approval_requirement()` checks if the operation needs user consent
2. User can approve, reject, or amend network policies
3. Approved decisions are cached for similar future operations
4. `DeferredNetworkApproval` handles async permission for network-accessing operations

### Auto-Reject Policies

Fine-grained control via config:
```toml
[approval_policy.reject]
sandbox_escalation = true      # Auto-reject sandbox escapes
execpolicy_rule = true         # Auto-reject policy violations
mcp_elicitations = true        # Auto-reject MCP prompts
```

### Go Harness Takeaway

The two-axis model (sandbox level x approval policy) is the key insight. These are independent dimensions:
- **Sandbox**: What *can* the agent do? (filesystem scope, network access, process capabilities)
- **Approval**: What *should* the agent ask about? (everything, just commands, nothing)

Implement these as separate middleware layers in Go. The caching of approval decisions is also important for UX -- don't ask the same question twice.

---

## 6. Novel / Clever Architectural Patterns

### 6.1 Protocol Abstraction (`Op`/`EventMsg`)

The internal protocol completely isolates frontend implementations from core logic. The app-server translates between JSON-RPC v2 and the internal protocol. This means TUI, headless CLI, IDE plugins, and MCP servers all share identical core behavior.

**Go equivalent**: Define a `harness.Submission` / `harness.Event` interface. All frontends (CLI, HTTP API, gRPC) produce submissions and consume events through the same channel-based interface.

### 6.2 Dual-Role MCP (Client + Server)

Codex functions as **both** an MCP client (consuming external tools) and an MCP server (exposing its tools to external clients). This bidirectional MCP integration means Codex tools are composable with the broader MCP ecosystem.

**Go opportunity**: Implement MCP client support to consume external tool servers, and optionally expose your harness tools as an MCP server for IDE integration.

### 6.3 Configuration Layer Stack

Six-priority configuration cascade:
1. Built-in defaults
2. `~/.codex/config.toml` (user global)
3. `.codex/config.toml` (project-specific)
4. Named profiles (fast/slow/custom)
5. CLI overrides (`--set key=value`)
6. Cloud requirements (highest priority, server-enforced)

`ConfigLayerStack` manages validation and constraint enforcement. Cloud requirements can restrict values for managed enterprise environments.

**Go equivalent**: Use a layered config approach (e.g., Viper with multiple sources) but add the concept of "cloud constraints" that override local config for enterprise/team deployments.

### 6.4 Head-Tail Buffer for Process Output

The `head_tail_buffer.rs` in `unified_exec` maintains both the beginning and end of process output. This is clever for long-running commands -- you capture the initial output (often containing errors/headers) and the final output (results/exit codes) without storing everything in between.

**Go implementation**: A ring buffer that keeps the first N and last M lines of output, discarding the middle.

### 6.5 Rollout Recorder (JSONL Event Log)

`RolloutRecorder` persists all events to JSONL format, enabling:
- Session history reconstruction
- Turn replay and branching (resume from any point, fork conversations)
- Audit trails for all executed operations

**Go equivalent**: Append-only JSONL log of all events. This is trivially implementable and enormously valuable for debugging, replay, and audit.

### 6.6 Session-Scoped Model Client with Sticky Routing

`ModelClient` lifetime matches conversation sessions. `ModelClientSession` scopes to individual turns with sticky routing headers for server affinity. This ensures consistent model behavior within a conversation even across a load-balanced API fleet.

### 6.7 Skills and Memories System

The codebase includes `skills/` and `memories/` modules, suggesting persistent agent capabilities:
- **Skills**: Reusable agent behaviors (stored in `.codex/skills/`)
- **Memories**: Cross-session persistence of learned context

---

## 7. Summary: Key Patterns for Go Harness

| Pattern | Codex Approach | Go Harness Recommendation |
|---------|---------------|--------------------------|
| Core abstraction | `Op`/`EventMsg` queue protocol | Channel-based `Submission`/`Event` with interface types |
| Agent loop | Streaming SSE with tool dispatch | goroutine-based loop with `select` on channels |
| Sandboxing | OS-native (Seatbelt, Bubblewrap) | `exec.Cmd` with platform wrappers; start with macOS `sandbox-exec` |
| Conversation state | Stateless full-history per request | Same -- send full history, implement compaction |
| Tool definition | JSON Schema factory functions | Go structs with `jsonschema` tags and handler functions |
| Approval | Two-axis (sandbox x approval) with caching | Middleware layers with decision cache |
| Persistence | JSONL rollout recorder | Append-only JSONL event log |
| Configuration | 6-layer priority cascade | Viper or similar with layered sources |
| MCP | Bidirectional (client + server) | MCP client for tool consumption; optional server for IDE |
| Process output | Head-tail buffer | Ring buffer keeping first N + last M lines |

---

## Sources

- [OpenAI Codex GitHub Repository](https://github.com/openai/codex)
- [Unrolling the Codex Agent Loop (OpenAI Blog)](https://openai.com/index/unrolling-the-codex-agent-loop/)
- [Codex CLI Security Documentation](https://developers.openai.com/codex/security/)
- [Codex CLI Features](https://developers.openai.com/codex/cli/features/)
- [Codex CLI Configuration Reference](https://developers.openai.com/codex/config-reference/)
- [ZenML Analysis: Building Production-Ready AI Agents](https://www.zenml.io/llmops-database/building-production-ready-ai-agents-openai-codex-cli-architecture-and-agent-loop-design)
- [DeepWiki: openai/codex](https://deepwiki.com/openai/codex)
- [Codex CLI Command Line Reference](https://developers.openai.com/codex/cli/reference/)
- [OpenAI Apply Patch Tool Documentation](https://platform.openai.com/docs/guides/tools-apply-patch)
- [InfoQ: OpenAI Begins Article Series on Codex CLI Internals](https://www.infoq.com/news/2026/02/codex-agent-loop/)
