# Claude Code UX Research: Startup, Help, Slash Commands, and Navigation

**Date:** 2026-03-15
**Version tested:** Claude Code v2.1.55
**Model:** Opus 4.6 (Claude Max)

---

## 1. Startup Experience

### Initial Launch Screen

When the user runs `claude` from a terminal, the startup screen renders immediately:

```
 ▐▛███▜▌   Claude Code v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/claude-tauri-boilerplate

────────────────────────────────────────────────────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────────────────────────────────────────────────────
  dennisonbertram@Mac [07:19:14] [~/Develop/claude-tauri-boilerplate]  [main]   1 MCP server failed · /mcp
```

### Startup Layout Breakdown

The startup screen has exactly **four visual regions**:

#### Region 1: Logo + Version Header
- **Logo:** A stylized Claude logo using Unicode box-drawing characters (`▐▛███▜▌`, `▝▜█████▛▘`, `▘▘ ▝▝`)
- **Version:** `Claude Code v2.1.55` displayed to the right of the logo
- **Model info:** `Opus 4.6 · Claude Max` (model name + subscription tier)
- **Working directory:** `~/Develop/claude-tauri-boilerplate` (uses `~` shorthand)

#### Region 2: Conversation Area
- Empty on first launch (no history to display)
- Grows upward as messages are exchanged

#### Region 3: Input Area
- Delimited by horizontal rules (`────`) above and below
- Prompt character: `❯` (Unicode U+276F, heavy right-pointing angle bracket)
- Cursor appears after the prompt character
- The separator line above the input can display session name on the right side (e.g., `── cli-command-testing ──`)

#### Region 4: Status Bar (Bottom)
- **User identity:** `dennisonbertram@Mac`
- **Timestamp:** `[07:19:14]` (local time in brackets)
- **Working directory:** `[~/Develop/claude-tauri-boilerplate]`
- **Git branch:** `[main]` (in brackets)
- **Alerts:** `1 MCP server failed · /mcp` (contextual alerts with actionable slash command)

### Key Observations
- No loading screen or splash screen
- No onboarding wizard on subsequent launches
- The logo is always present, never scrolls away (it is the first thing in the conversation area)
- MCP server connection status is evaluated at startup and alerts appear immediately
- The status bar is always visible at the very bottom of the terminal

---

## 2. Help System (`/help`)

### Help Dialog Structure

The help system uses a **tabbed dialog** with three tabs:

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Claude Code v2.1.55  general   commands   custom-commands  (←/→ or tab to cycle)            │
│                                                                                              │
│ [Tab content here]                                                                           │
│                                                                                              │
│ For more help: https://code.claude.com/docs/en/overview                                      │
│ Esc to cancel                                                                                │
└──────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Navigation:** Left/Right arrow keys or Tab to cycle between tabs. Escape to dismiss.

### Tab 1: General (Shortcuts Reference)

```
  Claude understands your codebase, makes edits with your permission, and executes commands
  — right from your terminal.

  Shortcuts
  ! for bash mode           double tap esc to clear input        ctrl + shift + - to undo
  / for commands            shift + tab to auto-accept edits     ctrl + z to suspend
  @ for file paths          ctrl + o for verbose output          ctrl + v to paste images
  & for background          ctrl + t to toggle tasks             meta + p to switch model
                            \⏎ for newline                       meta + o to toggle fast mode
                                                                 ctrl + s to stash prompt
                                                                 /keybindings to customize
```

### Complete Keyboard Shortcuts Reference

| Shortcut | Action |
|----------|--------|
| `!` | Enter bash mode (as first character) |
| `/` | Open command palette / slash commands |
| `@` | File path picker / autocomplete |
| `&` | Background task mode |
| `double tap Esc` | Clear input |
| `Shift + Tab` | Cycle permission modes (default -> plan -> auto-accept -> default) |
| `Ctrl + O` | Toggle verbose output |
| `Ctrl + T` | Toggle tasks panel |
| `Ctrl + V` | Paste images |
| `Ctrl + Z` | Suspend Claude Code |
| `Ctrl + Shift + -` | Undo |
| `Ctrl + S` | Stash prompt |
| `Meta + P` | Switch model |
| `Meta + O` | Toggle fast mode |
| `\Enter` | Newline (within input) |
| `Enter` | Submit message (when on empty line after text) |
| `Ctrl + C` | Interrupt / cancel |
| `Escape` | Dismiss dialog / cancel tool execution |
| `Ctrl + B Ctrl + B` (twice) | Run current agent task in background |
| `Ctrl + E` | Explain (in permission prompt context) |
| `Tab` | Amend (in permission prompt context) |
| `Up arrow` | Edit queued messages / navigate history |

