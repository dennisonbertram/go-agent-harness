# TUI REPL Research: OpenCode and Cursor

**Date**: 2026-03-10
**Purpose**: Document TUI/interactive features of OpenCode and Cursor for reference in go-agent-harness REPL/CLI development.

---

## OpenCode

**Project**: https://github.com/opencode-ai/opencode | https://opencode.ai/docs/tui/
**Status**: Archived September 18, 2025 (read-only). Was a Go-based TUI, later rewrote to TypeScript with OpenTUI framework.
**Source**: Originally built in Go using Bubble Tea; the later/current version migrated to TypeScript with `@opentui/core` + Solid.js.

### Architecture

OpenCode uses a two-layer architecture:
- **Backend**: Go server exposing REST + SSE (Server-Sent Events) HTTP endpoints for streaming
- **TUI frontend**: Originally Bubble Tea (Go), later migrated to `@opentui/core` (TypeScript) + Solid.js for reactive rendering
- **Rendering**: Shiki for syntax highlighting, Marked for markdown parsing
- **Streaming**: SSE events (`message.updated`, `part.delta`) deliver incremental content to the TUI

The TUI follows the Elm Architecture (Model-Update-View) as implemented by Bubble Tea. The root `Model` struct contains all UI components and delegates events to them.

### UI Components / Panels

- **Chat Display** - scrollable message history with role-colored rendering (user/assistant/system)
- **Multi-line Prompt Input** - textarea with max 6-line height, Readline/Emacs-style editing
- **Sidebar** (visible at terminal width >120 columns, toggleable):
  - Session title + share URL
  - Token count and cost metrics
  - MCP server status indicators
  - Active language server list
  - Todo extraction from messages
  - Modified files with diff statistics
  - Current directory and version info
  - Dismissable onboarding panel
- **Diff Viewer** - shows proposed file changes in green/red, configurable stacked vs. auto style
- **Dialog Overlays**:
  - Model selection (`DialogModel`)
  - Agent switching (`DialogAgent`)
  - Session list (`DialogSessionList`)
  - Help / keybind reference (`DialogHelp`)
  - System status (`DialogStatus`)
  - Provider connection management (`DialogProviderList`)
  - Permission prompts for tool execution approval
- **Log Viewer** - for debugging
- **External Editor Integration** - opens `$EDITOR` (VS Code, Neovim, Vim, Nano, etc.) for composing long messages; GUI editors require `--wait` flag

### Keyboard Shortcuts

Leader key: `ctrl+x` (configurable). Avoids terminal conflicts. All major actions require leader + key.

**Navigation / Session:**
| Keybind | Action |
|---------|--------|
| `<leader>q`, `ctrl+c`, `ctrl+d` | Exit |
| `<leader>n` | New session |
| `<leader>l` | Session list |
| `<leader>g` | Session timeline |
| `<leader>b` | Toggle sidebar |
| `<leader>s` | Status view |
| `<leader>x` | Export session |
| `<leader>c` | Compact session (summarize) |
| `escape` | Interrupt running operation |
| `<leader>down/up/left/right` | Child session navigation |

**Message Scrolling:**
| Keybind | Action |
|---------|--------|
| `pageup/pagedown`, `ctrl+alt+b/f` | Page up/down |
| `ctrl+alt+y/e` | Line up/down |
| `ctrl+alt+u/d` | Half page |
| `ctrl+g / ctrl+alt+g`, `home/end` | First/last message |
| `<leader>y` | Copy message |
| `<leader>u / <leader>r` | Undo / Redo (Git-backed) |
| `<leader>h` | Toggle conceal (hide/show code) |

**Model & Agent:**
| Keybind | Action |
|---------|--------|
| `<leader>m` | Model list |
| `f2 / shift+f2` | Cycle models |
| `<leader>a` | Agent list |
| `tab / shift+tab` | Cycle agents |
| `ctrl+t` | Variant cycle |
| `ctrl+p` | Command palette |

**Prompt Input (Readline/Emacs-style):**
| Keybind | Action |
|---------|--------|
| `ctrl+a / ctrl+e` | Line home/end |
| `left/right`, `ctrl+b/f` | Move cursor |
| `alt+left/right`, `alt+b/f` | Move by word |
| `ctrl+shift+d` | Delete line |
| `ctrl+k / ctrl+u` | Delete to end/start of line |
| `alt+d`, `ctrl+w`, `alt+backspace` | Delete word |
| `home/end` | Buffer home/end |
| `return` | Submit |
| `shift+return`, `ctrl+return`, `alt+return`, `ctrl+j` | Insert newline |
| `ctrl+c` | Clear input |
| `ctrl+v` | Paste |
| `ctrl+- / .`, `super+z / shift+z` | Undo / Redo |
| `up/down` | History prev/next |
| `ctrl+z` | Terminal suspend |

