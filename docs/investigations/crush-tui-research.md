# Crush TUI Research

**Date**: 2026-03-10
**Subject**: Charmbracelet Crush — Terminal UI AI Coding Agent
**Source**: GitHub charmbracelet/crush, DeepWiki, release notes, user reviews

---

## Overview

Crush is an open-source, terminal-based AI coding agent built by Charmbracelet (the team behind Bubble Tea, Glow, Charm). It is written in Go and released under the name "the glamourous AI coding agent for your favourite terminal." It was previously known as OpenCode before rebranding. The project is at github.com/charmbracelet/crush.

---

## TUI Framework

- **Primary framework**: Bubble Tea v2 (github.com/charmbracelet/bubbletea) — Charmbracelet's own Elm-Architecture-inspired TUI framework for Go
- **Styling**: Lipgloss (github.com/charmbracelet/lipgloss) for layout composition via `lipgloss.JoinVertical()` and `lipgloss.JoinHorizontal()`
- **Markdown rendering**: Glamour (github.com/charmbracelet/glamour) for rich terminal markdown with a custom Chroma-based syntax theme called "charmtone"
- **Color system**: charmtone package (`github.com/charmbracelet/x/exp/charmtone`) — named semantic colors (Charple, Dolly, Pepper, Ash, Sriracha, Malibu, Julep, Coral)
- **Gradient rendering**: `go-colorful` for perceptual color interpolation; `uniseg` for grapheme cluster Unicode handling
- **Storage**: SQLite via pressly/goose migrations for sessions, messages, and file history

---

## Architecture: "Smart Model, Dumb Components"

Crush uses a deliberate "intelligent main model, simplified components" pattern:

- The single root `UI` struct in `internal/ui/model/ui.go` is the **only** Bubble Tea `Update()` handler
- All sub-components expose method APIs (`SetSize()`, `Focus()`, `Blur()`) returning `tea.Cmd` rather than implementing their own update cycles
- Shared resources injected via `common.Common` struct (styles, app services, config, terminal capabilities)
- A centralized `pubsub` event bus connects the backend agent loop to the TUI reactively

### UI State Machine

Four sequential application states control what view renders:
1. `onboarding` — first-run API key setup
2. `initialize` — loading session and configuration
3. `landing` — session selection / start screen
4. `chat` — active conversation view

---

## Layout and Panels

### Three-Panel Chat Layout

The chat page manages focus among three panels:

1. **Chat/message display area** — scrollable conversation history with lazy-rendered message items (user, assistant, tool variants)
2. **Sidebar** — shows file changes, model info, cost tracking, LSP diagnostics, MCP status; can be shown as overlay in compact mode
3. **Input/editor area** — multi-line textarea with file path completions and attachment support

### Responsive Breakpoints

- Width < 120 OR height < 30 triggers **compact mode** automatically
- In compact mode the sidebar appears as an overlay (toggle with `ctrl+d`) rather than a persistent panel
- Force compact mode: `ctrl+shift+c`
- A new UI refactor is in progress, accessible via `CRUSH_NEW_UI=1` environment variable

### Diff View

- Configurable diff mode: `"unified"` or `"split"` (set in `options.diff_mode` config)
- Diff scrolling supports left/right navigation
- Copy in diff view: `c` or `y` key bindings

---

## Keyboard Shortcuts

Based on documentation, issue tracker, and user reports:

| Shortcut | Action |
|---|---|
| `ctrl+p` | Open command palette (conflicts with Zellij; configurable keybindings requested but not yet shipped as of mid-2025) |
| `ctrl+g` | Focus chat input area |
| `ctrl+s` | Session management panel |
| `ctrl+f` | File attachment picker |
| `ctrl+o` | Open external editor |
| `ctrl+d` | Toggle sidebar overlay (compact mode) |
| `ctrl+t` | Toggle todo list (changed from `ctrl+space` in v0.42.0 for terminal compatibility) |
| `ctrl+c` | Exit application |
| `ctrl+a` | Line start in text input |
| `ctrl+e` | Line end in text input |
| `tab` | Trigger file path auto-completion in input |
| `c` / `y` | Copy in diff view |