### Tab 2: Commands (Built-in Slash Commands)

The commands tab shows a scrollable list with `❯` cursor indicator and up/down arrow navigation. Each command shows its name and a one-line description.

### Tab 3: Custom Commands

Shows user-defined custom slash commands loaded from plugins, skills, and user configuration. Each entry shows the command name, description, and source (e.g., `(user)`).

---

## 3. Complete Slash Commands Reference

### Full List of Built-in Slash Commands

The following is the complete list of built-in commands captured from the `/help` commands tab:

| Command | Description |
|---------|-------------|
| `/add-dir` | Add a new working directory |
| `/agents` | Manage agent configurations |
| `/chrome` | Claude in Chrome (Beta) settings |
| `/clear` | Clear conversation history and free up context |
| `/color` | Set the prompt bar color for this session |
| `/compact` | Clear conversation history but keep a summary in context. Optional: `/compact [instructions for summarization]` |
| `/config` | (Alias via `/status` Config tab) Configure Claude Code preferences |
| `/diff` | View uncommitted changes and per-turn diffs |
| `/doctor` | Diagnose and verify your Claude Code installation and settings |
| `/exit` | Exit the REPL |
| `/export` | Export the current conversation to a file or clipboard |
| `/extra-usage` | Configure extra usage to keep working when limits are hit |
| `/fast` | Toggle fast mode (Opus 4.6 only) |
| `/feedback` | Send feedback about Claude Code |
| `/ide` | Manage IDE integrations and show status |
| `/init` | Initialize a new CLAUDE.md file with codebase documentation |
| `/insights` | Generate a report analyzing your Claude Code sessions |
| `/install-github-app` | Set up Claude GitHub Actions for a repository |
| `/install-slack-app` | Install the Claude Slack app |
| `/keybindings` | Open or create your keybindings configuration file |
| `/login` | Log in with your Anthropic account |
| `/mcp` | Manage MCP server connections |
| `/memory` | View and edit CLAUDE.md memory files |
| `/mobile` | Show QR code to download the Claude mobile app |
| `/model` | Set the AI model for Claude Code |
| `/output-style` | Set the output style directly or from a selection menu |
| `/passes` | Share a free week of Claude Code with friends and earn extra usage |
| `/permissions` | Manage allow & deny tool permission rules |
| `/plan` | Enable plan mode or view the current session plan |
| `/plugins` | Manage Claude Code plugins |
| `/remote-control` | Connect this terminal for remote-control sessions |
| `/remote-env` | Configure the default remote environment for teleport sessions |
| `/rename` | Rename the current conversation |
| `/resume` | Resume a previous conversation |
| `/review` | Review a pull request |
| `/rewind` | Restore the code and/or conversation to a previous point |
| `/status` | Show Claude Code status including version, model, account, API connectivity, and tool statuses |
| `/statusline` | Set up Claude Code's status line UI |
| `/stickers` | Order Claude Code stickers |
| `/tasks` | List and manage background tasks |
| `/terminal-setup` | Install Shift+Enter key binding for newlines |
| `/theme` | Change the theme |
| `/todos` | Manage current todo items |

---

## 4. Detailed Slash Command UI Captures

### `/status` — Three-Tab Settings Panel

The status command opens a tabbed dialog with three tabs: **Status**, **Config**, **Usage**.

#### Status Tab
```
  Settings:  Status   Config   Usage  (←/→ or tab to cycle)

  Version: 2.1.55
  Session name: /rename to add a name
  Session ID: c1489b81-4e61-4f0d-98e0-a7f3e2c378fd
  cwd: /Users/dennisonbertram/Develop/claude-tauri-boilerplate
  Login method: Claude Max Account
  Email: dennison@withtally.com

  Model: Default Opus 4.6 · Most capable for complex work
  MCP servers: plugin:context7:context7 ✔, pencil ✔, tally-wallet ✘, claude-in-chrome ✔, ...
  Memory: user (~/.claude/CLAUDE.md), project (CLAUDE.md),
          auto memory (~/.claude/projects/.../memory/MEMORY.md)
  Setting sources: User settings

  System Diagnostics
   ⚠ Running native installation but config install method is 'global'
   ⚠ Leftover npm global installation at ...
```

