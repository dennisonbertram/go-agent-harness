# Claude Code: Complete Feature Reference for GUI Parity

> Comprehensive research of all Claude Code CLI features, organized by feature area.
> Purpose: serve as the definitive reference for building a Tauri-based GUI with full feature parity.

---

## Table of Contents

1. [Chat/Conversation Features](#1-chatconversation-features)
2. [Tool System](#2-tool-system)
3. [File System Operations](#3-file-system-operations)
4. [Code Intelligence](#4-code-intelligence)
5. [Git Integration](#5-git-integration)
6. [Project Context / Memory System](#6-project-context--memory-system)
7. [Agent / Subagent System](#7-agent--subagent-system)
8. [MCP (Model Context Protocol)](#8-mcp-model-context-protocol)
9. [UI/UX Features](#9-uiux-features)
10. [Settings and Configuration](#10-settings-and-configuration)
11. [Plan Mode](#11-plan-mode)
12. [Hooks System](#12-hooks-system)
13. [IDE Integrations](#13-ide-integrations)
14. [Notebook Support](#14-notebook-support)
15. [Memory / Auto-Memory](#15-memory--auto-memory)
16. [Skills System](#16-skills-system)
17. [Permissions System](#17-permissions-system)
18. [Checkpointing / Rewind](#18-checkpointing--rewind)
19. [CLI Flags and Non-Interactive Mode](#19-cli-flags-and-non-interactive-mode)
20. [Cost and Usage Tracking](#20-cost-and-usage-tracking)
21. [Desktop App](#21-desktop-app)
22. [Plugins System](#22-plugins-system)
23. [Agent Teams](#23-agent-teams)

---

## 1. Chat/Conversation Features

### Multi-Turn Conversations
- **What it does**: Users type natural language prompts and Claude responds. The conversation maintains full context within a session.
- **User interaction**: Type in the prompt box, press Enter. Claude streams a response token-by-token with a spinner/progress indicator.
- **UI elements**: Prompt input box, message history (user messages + assistant messages), streaming text with markdown rendering.
- **GUI implementation**: Chat panel with scrollable message history. Each message rendered as markdown. User input box at bottom with submit button and keyboard shortcut (Enter to send).

### Message Streaming (Token-by-Token)
- **What it does**: Responses appear character/token at a time, not all at once.
- **User interaction**: User sees text appearing progressively. A spinner animation shows while Claude is working (customizable "verbs" like "Thinking", "Pondering", etc.).
- **UI elements**: Animated text cursor, spinner with verb text, turn duration display (e.g., "Cooked for 1m 6s").
- **GUI implementation**: WebSocket or SSE-based streaming from the backend. Incremental DOM updates for each text delta. Configurable spinner/progress indicator.

### Session Management
- **What it does**: Sessions can be resumed, continued, forked, and named.
- **Key commands**:
  - `/resume [session]` or `/continue` -- resume a past session by ID or name
  - `/clear` (aliases: `/reset`, `/new`) -- clear conversation and start fresh
  - `/fork [name]` -- fork current conversation at current point
  - `/rename [name]` -- name the current session
  - `/export [filename]` -- export conversation as plain text
  - CLI: `claude --continue` (most recent), `claude --resume <id-or-name>`
  - CLI: `claude --fork-session` (with --resume or --continue)
  - CLI: `claude --session-id <uuid>`
  - CLI: `claude --from-pr <number>` -- resume sessions linked to a specific PR
- **UI elements**: Session picker list, session name in prompt bar, session history browser.
- **GUI implementation**: Session list panel showing past sessions with names, dates, and project. Resume/fork buttons. Current session name displayed in header/titlebar.

### Context Window Management
- **What it does**: `/compact [instructions]` compresses conversation history to free up context space. Auto-compaction triggers at ~95% capacity.
- **User interaction**: Run `/compact` manually or let auto-compaction handle it. `/context` shows a visual grid of context usage.
- **UI elements**: Context usage visualization (colored grid), optimization suggestions.
- **GUI implementation**: Context usage meter/progress bar in the status area. Manual compact button. Visual indicator when context is getting full.

### Prompt Suggestions
- **What it does**: After Claude responds, a grayed-out suggestion appears in the input based on conversation context. On first open, it suggests based on git history.
- **User interaction**: Press Tab to accept, Enter to accept and submit, or start typing to dismiss.
- **GUI implementation**: Ghost text / placeholder text in the input field that updates after each response.

### Side Questions (`/btw`)
- **What it does**: Ask a quick question without adding to conversation history. Available even while Claude is working. No tool access -- answers only from context.
- **User interaction**: `/btw <question>` -- answer appears in a dismissible overlay.
- **GUI implementation**: Modal/overlay for side question and answer. Does not affect main chat history.

### Bash Mode (`!` prefix)
- **What it does**: Run shell commands directly without Claude interpreting them. Output is added to conversation context.
- **User interaction**: Type `!` followed by any bash command. Output streams in real-time.
- **GUI implementation**: Special input mode triggered by `!` prefix. Terminal-like output rendering.

---

## 2. Tool System

### Built-In Tools (Complete List)

| Tool | Category | Description |
|------|----------|-------------|
| **Read** | File | Read files with line numbers. Supports images (PNG, JPG), PDFs (with page ranges), Jupyter notebooks |
| **Write** | File | Create new files or completely overwrite existing ones |
| **Edit** | File | Exact string replacement in files (diff-based editing) |
| **MultiEdit** | File | Multiple edits to a single file in one operation |
| **NotebookEdit** | File | Edit Jupyter notebook cells (replace, insert, delete) |
| **Glob** | Search | Fast file pattern matching (e.g., `**/*.ts`) |
| **Grep** | Search | Content search using ripgrep with regex support |
| **LS** | Search | List directory contents |
| **Bash** | Execution | Execute shell commands with timeout support |
| **BashOutput** | Execution | Read output from background bash commands |
| **KillBash** | Execution | Kill running bash processes |
| **WebSearch** | Web | Search the web and return formatted results |
| **WebFetch** | Web | Fetch and process web page content |
| **TodoWrite** | Organization | Write/update task list items |
| **TodoRead** | Organization | Read current task list |
| **Task/Agent** | Agent | Delegate work to subagents (renamed from Task to Agent in v2.1.63) |
| **ExitPlanMode** | Control | Exit plan mode and present proposed changes |
| **AskUserQuestion** | Interaction | Ask the user a clarifying question (used by subagents) |
| **ToolSearch** | Discovery | Search for and load deferred/MCP tools |
| **Skill** | Skills | Invoke a skill within the conversation |
| **EnterWorktree** | Git | Create isolated git worktree for the session |
| **TaskOutput** | Agent | Read output from background tasks |

### Tool Approval / Permission System
- **What it does**: Each tool use can require user approval. Three rule types: Allow (auto-approve), Ask (prompt), Deny (block).
- **User interaction**: Permission dialogs appear when Claude wants to use a tool. User clicks Allow/Deny. "Yes, don't ask again" option for session or permanent approval.
- **Tool type defaults**:
  - Read-only tools (Read, Grep, Glob): No approval required
  - Bash commands: Yes, with permanent per-project-directory approval
  - File modifications (Edit/Write): Yes, with session-end expiry
- **GUI implementation**: Permission dialog component with Allow/Deny/Always Allow buttons. Tool approval history panel. Permission rules editor.

### Tool Use Visualization
- **What it does**: Shows what tools Claude is calling, their inputs, and outputs. Verbose mode (`Ctrl+O`) shows full details.
- **User interaction**: Tool calls appear inline in the chat with collapsible details.
- **GUI implementation**: Collapsible tool call cards in the chat stream showing: tool name, input parameters, output/result, duration. Color-coded by tool type.

---

## 3. File System Operations

### Reading Files
- **What it does**: Read any file with line numbers (cat -n format). Supports offset/limit for large files. Can read images (displayed visually), PDFs (with page ranges, max 20 pages per request), and Jupyter notebooks.
- **GUI implementation**: File viewer panel with syntax highlighting, line numbers, and pagination for large files. Image preview for image files. PDF viewer component.

### Editing Files (Diff-Based)
- **What it does**: Exact string replacement. The `old_string` must be unique in the file. Supports `replace_all` for global replacements.
- **GUI implementation**: Diff viewer showing before/after. Accept/reject individual changes. Side-by-side or inline diff view.

### Creating Files
- **What it does**: Write tool creates new files or overwrites existing ones.
- **GUI implementation**: New file creation dialog. File tree update on creation.

### File Search (Glob)
- **What it does**: Fast pattern matching with glob syntax (e.g., `**/*.ts`, `src/**/*.tsx`). Returns matching paths sorted by modification time.
- **GUI implementation**: File search panel with glob pattern input. Results list with file paths and click-to-open.

### Content Search (Grep)
- **What it does**: Ripgrep-based content search. Supports regex, file type filtering, context lines (-A/-B/-C), case-insensitive search, multiline matching.
- **Output modes**: `content` (matching lines), `files_with_matches` (file paths only), `count` (match counts).
- **GUI implementation**: Search panel with pattern input, file type filter, case sensitivity toggle. Results with highlighted matches and context lines.

### File Mentions (`@` syntax)
- **What it does**: Type `@` in the prompt to trigger file path autocomplete. Selected files are added as context.
- **GUI implementation**: `@` trigger in input box with dropdown file picker. Autocomplete with fuzzy matching. Custom file suggestion scripts supported via `fileSuggestion` setting.

---

## 4. Code Intelligence

### Codebase Understanding
- **What it does**: Claude can read, search, and analyze code across the entire project. Uses Glob, Grep, Read tools to explore.
- **GUI implementation**: No special UI needed beyond tool visualization -- Claude uses the standard tools to navigate code.

### Code Review
- **What it does**: `/security-review` analyzes pending changes for security vulnerabilities. Custom code review via skills/subagents.
- **GUI implementation**: Security review panel showing identified risks (injection, auth issues, data exposure) with severity levels.

---

## 5. Git Integration

### Commit Creation
- **What it does**: Claude reads the diff, writes meaningful commit messages, and runs `git commit`. Supports conventional commit format. Configurable co-author attribution.
- **Settings**: `includeCoAuthoredBy` (default: true), `attribution.commit` for custom commit messages.
- **GUI implementation**: Commit dialog with pre-filled message, diff preview, file staging checkboxes.

### PR Creation
- **What it does**: Claude creates branches, pushes, and opens PRs via `gh` CLI. Analyzes commits and generates PR descriptions.
- **GUI implementation**: PR creation wizard with title, body editor, base branch selector, reviewer picker.

### Branch Management
- **What it does**: Create feature branches, switch branches, manage worktrees.
- **GUI implementation**: Branch selector/switcher in status bar. Branch creation dialog.

### Diff Viewing
- **What it does**: `/diff` opens interactive diff viewer showing uncommitted changes and per-turn diffs. Left/right arrows switch between git diff and individual turns. Up/down browse files.
- **GUI implementation**: Full diff viewer component with file tree, per-file diffs, and turn-based diff history.

### PR Review Status
- **What it does**: When on a branch with an open PR, shows a clickable PR link in footer with colored underline indicating review state.
- **Colors**: Green (approved), Yellow (pending), Red (changes requested), Gray (draft), Purple (merged).
- **Updates**: Auto-refreshes every 60 seconds. Requires `gh` CLI.
- **GUI implementation**: PR status badge in status bar with link to open PR in browser.

### PR Comments
- **What it does**: `/pr-comments [PR]` fetches and displays comments from a GitHub PR.
- **GUI implementation**: PR comments panel showing threaded comments.

---

## 6. Project Context / Memory System

### CLAUDE.md Files (Hierarchical)

| Scope | Location | Purpose | Shared? |
|-------|----------|---------|---------|
| Managed policy | `/Library/Application Support/ClaudeCode/CLAUDE.md` (macOS) | Org-wide instructions | Yes (IT-deployed) |
| Project | `./CLAUDE.md` or `./.claude/CLAUDE.md` | Team-shared project instructions | Yes (version control) |
| User | `~/.claude/CLAUDE.md` | Personal preferences | No |

- **Loading order**: Walks up directory tree from CWD. Parent directories loaded at launch. Subdirectory CLAUDE.md files loaded on-demand when Claude reads files there.
- **Features**: `@path` imports for referencing external files. Max recommended: 200 lines per file.
- **GUI implementation**: CLAUDE.md editor panel. File tree showing all loaded instruction files. Import resolution viewer.

### Rules System (`.claude/rules/`)
- **What it does**: Modular instruction files in `.claude/rules/`. Supports path-specific rules via YAML frontmatter `paths` field.
- **Path scoping**: Rules with `paths: ["src/api/**/*.ts"]` only load when Claude works with matching files.
- **User-level rules**: `~/.claude/rules/` for personal rules across all projects.
- **GUI implementation**: Rules editor with path pattern configuration. Visual indicator showing which rules are active.

### Auto Memory
- **What it does**: Claude automatically saves learnings across sessions (build commands, debugging insights, architecture notes). Stored in `~/.claude/projects/<project>/memory/`.
- **Structure**: `MEMORY.md` entrypoint (first 200 lines loaded at startup) + topic files (loaded on demand).
- **Toggle**: `/memory` command or `autoMemoryEnabled` setting.
- **GUI implementation**: Memory browser showing auto-memory entries. Toggle switch. Memory file editor.

### `/init` Command
- **What it does**: Analyzes codebase and generates a starting CLAUDE.md with build commands, test instructions, and project conventions.
- **GUI implementation**: "Initialize Project" button that generates CLAUDE.md with preview before saving.

### `/memory` Command
- **What it does**: Lists all loaded CLAUDE.md and rules files. Toggle auto memory. Open files for editing.
- **GUI implementation**: Memory management panel showing all instruction sources and their status.

---

## 7. Agent / Subagent System

### Built-In Subagents

| Agent | Model | Tools | Purpose |
|-------|-------|-------|---------|
| **Explore** | Haiku (fast) | Read-only | File discovery, code search, codebase exploration |
| **Plan** | Inherited | Read-only | Research during plan mode |
| **General-purpose** | Inherited | All tools | Complex multi-step tasks requiring exploration and modification |
| **Bash** | Inherited | Terminal | Running commands in separate context |
| **Claude Code Guide** | Haiku | Limited | Answering questions about Claude Code features |
| **statusline-setup** | Sonnet | Limited | Configuring status line |

### Custom Subagents
- **What it does**: User-defined agents with custom system prompts, tool access, model selection, and permissions.
- **Definition**: Markdown files with YAML frontmatter in `~/.claude/agents/` (user) or `.claude/agents/` (project).
- **Configuration fields**: `name`, `description`, `tools`, `disallowedTools`, `model`, `permissionMode`, `maxTurns`, `skills`, `mcpServers`, `hooks`, `memory`, `background`, `isolation`.
- **Management**: `/agents` command for interactive CRUD. `claude agents` CLI to list.
- **GUI implementation**: Agent configuration panel. Agent list with create/edit/delete. Model selector, tool picker, permission mode dropdown.

### Parallel Execution
- **What it does**: Multiple subagents can run simultaneously. Up to 10 concurrent tasks with intelligent queuing.
- **Background agents**: Run concurrently while user continues working. `Ctrl+B` to background a running task. `Ctrl+F` (twice) to kill all background agents.
- **GUI implementation**: Background task panel showing running agents with progress indicators. Kill button per agent.

### Worktree Isolation
- **What it does**: `isolation: "worktree"` gives each agent its own copy of the repo. Auto-cleaned if no changes.
- **CLI**: `claude --worktree` or `claude -w <name>`.
- **GUI implementation**: Worktree indicator in status bar. Worktree management panel.

### Agent Resumption
- **What it does**: Subagents can be resumed with full conversation history intact. Each creates a transcript at `~/.claude/projects/{project}/{sessionId}/subagents/agent-{agentId}.jsonl`.
- **GUI implementation**: Agent history panel with resume capability.

### Agent Memory
- **What it does**: Subagents can have persistent memory scoped to `user`, `project`, or `local`.
- **GUI implementation**: Per-agent memory viewer.

---

## 8. MCP (Model Context Protocol)

### Server Configuration
- **What it does**: Connect to external tools via MCP servers. Supports stdio (child process), SSE (remote), HTTP (streamable), and WebSocket transports.
- **Configuration files**: `.mcp.json` (project), `~/.claude.json` (user/local).
- **CLI management**:
  - `claude mcp add <name> [--transport stdio|sse|http] <command/url> [args...]`
  - `claude mcp list` -- list all configured servers
  - `claude mcp get <name>` -- details for a specific server
  - `claude mcp remove <name>` -- remove a server
- **In-session**: `/mcp` to check server status and manage connections.
- **GUI implementation**: MCP server management panel. Add/remove/configure servers. Connection status indicators. OAuth flow support for authenticated servers.

### MCP Tools
- **What it does**: MCP servers expose tools that Claude can call. Tool names follow `mcp__<server>__<tool>` format.
- **Permission rules**: `mcp__puppeteer__*` to allow all tools from a server.
- **GUI implementation**: MCP tool browser showing available tools per server with schemas and descriptions.

### MCP Prompts
- **What it does**: MCP servers can expose prompts as commands (`/mcp__<server>__<prompt>`).
- **GUI implementation**: MCP prompts appear in command palette alongside built-in commands.

### Managed MCP Configuration
- **What it does**: Admins can control which MCP servers users can add via `allowedMcpServers`, `deniedMcpServers`, `allowManagedMcpServersOnly`.
- **GUI implementation**: Admin panel for MCP server policies.

---

## 9. UI/UX Features

### Slash Commands (Complete List)

| Command | Purpose |
|---------|---------|
| `/add-dir <path>` | Add working directory |
| `/agents` | Manage subagent configurations |
| `/btw <question>` | Side question without history |
| `/chrome` | Configure Chrome integration |
| `/clear` (`/reset`, `/new`) | Clear conversation |
| `/color [color]` | Set prompt bar color |
| `/compact [instructions]` | Compress conversation |
| `/config` (`/settings`) | Open settings interface |
| `/context` | Visualize context usage |
| `/copy` | Copy last response to clipboard |
| `/cost` | Show token usage statistics |
| `/desktop` (`/app`) | Continue in desktop app |
| `/diff` | Interactive diff viewer |
| `/doctor` | Diagnose installation |
| `/effort [level]` | Set model effort level (low/medium/high/max/auto) |
| `/exit` (`/quit`) | Exit CLI |
| `/export [filename]` | Export conversation |
| `/extra-usage` | Configure extra usage for rate limits |
| `/fast [on\|off]` | Toggle fast mode |
| `/feedback` (`/bug`) | Submit feedback |
| `/fork [name]` | Fork conversation |
| `/help` | Show help |
| `/hooks` | View hook configurations |
| `/ide` | Manage IDE integrations |
| `/init` | Initialize project CLAUDE.md |
| `/insights` | Generate usage analytics report |
| `/install-github-app` | Set up GitHub Actions |
| `/install-slack-app` | Install Slack app |
| `/keybindings` | Open keybindings config |
| `/login` | Sign in |
| `/logout` | Sign out |
| `/mcp` | Manage MCP servers |
| `/memory` | Edit memory/CLAUDE.md files |
| `/mobile` (`/ios`, `/android`) | Show mobile app QR code |
| `/model [model]` | Select/change AI model |
| `/passes` | Share free week passes |
| `/permissions` (`/allowed-tools`) | View/update permissions |
| `/plan` | Enter plan mode |
| `/plugin` | Manage plugins |
| `/pr-comments [PR]` | Fetch PR comments |
| `/privacy-settings` | Privacy settings (Pro/Max only) |
| `/release-notes` | View changelog |
| `/reload-plugins` | Reload active plugins |
| `/remote-control` (`/rc`) | Enable remote control from claude.ai |
| `/remote-env` | Configure remote environment |
| `/rename [name]` | Rename session |
| `/resume [session]` (`/continue`) | Resume past session |
| `/review` | Deprecated (use plugin) |
| `/rewind` (`/checkpoint`) | Rewind conversation/code |
| `/sandbox` | Toggle sandbox mode |
| `/security-review` | Security vulnerability analysis |
| `/skills` | List available skills |
| `/stats` | Usage visualization |
| `/status` | Show version/model/account info |
| `/statusline` | Configure status line |
| `/stickers` | Order stickers |
| `/tasks` | List background tasks |
| `/terminal-setup` | Configure terminal keybindings |
| `/theme` | Change color theme |
| `/upgrade` | Upgrade plan |
| `/usage` | Show plan usage/rate limits |
| `/vim` | Toggle Vim editing mode |

**Bundled Skills (also appear as slash commands)**:
| Skill | Purpose |
|-------|---------|
| `/batch <instruction>` | Orchestrate large-scale parallel changes across codebase |
| `/claude-api` | Load Claude API reference for your project language |
| `/debug [description]` | Troubleshoot current session from debug log |
| `/loop [interval] <prompt>` | Run a prompt repeatedly on an interval |
| `/simplify [focus]` | Review recent changes for code reuse/quality, then fix |

### Keyboard Shortcuts (Complete Reference)

#### General Controls
| Shortcut | Description |
|----------|-------------|
| `Ctrl+C` | Cancel current input or generation |
| `Ctrl+F` | Kill all background agents (press twice in 3s to confirm) |
| `Ctrl+D` | Exit session (EOF) |
| `Ctrl+G` | Open prompt in default text editor |
| `Ctrl+L` | Clear terminal screen (keeps conversation) |
| `Ctrl+O` | Toggle verbose output |
| `Ctrl+R` | Reverse search command history |
| `Ctrl+V` / `Cmd+V` / `Alt+V` | Paste image from clipboard |
| `Ctrl+B` | Background running tasks (tmux: press twice) |
| `Ctrl+T` | Toggle task list |
| `Left/Right arrows` | Cycle through dialog tabs |
| `Up/Down arrows` | Navigate command history |
| `Esc+Esc` | Rewind/summarize menu |
| `Shift+Tab` / `Alt+M` | Toggle permission modes (Normal -> Auto-Accept -> Plan) |
| `Alt+P` / `Option+P` | Switch model |
| `Alt+T` / `Option+T` | Toggle extended thinking |

#### Text Editing
| Shortcut | Description |
|----------|-------------|
| `Ctrl+K` | Delete to end of line |
| `Ctrl+U` | Delete entire line |
| `Ctrl+Y` | Paste deleted text |
| `Alt+Y` (after Ctrl+Y) | Cycle paste history |
| `Alt+B` | Move cursor back one word |
| `Alt+F` | Move cursor forward one word |

#### Multiline Input
| Method | Shortcut |
|--------|----------|
| Quick escape | `\` + Enter |
| macOS default | `Option+Enter` |
| Shift+Enter | Works in iTerm2, WezTerm, Ghostty, Kitty |
| Control sequence | `Ctrl+J` |

#### Quick Commands
| Shortcut | Description |
|----------|-------------|
| `/` at start | Command/skill menu |
| `!` at start | Bash mode |
| `@` | File path autocomplete |

### Vim Editing Mode
- **What it does**: Full Vim keybinding support with Normal/Insert modes, motions (hjkl, w, e, b, 0, $, gg, G, f/F/t/T), operators (d, c, y with text objects), and visual mode commands.
- **Toggle**: `/vim` command or `/config`.
- **GUI implementation**: Optional Vim mode for the input field. Mode indicator (NORMAL/INSERT).

### Theme System
- **What it does**: `/theme` to change color themes. Light and dark variants, colorblind-accessible (daltonized) themes, ANSI themes using terminal palette.
- **GUI implementation**: Theme picker dialog. Light/dark mode toggle. Custom theme support.

### Syntax Highlighting
- **What it does**: Code blocks in responses use syntax highlighting (native build only).
- **GUI implementation**: Use a syntax highlighting library (Prism, Shiki, etc.) for code blocks in rendered markdown.

### Task List
- **What it does**: When working on complex tasks, Claude creates a visible task list with pending/in-progress/complete indicators.
- **Toggle**: `Ctrl+T` to show/hide. Up to 10 tasks visible. Persists across compactions.
- **GUI implementation**: Task list sidebar/panel with status icons and progress indicators.

### Status Line
- **What it does**: Configurable status bar showing custom information. `/statusline` to configure. Supports command-based dynamic content.
- **GUI implementation**: Configurable status bar at bottom of window.

### Progress Indicators
- **What it does**: Spinner with customizable verbs while Claude works. Turn duration display. Terminal progress bar in supported terminals.
- **Settings**: `spinnerVerbs` (custom verbs), `showTurnDuration`, `terminalProgressBarEnabled`, `prefersReducedMotion`.
- **GUI implementation**: Animated spinner/progress indicator. Duration timer. Disable animations setting for accessibility.

### Image Support
- **What it does**: Paste images from clipboard (`Ctrl+V`). Read tool can display images. Claude is multimodal and can analyze images.
- **GUI implementation**: Image paste support in input. Image rendering in message history. Image file preview.

### Markdown Rendering
- **What it does**: All Claude responses rendered as markdown with headers, lists, code blocks, tables, links, etc.
- **GUI implementation**: Full markdown renderer with support for GFM (GitHub Flavored Markdown), code blocks with syntax highlighting, tables, and inline formatting.

### Copy to Clipboard
- **What it does**: `/copy` copies last response. When code blocks are present, shows interactive picker to select individual blocks or full response.
- **GUI implementation**: Copy button on each message and each code block.

---

## 10. Settings and Configuration

### Settings Files Hierarchy

| Level | Location | Scope |
|-------|----------|-------|
| Managed | Server-managed, plist/registry, or `managed-settings.json` | Organization-wide, cannot be overridden |
| User | `~/.claude/settings.json` | All your projects |
| Project | `.claude/settings.json` | This project (shared via VCS) |
| Local | `.claude/settings.local.json` | This project (not committed) |

### Key Settings Fields

| Setting | Description |
|---------|-------------|
| `permissions` | Allow/Ask/Deny rules for tools |
| `hooks` | Lifecycle event hooks |
| `env` | Environment variables for sessions |
| `model` | Default model override |
| `availableModels` | Restrict model selection |
| `effortLevel` | Model effort level (low/medium/high) |
| `outputStyle` | Response style adjustment |
| `language` | Preferred response language |
| `autoMemoryEnabled` | Toggle auto-memory |
| `autoMemoryDirectory` | Custom memory storage location |
| `cleanupPeriodDays` | Session cleanup period (default: 30) |
| `includeGitInstructions` | Include git workflow instructions in system prompt |
| `attribution` | Customize git commit/PR attribution |
| `alwaysThinkingEnabled` | Extended thinking by default |
| `plansDirectory` | Custom plans storage location |
| `showTurnDuration` | Show turn duration messages |
| `spinnerVerbs` | Custom spinner action verbs |
| `spinnerTipsEnabled` | Show tips during spinner |
| `spinnerTipsOverride` | Custom spinner tips |
| `terminalProgressBarEnabled` | Terminal progress bar |
| `prefersReducedMotion` | Reduce UI animations |
| `statusLine` | Custom status line config |
| `fileSuggestion` | Custom `@` file autocomplete |
| `respectGitignore` | File picker respects .gitignore |
| `forceLoginMethod` | Restrict login to claudeai or console |
| `autoUpdatesChannel` | stable or latest |
| `teammateMode` | Agent team display mode |
| `claudeMdExcludes` | Exclude specific CLAUDE.md files |
| `apiKeyHelper` | Custom script for API key generation |
| `enableAllProjectMcpServers` | Auto-approve project MCP servers |
| `companyAnnouncements` | Startup announcements |

### Model Selection
- **What it does**: `/model` to switch models mid-session. `--model` CLI flag. Supports aliases (`sonnet`, `opus`, `haiku`) or full model IDs.
- **Effort levels**: `/effort low|medium|high|max|auto`. Adjustable via left/right arrows in model picker.
- **Extended thinking**: Toggle with `Alt+T`. "ultrathink" keyword in skills enables it.
- **Fast mode**: `/fast [on|off]` for same model with faster output.
- **GUI implementation**: Model selector dropdown. Effort level slider. Extended thinking toggle. Fast mode toggle.

### API Key Management
- **What it does**: Supports Anthropic API key (`ANTHROPIC_API_KEY`), Claude.ai subscription (Pro/Max/Teams), Amazon Bedrock, Google Vertex AI.
- **GUI implementation**: API key input field in settings. Auth status indicator. Login/logout buttons.

### Output Styles
- **What it does**: Configure response style (e.g., "Explanatory") via `outputStyle` setting or `/config`.
- **GUI implementation**: Output style selector in settings.

---

## 11. Plan Mode

### How Plan Mode Works
- **What it does**: Read-only research and planning phase. Claude can analyze codebase but cannot modify files or execute commands.
- **Entry methods**: `Shift+Tab` (cycle through modes), `/plan` command, `--permission-mode plan` CLI flag, `defaultMode: "plan"` in settings.
- **Workflow**:
  1. Research & Analyze (read files, search code)
  2. Create a plan (comprehensive strategy)
  3. Present for approval (uses `exit_plan_mode` tool)
  4. Wait for user confirmation
  5. Execute changes after approval
- **Exit**: User approves the plan, Claude transitions to execution mode.
- **Benefits**: Fast responses (no tool execution), fewer tokens, safe exploration.

### Permission Mode Cycling
- **What it does**: `Shift+Tab` cycles: Normal -> Auto-Accept -> Plan -> Normal
- **Visual indicator**: Status text below prompt (`normal mode`, `accept edits on`, `plan mode on`).
- **GUI implementation**: Permission mode selector (dropdown or toggle group). Visual indicator of current mode. Plan approval dialog when Claude presents a plan.

---

## 12. Hooks System

### Hook Events (Complete Reference)

| Event | When It Fires | Supports Matchers | Can Block |
|-------|--------------|-------------------|-----------|
| **SessionStart** | Session begins/resumes | Yes (startup, resume, clear, compact) | No |
| **SessionEnd** | Session terminates | Yes (exit reasons) | No |
| **InstructionsLoaded** | CLAUDE.md or rules loaded | No | No |
| **UserPromptSubmit** | User submits prompt | No | Yes |
| **PreToolUse** | Before tool execution | Yes (tool names) | Yes |
| **PermissionRequest** | Permission dialog appears | Yes (tool names) | Yes |
| **PostToolUse** | Tool completes successfully | Yes (tool names) | Yes |
| **PostToolUseFailure** | Tool execution fails | Yes (tool names) | No |
| **Notification** | Notification sent | Yes (notification types) | No |
| **SubagentStart** | Subagent spawned | Yes (agent types) | No |
| **SubagentStop** | Subagent finishes | Yes (agent types) | Yes |
| **Stop** | Claude finishes responding | No | Yes |
| **TeammateIdle** | Agent team teammate going idle | No | Yes |
| **TaskCompleted** | Task marked complete | No | Yes |
| **ConfigChange** | Configuration changes | Yes (config sources) | Yes |
| **WorktreeCreate** | Worktree being created | No | Yes (output) |
| **WorktreeRemove** | Worktree being removed | No | No |
| **PreCompact** | Before context compaction | Yes (manual, auto) | No |
| **PostCompact** | After context compaction | Yes (manual, auto) | No |
| **Elicitation** | MCP server requests input | Yes (MCP server name) | Yes |
| **ElicitationResult** | User responds to elicitation | Yes (MCP server name) | Yes |

### Hook Handler Types

| Type | Description |
|------|-------------|
| **command** | Execute shell commands. Receives JSON on stdin, returns decisions via exit codes and stdout |
| **http** | Send POST requests with JSON payload. Returns decisions via HTTP response body |
| **prompt** | Single-turn LLM evaluation (e.g., safety check) |
| **agent** | Multi-turn agentic verification with tool access |

### Hook Configuration
- **Locations**: `~/.claude/settings.json` (user), `.claude/settings.json` (project), `.claude/settings.local.json` (local), plugin `hooks/hooks.json`, skill/agent frontmatter.
- **Structure**: Event -> Matcher (regex) -> Handler(s).
- **In-session**: `/hooks` to view all configured hooks.
- **GUI implementation**: Hook configuration editor with event selector, matcher input, handler type selector. Hook status dashboard showing which hooks are active.

---

## 13. IDE Integrations

### VS Code Extension
- **Features**: Native graphical chat panel, checkpoint-based undo, `@`-mention file references, parallel conversations.
- **Context sharing**: Auto-detects which file is open, highlighted code, Problems panel errors.
- **Diff viewing**: Changes open in VS Code's native diff viewer.
- **Settings**: `claudeCode.initialPermissionMode` for default permission mode.
- **GUI relevance**: Our Tauri GUI replaces the need for IDE integration, but we should support similar features (file context, diff viewing, error panel).

### JetBrains Integration
- **Features**: Interactive diff viewing, selection context sharing, quick launch (Cmd+Esc / Ctrl+Esc).
- **Plugin**: Coordinates with Claude Code CLI in terminal.

### IDE Connection
- **What it does**: `/ide` to manage IDE integrations. `--ide` flag to auto-connect on startup. Diagnostic errors from IDE shared automatically.
- **GUI implementation**: IDE connection panel showing connected IDEs. Error/diagnostic feed from connected IDE.

---

## 14. Notebook Support

### Jupyter Notebook Reading
- **What it does**: Read tool can read `.ipynb` files and returns all cells with outputs, combining code, text, and visualizations.
- **GUI implementation**: Notebook viewer with cell rendering, code cells with syntax highlighting, markdown cells rendered, output cells displayed.

### Jupyter Notebook Editing
- **What it does**: NotebookEdit tool can replace, insert, or delete cells. Supports code and markdown cell types.
- **GUI implementation**: Notebook cell editor. Cell type selector. Insert/delete buttons.

---

## 15. Memory / Auto-Memory

### Auto-Memory System
- **What it does**: Claude saves notes automatically across sessions -- build commands, debugging insights, architecture notes, code style preferences.
- **Storage**: `~/.claude/projects/<project>/memory/` -- `MEMORY.md` (entrypoint, first 200 lines loaded) + topic files (on-demand).
- **Toggle**: `/memory` command, `autoMemoryEnabled` setting, `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` env var.
- **Custom directory**: `autoMemoryDirectory` setting.
- **Shared**: All worktrees/subdirectories within same git repo share one auto-memory directory.
- **GUI implementation**: Memory browser panel. Toggle switch. Memory file editor with MEMORY.md preview.

### Subagent Persistent Memory
- **What it does**: Subagents can maintain their own memory directory across sessions.
- **Scopes**: `user` (`~/.claude/agent-memory/<name>/`), `project` (`.claude/agent-memory/<name>/`), `local` (`.claude/agent-memory-local/<name>/`).
- **GUI implementation**: Per-agent memory viewer accessible from agent configuration panel.

---

## 16. Skills System

### Skills Overview
- **What it does**: Reusable prompt-based extensions. Each skill is a `SKILL.md` file with YAML frontmatter + markdown instructions.
- **Invocation**: `/skill-name` by user, or auto-loaded by Claude when relevant.
- **Locations**: `~/.claude/skills/` (personal), `.claude/skills/` (project), plugin skills, enterprise managed.

### Skill Frontmatter Fields

| Field | Description |
|-------|-------------|
| `name` | Display name and `/slash-command` |
| `description` | When Claude should use this skill |
| `argument-hint` | Autocomplete hint for arguments |
| `disable-model-invocation` | Prevent Claude from auto-loading |
| `user-invocable` | Set to false to hide from `/` menu |
| `allowed-tools` | Tool restrictions when skill is active |
| `model` | Model override for this skill |
| `context` | Set to `fork` to run in subagent |
| `agent` | Subagent type when `context: fork` |
| `hooks` | Skill-scoped lifecycle hooks |

### Skill Features
- **Arguments**: `$ARGUMENTS`, `$ARGUMENTS[N]`, `$N` placeholders.
- **Dynamic context**: `!`command`` syntax runs shell commands before sending to Claude.
- **Supporting files**: Templates, examples, scripts in skill directory.
- **Subagent execution**: `context: fork` runs skill in isolated subagent.
- **GUI implementation**: Skill browser/manager. Skill creation wizard. Skill editor with frontmatter visual editor.

### Bundled Skills
- `/batch` -- parallel codebase changes
- `/claude-api` -- API reference loader
- `/debug` -- session troubleshooting
- `/loop` -- repeated prompt execution
- `/simplify` -- code quality review + fix

---

## 17. Permissions System

### Permission Modes

| Mode | Description |
|------|-------------|
| `default` | Standard: prompts for permission on first use |
| `acceptEdits` | Auto-accepts file edits, still prompts for bash |
| `plan` | Read-only: analyze but not modify |
| `dontAsk` | Auto-denies unless pre-approved |
| `bypassPermissions` | Skips all prompts (dangerous, isolated environments only) |

### Permission Rules
- **Format**: `Tool` or `Tool(specifier)` with glob support.
- **Evaluation order**: deny -> ask -> allow (first match wins).
- **Tool-specific syntax**:
  - `Bash(npm run *)` -- wildcard bash commands
  - `Read(./.env)` -- specific file paths (gitignore syntax)
  - `Edit(/src/**/*.ts)` -- path patterns relative to project root
  - `WebFetch(domain:example.com)` -- domain restrictions
  - `mcp__server__tool` -- MCP tool patterns
  - `Agent(AgentName)` -- subagent restrictions
  - `Skill(name)` -- skill restrictions

### Sandboxing
- **What it does**: OS-level filesystem and network isolation for Bash commands. Complementary to permissions.
- **Toggle**: `/sandbox` command.
- **GUI implementation**: Sandbox status indicator. Domain allowlist editor.

### GUI Implementation
- Permission rules editor (visual rule builder with tool type, specifier, and action).
- Permission mode selector (radio buttons or dropdown).
- Permission request dialogs with Allow/Deny/Always options.
- Permission history log.

---

## 18. Checkpointing / Rewind

### How It Works
- **What it does**: Automatically captures code state before each edit. Every user prompt creates a checkpoint. Persists across sessions.
- **Access**: `Esc+Esc` or `/rewind` to open rewind menu.
- **Actions**:
  - **Restore code and conversation** -- revert both to that point
  - **Restore conversation** -- rewind messages, keep current code
  - **Restore code** -- revert files, keep conversation
  - **Summarize from here** -- compress messages from this point forward
- **Limitations**: Does not track bash command changes or external edits. Not a replacement for git.

### GUI Implementation
- Timeline/checkpoint viewer showing each prompt as a checkpoint.
- Restore buttons per checkpoint (code, conversation, or both).
- Summarize option with instruction field.
- Visual diff showing what would change on restore.

---

## 19. CLI Flags and Non-Interactive Mode

### Key CLI Flags

| Flag | Description |
|------|-------------|
| `--print` / `-p` | Non-interactive mode: print response and exit |
| `--output-format` | Output format: `text`, `json`, `stream-json` |
| `--input-format` | Input format: `text`, `stream-json` |
| `--model` | Set model for session |
| `--continue` / `-c` | Continue most recent conversation |
| `--resume` / `-r` | Resume specific session |
| `--max-turns` | Limit agentic turns (print mode) |
| `--max-budget-usd` | Maximum dollar spend (print mode) |
| `--allowedTools` | Tools to auto-approve |
| `--disallowedTools` | Tools to block entirely |
| `--tools` | Restrict available tools |
| `--system-prompt` | Replace entire system prompt |
| `--append-system-prompt` | Append to system prompt |
| `--system-prompt-file` | Load system prompt from file |
| `--permission-mode` | Set permission mode |
| `--dangerously-skip-permissions` | Skip all permission prompts |
| `--add-dir` | Add additional working directories |
| `--worktree` / `-w` | Start in isolated git worktree |
| `--agent` | Specify agent for session |
| `--agents` | Define subagents via JSON |
| `--mcp-config` | Load MCP servers from JSON |
| `--json-schema` | Validated JSON output matching schema (print mode) |
| `--effort` | Set effort level (low/medium/high/max) |
| `--chrome` / `--no-chrome` | Enable/disable Chrome integration |
| `--debug` | Enable debug mode with category filtering |
| `--verbose` | Verbose logging |
| `--remote` | Create web session on claude.ai |
| `--remote-control` / `--rc` | Enable remote control |
| `--teleport` | Resume web session locally |
| `--fork-session` | Fork when resuming |
| `--from-pr` | Resume sessions linked to PR |
| `--ide` | Auto-connect to IDE |
| `--name` / `-n` | Name the session |
| `--fallback-model` | Fallback model when overloaded |
| `--include-partial-messages` | Include partial streaming events |
| `--no-session-persistence` | Don't save session to disk |
| `--setting-sources` | Control which setting sources load |
| `--settings` | Additional settings JSON file/string |
| `--betas` | Beta API headers |
| `--plugin-dir` | Load plugins from directory |
| `--disable-slash-commands` | Disable all skills/commands |

### Non-Interactive Mode (Print Mode)
- **What it does**: `claude -p "query"` runs without interactive UI, prints response, and exits.
- **Piping**: `cat file | claude -p "query"` processes piped content.
- **JSON output**: `--output-format json` or `stream-json` for programmatic use.
- **Structured output**: `--json-schema` for validated JSON matching a schema.
- **GUI relevance**: Used for the Agent SDK integration. Our GUI backend can use this for programmatic queries.

---

## 20. Cost and Usage Tracking

### Cost Tracking
- **What it does**: `/cost` shows API token usage statistics. `/usage` shows plan usage limits and rate limit status.
- **Info displayed**: Input/output tokens, cache hits, estimated cost.
- **GUI implementation**: Cost dashboard showing session costs, cumulative costs, token breakdown.

### Context Usage
- **What it does**: `/context` shows context window fill level as a colored grid with optimization suggestions.
- **GUI implementation**: Context usage meter/bar. Warnings when approaching capacity. Optimization suggestions panel.

### Rate Limiting
- **What it does**: Plan-based rate limits (TPM/RPM). 5-hour session time limit.
- **GUI implementation**: Rate limit status indicator. Usage percentage display.

---

## 21. Desktop App

### Desktop App Features
- **What it does**: Native desktop application (macOS and Windows) that provides GUI for Claude Code.
- **Features**: Parallel sessions with git isolation, visual diff review, app previews, PR monitoring, permission modes, connectors.
- **Shared config**: Same CLAUDE.md, MCP servers, hooks, skills, settings as CLI.
- **Model selection**: Model selected from dropdown before session begins, cannot be changed mid-session.
- **GUI relevance**: Our Tauri app should provide feature parity with this desktop app.

### Session Handoff
- **What it does**: `/desktop` (or `/app`) continues current CLI session in the desktop app.
- **GUI implementation**: Import session from CLI. Export session to CLI.

---

## 22. Plugins System

### Plugin Features
- **What it does**: Extend Claude Code with packages of commands, agents, hooks, MCP servers, and skills.
- **Management**: `/plugin` to manage. `claude plugin install <name>@<marketplace>`.
- **Structure**: Plugin contains commands, agents, hooks, MCP server configs, and skills directories.
- **Marketplaces**: Plugin sources can be configured and restricted by admins.
- **GUI implementation**: Plugin marketplace browser. Install/uninstall buttons. Plugin configuration panel.

---

## 23. Agent Teams

### Agent Teams
- **What it does**: Multiple agents working in parallel, communicating with each other. Unlike subagents (within single session), agent teams coordinate across separate sessions.
- **Display modes**: `auto` (picks best), `in-process`, `tmux`.
- **Configuration**: `--teammate-mode` flag, `teammateMode` setting.
- **Events**: `TeammateIdle` hook fires when a teammate goes idle.
- **GUI implementation**: Multi-agent workspace view showing team members, their status, and communication. Split-pane or tab-based agent views.

---

## Implementation Priority for GUI

### Must-Have (Core)
1. Chat with streaming responses
2. Message history with markdown rendering
3. Tool use visualization (collapsible cards)
4. Permission dialogs
5. File operations (read/edit/create with diff view)
6. Session management (resume, clear, fork)
7. CLAUDE.md / memory management
8. Model selection and configuration
9. Settings panel
10. Context usage indicator

### Should-Have (Important)
1. Plan mode with approval workflow
2. Subagent visualization (running agents, background tasks)
3. Git integration (commit, PR, diff viewer)
4. Slash commands / command palette
5. Keyboard shortcuts
6. MCP server management
7. Checkpointing / rewind
8. Task list display
9. File search (glob/grep) UI
10. Theme support (light/dark)

### Nice-to-Have (Enhancement)
1. Skills management UI
2. Hooks configuration UI
3. Plugin management
4. Agent teams workspace
5. IDE connection
6. Vim mode
7. PR review status
8. Cost/usage dashboard
9. Notebook support
10. Chrome browser integration

---

## Sources

- [Claude Code Official Documentation](https://code.claude.com/docs)
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [Claude Code Interactive Mode](https://code.claude.com/docs/en/interactive-mode)
- [Claude Code Commands Reference](https://code.claude.com/docs/en/commands)
- [Claude Code Permissions](https://code.claude.com/docs/en/permissions)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks)
- [Claude Code Skills](https://code.claude.com/docs/en/skills)
- [Claude Code Subagents](https://code.claude.com/docs/en/sub-agents)
- [Claude Code Memory](https://code.claude.com/docs/en/memory)
- [Claude Code Settings](https://code.claude.com/docs/en/settings)
- [Claude Code MCP](https://code.claude.com/docs/en/mcp)
- [Claude Code Checkpointing](https://code.claude.com/docs/en/checkpointing)
- [Claude Code VS Code](https://code.claude.com/docs/en/vs-code)
- [Claude Code JetBrains](https://code.claude.com/docs/en/jetbrains)
- [Claude Code Desktop](https://code.claude.com/docs/en/desktop)
- [GitHub - anthropics/claude-code](https://github.com/anthropics/claude-code)
- [Claude Code System Prompts Reference](https://github.com/Piebald-AI/claude-code-system-prompts)
- [SmartScope Claude Code Reference Guide](https://smartscope.blog/en/generative-ai/claude/claude-code-reference-guide/)
- [Shipyard Claude Code Cheatsheet](https://shipyard.build/blog/claude-code-cheat-sheet/)