### Input Modes / Slash Commands

**Slash commands** (typed in prompt with `/`):
- `/connect` - Add provider credentials
- `/compact` - Summarize/compress session context
- `/details` - Toggle tool execution visibility
- `/editor` - Open external editor
- `/export` - Save conversation as markdown
- `/models` - List available LLM options
- `/new` - Start fresh session
- `/sessions` - Switch sessions
- `/undo` / `/redo` - Revert changes (Git-backed)
- `/share` - Enable session sharing via URL
- `/thinking` - Display model reasoning blocks (thinking tokens)
- `/sidebar` - Toggle sidebar

**Special input prefixes:**
- `@filename` - Fuzzy file reference; includes file content in context. Supports line ranges: `@file.ts#10-20`
- `@folder/` - Directory navigation in autocomplete
- `@agent` - Switch to a specific agent
- `!command` - Execute shell command directly; output becomes tool result
- `/skill-name` - Invoke a configured skill

### Autocomplete System

A multi-trigger autocomplete fires on `@`, `/`, with frecency-ranked file suggestions, MCP resource listing, and agent/command enumeration. File search uses fuzzy matching against the working directory.

### Streaming / Real-time Output

- SSE streaming from the backend delivers `part.delta` events incrementally
- The TUI auto-scrolls as content arrives (state machine-managed)
- A `PartCache` optimizes re-renders: cache hits skip expensive markdown re-rendering
- Spinners and status indicators show during active tool execution
- Thinking/reasoning blocks can be toggled with `/thinking`

### Multi-turn Conversation Support

Full multi-turn support with persistent sessions. Sessions are listed, switchable, and have a timeline view. Sessions can be exported as markdown, shared via URL, or compacted (summarized to reduce context size).

### Terminal Rendering

- **30+ built-in themes** with light/dark mode support
- **System theme generation** via OSC queries to terminal color palette
- **Real-time theme switching** without restart (`<leader>t`)
- **Markdown rendering**: Full support via Marked parser
- **Syntax highlighting**: Shiki-based, 62 color properties covering keywords, strings, comments, diffs
- **Diff visualization**: Stacked or auto style, green/red coloring
- **Code concealment**: Toggle visibility of code blocks (`<leader>h`)
- **Diff wrapping**: Word wrap or no-wrap mode toggle

### Tool Output Display

- Tool execution results rendered inline in chat
- Toggle tool detail visibility (`/details`)
- Permission prompts for sensitive tool operations (user must approve before execution)
- Generic tool output toggle
- Git-backed undo/redo for file changes made by tools

### Non-Interactive (Script) Mode

```bash
opencode -p "prompt"        # run without TUI
opencode -p "prompt" --json # JSON output format
opencode -p "prompt" -q     # quiet mode
```

### Notable / Unique Features

- **Git-backed undo/redo**: File changes by the LLM can be reverted using `/undo` and `/redo`, which wrap Git operations
- **Session sharing**: Generate a shareable URL for a conversation
- **MCP integration**: MCP server status shown in sidebar; MCP resources available in `@` autocomplete
- **Cost display**: Token count and USD cost shown per-session in sidebar
- **Wide-terminal sidebar**: Sidebar auto-shows at >120 columns and can be pinned
- **Prompt stashing**: Draft prompts can be saved/restored
- **Skills integration**: Custom slash commands via Markdown skill files
- **Multi-agent**: Agent switching mid-session via `@agent` or `<leader>a`

---

## Cursor

**Project**: https://cursor.com | https://cursor.com/docs
**Type**: GUI code editor (VS Code fork with AI layer). NOT a terminal/TUI tool — it is a desktop GUI application. Has a terminal integration and a CLI, but its primary interface is graphical.

### Architecture

Cursor is a VS Code fork extended with an AI layer. It runs as a native desktop application (Electron-based). The AI interactions happen in a chat sidebar panel within the GUI. Cursor does not have a standalone TUI mode — it is fundamentally a GUI editor with terminal integration.