**Key details:**
- MCP servers show status icons: `✔` (connected), `✘` (failed), `△` (needs authentication)
- Memory files are listed with their paths
- System diagnostics show warnings with `⚠` prefix

#### Config Tab
```
  Configure Claude Code preferences

  ╭─────────────────────────────────────────────────────────────────────╮
  │ ⌕ Search settings...                                                │
  ╰─────────────────────────────────────────────────────────────────────╯

  ❯ Auto-compact                              true
    Show tips                                 true
    Reduce motion                             false
    Thinking mode                             true
    Fast mode (Opus 4.6 only)                 false
    Rewind code (checkpoints)                 true
    Verbose output                            false
    Terminal progress bar                     true
    Default permission mode                   Default
    Respect .gitignore in file picker         true
    Auto-update channel                       disabled (config)
    Theme                                     Dark mode (ANSI colors only)
    Notifications                             Auto
    Output style                              default
    Language                                  Default (English)
    Editor mode                               normal
    Show PR status footer                     true
    Model                                     Default (recommended)
    Auto-connect to IDE (external terminal)   false
    Claude in Chrome enabled by default       true
    Teammate mode                             auto
    Enable Remote Control for all sessions    default
    Use custom API key: DpolnG-...            false
```

**Key details:**
- Searchable settings list with `⌕ Search settings...` input field
- Settings display as key-value pairs with current values
- Navigation hint: `Type to filter · Enter/↓ to select · Esc to clear`

#### Usage Tab
```
  Current session
  ███                                                6% used
  Resets 12pm (America/New_York)

  Current week (all models)
  ████████████                                       24% used
  Resets Mar 19 at 11pm (America/New_York)

  Current week (Sonnet only)
  ██████▌                                            13% used
  Resets Mar 20 at 9am (America/New_York)
```

**Key details:**
- Three usage meters: session, weekly (all models), weekly (Sonnet)
- Each meter shows a Unicode block progress bar with percentage
- Reset times shown with timezone

### `/model` — Model Selection

```
 Select model
 Switch between Claude models. Applies to this session and future sessions.

 ❯ 1. Default (recommended) ✔  Opus 4.6 · Most capable for complex work
   2. Opus (1M context)        Opus 4.6 with 1M context · Billed as extra usage · $10/$37.50 per Mtok
   3. Sonnet                   Sonnet 4.6 · Best for everyday tasks
   4. Sonnet (1M context)      Sonnet 4.6 with 1M context · Billed as extra usage · $6/$22.50 per Mtok
   5. Haiku                    Haiku 4.5 · Fastest for quick answers

 ▌▌▌ High effort (default) ← → to adjust

 Use /fast to turn on Fast mode (Opus 4.6 only).

 Enter to confirm · Esc to exit
```

**Key details:**
- Numbered list (1-5) with arrow keys for selection
- Current selection marked with `✔`
- Effort slider at bottom: `▌▌▌ High effort (default)` adjustable with left/right arrows
- Pricing info shown for 1M context variants

### `/compact` — Conversation Compaction

```
❯ /compact

✢ Compacting conversation…
```

**Key details:**
- Spinner symbol rotates: `✢` -> `✶` -> `✻` -> `✽` -> (cycles)
- Text: "Compacting conversation..."
- After completion, conversation history is replaced with a summary
- Accepts optional instructions: `/compact [instructions for summarization]`

### `/clear` — Clear History

```
❯ /clear
  ⎿  (no content)
```

**Key details:**
- Immediately clears all conversation history
- Shows `(no content)` as the result
- Resets the conversation to initial state (logo + prompt)
- All previous messages and tool outputs are removed

### `/memory` — Memory Management

```
 Memory

   Auto-memory (research preview): on

 ❯ 1. User memory                               Saved in ~/.claude/CLAUDE.md
   2. Project memory                             Checked in at ./CLAUDE.md
   3. ~/.claude/projects/.../memory/MEMORY.md   auto memory entrypoint
   4. Open auto-memory folder

 Learn more: https://code.claude.com/docs/en/memory

 Enter to confirm · Esc to cancel
```

