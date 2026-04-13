# CLI Capabilities Review: go-agent-harness

Date: 2026-03-30

## Executive Summary

`harnesscli` is a dual-mode CLI with both **non-TUI (streaming) mode** for scripting and pipelines, and an **interactive TUI mode** built with BubbleTea and Lipgloss. The TUI is feature-rich with 10 slash commands, extensive keyboard shortcuts, multiple overlay dialogs, and conversation management. The non-TUI mode is minimal by design—focused purely on event streaming and output piping.

---

## 1. What The CLI Currently Does

`harnesscli` is the client for the `go-agent-harness` service. It can:

1. **Start runs** against a local or remote harness server (default: `http://localhost:8080`)
2. **Stream run events** as SSE (Server-Sent Events) in non-TUI mode
3. **Render an interactive terminal UI** for conversational interaction with extended conversation history
4. **Manage profiles, models, API keys, and conversation state** via the TUI
5. **Export transcripts** to markdown files
6. **Display cost, token, and context usage statistics**
7. **Authenticate** with the harness server (auth helper subcommand)

The main entry point is `cmd/harnesscli/main.go`, which routes to either:
- Non-TUI streaming mode (default)
- TUI mode (`-tui` flag)
- Auth subcommands (`auth login`)
- Utility commands (`-list-profiles`)

---

## 2. Commands and Subcommands

### Top-Level Flags (Non-TUI Mode)

These flags control single-run, streaming behavior (piping-friendly):

| Flag | Type | Purpose |
|------|------|---------|
| `-base-url` | string | Harness API URL (default: `http://localhost:8080`) |
| `-prompt` | string | **Required** (unless `-tui` or `-list-profiles`). The prompt text sent to the harness |
| `-model` | string | Override the model for this run |
| `-system-prompt` | string | Override system prompt for this run |
| `-agent-intent` | string | Startup intent for prompt routing (e.g., `code_review`) |
| `-task-context` | string | Harness task context injected into startup prompt |
| `-prompt-profile` | string | Prompt profile override for model routing |
| `-prompt-behavior` | CSV list | Behavior extension IDs (repeatable or comma-separated) |
| `-prompt-talent` | CSV list | Talent extension IDs (repeatable or comma-separated) |
| `-prompt-custom` | string | Custom prompt extension text |

### Utility Commands (Top-Level)

| Command | Purpose | Example |
|---------|---------|---------|
| `-list-profiles` | Fetch and display available profiles from the server | `harnesscli -list-profiles` |
| `-tui` | Launch interactive terminal UI | `harnesscli -tui` |

### Auth Subcommand

```bash
harnesscli auth login [-server URL] [-tenant TENANT_ID] [-name NAME]
```

Generates an API key locally (for localhost) or via the server. Saves config to `~/.harness/config.json`.

---

## 3. TUI Mode: Slash Commands

When running with `-tui`, the following slash commands are wired into the command registry (10 total):

| Command | Aliases | Description | Effect |
|---------|---------|-------------|--------|
| `/help` | — | Show help dialog | Opens help overlay with 3 tabs: Commands, Keybindings, About |
| `/clear` | — | Clear conversation history | Resets viewport, transcript, thinking state |
| `/context` | — | View context window usage | Opens context grid overlay showing token counts |
| `/stats` | — | Show cost and token statistics | Opens stats panel overlay with per-day cost data |
| `/export` | — | Export conversation to markdown | Exports transcript to file in default export dir |
| `/model` | — | Select AI model | Opens model switcher overlay (2-level: model + reasoning effort) |
| `/keys` | — | Manage provider API keys | Opens API key configuration overlay |
| `/profiles` | — | View and select a profile for next run | Opens profile picker overlay |
| `/subagents` | — | View active subagent processes | Loads and displays subagent status |
| `/quit` | — | Quit the TUI | Cleanly exits the application |

All slash commands are defined in `cmd/harnesscli/tui/cmd_parser.go` in the `builtinCommandEntries()` function.

---

## 4. TUI Mode: Keyboard Shortcuts