**AI interaction components:**
- Chat sidebar (right panel, Cmd+L)
- Inline AI edit bar (Cmd+K, appears inline in editor)
- Command palette (Cmd/Ctrl+Shift+P)
- Agent sidepane (Cmd+I)
- Background Agent (cloud-icon or Cmd/Ctrl+E)

### UI Components / Panels

**Three-panel layout** (fixed, limited customization):
- **Left sidebar**: File explorer, source control, extensions
- **Center**: Code editor with inline AI diff display
- **Right panel**: AI chat / agent interface

**Chat/Agent panel contains:**
- Multi-turn conversation history
- Streaming AI responses
- Tool execution output (file edits, terminal commands, search results)
- Checkpoint restore controls
- Pinned conversations
- Message queue display

**Bottom panel:**
- Integrated terminal (split-pane support)
- Problems / Output / Debug Console
- Ask Mode terminal read access

**Overlay / Inline UI:**
- Inline diff display (green/red) for AI-suggested edits
- Inline edit bar (Cmd+K) for localized code changes
- Plan Mode document view (saved as markdown files)
- Mermaid diagram rendering in plan documents
- Interactive UIs in agent chats (March 2026 addition)
- Visual Editor overlay (browser-mode for CSS/layout editing)

### Agent Modes

Cursor has four distinct agent modes, selectable via the mode picker or keyboard:

| Mode | Purpose | Activation |
|------|---------|-----------|
| **Agent** | Full autonomous task execution (file edits, terminal, search) | Default |
| **Plan** | Generates reviewable implementation plans before coding | Shift+Tab or mode picker |
| **Debug** | Runtime log instrumentation for bug hunting | Mode picker |
| **Ask** | Read-only Q&A with codebase context | Mode picker |

**Plan Mode** workflow:
1. Agent queries for clarification
2. Codebase analysis
3. Plan generated (saved as disk file, editable markdown)
4. User reviews/edits plan
5. Click to build
- Plans saved automatically to home dir; "Save to workspace" for team sharing
- Inline Mermaid diagram generation within plans

**Debug Mode** (v2.2, December 2025):
- Instruments code with runtime logs
- Generates multiple hypotheses about root cause
- Verifies fixes via agent loop
- Works across multiple stacks/languages

### Keyboard Shortcuts

**AI-specific:**
| Shortcut (Mac) | Shortcut (Win/Linux) | Action |
|----------------|---------------------|--------|
| Cmd+L | Ctrl+L | Open AI chat panel |
| Cmd+Shift+L | Ctrl+Shift+L | New chat |
| Cmd+I | Ctrl+I | Open Agent sidepane |
| Cmd+K | Ctrl+K | Inline AI edit |
| Cmd+E | Ctrl+E | Start Background Agent |
| Shift+Tab | Shift+Tab | Switch to Plan Mode |
| Tab | Tab | Accept AI suggestion |
| Cmd+→ | Ctrl+→ | Accept next word of suggestion |
| Esc | Esc | Reject suggestion |

**Message Queue (in active agent runs):**
| Key | Action |
|-----|--------|
| Enter | Queue follow-up message |
| Cmd+Enter | Send immediately (bypasses queue) |

**General editor (VS Code-inherited):**
- Cmd/Ctrl+Shift+P: Command palette
- F3: Mission Control window manager
- Standard VS Code editor shortcuts apply

### Streaming / Real-time Output

- AI responses stream token-by-token in the chat panel
- File edits display as animated inline diffs (green additions, red deletions)
- Terminal command execution visible in the bottom terminal panel
- Multi-Agent runs: all agents execute in parallel; results judged and best solution selected
- Background Agents run in remote cloud environment; output streams back

### Multi-turn Conversation Support

Full multi-turn support within chat sessions. Features:
- **Checkpoints**: Automatic codebase snapshots before significant changes; users can preview and restore
- **Session persistence** across editor restarts
- **Pinned conversations** in sidebar for quick reference
- **Message queue**: Queue multiple messages during active agent runs; reorder queued items

### Terminal Integration

- Integrated terminal with split-pane support
- Agent can execute shell commands in the terminal
- Ask Mode has read-only terminal access (for git log, etc.)
- "Terminal: Select Default Profile" configurable via command palette
- Background Agent provisions separate cloud-hosted environment with cloned repo

### Terminal Rendering / Display

Cursor is a GUI application; rendering is HTML/CSS within Electron, not escape sequences:
- Markdown rendering in chat panel
- Syntax highlighting in code blocks
- Side-by-side diff display for file changes
- Mermaid diagram rendering in plan documents
- Visual Editor: overlay for live CSS/layout manipulation in browser preview
- MCP Apps (March 2026): charts, diagrams, whiteboards rendered inside Cursor chat