**Key details:**
- Lists all memory file locations with their types
- User memory vs project memory vs auto memory
- Numbered selection list
- Option to open the auto-memory folder

### `/diff` — Uncommitted Changes

```
 Uncommitted changes (git diff HEAD)

 0 files changed

 Working tree is clean

 ↑/↓ select · Enter view · Esc close
```

**Key details:**
- Shows git diff HEAD output
- Lists changed files count
- Can navigate and view individual file diffs
- Shows "Working tree is clean" when no changes

### `/doctor` — Installation Diagnostics

```
 Diagnostics
 └ Currently running: native (2.1.55)
 └ Path: /Users/dennisonbertram/.local/share/claude/versions/2.1.55
 └ Invoked: /Users/dennisonbertram/.local/share/claude/versions/2.1.55
 └ Config install method: global
 └ Search: OK (bundled)
 Warning: Multiple installations found
 └ npm-global at /Users/dennisonbertram/.nvm/versions/node/v22.21.1/bin/claude
 └ native at /Users/dennisonbertram/.local/bin/claude
 Warning: Running native installation but config install method is 'global'
 Fix: Run claude install to update configuration
 Warning: Leftover npm global installation at ...
 Fix: Run: npm -g uninstall @anthropic-ai/claude-code

 Updates
 └ Auto-updates: disabled (config)
 └ Auto-update channel: latest
 └ Stable version: 2.1.58
 └ Latest version: 2.1.76

 Version Locks
 └ 2.1.55: PID 30521 (running)

 Context Usage Warnings
 └ ⚠ Large MCP tools context (~57,545 tokens > 25,000)
   └ MCP servers:
     └ claude_ai_Linear: 37 tools (~17,464 tokens)
     └ claude_ai_Box: 30 tools (~14,160 tokens)
     └ ...

 Press Enter to continue…
```

**Key details:**
- Tree structure with `└` connectors
- Sections: Diagnostics, Updates, Version Locks, Context Usage Warnings
- Warnings use `Warning:` prefix with `Fix:` suggestions
- Shows token estimates for MCP tools context
- "Press Enter to continue..." at the bottom

### `/permissions` — Permission Rules

```
 Permissions:  Allow   Ask   Deny   Workspace  (←/→ or tab to cycle)

 Claude Code won't ask before using allowed tools.
 ╭────────────────────────────────────────────────────╮
 │ ⌕ Search…                                          │
 ╰────────────────────────────────────────────────────╯

 ❯ 1.  Add a new rule…
   2.  mcp__claude-in-chrome__computer
   3.  mcp__claude-in-chrome__find
   ...

 Press ↑↓ to navigate · Enter to select · Type to search · Esc to cancel
```

**Key details:**
- Four tabs: Allow, Ask, Deny, Workspace
- Searchable list with search input box
- Can add new rules or modify existing ones
- Lists individual tool permissions

### `/theme` — Theme Picker

```
 Theme

 Choose the text style that looks best with your terminal

   1. Dark mode
   2. Light mode
   3. Dark mode (colorblind-friendly)
   4. Light mode (colorblind-friendly)
 ❯ 5. Dark mode (ANSI colors only) ✔
   6. Light mode (ANSI colors only)

╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 1  function greet() {
 2 -  console.log("Hello, World!");
 2 +  console.log("Hello, Claude!");
 3  }
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Syntax theme: ansi (ctrl+t to disable)

 Enter to select · Esc to cancel
```

**Key details:**
- 6 theme options (dark/light x normal/colorblind/ANSI-only)
- Live preview of a code diff below the selection
- Syntax theme toggle with `Ctrl+T`
- Current selection marked with `✔`

### `/export` — Export Conversation

```
 Export Conversation
 Select export method:

 ❯ 1. Copy to clipboard  Copy the conversation to your system clipboard
   2. Save to file       Save the conversation to a file in the current directory

 Esc to cancel
```

### `/plan` — Plan Mode Toggle

```
❯ /plan
  ⎿  Enabled plan mode
```

When plan mode is enabled, the status bar shows:
```
  ⏸ plan mode on (shift+tab to cycle)
```