| Key(s) | Action | Context |
|--------|--------|---------|
| `Enter` | Submit prompt / run query | Input area |
| `Shift+Enter` / `Ctrl+J` | Insert newline | Input area |
| `Up` / `Ctrl+P` | Scroll up | Viewport |
| `Down` / `Ctrl+N` | Scroll down | Viewport |
| `PgUp` | Page up | Viewport |
| `PgDn` | Page down | Viewport |
| `/` | Slash command mode + autocomplete | Input area |
| `@` | Mention file (autocomplete) | Input area |
| `Esc` | Interrupt / cancel active run | Global |
| `Ctrl+H` / `?` | Open help dialog | Global |
| `Ctrl+C` | Quit | Global |
| `Ctrl+S` | Copy last response to clipboard | Global |
| `Ctrl+O` | Plan mode / expand tool call | Viewport or tool view |
| `Ctrl+E` | Open external editor | Input area |

---

## 5. TUI Mode: Overlay Dialogs and Components

The TUI includes multiple overlay dialogs and UI components:

| Component | Purpose | When Shown |
|-----------|---------|-----------|
| **Help Dialog** (`helpdialog`) | 3-tab help: Commands, Keybindings, About | `/help` or `Ctrl+H` / `?` |
| **Context Grid** (`contextgrid`) | Token usage, model, provider, reasoning info | `/context` |
| **Stats Panel** (`statspanel`) | Cost and token statistics per day | `/stats` |
| **Model Switcher** (`modelswitcher`) | 2-level model + reasoning effort picker | `/model` |
| **API Keys Config** (`configpanel`) | API key entry and provider selection | `/keys` |
| **Profile Picker** (`profilepicker`) | Load and select profiles for next run | `/profiles` |
| **Slash Autocomplete** (`slashcomplete`) | Dropdown command list | When typing `/` |
| **Thinking Bar** (`thinkingbar`) | Shows reasoning/thinking indicator | During model thinking |
| **Tool Use Display** (`tooluse`) | Expanded/collapsed tool call UI with timers | Throughout streaming |
| **Diff View** (`diffview`) | Renders code diffs in tool output | When tools output diffs |
| **Message Bubble** (`messagebubble`) | Renders assistant and user messages | Throughout conversation |
| **Permission Prompt** (`permissionprompt`) | Tool approval request UI | When tool approval needed |
| **Permissions Panel** (`permissionspanel`) | Lists pending tool approvals | When approvals pending |
| **Session Picker** (`sessionpicker`) | Load previous conversations | (component present, usage TBD) |
| **Input Area** (`inputarea`) | Main text input with autocomplete | Always at bottom |
| **Status Bar** (`statusbar`) | Shows run status, model, errors | Always at top |
| **Viewport** (`viewport`) | Scrollable message history | Main content area |
| **Interrupt UI** (`interruptui`) | Cancel confirmation | When run is active |

---

## 6. Non-TUI (Streaming) CLI Mode

### Mode of Operation

When run **without `-tui`**, the CLI:

1. **Parses flags** from command line arguments
2. **Starts a run** via `POST /v1/runs` with the prompt and metadata
3. **Streams SSE events** from `GET /v1/runs/{id}/events`
4. **Outputs events line-by-line** to stdout in the format: `EVENT_TYPE JSON_PAYLOAD`
5. **Exits** after the terminal event is received

### Output Format

```
run_id=<RUN_ID>
assistant.message.delta {...event JSON...}
tool.call.started {...event JSON...}
tool.output.delta {...event JSON...}
run.completed {...event JSON...}
terminal_event=run.completed
```

### Key Design Points

- **Line-by-line output**: Each event is a line with event type prefix, enabling `grep`, `jq`, and pipeline processing
- **No fancy rendering**: Pure JSON, no ANSI colors or terminal UI
- **Pipeable**: Designed to work with shell pipes, `jq`, file redirection, etc.
- **Streaming by default**: Events arrive as they occur; no buffering or pagination
- **Terminal event detection**: Exits cleanly when a terminal event (`run.completed`, `run.failed`, `run.cancelled`, `run.cost_limit_reached`) is received

### Example Usage