### Tool Output Display

Tool calls (file reads/writes, terminal commands, web search, browser control) appear as collapsible sections in the chat panel. Users can expand to see full output. The agent can:
- Read/write files
- Run terminal commands (shown in terminal panel)
- Perform semantic and name-based code search
- Execute web search
- Control browser (screenshots, navigation, element interaction)
- Generate images

### BugBot (v1.0, June 2025)

Separate from the interactive REPL — connects to GitHub and automatically reviews PRs, leaves inline comments, links to relevant code in Cursor. Requires Max mode (Pro subscription).

### Background Agent

Remote cloud environment provisioned by Cursor:
- Clones the GitHub repo
- Works on a separate branch
- Pushes changes back
- Streams progress to the Cursor chat panel
- Activated via cloud icon or Cmd/Ctrl+E (requires privacy mode off)

### Notable / Unique Features

- **Four agent modes** (Agent/Plan/Debug/Ask) each with distinct strategy
- **Multi-Agent Judging**: runs multiple agents in parallel, auto-selects best result with explanation
- **Visual Editor**: live browser-rendered CSS/layout editing overlaid on running app
- **Checkpoint snapshots**: auto-save before destructive changes with restore capability
- **Mermaid diagrams in plans**: auto-generated visual plans
- **MCP Apps** (March 2026): interactive charts, diagrams, whiteboards in chat
- **BugBot**: automated GitHub PR review
- **Background Agent**: full remote cloud execution environment

---

## Key Features Comparison Table

| Feature | OpenCode | Cursor |
|---------|----------|--------|
| **Interface type** | True TUI (terminal, escape sequences) | GUI (Electron/VS Code fork) |
| **Primary interaction** | Terminal REPL / slash commands | GUI chat sidebar + inline editor |
| **Streaming output** | SSE-based, token streaming in TUI | Token streaming in GUI chat panel |
| **Multi-turn conversations** | Yes, persistent sessions with timeline | Yes, persistent sessions with checkpoints |
| **Session management** | Multiple sessions, list/switch/export | Multiple chats, pinned conversations |
| **Markdown rendering** | Yes (Marked + Shiki syntax highlight) | Yes (HTML-rendered in Electron) |
| **Syntax highlighting** | Yes (Shiki, 62 color properties, 30+ themes) | Yes (VS Code-native, full language support) |
| **Diff display** | Yes (stacked/auto, green/red, toggleable) | Yes (inline animated diffs) |
| **Tool output display** | Inline, toggleable detail level | Collapsible sections in chat panel |
| **Keyboard shortcuts** | Extensive, configurable, leader-key based | VS Code-inherited + AI-specific shortcuts |
| **Agent modes** | Single mode (multi-agent via @agent) | Agent / Plan / Debug / Ask modes |
| **Plan Mode** | Session compaction only | Full plan-before-build with Mermaid diagrams |
| **Undo/redo tool changes** | Yes, Git-backed `/undo` `/redo` | Yes, checkpoints with restore |
| **File references** | `@filename` fuzzy, with line ranges | Via chat context (`@file` syntax) |
| **Shell execution** | `!command` prefix inline | Via agent terminal tool |
| **External editor** | Yes (`$EDITOR` for long input) | N/A (IS the editor) |
| **MCP support** | Yes (status in sidebar, resources in @) | Yes (one-click install, MCP Apps) |
| **Cost display** | Yes (tokens + USD in sidebar) | Yes (shown in agent panel) |
| **Non-interactive/script mode** | Yes (`-p "prompt"`, `--json`) | No native headless mode |
| **Model selection** | `f2` cycle, `<leader>m` list (75+ models) | Model picker in chat panel |
| **Themes** | 30+ built-in, system palette detection | VS Code themes |
| **Remote/cloud agents** | No | Yes (Background Agent, remote env) |
| **PR review** | No | Yes (BugBot via GitHub) |
| **Language servers** | Listed in sidebar | Native VS Code LSP support |
| **Config file** | `tui.json` (UI), `opencode.json` (server) | `.cursor/rules`, `cursor.json` |
| **Open source** | Yes (archived) | No (proprietary) |
| **Primary language** | Go + TypeScript | TypeScript (Electron/VS Code) |

---

## Relevant for Go Agent Harness