### `/fast` — Fast Mode Toggle

```
 ↯ Fast mode (research preview)
 High-speed mode for Opus 4.6. Billed as extra usage at a premium rate. Separate rate limits apply.

   Fast mode  OFF  $30/$150 per Mtok

 Learn more: https://code.claude.com/docs/en/fast-mode

 Tab to toggle · Enter to confirm · Esc to cancel
```

### `/output-style` — Output Style Picker

```
 Preferred output style

 This changes how Claude Code communicates with you

 ❯ 1. Default ✔    Claude completes coding tasks efficiently and provides concise responses
   2. Explanatory  Claude explains its implementation choices and codebase patterns
   3. Learning     Claude pauses and asks you to write small pieces of code for hands-on practice

 Enter to confirm · Esc to cancel
```

### `/resume` — Session Picker

```
Resume Session (1 of 20)
╭──────────────────────────────────────────────────────────────────────╮
│ ⌕ Search…                                                            │
╰──────────────────────────────────────────────────────────────────────╯

❯ /clear
  1 second ago · main · 14.4KB

  This session is being continued from a previous conversation...
  1 minute ago · main · 9.6KB

  I want to do research on how to create feature parity...
  4 minutes ago · main · 135.1KB

  ...

Ctrl+A to show all projects · Ctrl+B to toggle branch · Ctrl+V to preview
Ctrl+R to rename · Type to search · Esc to cancel
```

**Key details:**
- Shows session count (`1 of 20`)
- Searchable with `⌕ Search…` input
- Each session shows: first message preview, time ago, branch, size
- Rich keyboard shortcuts for filtering and preview

### `/rewind` — Conversation/Code Restore

```
 Rewind

 Restore the code and/or conversation to the point before…

   /clear
   No code changes

   /export
   No code changes

   /plan
   No code changes

 ❯ (current)

 Enter to continue · Esc to exit
```

**Key details:**
- Lists conversation turns as restore points
- Shows whether code changes were made at each point
- Can restore both conversation and code state

### `/agents` — Agent Management

```
╭──────────────────────────────────────────────────────────────────────╮
│ Agents                                                               │
│ 9 agents                                                             │
│                                                                      │
│ ❯ Create new agent                                                   │
│                                                                      │
│   Plugin agents                                                      │
│   codex-agent:codex-delegator · inherit                              │
│   plugin-dev:agent-creator · sonnet                                  │
│   plugin-dev:plugin-validator · inherit                              │
│   plugin-dev:skill-reviewer · inherit                                │
│                                                                      │
│   Built-in agents (always available)                                 │
│   claude-code-guide · haiku                                          │
│   Explore · haiku                                                    │
│   general-purpose · inherit                                          │
│   Plan · inherit                                                     │
│   statusline-setup · sonnet                                          │
╰──────────────────────────────────────────────────────────────────────╯
```

**Key details:**
- Shows total agent count
- Categorized: Plugin agents, Built-in agents
- Each agent shows name and model (haiku, sonnet, inherit)
- Option to create new agents

### `/tasks` — Background Task Manager

```
 Background tasks

 No tasks currently running

 ↑/↓ to select · Enter to view · Esc to close
```

### `/mcp` — MCP Server Management

```
 Manage MCP servers
 9 servers

   User MCPs (/Users/dennisonbertram/.claude.json)
 ❯ pencil · ✔ connected
   tally-wallet · ✘ failed

   claude.ai
   claude.ai Box · ✔ connected
   claude.ai Gmail · ✔ connected
   claude.ai Google Calendar · △ needs authentication
   claude.ai Linear · ✔ connected
   claude.ai Notion · ✔ connected

   Built-in MCPs (always available)
   claude-in-chrome · ✔ connected
   plugin:context7:context7 · ✔ connected

 ※ Run claude --debug to see error logs
```

**Status icons:**
- `✔` = connected (green)
- `✘` = failed (red)
- `△` = needs authentication (yellow/warning)

### `/ide` — IDE Integration

```
 Select IDE
 Connect to an IDE for integrated development features.

 No available IDEs detected. Make sure your IDE has the Claude Code extension
 or plugin installed and is running.

 Enter to confirm · Esc to cancel
```

### `/color` — Prompt Color