Note: Keybindings are currently hardcoded (configurable keybindings are a heavily requested feature, tracked in issues #737 and #836). Emacs-style bindings requested in issue #440.

---

## Input and Completion System

- **Multi-line textarea** handles user input with file attachment support
- **Slash-completion**: typing `/` triggers file path completion in the input field; slash-completing an image file attaches it automatically (issue #996)
- **Paste detection**: pasting 10+ lines is treated as a file attachment rather than inline text
- **Drag-and-drop**: supports dropping multiple files onto the TUI simultaneously (added in v0.36.0, Windows-compatible)
- **Stdin piping**: `crush run "prompt" < file.go` or `curl ... | crush run "summarize"`
- No shell `!command` prefix mode (tracked as feature request in issue #680, comparing to gemini-cli's `!` syntax)

---

## Slash Commands and Special Input Modes

Crush does **not** have a traditional slash command system for user actions. Instead:

- **Natural language** is the primary interaction mode
- **Command palette** (`ctrl+p`) provides discoverable UI-level commands (model switching, session management, summarization)
- **Slash file completion** (`/path`) is used for file references in input, not command dispatch
- **`crush run` non-interactive mode**: automation via shell scripting without entering the TUI
- **`--yolo` flag**: skips all permission prompts (equivalent to an "autonomous mode")
- **`crush stats`**: usage statistics dashboard (added v0.36.0)
- **`crush models [search]`**: list available models
- **`crush dirs`**: show config/data directory paths
- **`crush logs`**: recent log output
- **`crush login`**: OAuth authentication flow

---

## Streaming and Real-Time Output

- Streaming uses incremental callbacks via `fantasy.Agent.Stream` with an `AgentStreamCall` struct
- Callback handlers:
  - `OnTextDelta` — appends text increments, strips leading newlines on first chunk
  - `OnReasoningStart/Delta/End` — handles extended thinking from Claude, Gemini, o1/o3
  - `OnToolCall/OnToolResult` — manages tool execution within streaming flow
  - `OnStepFinish` — updates session usage stats after each provider turn
- Each callback persists immediately to SQLite, enabling real-time TUI updates via pubsub
- The UI subscribes to pubsub channels; events are sent to Bubble Tea via `program.Send()` (no polling)
- Streaming responses appear incrementally in the chat area as the agent thinks and generates

---

## Multi-Turn Conversation Support

- **Full session persistence**: all messages stored in local SQLite (`crush.db`)
- **Session management**: multiple named sessions per project, switchable via `ctrl+s` panel
- **Context window management**: automatic summarization when approaching model token limits
  - Large models (>200k context): 20k token buffer before triggering
  - Smaller models: 20% of context window as buffer
  - Summary stored as a special message; subsequent calls only load messages from that point forward
  - Earlier messages preserved in DB but excluded from LLM context
- **Coordinator-based sequencing**: `agent.Coordinator` queues prompts per session (one at a time)
- **Mid-session model switching**: switch LLMs while preserving full conversation history
- **Active session tracking**: `SessionService` maintains current session state across restarts

---

## Terminal Rendering

### Markdown and Syntax Highlighting

- Glamour renders assistant markdown with a full charmtone color theme
- Headings: charmtone Malibu color
- Inline code: charmtone Coral on Charcoal background
- Code blocks: 26-token Chroma ruleset exported via `ChromaTheme()` method
- "Thinking" / reasoning content blocks: rendered with `PlainMarkdown` variant — muted colors (`fgMuted` on `bgBaseLighter`) so they visually recede from main content
- True color, 256-color, ANSI, and no-color terminal profiles all supported via capability detection

### Terminal Capability Detection

Crush actively detects terminal features:
- `tea.EnvMsg` — reads `TERM`, `COLORTERM`, environment variables
- `tea.TerminalVersionMsg` — version string queries
- `tea.KeyboardEnhancementsMsg` — enhanced keyboard support (for chords, modifier keys)
- **Kitty Graphics Protocol** detection for image display in terminals that support it
- `CellSize()` for image scaling calculations

### Gradient Rendering

Three functions for horizontal foreground gradients:
- `ForegroundGrad()` — per-grapheme-cluster ANSI coloring
- `ApplyForegroundGrad()` — ready-to-print joined string
- `ApplyBoldForegroundGrad()` — gradient with bold

---

## Tool Output Display

- Tools render as distinct message items in the chat list (separate from user and assistant messages)
- Tool execution flow: result persisted to DB → broadcast via pubsub → TUI receives event → rendered as tool result item in message list
- **Permission dialogs**: modal overlay appears before shell or file write operations (can be bypassed with `--yolo`)
- **Permission dialog stack**: layered modal system supports multiple overlapping dialogs (e.g., permission dialog over model selector)
- Tool categories: bash execution, file view/edit/write/multi-edit, ls/glob/grep, LSP queries, MCP resource access, network fetch, recursive sub-agent invocation
- Tool availability varies by agent type: **Coder Agent** gets full access; **Task Agent** gets read-only tools only
- LSP diagnostics appear in the sidebar, updated reactively as LSP servers report

---

## Project Context Files

- `.crush.json` — project-local config (highest priority)
- `crush.json` — project config (alternative name)
- `~/.config/crush/crush.json` — global user config
- `.crushignore` — gitignore-syntax file exclusions for context
- **Skills system**: folders containing `SKILL.md` that Crush discovers and activates on demand (Agent Skills open standard)
- `crush.md` — project-level instructions file (similar to CLAUDE.md), loaded to give the agent project context

---

## Unique and Notable TUI Features

1. **Real-time cost tracking**: current session API cost displayed in the sidebar with live updates
2. **Model display**: current model name always visible; model switching mid-session preserves context
3. **Extended thinking display**: reasoning blocks from Claude/Gemini/o1 rendered in muted `PlainMarkdown` style to visually distinguish from main response
4. **Animated status indicators**: `StepMsg`/`TickMsg` drive progress animations during tool execution and agent thinking
5. **Mouse support**: click and scroll interactions in dialogs and message list; scroll threshold is 5 lines
6. **Usage statistics dashboard**: `crush stats` command shows per-project usage in a visual chart
7. **File change history**: `HistoryService` tracks all files read/written during session; displayed in sidebar
8. **Sub-agent support**: recursive agent invocation where nested agents appear as message children in the chat
9. **MCP integration**: MCP servers (stdio, HTTP, SSE transports) contribute tools that appear alongside built-in tools with permission gating
10. **LSP UI indicator**: configured LSP servers appear in UI marked "unstarted" until first activated (added v0.42.0)
11. **Session title editing**: session titles can be renamed after creation
12. **Session deletion**: sessions deletable from the session management panel

---

## Comparison to OpenCode TUI

The user question references OpenCode's TUI (which uses Bubble Tea, leader key `ctrl+x`, `@file` refs, `!command` prefix, sidebar). Crush is in fact the renamed successor to OpenCode — the same project, rebranded by Charmbracelet. Key differences/evolutions:

| Feature | OpenCode (legacy) | Crush (current) |
|---|---|---|
| Framework | Bubble Tea | Bubble Tea v2 |
| Leader key | `ctrl+x` | `ctrl+p` (command palette) |
| File references | `@file` prefix | `/` slash-completion + `ctrl+f` picker |
| Shell command prefix | `!command` | Not implemented (feature request #680) |
| Sidebar | Persistent | Responsive: persistent or overlay based on terminal size |
| Theming | — | Full charmtone palette, single unified theme |
| Model switching | Per-session start | Mid-session, context preserved |
| Context management | Manual | Automatic summarization at token threshold |
| Image support | — | Kitty Graphics Protocol + file drag-drop |
| Extended thinking | — | `PlainMarkdown` muted rendering for reasoning blocks |

---

## Sources

- [GitHub: charmbracelet/crush](https://github.com/charmbracelet/crush)
- [DeepWiki: TUI Architecture](https://deepwiki.com/charmbracelet/crush/5.1-tui-architecture)
- [DeepWiki: TUI Architecture and appModel](https://deepwiki.com/charmbracelet/crush/5.1-tui-architecture-and-appmodel)
- [DeepWiki: Styling System](https://deepwiki.com/charmbracelet/crush/5.8-styling-system)
- [DeepWiki: Streaming and Auto-Summarization](https://deepwiki.com/charmbracelet/crush/4.5-streaming-and-auto-summarization)
- [DeepWiki: Conversation Management](https://deepwiki.com/charmbracelet/crush/4.4-conversation-management-and-summarization)
- [DeepWiki: CLI Usage](https://deepwiki.com/charmbracelet/crush/2.2-cli-usage)
- [DeepWiki: Tool System / MCP Integration](https://deepwiki.com/charmbracelet/crush/6.3-mcp-tool-integration)
- [DeepWiki: Configuration](https://deepwiki.com/charmbracelet/crush/2.2-configuration)
- [Release v0.36.0](https://github.com/charmbracelet/crush/releases/tag/v0.36.0)
- [Release v0.42.0](https://github.com/charmbracelet/crush/releases/tag/v0.42.0)
- [The New Stack: Review of Crush (Ex-OpenCode AI)](https://thenewstack.io/terminal-user-interfaces-review-of-crush-ex-opencode-al/)
- [Testing out Crush (graham helton)](https://grahamhelton.com/blog/crushing-it)
- [Does Developer Delight Matter — tessl.io](https://tessl.io/blog/does-developer-delight-matter-in-a-cli-the-case-of-charm-s-crush/)
- [Crush CLI overview — atalupadhyay.wordpress.com](https://atalupadhyay.wordpress.com/2025/08/12/crush-cli-the-next-generation-ai-coding-agent/)
- [Issue #737: configurable keybindings](https://github.com/charmbracelet/crush/issues/737)
- [Issue #836: ability to configure key bindings](https://github.com/charmbracelet/crush/issues/836)
- [Issue #680: local command execution / shell mode](https://github.com/charmbracelet/crush/issues/680)
- [Issue #996: slash-completion of image files](https://github.com/charmbracelet/crush/issues/996)