```bash
# Basic prompt
harnesscli -prompt "Summarize this repo" | jq '.context_window'

# Pipe events through grep
harnesscli -prompt "Fix the bug" 2>/dev/null | grep "tool.call"

# Custom model and profile
harnesscli -prompt "Review code" -model gpt-4 -prompt-profile advanced

# Behaviors and talents
harnesscli -prompt "Help me" -prompt-behavior safety,vision -prompt-talent coding
```

---

## 7. What's Missing: Gaps vs. Full-Featured CLI Agents

Compared to a mature CLI agent like **Claude Code** or **Hermes CLI**, `harnesscli` lacks:

### Non-TUI Gaps

| Gap | Impact | Severity |
|-----|--------|----------|
| **No interactive input** | Cannot pause for user input mid-run; no `/continue` or `/input` via CLI | Medium |
| **No run control from CLI** | Cannot cancel, steer, or replay runs from command line | Medium |
| **No structured output formats** | No CSV, YAML, or table output options; JSON only | Low |
| **No persistent session management** | No built-in way to list or access previous runs/conversations | Medium |
| **No conversation querying** | Cannot search or filter conversations from CLI | Low |
| **No subagent or cron management** | No CLI to create/list/manage subagents or scheduled jobs | Low |

### TUI Gaps

| Gap | Impact | Severity |
|-----|--------|----------|
| **No slash command aliasing** | No way to create custom command shortcuts | Low |
| **No command history** | No up-arrow recall of previous prompts | Medium |
| **No persistent settings save** | Model selection, gateway, API keys only cached in-session | Low |
| **No theme customization** | No way to override colors or layout from config | Low |
| **No external editor integration** | `Ctrl+E` opens editor but integration is basic | Low |
| **No file attachment/upload** | No way to directly upload or attach files; `@mention` is autocomplete only | Medium |
| **No multi-line command args** | Slash commands don't support escaped newlines or complex strings | Low |
| **No command output formatting** | Overlay data (stats, context) are text-rendered, not tabular | Low |
| **No conversation search** | Cannot search within transcript or across conversations | Medium |
| **No plugin/extension system** | Cannot add custom commands or overlays without recompiling | Medium |

---

## 8. Architecture and File Organization

### Main Files

| File | Purpose |
|------|---------|
| `cmd/harnesscli/main.go` | Entry point; flag parsing; dispatch logic; non-TUI streaming |
| `cmd/harnesscli/auth.go` | Auth subcommand; API key generation and config storage |
| `cmd/harnesscli/config/config.go` | Persistent config loading/saving (profiles, gateway, API keys) |
| `cmd/harnesscli/tui/model.go` | Root BubbleTea model; 2K+ lines; handles all TUI logic |
| `cmd/harnesscli/tui/cmd_parser.go` | Slash command parsing and registry |
| `cmd/harnesscli/tui/keys.go` | Keyboard binding definitions |
| `cmd/harnesscli/tui/api.go` | HTTP client calls to harness server (non-TUI and TUI) |
| `cmd/harnesscli/tui/bridge.go` | SSE event subscription bridge for TUI |
| `cmd/harnesscli/tui/messages.go` | Custom BubbleTea message types |

### Component Directories

All TUI components reside in `cmd/harnesscli/tui/components/`:

- **Layout & Structure**: `layout/`, `viewport/`, `statusbar/`, `inputarea/`
- **Overlays**: `helpdialog/`, `contextgrid/`, `statspanel/`, `modelswitcher/`, `configpanel/`, `profilepicker/`, `permissionprompt/`, `sessionpicker/`
- **Content Rendering**: `messagebubble/`, `tooluse/`, `diffview/`, `thinkingbar/`, `spinner/`
- **Utility**: `slashcomplete/`, `transcriptexport/`, `interruptui/`, `streamrenderer/`, `costdisplay/`, `outputmode/`

---

## 9. Conversation and State Management

### Session Lifecycle

1. **Init**: TUI loads persisted config (models, gateway, API keys) from `~/.harness/config.json`
2. **First Run**: Starts a run, receives `run_id`; sets `conversation_id = run_id` if no prior conversation
3. **Subsequent Runs**: Passes `conversation_id` so harness links them into a single conversation
4. **Export**: `/export` saves transcript to markdown in default export dir
5. **Clear**: `/clear` resets viewport and transcript in-memory only (no persistence)