```
❯ /color
  ⎿  Please provide a color. Available colors: red, blue, green, yellow, purple, orange, pink, cyan
```

**Key details:**
- Inline response (not a dialog)
- 8 color options
- Changes the prompt bar color for the current session

### `/rename` — Session Rename

```
❯ /rename
  ⎿  Session and agent renamed to: cli-command-testing
```

**Key details:**
- Auto-generates a name based on conversation content
- The session name appears in the separator line above the input area:
  `───────────────────────────────────────── cli-command-testing ──`

### `/login` — Authentication

```
 Claude Code can be used with your Claude subscription or billed based on
 API usage through your Console account.

 Select login method:

 ❯ 1. Claude account with subscription · Pro, Max, Team, or Enterprise
   2. Anthropic Console account · API usage billing
   3. 3rd-party platform · Amazon Bedrock, Microsoft Foundry, or Vertex AI
```

### `/extra-usage` — Extra Usage Configuration

Same login flow as `/login` but prefixed with:
```
Starting new login following /extra-usage. Exit with Ctrl-C to use existing account.
```

---

## 5. Permission Modes

Claude Code has three permission modes that cycle with **Shift+Tab**:

### Mode 1: Default (Normal)
- No footer indicator (status bar shows normal info)
- Claude asks permission for each tool use
- This is the standard interactive mode

### Mode 2: Plan Mode
```
  ⏸ plan mode on (shift+tab to cycle)
```
- Footer shows pause icon `⏸` with "plan mode on"
- Claude creates plans but does not execute them
- Useful for reviewing what Claude would do before approving

### Mode 3: Auto-Accept Edits
```
  ⏵⏵ accept edits on (shift+tab to cycle)
```
- Footer shows double-play icons `⏵⏵` with "accept edits on"
- Claude automatically applies file edits without asking
- Still asks for bash commands and other risky operations

### Cycling Behavior
```
Default → Plan Mode → Auto-Accept Edits → Default → ...
```

Each mode change is reflected immediately in the status bar footer. The `(shift+tab to cycle)` hint reminds users how to switch modes.

---

## 6. Input Prefixes and Special Characters

### `!` — Bash Mode
- Typing `!` as the first character of input enters bash mode
- The command after `!` is executed directly as a bash command
- Example: `! ls -la` runs `ls -la`

### `/` — Command Palette
- Typing `/` shows an autocomplete dropdown of available slash commands
- As you type more characters, the list filters (e.g., `/co` filters to `/compact`, `/color`, `/code-review`, etc.)
- Custom commands from plugins and skills also appear
- Each entry shows command name and description

```
❯ /co
────────────────────────────────────────────────
  /code-review     Performs external code review...
  /codex           (codex-agent) Delegate a task...
  /codex-agent     Delegates scoped coding tasks...
  /copy-site       Clone a website's design...
  /command-dev...  (plugin-dev) Create a slash...
```

### `@` — File Path Picker
- Typing `@` shows a file browser dropdown
- Lists files and directories in the current working directory
- Files are prefixed with `+`

```
❯ @
────────────────────────────────────────────────
  + pnpm-lock.yaml
  + test/
  + tsconfig.base.json
  + node_modules/
  + .claude/
  + docs/
```

### `&` — Background Task
- Typing `&` as the first character enters background task mode
- The task runs in the background while the user can continue interacting

---

## 7. Tool Use Permission Prompts

When Claude wants to use a tool that requires permission:

```
 Read file

  Read(~/.zshrc)

 Do you want to proceed?
 ❯ 1. Yes
   2. Yes, allow reading from dennisonbertram/ during this session
   3. No

 Esc to cancel · Tab to amend
```

**Key details:**
- Shows the tool name and operation
- Three options: Yes (once), Yes (allow for session), No
- `Esc` to cancel, `Tab` to amend the operation
- Option 2 grants broader permission for the session scope

### Agent Task Execution Display

When an agent (subagent) is running:

```
⏺ statusline-setup(Configure statusline from PS1)
  ⎿  ❯ Configure my statusLine from my shell PS1 configuration
     Read(~/.zshrc)
     Read(~/.claude/settings.json)
     ctrl+b ctrl+b (twice) to run in background
```