The go-agent-harness currently has a basic `demo-cli` REPL. Key patterns from OpenCode and Cursor worth considering:

### 1. Leader-Key Command Architecture (OpenCode)
OpenCode's `ctrl+x` leader key pattern avoids terminal conflicts while providing rich shortcut space. The harness's current `/model`, `/help` slash command system is already aligned with this pattern. Consider expanding with a configurable leader key for shortcuts.

### 2. Sidebar for Metadata (OpenCode)
OpenCode's right sidebar (auto-shows at >120 columns) displays token count, cost, MCP status, and modified files. The harness tracks cost already via `internal/provider/pricing/` — surfacing this in a sidebar is a natural next step.

### 3. SSE-Based Streaming Architecture (OpenCode)
OpenCode's architecture — Go backend with SSE streaming to a TUI frontend — is essentially what the harness does. The `part.delta` event pattern maps directly to tool-call streaming in the harness's runner.

### 4. Slash Commands with Autocomplete (OpenCode)
OpenCode's `/command` system with autocomplete is directly applicable. The harness already has `/model` and `/help`; `/compact`, `/sessions`, `/export`, `/undo` are all realistic additions.

### 5. `@filename` File References (OpenCode)
Fuzzy file references with `@` at the prompt level, including line ranges (`@file.ts#10-20`), would reduce friction for users attaching file context without manually writing tool calls.

### 6. `!command` Shell Prefix (OpenCode)
Inline shell execution via `!` prefix is a natural harness extension. Currently bash is a tool call; `!ls` shorthand would be ergonomic.

### 7. Git-Backed Undo (OpenCode)
`/undo` and `/redo` backed by Git commits is elegant for a coding assistant. Given the harness's bash tool can modify files, this would provide a safety net.

### 8. Agent Modes (Cursor)
Cursor's four modes (Agent/Plan/Debug/Ask) formalize different interaction patterns. The harness's tool tiers (Core vs Deferred) already move in this direction. A named "Ask" mode (read-only tools) vs "Agent" mode (full tools) is a direct analogue.

### 9. Plan Mode (Cursor)
Separating plan-generation from execution — with a reviewable markdown plan file before tool-calling begins — reduces runaway agent behavior. This maps to the harness's `HARNESS_MAX_STEPS` concern: a plan mode could be low-step and produce a document, then a separate run executes the plan.

### 10. Message Queue (Cursor)
Cursor queues follow-up messages during active agent runs (Enter = queue, Cmd+Enter = send now). The harness's HTTP streaming architecture could support this: POST new messages that join the next iteration of the step loop.

### 11. Tool Output Visibility Toggle (OpenCode)
OpenCode's `/details` toggle for tool output reduces noise during normal use while allowing deep inspection. The harness's SSE stream currently emits all tool events; a client-side filter toggle is worth adding to the CLI.

### 12. Non-Interactive Mode (OpenCode)
`opencode -p "prompt" --json` for scripting is a pattern the harness CLI already supports via `harnesscli`. The `--json` output flag for machine-readable streaming is especially useful for CI/CD pipelines.

---

## Sources

- [OpenCode TUI Docs](https://opencode.ai/docs/tui/)
- [OpenCode Keybinds Docs](https://opencode.ai/docs/keybinds/)
- [OpenCode GitHub](https://github.com/opencode-ai/opencode)
- [DeepWiki: sst/opencode TUI Architecture](https://deepwiki.com/sst/opencode/6.1-tool-framework)
- [DeepWiki: sst/opencode TUI Theming & Commands](https://deepwiki.com/sst/opencode/6.4-tui-theming-keybinds-and-commands)
- [DeepWiki: sst/opencode CLI Commands](https://deepwiki.com/sst/opencode/6.2-cli-commands)
- [Cursor Features](https://cursor.com/features)
- [Cursor Agent Docs](https://cursor.com/docs/agent/overview)
- [Cursor Plan Mode Docs](https://cursor.com/docs/agent/plan-mode)
- [Cursor Changelog 2.2](https://cursor.com/changelog/2-2)
- [Cursor Introducing Plan Mode](https://cursor.com/blog/plan-mode)
- [Cursor Introducing Debug Mode](https://cursor.com/blog/debug-mode)
- [design.dev Cursor Shortcuts Guide](https://design.dev/guides/cursor-shortcuts/)
- [DEV Community: OpenCode overview](https://dev.to/wonderlab/open-source-project-of-the-day-part-4-opencode-a-powerful-ai-coding-agent-built-for-the-g05)