### Persisted State

- **Model and provider selection**: Starred models stored in config
- **Gateway selection**: Current gateway (direct or openrouter) persisted
- **API keys**: Keyed by provider name, stored encrypted in config
- **Conversation history**: Kept in-memory during session; not auto-saved; requires explicit `/export`

---

## 10. Event Streaming and Real-Time Updates

The TUI uses an **SSE bridge** (`cmd/harnesscli/tui/bridge.go`) to:

1. Subscribe to `GET /v1/runs/{id}/events`
2. Decode SSE events into internal message types
3. Feed into the BubbleTea event loop
4. Update UI components in real-time as events arrive

Tracked event families:

- **Lifecycle**: `run.started`, `run.completed`, `run.failed`, `run.cancelled`
- **Assistant deltas**: `assistant.message.delta`, `assistant.thinking.delta`
- **Tool events**: `tool.call.started`, `tool.call.completed`, `tool.output.delta`
- **Usage**: `usage.delta` (for cost/token stats)
- **Context**: `context.window.snapshot` (for context grid)

---

## 11. Prompt Extensions and Model Selection

### Prompt Extensions

The CLI supports three types of prompt extensions:

1. **Behaviors** (`-prompt-behavior`): Safety, vision, etc.
2. **Talents** (`-prompt-talent`): Coding, writing, etc.
3. **Custom text** (`-prompt-custom`): Arbitrary extension string

All are forwarded to the harness in `prompt_extensions` JSON and enable model/system-prompt routing.

### Model and Provider Selection

- **TUI**: `/model` command opens 2-level picker (model + reasoning effort)
- **Non-TUI**: `-model` flag sets model for the single run
- **Runtime**: Models fetched from `GET /v1/models` and cached; OpenRouter models merged in if enabled
- **Gateway**: Can route all models via OpenRouter (configurable via `/model` overlay)

---

## 12. Cost and Token Tracking

The TUI **does not** send cost/token data itself; it **receives** it from the harness via SSE events:

- **Cost**: `usage.delta` events supply `cost_usd`; accumulated in `cumulativeCostUSD`
- **Tokens**: Context snapshots provide token counts; displayed in `/context` overlay and `/stats` overlay
- **Display**: Stats panel renders per-day cost trends; context grid shows cumulative usage

---

## Summary of Current Capability

| Category | Status | Notes |
|----------|--------|-------|
| **Single-run streaming** | ✓ Full | Works well; pipeline-friendly output |
| **Interactive TUI conversation** | ✓ Full | Rich UI, 10 commands, multiple overlays |
| **Model/provider switching** | ✓ Full | TUI modal picker; CLI flag for single runs |
| **API key management** | ✓ Full | `/keys` overlay; persistent in config |
| **Profile selection** | ✓ Full | `/profiles` command; persistent starred models |
| **Cost/token tracking** | ✓ Full | Real-time stats panel and context grid |
| **Transcript export** | ✓ Full | `/export` saves to markdown |
| **Slash commands** | ✓ Full | 10 commands; extensible registry |
| **Keyboard shortcuts** | ✓ Full | 13 bindings; covers common actions |
| **Tool approval/permission** | ✓ Full | UI overlay for blocking/approving tools |
| **Run control (input/continue/steer)** | ✗ Missing | Not exposed in non-TUI CLI |
| **Conversation search** | ✗ Missing | No search within or across conversations |
| **Persistent command history** | ✗ Missing | No prompt recall across sessions |
| **File attachment UI** | ✗ Missing | `@mention` is autocomplete; not real attachment |
| **Plugin/extension system** | ✗ Missing | No way to add custom commands without recompile |

---

## Conclusion

`harnesscli` is a **dual-mode** agent CLI. The **non-TUI mode** is minimal but solid—ideal for scripts, pipelines, and automation. The **TUI mode** is sophisticated, with rich conversation management, real-time streaming, overlay dialogs, and keyboard-driven navigation. The main gaps are around run control from CLI, conversation search/history, and a plugin system. For a single-shot, streaming agent use case, it is feature-complete. For interactive multiturn conversation with advanced session management, it is also strong. For scripted, batch, or programmatic agent orchestration, the gaps become more apparent.