**Key details:**
- Agent name shown in bold: `statusline-setup(Configure statusline from PS1)`
- Tool uses listed with `Read()` syntax
- Hint to run in background: `ctrl+b ctrl+b (twice) to run in background`
- When expanded, shows tool results; when collapsed, shows `+N more tool uses (ctrl+o to expand)`

### Interruption UX

```
  ⎿  Interrupted · What should Claude do instead?
```

- Escape interrupts the current operation
- Shows "Interrupted" with prompt for alternative instructions
- User can type new instructions or start fresh

---

## 8. Message Display Format

### User Messages
```
❯ what files are in the current directory?
```
- Prefixed with `❯` prompt character
- No special formatting

### Assistant Messages
```
⏺ Read 1 file (ctrl+o to expand)

⏺ Here's what's in the project root:

  - CLAUDE.md — Project policies for AI agents
  - apps/ — Tauri desktop frontend + Hono/Bun backend server
  ...
```

- Prefixed with `⏺` (filled circle) for each message block
- Tool use blocks are collapsible with `ctrl+o`
- Markdown formatting is rendered (lists, bold, code blocks, etc.)

### Collapsed vs Expanded Tool Output
```
# Collapsed:
⏺ Reading 1 file… (ctrl+o to expand)

# Expanded:
⏺ Read(file.txt)
  ⎿  [file contents]
```

### Spinner/Loading States
The spinner cycles through Unicode symbols:
- `✢` (four-pointed star)
- `✶` (six-pointed star)
- `✻` (teardrop star)
- `✽` (heavy teardrop star)

Accompanied by whimsical verbs:
- `Flambéing…`
- `Shimmying…`
- `Envisioning…`
- `Compacting conversation…`

### Dismissed Dialog Messages
```
❯ /help
  ⎿  Help dialog dismissed

❯ /status
  ⎿  Status dialog dismissed

❯ /model
  ⎿  Kept model as Default (recommended)
```

Each slash command that is dismissed shows a feedback message with `⎿` (left floor bracket) connector.

---

## 9. Status Bar Components

The status bar at the bottom of the terminal contains:

```
  dennisonbertram@Mac [07:19:14] [~/Develop/claude-tauri-boilerplate]  [main]   1 MCP server failed · /mcp
```

| Component | Format | Purpose |
|-----------|--------|---------|
| User | `user@host` | Identity |
| Time | `[HH:MM:SS]` | Current local time in brackets |
| Directory | `[~/path/to/dir]` | Working directory in brackets |
| Git branch | `[branch]` | Current git branch in brackets |
| Alerts | `N MCP server failed · /mcp` | Contextual alerts with actionable commands |
| Permission mode | `⏸ plan mode on` / `⏵⏵ accept edits on` | Current permission mode (when not default) |

---

## 10. CLI Command-Line Flags

The CLI supports extensive flags for programmatic/non-interactive use:

### Key Flags for GUI Implementation

| Flag | Purpose | GUI Relevance |
|------|---------|---------------|
| `--model <model>` | Set model | Model selector dropdown |
| `--permission-mode <mode>` | Set permission mode | Permission toggle |
| `-r, --resume [id]` | Resume session | Session picker |
| `-c, --continue` | Continue most recent | Quick resume button |
| `-p, --print` | Non-interactive output | Headless/API mode |
| `--output-format <format>` | text/json/stream-json | Output processing |
| `--system-prompt <prompt>` | Custom system prompt | Settings |
| `--effort <level>` | low/medium/high | Effort slider |
| `--max-budget-usd <amount>` | Spending limit | Budget control |
| `--mcp-config <configs>` | MCP server config | MCP management |
| `-w, --worktree [name]` | Git worktree | Branch isolation |
| `--agent <agent>` | Select agent | Agent picker |
| `--allowedTools <tools>` | Allow specific tools | Permission rules |
| `--json-schema <schema>` | Structured output | Output validation |
| `--chrome` / `--no-chrome` | Chrome integration | Browser toggle |
| `--ide` | Auto-connect IDE | IDE integration |

### Permission Mode Values
- `default` — Ask for each action
- `acceptEdits` — Auto-accept file edits
- `plan` — Plan only, no execution
- `bypassPermissions` — Skip all checks (sandbox only)
- `dontAsk` — Accept everything

### CLI Subcommands
| Subcommand | Purpose |
|------------|---------|
| `claude agents` | List configured agents |
| `claude auth` | Manage authentication |
| `claude doctor` | Health check for auto-updater |
| `claude install [target]` | Install native build |
| `claude mcp` | Configure MCP servers |
| `claude plugin` | Manage plugins |
| `claude setup-token` | Set up long-lived auth token |
| `claude update` / `claude upgrade` | Check for and install updates |

---

## 11. GUI Implementation Notes

### Startup Screen
- Render the Claude logo with appropriate styling (the ASCII art is specific Unicode characters)
- Show version, model, and subscription tier prominently
- Display the working directory path
- The status bar must always be visible and updated in real-time

### Help System
- Implement as a modal dialog with tab navigation
- Three tabs: General (shortcuts), Commands (scrollable list), Custom Commands
- Keyboard navigation with arrow keys and Tab
- Escape to dismiss

### Slash Commands
- Implement command palette with fuzzy search/filtering
- Show command name and description in dropdown
- Support both built-in and custom commands
- Each command opens its own appropriate UI (dialog, inline response, etc.)

### Dialog Pattern
Most slash commands follow one of these patterns:
1. **Tabbed dialog** — `/status`, `/help`, `/permissions` (multi-section content)
2. **Selection list** — `/model`, `/theme`, `/output-style`, `/export` (pick one option)
3. **Scrollable list** — `/resume`, `/agents`, `/mcp` (browse and select)
4. **Inline response** — `/clear`, `/compact`, `/color`, `/rename`, `/plan` (immediate feedback)
5. **Full-screen flow** — `/login`, `/extra-usage` (multi-step authentication)

### Common UI Elements
- `❯` cursor for list selection
- `✔` checkmark for current/selected option
- `⌕ Search…` input box with rounded border (`╭─╮`, `╰─╯`)
- `⏸` pause icon for plan mode
- `⏵⏵` double-play icon for auto-accept mode
- `⎿` tree connector for nested output
- `✢✶✻✽` rotating spinner characters
- `└` tree connector for hierarchical output
- `⚠` warning indicator
- `✔✘△` status indicators (success/fail/warning)
- `█` filled block for progress bars
- `╌` dashed line for separators

### Navigation Patterns
- **Arrow keys:** Up/Down to navigate lists, Left/Right to switch tabs
- **Tab:** Cycle tabs in tabbed dialogs
- **Enter:** Confirm/select
- **Escape:** Cancel/dismiss
- **Type:** Filter/search in searchable lists
- **Numbered shortcuts:** Some dialogs support pressing 1-5 to select

### Session Name Display
When a session is renamed, the name appears right-aligned in the input area separator:
```
──────────────────────────────────────────────── cli-command-testing ──
❯
──────────────────────────────────────────────────────────────────────
```

---

## 12. Summary of All UI Panels/Dialogs

| Command | Dialog Type | Has Tabs | Searchable | Scrollable |
|---------|------------|----------|------------|------------|
| `/help` | Modal dialog | Yes (3) | No | Yes (commands tab) |
| `/status` | Modal dialog | Yes (3) | Yes (config tab) | Yes |
| `/model` | Selection list | No | No | No |
| `/theme` | Selection list | No | No | No (with preview) |
| `/permissions` | Modal dialog | Yes (4) | Yes | Yes |
| `/mcp` | Scrollable list | No | No | Yes |
| `/agents` | Scrollable list (boxed) | No | No | Yes |
| `/resume` | Scrollable list | No | Yes | Yes |
| `/export` | Selection list | No | No | No |
| `/output-style` | Selection list | No | No | No |
| `/fast` | Toggle dialog | No | No | No |
| `/memory` | Selection list | No | No | No |
| `/diff` | Scrollable list | No | No | Yes |
| `/rewind` | Scrollable list | No | No | Yes |
| `/doctor` | Full-page display | No | No | Yes |
| `/tasks` | Scrollable list | No | No | Yes |
| `/ide` | Selection list | No | No | No |
| `/login` | Selection list | No | No | No |
| `/color` | Inline response | N/A | N/A | N/A |
| `/clear` | Inline response | N/A | N/A | N/A |
| `/compact` | Inline response | N/A | N/A | N/A |
| `/plan` | Inline response | N/A | N/A | N/A |
| `/rename` | Inline response | N/A | N/A | N/A |
