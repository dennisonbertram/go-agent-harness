# TUI UX Research Summary: Comprehensive Analysis

**Date:** 2026-03-15
**Sources analyzed:**
- `claude-code-ux-chat-streaming.md` (v2.1.55, Opus 4.6, tmux captures)
- `claude-code-ux-startup-navigation.md` (v2.1.55, Opus 4.6, tmux captures)
- `claude-code-ux-status-errors-advanced.md` (v2.1.55, macOS tmux captures)
- `claude-code-ux-tool-use-diffs.md` (v2.1.55, Darwin 24.1.0 ARM64)
- `crush-review.md` (charmbracelet/crush, Bubble Tea, Go)
- `opencode-review.md` (opencode-ai, TypeScript+Zig)
- `pi-review.md` (badlogic/pi-mono + can1357/oh-my-pi)

This document is the exhaustive extraction of every documented UX pattern across all research files, grouped by implementation difficulty tier. It is intended to drive GitHub ticket creation for the go-agent-harness TUI.

---

## Table of Contents

1. [Complete UX Feature List](#1-complete-ux-feature-list)
2. [Visual Hierarchy — Exact Layout, Spacing, Characters](#2-visual-hierarchy)
3. [Status Bar Components — All States](#3-status-bar-components)
4. [Chat and Streaming UX](#4-chat-and-streaming-ux)
5. [Slash Commands — Complete Reference](#5-slash-commands)
6. [Input Area](#6-input-area)
7. [Tool Use Display](#7-tool-use-display)
8. [Error Handling UX](#8-error-handling-ux)
9. [Startup / Welcome Screen](#9-startup-and-welcome-screen)
10. [Help System](#10-help-system)
11. [Markdown Rendering](#11-markdown-rendering)
12. [ANSI Color Codes — Complete Palette](#12-ansi-color-palette)
13. [Implementation Difficulty Tiers](#13-implementation-difficulty-tiers)

---

## 1. Complete UX Feature List

Every documented UI element, interaction pattern, and visual component across all source files.

### 1.1 Layout Regions (Persistent)

| Region | Description | Fixed/Scrollable |
|--------|-------------|-----------------|
| Header/Logo | Branded ASCII logo + version + model + working dir | Fixed top (in scrollback) |
| Conversation area | Messages, tool calls, responses, inline results | Scrollable |
| Input area | Between two `────` separator lines; `❯` prompt | Fixed bottom |
| Status bar | Single line (sometimes two) below input | Fixed bottom |

### 1.2 Message Types

| Type | Indicator | Background | Notes |
|------|-----------|-----------|-------|
| User message | `❯` prefix | Full-width gray (`[100m`) | Bright white text on dark gray |
| AI text response | `⏺` prefix (bright white) | None | Indented body text |
| AI tool invocation | `⏺` prefix (bright green) | None | Shows tool name + count |
| Tool result | `⎿` tree connector | None | Indented beneath tool call |
| Spinner / loading | `✶·✻✽✳✢` rotating + whimsical verb | None | Appears while LLM thinking |
| Interruption notice | `⎿ Interrupted · What should Claude do instead?` | None | Below last user message |
| System/exit message | Dim text, no prefix | None | `Resume this session with:...` |

### 1.3 Interaction Patterns

| Pattern | Trigger | Behavior |
|---------|---------|----------|
| Command palette | Type `/` | Dropdown with filtered slash commands |
| File picker | Type `@` | Dropdown with file/dir list |
| Bash mode | Type `!` as first char | Direct shell command |
| Background mode | Type `&` as first char | Background task execution |
| Tool expand/collapse | `Ctrl+O` | Toggle collapsed vs detailed transcript |
| Permission mode cycle | `Shift+Tab` | Rotates Default → Accept Edits → Plan |
| Interrupt generation | `Escape` | Stops LLM response immediately |
| Interrupt (second) | Double `Escape` | Clears input (two-step safety) |
| Edit in external editor | `Ctrl+G` | Opens `$EDITOR` (or nano) |
| Stash prompt | `Ctrl+S` | Saves current input for later |
| Paste image | `Ctrl+V` | Pastes image into input |
| Switch model | `Meta+P` | Model picker shortcut |
| Toggle fast mode | `Meta+O` | Fast mode toggle shortcut |
| Undo | `Ctrl+Shift+-` | Undo last code change |
| Suspend | `Ctrl+Z` | Suspends process like any terminal app |
| Toggle tasks | `Ctrl+T` | Shows/hides background tasks |
| Show/collapse history | `Ctrl+E` | Expand or collapse previous messages |
| Verbose toggle | `Ctrl+O` | Toggles detailed tool output view |

### 1.4 Dialog Types

| Dialog Type | Examples | Characteristics |
|-------------|---------|-----------------|
| Tabbed dialog | `/help`, `/status`, `/permissions` | Tabs nav with ←/→ or Tab |
| Selection list | `/model`, `/theme`, `/output-style`, `/export` | Numbered, `❯` cursor, `✔` for current |
| Scrollable list | `/resume`, `/agents`, `/mcp`, `/tasks` | Search box, metadata per item |
| Boxed scrollable | `/agents` | `╭──╮` border around entire dialog |
| Inline response | `/clear`, `/compact`, `/plan`, `/color`, `/rename` | `⎿` connector, no dialog |
| Full-screen flow | `/login`, `/extra-usage` | Takes over terminal, modal |
| Full-page info | `/doctor` | Scrollable, "Press Enter to continue..." |
| Toggle dialog | `/fast` | Single toggle (`OFF`/`ON`) with pricing |

### 1.5 Permission Prompt Types

| Type | Tool category | Extra options |
|------|--------------|---------------|
| Bash command | Bash outside working dir | Yes / No + `ctrl+e to explain` |
| File read | Read outside working dir | Yes / Yes (allow dir this session) / No |
| File edit | All file edits | Yes / Yes (allow all this session) / No + `Tab to amend` |
| File create | All file writes | Yes / Yes (allow all this session) / No |
| Destructive op | Claude-generated question | Yes/No/Type something/Chat about this |
| Agent task running | Subagent display | `ctrl+b ctrl+b (twice) to run in background` |

### 1.6 Progress / Loading States

| State | Display | Notes |
|-------|---------|-------|
| Thinking (early) | `✻ Determining… (thought for 2s)` | Spinner + verb + duration |
| Thinking (long) | `· Razzmatazzing… (37s · ↓ 1.8k tokens)` | Duration + token count |
| Thinking (very long) | `✽ Waddling… (1m 20s · ↓ 4.2k tokens)` | Minutes format |
| Tool in-progress | `⏺ Reading 1 file… (ctrl+o to expand)` | Trailing ellipsis |
| Compacting | `✢ Compacting conversation…` | Distinct spinner symbol |
| Work complete | `✻ Worked for 1m 0s` | Summary after completion |
| Tips during loading | `⎿  Tip: [tip text]` | Shown below spinner |

---

## 2. Visual Hierarchy

### 2.1 Exact Layout Structure

```
 ▐▛███▜▌   Claude Code v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/project-name
                                           ← blank line after logo
[37m[100m❯ [97mUser message text here                        [39m[49m
[100m                                               [39m[49m
                                           ← blank line after user msg
[92m⏺[39m Read 1 file (ctrl+o to expand)        ← green circle for tool use
  [97m⎿  [39mfile contents or summary...

[97m⏺[39m Response text paragraph one            ← white circle for text
  Continuation lines indented 2 spaces
  to align with text after the prefix

  More body text here
                                           ← blank line between turns
[37m[100m❯ [97mNext user message here                        [39m[49m
[100m                                               [39m[49m

────────────────────────────────────────────── session-name ──
❯ [7m [0m
──────────────────────────────────────────────────────────────
  dennisonbertram@Mac [21:06:00] [~/Develop/project]  [main *]   1 MCP server failed · /mcp
```

### 2.2 Separator Lines

| Element | Character | Unicode | ANSI style | Width |
|---------|-----------|---------|-----------|-------|
| Input top border | `─` | U+2500 | `[2m[37m` (dim white) | Full terminal width |
| Input bottom border | `─` | U+2500 | `[2m[37m` (dim white) | Full terminal width |
| Dialog borders | `────` (solid) | U+2500 | Default | Full width |
| Diff borders | `╌` (dashed) | U+254C | Default | Full width |
| Config tab separator | `╌` (dashed) | U+254C | Default | Full width |

Session name appears in the TOP separator, right-aligned:
```
──────────────────────────────────────────── cli-command-testing ──
```

### 2.3 Indentation Rules

| Element | Indent | Notes |
|---------|--------|-------|
| User messages (`❯`) | 0 spaces, flush left | `❯ ` prefix |
| Tool calls (`⏺`) | 0 spaces, flush left | `⏺ ` prefix |
| Tool results (`⎿`) | 2 spaces from left | `  ⎿  ` (2 spaces + char + 2 spaces) |
| Tool result content | 4-6 spaces | Under the `⎿` |
| Response text body | 2 spaces | Aligns with text after `⏺ ` |
| Diff lines | 6 spaces | Line numbers right-aligned within gutter |
| Continuation of user msg | 2 spaces | Wraps inside gray background |
| List items | 2 spaces + `- ` | Within response body |

### 2.4 Spacing Pattern Between Messages

```
[User message block]     ← gray bg, blank trailing line included
                         ← explicit blank line
[Tool call line]
  ⎿  [tool result]
                         ← blank line
[Response text block]
                         ← blank line before next message
[Next user message]
```

### 2.5 Icon / Symbol Reference Table

| Symbol | Unicode | ANSI color | Role |
|--------|---------|-----------|------|
| `❯` | U+276F | `[37m` on user msg; no color on input prompt | User message prefix / input prompt |
| `⏺` | U+23FA | `[97m` (bright white) text; `[92m` (bright green) tool | AI response / tool call |
| `⎿` | U+239F | `[97m` | Tree connector (results beneath parent) |
| `─` | U+2500 | `[2m[37m` (dim white) | Horizontal separator |
| `╌` | U+254C | Default | Dashed separator (diffs, settings) |
| `✶` | U+2736 | Default | Spinner state 1 |
| `·` | U+00B7 | Default | Spinner state 2 |
| `✻` | U+273B | Default | Spinner state 3 |
| `✽` | U+273D | Default | Spinner state 4 |
| `✳` | U+2733 | Default | Spinner state 5 |
| `✢` | U+2722 | Default | Compaction spinner / spinner state 6 |
| `▐▛███▜▌` / `▝▜█████▛▘` / `▘▘ ▝▝` | Block chars | `[91m` bright red | Logo |
| `╭─╮` / `╰─╯` | U+256D-U+256F | Default | Search box border |
| `╭──╮` / `╰──╯` | Box-drawing | Default | Dialog box border |
| `┌─┬┐` / `│├┼┤` / `└┴┘` | Box-drawing | Default | Table borders |
| `⌕` | U+2315 | Default | Search field icon |
| `✔` | U+2714 | Green (`[32m`) | Selected / connected |
| `✘` | U+2718 | Red (`[31m`) | Failed |
| `△` | U+25B3 | Yellow | Needs authentication |
| `⚠` | U+26A0 | Default | Warning |
| `⏸` | U+23F8 | Default | Plan mode indicator |
| `⏵⏵` | U+23F5×2 | Default | Auto-accept edits indicator |
| `↯` | U+21AF | Default | Fast mode icon |
| `☐` | U+2610 | Default | Confirm checkbox icon |
| `●` | U+25CF | Per-model color | Token chart legend |
| `░▒▓█` | U+2591-U+2588 | Default | Stats heatmap / progress bars |
| `⛁⛀⛶⛝` | Specialized | Default | Context usage grid |
| `└` | U+2514 | Default | Tree connector (doctor output) |
| `※` | U+203B | Default | Note/tip indicator |

### 2.6 Logo ASCII Art

```
 ▐▛███▜▌   [1mClaude Code[0m v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/current-directory
```

Logo uses: `▐▛▜▌▝▛▘` (Unicode block element characters U+2590-U+259F range).
ANSI: `[91m` (bright red) for filled blocks, `[40m` (black bg) for center blocks.

### 2.7 Box-Drawing Borders

Rounded box (used for search inputs and `/agents`):
```
╭──────────────────────────────────────────────────╮
│ ⌕ Search…                                        │
╰──────────────────────────────────────────────────╯
```

Hard box (used for tabbed dialogs like `/help`, `/status`):
```
┌──────────────────────────────────────────────────┐
│ Claude Code v2.1.55  general   commands           │
│                                                  │
└──────────────────────────────────────────────────┘
```

Welcome screen box:
```
╭─── Claude Code v2.1.55 ────────────────────────────╮
│                          │ Tips for getting started  │
│    Welcome back Dennison!│ Run /init to create...    │
│         ▐▛███▜▌          │ ─────────────────────     │
│        ▝▜█████▛▘         │ Recent activity           │
│          ▘▘ ▝▝           │ No recent activity        │
│   Opus 4.6 · Claude Max  │                           │
│ /private/tmp/claude-test │                           │
╰────────────────────────────────────────────────────╯
```

---

## 3. Status Bar Components

The status bar is a persistent line at the very bottom of the terminal. It never scrolls away.

### 3.1 Default State

```
  dennisonbertram@Mac [21:00:18] [~/Develop/project]  [main *]
```

### 3.2 All Status Bar States

| State | Display |
|-------|---------|
| Default (clean) | `  user@host [HH:MM:SS] [~/path/to/dir]  [branch]` |
| Dirty git | `  user@host [HH:MM:SS] [~/path]  [main *]` — `*` in red |
| MCP failures | Appends: `  N MCP server(s) failed · /mcp` |
| MCP success | Appends: `  Claude in Chrome enabled · /chrome` |
| Keyboard hint | Appends: `  ctrl+g to edit in nano` |
| Plan mode | Second line: `  ⏸ plan mode on (shift+tab to cycle)` |
| Accept edits | Second line: `  ⏵⏵ accept edits on (shift+tab to cycle)` |
| Escape hint | `  Esc to clear again` (after first Escape with text in input) |
| Dialog footer | Replaced by dialog-specific hints when dialogs are open |

### 3.3 Component Breakdown

| Component | Format | ANSI Code |
|-----------|--------|-----------|
| Username | `user@host` | `[1m[32m` (bold green) |
| Timestamp | `[HH:MM:SS]` — 24h in brackets | `[34m` (blue) |
| Working dir | `[~/path/to/dir]` | `[37m` (white) |
| Git branch | `[branch]` | `[32m` (green) |
| Dirty indicator | `*` inside branch brackets | `[31m` (red) |
| MCP failure count | `N MCP servers failed · /mcp` | `[91m` (bright red) |
| MCP success notice | `Feature enabled · /command` | Default |
| Permission mode | `⏸ plan mode on` or `⏵⏵ accept edits on` | Default |
| Cycle hint | `(shift+tab to cycle)` | Dim |

---

## 4. Chat and Streaming UX

### 4.1 User Messages

```
[37m[100m❯ [97mMessage text here                                    [39m[49m
[97m[100m                                                         [39m[49m
```

- Full-width gray background (`[100m` = bright black / dark gray)
- `❯` prompt character in `[37m]` (white), text in `[97m]` (bright white)
- Trailing blank line with gray background (bottom padding)
- Wraps with 2-space indent for continuation lines, all within gray background

### 4.2 AI Response Messages

```
[97m⏺[39m Response text line one
  Second line indented 2 spaces
```

- No background color
- `⏺` prefix in `[97m]` (bright white) for text, `[92m]` (bright green) for tool calls
- Body text at 2-space indent from left margin
- Paragraph breaks shown as blank lines within the response block

### 4.3 Loading Spinner

```
✶ Leavening…
  ⎿  Tip: Name your conversations with /rename to find them easily
```

Spinner symbol cycles: `✶ · ✻ ✽ ✳ ✢` at approximately 100-200ms interval.

**Whimsical loading verbs** (randomly selected per request):
- Twisting, Leavening, Drizzling, Sauteing, Channeling, Determining
- Waddling, Beboppin', Razzmatazzing, Flambéing, Shimmying, Envisioning
- Vibing, Frolicking (many more in pool)

**During extended thinking**, metrics are added:
```
· Razzmatazzing… (37s · ↓ 1.8k tokens)
✳ Determining… (1m 20s · ↓ 4.2k tokens)
```

Format: `<spinner> <verb>… (<duration> · ↓ <token_count> tokens)`

**After completion:**
```
✻ Worked for 1m 0s
```

**Tips shown during loading** (below spinner):
```
  ⎿  Tip: Run /install-github-app to tag @claude in PRs
  ⎿  Tip: Name your conversations with /rename...
  ⎿  Tip: Run Claude Code locally or remotely using the Claude desktop app
```

### 4.4 Streaming Behavior

- During spinner phase: no text visible
- Text appears as a block once first tokens arrive (not character-by-character in visible area)
- View scrolls down as content streams in
- Response grows from top to bottom
- No visible cursor or typing animation in response area
- The TUI re-renders the entire response view on each update (React/Ink model)

### 4.5 Interruption UX

**Escape during response:**
```
❯ Write a detailed essay about the history of the internet...
  ⎿  Interrupted · What should Claude do instead?
```

- `⎿` connects notice to user message
- Partial response is fully discarded (not shown)
- Input area becomes active immediately for redirect instruction
- Text `"Interrupted"` + separator `" · "` + prompt `"What should Claude do instead?"`
- Styling: `[37m]` (white)

**First Escape on non-empty input:**
```
  Esc to clear again
```
Status bar shows the hint; second Escape actually clears.

**Ctrl+C:**
- During response: same as Escape (interrupts)
- During input with text: clears input
- At empty prompt: no effect
- Does NOT exit Claude Code

### 4.6 Conversation Flow

```
❯ [user message]                      ← gray background block
[blank line]
⏺ [tool call]                         ← green circle
  ⎿  [tool result]
[blank line]
⏺ [response text]                     ← white circle
  [body text indented]
[blank line]
❯ [next user message]                 ← gray background block
```

No explicit separator lines between turns (blank lines only). Only the input area has border lines.

### 4.7 History Navigation

| Shortcut | Action |
|----------|--------|
| `Ctrl+O` | Toggle collapsed/expanded (detailed transcript) |
| `Ctrl+E` | Show/collapse all previous messages |

When in detailed mode:
```
Showing detailed transcript · ctrl+o to toggle · ctrl+e to collapse
```

When messages collapsed:
```
ctrl+e to show 108 previous messages
```

### 4.8 Exit and Session Resume

**`/exit` command:**
```
❯ /exit
  ⎿  Goodbye!     (or "Catch you later!")
```

**After exit (in terminal):**
```
[2mResume this session with:[0m
[2mclaude --resume 6fea0837-3902-4e2c-98f7-6f9d6a961c7d[0m
```

Both lines in dim styling (`[2m`). Session UUID is provided.

---

## 5. Slash Commands

### 5.1 Complete Built-in Command List

| Command | Description | Dialog Type |
|---------|-------------|-------------|
| `/add-dir` | Add a new working directory | Inline |
| `/agents` | Manage agent configurations | Boxed scrollable list |
| `/chrome` | Claude in Chrome (Beta) settings | Dialog |
| `/clear` | Clear conversation history | Inline: `(no content)` |
| `/color` | Set prompt bar color for session | Inline: lists available colors |
| `/compact` | Clear history but keep summary; optional: `/compact [instructions]` | Inline: spinner then result |
| `/config` | (Alias inside `/status` Config tab) | Tabbed dialog (Status/Config/Usage) |
| `/context` | Visualize context usage as 10x10 grid | Inline: grid output |
| `/copy` | Copy Claude's last response as markdown | Inline |
| `/diff` | View uncommitted changes and per-turn diffs | Scrollable list |
| `/doctor` | Diagnose installation and settings | Full-page with tree output |
| `/exit` | Exit the REPL | Inline: goodbye message |
| `/export` | Export conversation to clipboard or file | Selection list (2 options) |
| `/extra-usage` | Configure extra usage | Full-screen login flow |
| `/fast` | Toggle fast mode (Opus 4.6 only) | Toggle dialog with pricing |
| `/feedback` | Send feedback | Dialog |
| `/fork` | Fork conversation at current point | Inline: shows UUID |
| `/help` | Show help and commands | Tabbed dialog (general/commands/custom) |
| `/hooks` | Configure hooks | Dialog |
| `/ide` | Manage IDE integrations | Selection list |
| `/init` | Initialize CLAUDE.md | Inline |
| `/insights` | Generate session analysis report | Dialog |
| `/install-github-app` | Set up Claude GitHub Actions | Dialog |
| `/install-slack-app` | Install Claude Slack app | Dialog |
| `/keybindings` | Open keybindings config file | Inline |
| `/login` | Log in with Anthropic account | Full-screen selection |
| `/logout` | Sign out | Inline |
| `/mcp` | Manage MCP server connections | Scrollable list with status icons |
| `/memory` | View and edit CLAUDE.md files | Selection list |
| `/mobile` | Show QR code for mobile app | Dialog |
| `/model` | Set AI model | Numbered selection with effort slider |
| `/output-style` | Set output style | Selection list (3 options) |
| `/passes` | Share free week of Claude Code | Dialog |
| `/permissions` | Manage allow/ask/deny rules | 4-tab dialog with search |
| `/plan` | Enable plan mode | Inline: `Enabled plan mode` |
| `/plugins` | Manage plugins | Dialog |
| `/pr-comments` | Get GitHub PR comments | Inline |
| `/privacy-settings` | View/update privacy | Dialog |
| `/release-notes` | Show release notes | Dialog |
| `/remote-control` | Connect for remote-control sessions | Dialog |
| `/remote-env` | Configure remote environment | Dialog |
| `/rename` | Rename current conversation | Inline: auto-generates name |
| `/resume` | Resume previous conversation | Searchable scrollable list |
| `/review` | Review a pull request | Dialog |
| `/rewind` | Restore code/conversation to previous point | Scrollable checkpoint list |
| `/sandbox` | Configure sandbox | Toggle dialog |
| `/security-review` | Security review of pending changes | Dialog |
| `/skills` | List available skills | Inline |
| `/stats` | Usage statistics dashboard | 2-tab dialog (Overview/Models) |
| `/status` | Show Claude Code status | 3-tab dialog (Status/Config/Usage) |
| `/statusline` | Set up status line UI | Dialog |
| `/stickers` | Order Claude Code stickers | Dialog |
| `/tasks` | List and manage background tasks | Scrollable list |
| `/terminal-setup` | Install Shift+Enter key binding | Dialog |
| `/theme` | Change theme | Selection list with live preview |
| `/todos` | Manage todo items | Dialog |

### 5.2 Slash Command Autocomplete

When typing `/` in the input area:

```
❯ /co
────────────────────────────────────────────────
  /code-review     Performs external code review...
  /codex           (codex-agent) Delegate a task...
  /codex-agent     Delegates scoped coding tasks...
  /copy-site       Clone a website's design...
  /command-dev...  (plugin-dev) Create a slash...
```

- Dropdown appears below input area
- As you type, list filters in real-time (fuzzy match)
- Each entry: command name (left-aligned) + description (right-aligned, truncated with `...`)
- Selected item: `[94m` (bright blue) + `[7m` (reverse video)
- Enter to select, Escape to dismiss
- Custom commands from plugins/skills also appear, labeled with source (e.g., `(codex-agent)`)

### 5.3 Dismissed Dialog Feedback

Each slash command that opens a dialog shows feedback when dismissed:

```
❯ /help
  ⎿  Help dialog dismissed

❯ /status
  ⎿  Status dialog dismissed

❯ /model
  ⎿  Kept model as Default (recommended)

❯ /export
  ⎿  Export cancelled

❯ /resume
  ⎿  Resume cancelled
```

All use the `⎿` tree connector. Messages are conversational and specific.

### 5.4 Key Slash Command Captures

#### `/status` — Three-Tab Settings Panel

```
  Settings:  Status   Config   Usage  (←/→ or tab to cycle)

  Version: 2.1.55
  Session name: /rename to add a name
  Session ID: c1489b81-4e61-4f0d-98e0-a7f3e2c378fd
  cwd: /Users/.../project
  Login method: Claude Max Account
  Email: user@example.com

  Model: Default Opus 4.6 · Most capable for complex work
  MCP servers: context7 ✔, pencil ✔, tally-wallet ✘, ...
  Memory: user (~/.claude/CLAUDE.md), project (CLAUDE.md), auto memory (...)
  Setting sources: User settings

  System Diagnostics
   ⚠ Running native installation but config install method is 'global'
```

#### `/model` — Model Selection

```
 Select model
 Switch between Claude models.

 ❯ 1. Default (recommended) ✔  Opus 4.6 · Most capable for complex work
   2. Opus (1M context)        Opus 4.6 with 1M context · $10/$37.50 per Mtok
   3. Sonnet                   Sonnet 4.6 · Best for everyday tasks
   4. Sonnet (1M context)      Sonnet 4.6 with 1M context · $6/$22.50 per Mtok
   5. Haiku                    Haiku 4.5 · Fastest for quick answers

 ▌▌▌ High effort (default) ← → to adjust

 Enter to confirm · Esc to exit
```

Effort slider: `▌▌▌` = High, `▌▌` = Medium, `▌` = Low. Adjusted with ←/→.

#### `/context` — Context Usage Visualization

```
❯ /context
  ⎿  Context Usage
     ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   claude-opus-4-6 · 41k/200k tokens (20%)
     ⛁ ⛀ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   Estimated usage by category
     ...                       ⛁ System prompt: 4.5k tokens (2.2%)
     ...                       ⛁ System tools: 18.7k tokens (9.4%)
     ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝   ⛁ Messages: 6.9k tokens (3.4%)
                               ⛶ Free space: 125k (62.7%)
                               ⛝ Autocompact buffer: 33k (16.5%)
```

- 10x10 grid of cylinder icons: `⛁` (used), `⛀` (partial), `⛶` (free), `⛝` (autocompact buffer)
- Category breakdown listed on right side of grid
- Token counts in `Xk` format with percentages

#### `/resume` — Session Picker

```
Resume Session (1 of 20)
╭──────────────────────────────────────────────────────╮
│ ⌕ Search…                                            │
╰──────────────────────────────────────────────────────╯

  current worktree

❯ Run git status and show me the output
  8 seconds ago · main · 28.4KB

  read the file CLAUDE.md
  30 seconds ago · main · 81.3KB

Ctrl+A to show all projects · Ctrl+B to toggle branch
Ctrl+W to show all worktrees · Ctrl+V to preview
Ctrl+R to rename · Type to search · Esc to cancel
```

- `Resume Session (X of Y)` header with count
- Search box at top (rounded border)
- Grouped by context: `current worktree` label
- Per-session: first message text + relative timestamp + branch + file size
- Rich keyboard shortcuts

#### `/stats` — Usage Dashboard

**Overview tab:**
```
    Overview   Models  (←/→ or tab to cycle)

      Mar Apr May Jun ...  Dec Jan Feb Mar
      ···········████████░░░▒█▓▓██
  Mon ···········▒▓░░▓██░█
  Wed ···········░█░▓░▒▒█▒█
  Fri ···········░░░▓█▓▒█▓▓
      Less ░ ▒ ▓ █ More

  Favorite model: Opus 4.5    Total tokens: 14.4m
  Sessions: 863               Longest session: 8d 8h 42m
  Active days: 67/68          Longest streak: 45 days
  Most active day: Mar 8      Current streak: 21 days

  You've used ~27x more tokens than Don Quixote

  Esc to cancel · r to cycle dates · ctrl+s to copy
```

**Models tab:**
```
  Tokens per Day
    919k ┼       ╭╮
    ...  ASCII line chart
       0 ┼──────────────────

  ● Opus 4.5 · ● Sonnet 4.5 · ● Opus 4.6

  ● Opus 4.5 (59.9%)          ● Opus 4.6 (17.9%)
    In: 2.9m · Out: 5.7m        In: 906.2k · Out: 1.7m
```

- GitHub-style activity heatmap using `░▒▓█`
- ASCII line chart for tokens per day
- Per-model breakdown with colored dots (`●`)

#### `/theme` — Theme Picker

```
 Theme
 Choose the text style that looks best with your terminal

   1. Dark mode
   2. Light mode
   3. Dark mode (colorblind-friendly)
   4. Light mode (colorblind-friendly)
 ❯ 5. Dark mode (ANSI colors only) ✔
   6. Light mode (ANSI colors only)

╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 1  function greet() {
 2 -  console.log("Hello, World!");
 2 +  console.log("Hello, Claude!");
 3  }
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Syntax theme: ansi (ctrl+t to disable)

 Enter to select · Esc to cancel
```

- 6 themes: dark/light × normal/colorblind/ANSI-only
- Live code diff preview below selection
- `Ctrl+T` to disable syntax theme

#### `/doctor` — Diagnostics

Tree-connector format with `└` for hierarchy:
```
 Diagnostics
 └ Currently running: native (2.1.55)
 └ Path: /Users/.../.local/share/claude/versions/2.1.55
 └ Config install method: global
 └ Search: OK (bundled)
 Warning: Multiple installations found
 └ npm-global at ...
 └ native at ...
 Fix: Run claude install to update configuration

 Updates
 └ Stable version: 2.1.58
 └ Latest version: 2.1.76

 Context Usage Warnings
 └ ⚠ Large MCP tools context (~57,545 tokens > 25,000)
   └ claude_ai_Linear: 37 tools (~17,464 tokens)

 Press Enter to continue…
```

#### `/permissions` — Four-Tab Permission Manager

```
 Permissions:  Allow   Ask   Deny   Workspace  (←/→ or tab to cycle)

 Claude Code won't ask before using allowed tools.
 ╭────────────────────────────────────╮
 │ ⌕ Search…                          │
 ╰────────────────────────────────────╯

 ❯ 1.  Add a new rule…
   2.  mcp__claude-in-chrome__computer
   3.  mcp__claude-in-chrome__find
   ...
 ↓ 10. mcp__...

 Press ↑↓ to navigate · Enter to select · Type to search · Esc to cancel
```

Tabs: Allow, Ask, Deny, Workspace.
Context description changes per tab.
`↓` indicator at bottom signals more items.

#### `/agents` — Agent Manager

```
╭──────────────────────────────────────────────╮
│ Agents                                       │
│ 9 agents                                     │
│                                              │
│ ❯ Create new agent                           │
│                                              │
│   Plugin agents                              │
│   codex-agent:codex-delegator · inherit      │
│   plugin-dev:agent-creator · sonnet          │
│                                              │
│   Built-in agents (always available)         │
│   claude-code-guide · haiku                  │
│   Explore · haiku                            │
│   general-purpose · inherit                  │
╰──────────────────────────────────────────────╯
```

Unique: entire list is boxed with `╭──╮` border.
Two sections: Plugin agents / Built-in agents.
Model shown per agent (haiku, sonnet, inherit).

#### `/output-style`

```
 ❯ 1. Default ✔    Claude completes coding tasks efficiently and provides concise responses
   2. Explanatory  Claude explains its implementation choices and codebase patterns
   3. Learning     Claude pauses and asks you to write small pieces of code
```

#### `/memory`

```
 Memory
   Auto-memory (research preview): on

 ❯ 1. User memory              Saved in ~/.claude/CLAUDE.md
   2. Project memory           Checked in at ./CLAUDE.md
   3. ~/.claude/projects/.../memory/MEMORY.md   auto memory entrypoint
   4. Open auto-memory folder

 Learn more: https://code.claude.com/docs/en/memory
 Enter to confirm · Esc to cancel
```

#### `/fast`

```
 ↯ Fast mode (research preview)
 High-speed mode for Opus 4.6.

   Fast mode  OFF  $30/$150 per Mtok

 Tab to toggle · Enter to confirm · Esc to cancel
```

#### `/status` Usage Tab

```
  Current session
  ███                                   6% used
  Resets 12pm (America/New_York)

  Current week (all models)
  ████████████                          24% used
  Resets Mar 19 at 11pm (America/New_York)

  Current week (Sonnet only)
  ██████▌                               13% used
  Resets Mar 20 at 9am (America/New_York)
```

Block progress bars using `█` (filled) and `▌` (half-filled).

---

## 6. Input Area

### 6.1 Structure

```
────────────────────────────────────────────── [session-name] ──
❯ [cursor]
──────────────────────────────────────────────────────────────
  [status bar]
```

- Two `─` (U+2500) separator lines, dim white (`[2m[37m]`), full terminal width
- `❯` prompt (U+276F) with cursor: `[7m [0m` (reverse video block)
- When session is named, it appears right-aligned in the TOP separator:
  `──────────────────────── cli-command-testing ──`
- Input expands vertically for multi-line content

### 6.2 Multi-line Input

- `\Enter` (backslash + Enter): insert newline in input
- Continuation lines indented 2 spaces:
  ```
  ❯ First line of a long message that explains
    what I want Claude to do in detail.
  ```
- `Ctrl+G`: opens `$EDITOR` (or nano) for complex editing
- `Enter` on empty final line: submits message

**Note:** Some sources indicate "Enter on empty line submits" (double-Enter behavior for multi-line), while another indicates `\Enter` for newline. The actual submit behavior is Enter on empty line OR Shift+Enter as immediate submit depending on terminal emulator configuration.

### 6.3 Input Prefixes

| Prefix | Trigger | Behavior |
|--------|---------|----------|
| `/` | Typing `/` | Opens slash command autocomplete dropdown |
| `@` | Typing `@` | Opens file picker; files prefixed with `+` in list |
| `!` | `!` as first char | Bash mode — runs command directly |
| `&` | `&` as first char | Background task mode |

**File picker dropdown:**
```
❯ @
────────────────────────────────────
  + pnpm-lock.yaml
  + test/
  + tsconfig.base.json
  + node_modules/
  + .claude/
  + docs/
```

### 6.4 Keyboard Shortcuts (Complete)

| Shortcut | Action |
|----------|--------|
| `Enter` | Submit (single-line) or submit on empty line (multi-line) |
| `\Enter` | Insert newline (multi-line input) |
| `Shift+Enter` | Immediate submit (if terminal supports) |
| `Escape` | First: show "Esc to clear again"; Second: clear input |
| `Ctrl+C` | Clear input (during input); interrupt response |
| `Ctrl+U` | Clear current input line |
| `Ctrl+G` | Open in `$EDITOR` (nano fallback) |
| `Ctrl+S` | Stash current prompt |
| `Ctrl+V` | Paste image |
| `Ctrl+Z` | Suspend Claude Code |
| `Ctrl+O` | Toggle verbose/detailed transcript |
| `Ctrl+T` | Toggle tasks panel |
| `Ctrl+E` | Show/collapse all previous messages |
| `Ctrl+Shift+-` | Undo code change |
| `Ctrl+B Ctrl+B` | Run current agent in background |
| `Ctrl+B Ctrl+B` (twice) | Background mode for agents |
| `Meta+P` | Switch model |
| `Meta+O` | Toggle fast mode |
| `Shift+Tab` | Cycle permission modes (Default → Accept Edits → Plan) |
| `Up arrow` | Navigate/edit queued messages |

---

## 7. Tool Use Display

### 7.1 Collapsed View (Default)

Each tool type has a standardized summary format:

| Tool | Collapsed Display |
|------|-------------------|
| Read (single) | `⏺ Read 1 file (ctrl+o to expand)` |
| Read (multiple) | `⏺ Read 3 files (ctrl+o to expand)` |
| Read (in progress) | `⏺ Reading 1 file… (ctrl+o to expand)` |
| Edit/Update | `⏺ Update(test.js)` then `  ⎿  Added 1 line` |
| Write (new file) | `⏺ Write(utils.js)` then `  ⎿  Wrote 2 lines to utils.js` |
| Bash | `⏺ Bash(command here)` then `  ⎿  [output]` |
| Grep/Glob | `⏺ Searched for 1 pattern (ctrl+o to expand)` |
| Search in-progress | `⏺ Searching for 3 patterns… (ctrl+o to expand)` |
| Agent/subagent | `⏺ statusline-setup(Configure statusline from PS1)` |

**Color distinction:**
- `[92m⏺[39m]` (bright green): Tool invocation / action
- `[97m⏺[39m]` (bright white): Text response content

### 7.2 Expanded View (Ctrl+O)

When expanded, shows:
- Full absolute paths instead of just filenames
- `Read(config.js)`, `Search(pattern: "*.js", path: "/private/tmp/...")` format
- Result counts: `Read 3 lines`, `Found 4 lines`, `Found 3 files`
- Post-tool hooks: `1 PostToolUse hook ran`
- Timestamps on right margin: `09:09 PM claude-opus-4-6`
- Truncated long output: `... +6 lines (ctrl+o to expand)`

Expanded bash example:
```
⏺ Bash(cat /etc/hosts)
  ⎿  ##
     # Host Database
     #
     ... +6 lines (ctrl+o to expand)
```

### 7.3 Permission Prompts — File Edit

```
────────────────────────────────────────────────────────────
 Edit file
 test.js
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 1 -function hello() { return "world"; }
 2 -function goodbye() { return "farewell"; }
 1 +function hello(name) { return "Hello, " + name + "!"; }
 2 +function farewell() { return "farewell"; }
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Do you want to make this edit to test.js?
 ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No

 Esc to cancel · Tab to amend
────────────────────────────────────────────────────────────
```

### 7.4 Permission Prompts — File Create

```
 Create file
 utils.js
╌╌╌╌╌╌╌╌╌╌
  1 function add(a, b) { return a + b; }
  2 function subtract(a, b) { return a - b; }
╌╌╌╌╌╌╌╌╌╌
 Do you want to create utils.js?
 ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No
```

Differences: "Create file" header, no `-/+` prefixes (all new), question says "create" not "edit".

### 7.5 Permission Prompts — Bash Command

```
 Bash command

   cat /etc/hosts
   Display contents of /etc/hosts

 Do you want to proceed?
 ❯ 1. Yes
   2. Yes, allow reading from etc/ from this project
   3. No

 Esc to cancel · Tab to amend · ctrl+e to explain
```

- Shows command text + AI-generated description
- Scope expansion option is context-specific (directory-level)
- `ctrl+e to explain` for additional context

### 7.6 Destructive Operation Prompt (Claude-Generated)

```
 ☐ Confirm

Do you want to permanently delete /tmp/claude-ux-test/config.js?

❯ 1. Yes, delete it
     Permanently remove config.js
  2. No, keep it
     Cancel the deletion
  3. Type something.
──────────────────────────────────────────────────────────
  4. Chat about this

Enter to select · ↑/↓ to navigate · Esc to cancel
```

After response:
```
⏺ User answered Claude's questions:
  ⎿  · Do you want to permanently delete /tmp/claude-ux-test/config.js? → No, keep it
```

### 7.7 Diff Display Format

```
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 1 -old line content here
 2 -another old line
 1 +new line content here
 2 +replacement line
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
```

- Unified diff style (not side-by-side)
- `╌` dashed lines above and below
- Line numbers in gutter (old line numbers for `-`, new line numbers for `+`)
- `-` prefix in red for deletions
- `+` prefix in green for additions
- Unchanged lines: just line number, no prefix
- Full-width spanning terminal

**Post-approval diff summary patterns:**
- `Added 1 line`
- `Added 2 lines, removed 2 lines`
- `Wrote 2 lines to utils.js`

### 7.8 Multi-File Operations

Sequential edit queue: each edit shown separately for approval.
Multi-file reads batched: `⏺ Read 3 files (ctrl+o to expand)`.

Expanded view shows each read separately:
```
⏺ Read(config.js)
  ⎿  Read 2 lines

⏺ Read(test.js)
  ⎿  Read 3 lines
```

### 7.9 Agent/Subagent Task Display

```
⏺ statusline-setup(Configure statusline from PS1)
  ⎿  ❯ Configure my statusLine from my shell PS1 configuration
     Read(~/.zshrc)
     Read(~/.claude/settings.json)
     ctrl+b ctrl+b (twice) to run in background
```

When collapsed: `+N more tool uses (ctrl+o to expand)`.

### 7.10 Permission Mode: Auto-Accept

After selecting "Yes, allow all edits during this session":
- Status bar: `⏵⏵ accept edits on (shift+tab to cycle)`
- All subsequent file edits auto-approved
- Tool calls still appear in conversation

---

## 8. Error Handling UX

### 8.1 Tool Errors (Conversational, Not Technical)

Tool errors are NOT shown as error boxes or banners. Claude interprets the error and communicates it conversationally:

```
❯ Read the file /tmp/this-file-does-not-exist-at-all.txt...

⏺ Read 1 file (ctrl+o to expand)

⏺ That file doesn't exist at /tmp/this-file-does-not-exist-at-all.txt.
```

The tool call still appears collapsed. The actual error is in the expanded view. Claude's response is natural language.

### 8.2 System-Level Errors (Pre-Launch)

Shown as plain text before the interface loads:

```
Error: Claude Code cannot be launched inside another Claude Code session.
Nested sessions share runtime resources and will crash all active sessions.
To bypass this check, unset the CLAUDECODE environment variable.
```

- No colored error boxes
- Includes actionable fix when available

### 8.3 MCP Server Failures

In status bar: `1 MCP server failed · /mcp` (in bright red `[91m]`).
In `/mcp` dialog: `✘ failed` with red indicator per server.
In `/status` Status tab: `✘` per failed server in the MCP list.

### 8.4 Diagnostics / Warnings (Non-Fatal)

In `/doctor` output:
```
 Warning: Multiple installations found
 └ npm-global at /Users/...
 Fix: Run claude install to update configuration

 └ ⚠ Large MCP tools context (~57,545 tokens > 25,000)
```

In `/status` Status tab:
```
  System Diagnostics
   ⚠ Running native installation but config install method is 'global'
   ⚠ Leftover npm global installation at ...
```

### 8.5 Escape-Key Safety

First Escape with text in input: `Esc to clear again` shown in status bar (does NOT clear yet).
Second Escape: clears input.

This prevents accidental clearing of long inputs.

---

## 9. Startup and Welcome Screen

### 9.1 Standard Launch (Returning User)

```
 ▐▛███▜▌   Claude Code v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/project-name

────────────────────────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────────────────────────
  dennisonbertram@Mac [07:19:14] [~/Develop/project]  [main]   1 MCP server failed · /mcp
```

**Four regions visible immediately:**
1. Logo + version header (3 lines)
2. Blank line
3. Separator → input area → separator
4. Status bar

**No loading screen, no splash, no onboarding on subsequent launches.**

Logo stays in scrollback (first item in conversation area, never scrolls off until there is enough content to push it).

### 9.2 Welcome Screen (Alternate — Box Layout)

Documented in `claude-code-ux-tool-use-diffs.md`:

```
╭─── Claude Code v2.1.55 ──────────────────────────────────────╮
│                           │ Tips for getting started          │
│    Welcome back Dennison! │ Run /init to create CLAUDE.md     │
│                           │ ─────────────────────────────     │
│          ▐▛███▜▌          │ Recent activity                   │
│         ▝▜█████▛▘         │ No recent activity                │
│           ▘▘ ▝▝           │                                   │
│    Opus 4.6 · Claude Max  │                                   │
│  /private/tmp/claude-test │                                   │
╰────────────────────────────────────────────────────────────────╯
```

**Layout:** Two-column inside a box-drawing border. Left: logo, model, directory. Right: tips and recent activity.

### 9.3 Startup Behavior

- MCP server connections evaluated at startup; failures appear in status bar immediately
- No onboarding wizard on subsequent launches
- Session name (if set) appears in the input separator: `───── session-name ──`

### 9.4 Session Resume Display

```
claude --resume 6fea0837-3902-4e2c-98f7-6f9d6a961c7d
```

After `/exit`, displayed in dim (`[2m]`) text. Session UUID provided for exact resumption.

---

## 10. Help System

### 10.1 Structure

`/help` opens a tabbed dialog with three tabs:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Claude Code v2.1.55  general   commands   custom-commands  (←/→ or tab to cycle) │
│                                                                              │
│ [Tab content]                                                                │
│                                                                              │
│ For more help: https://code.claude.com/docs/en/overview                      │
│ Esc to cancel                                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

Navigation: Left/Right arrows or Tab. Escape to dismiss.

### 10.2 Tab 1: General

```
  Claude understands your codebase, makes edits with your permission, and
  executes commands — right from your terminal.

  Shortcuts
  ! for bash mode           double tap esc to clear input        ctrl + shift + - to undo
  / for commands            shift + tab to auto-accept edits     ctrl + z to suspend
  @ for file paths          ctrl + o for verbose output          ctrl + v to paste images
  & for background          ctrl + t to toggle tasks             meta + p to switch model
                            \⏎ for newline                       meta + o to toggle fast mode
                                                                 ctrl + s to stash prompt
                                                                 ctrl + g to edit in $EDITOR
                                                                 /keybindings to customize
```

Layout: Three columns of shortcuts, no borders, plain text.

### 10.3 Tab 2: Commands

Scrollable list with `❯` cursor:
```
  /add-dir         Add a new working directory
  /agents          Manage agent configurations
  /chrome          Claude in Chrome (Beta) settings
  ...
```

Each command: name (left) + description (right), one per line, scrollable with arrows.

### 10.4 Tab 3: Custom Commands

Shows user-defined commands with source labels:
```
  /code-review     (user) Perform a code review...
  /codex           (codex-agent) Delegate a task...
```

### 10.5 Dialog Footer (All Tabs)

```
 For more help: https://code.claude.com/docs/en/overview
 Esc to cancel
```

---

## 11. Markdown Rendering

### 11.1 Headers

| Header Level | Rendering | Example |
|-------------|-----------|---------|
| Title/H1 (response title) | `[1;3;4m]` bold+italic+underline | `⏺ Python Hello World` |
| Section header (H2) | `[1m]` bold only | `  Connection Model` |
| Sub-header (H3) | `[1m]` bold, indented | `  Type A vs Type B` |

### 11.2 Code Blocks

```python
  print("Hello, World!")
```

With ANSI highlighting:
- `print` → `[36m]` (cyan) — function/builtin
- `"Hello, World!"` → `[31m]` (red) — string literal

```javascript
  function reverseString(str) {
    return str.split("").reverse().join("");
  }
```

With ANSI highlighting:
- `function` → `[34m]` (blue) — keyword
- `reverseString(str)` → `[33m]` (yellow) — function name
- `""` → `[31m]` (red) — string literal
- `return` → `[34m]` (blue) — keyword

**Important:** Code blocks have NO border, NO background, NO language label in the CLI. Just syntax-highlighted text with 2-space indent.

### 11.3 Inline Code

```
  Run it with python hello.py and you'll see Hello, World! printed
```

- `[94m]` (bright blue) for inline code
- No backtick rendering, no background
- Examples: `` `print()` ``, `` `python hello.py` ``, `` `send()` ``

### 11.4 Bold Text

```
  TCP is connection-oriented
```

- `[1m]` (bold weight only)
- No color change

### 11.5 Lists

```
  - print() is a built-in function that outputs text to the console
  - The string "Hello, World!" is passed as an argument
```

- `- ` prefix (dash + space)
- 2-space indent from left margin
- No special color on bullet character
- Can contain inline code (bright blue) and bold text inline

### 11.6 Tables

```
  ┌────────────────────┬──────────────────────┬─────────────────┐
  │      Feature       │         TCP          │       UDP       │
  ├────────────────────┼──────────────────────┼─────────────────┤
  │ Connection         │ Required (handshake) │ None            │
  ├────────────────────┼──────────────────────┼─────────────────┤
  │ Reliability        │ Guaranteed delivery  │ Best-effort     │
  └────────────────────┴──────────────────────┴─────────────────┘
```

- Unicode single-line box-drawing: `┌ ─ ┬ ┐ │ ├ ┼ ┤ └ ┴ ┘`
- No special color on table borders (default terminal color)
- Content padded within cells
- No bold header row (header same style as data)
- Centered content in header row

### 11.7 Syntax Highlighting Color Scheme

| Token type | ANSI code | Color name |
|-----------|-----------|-----------|
| Keywords (`function`, `return`, `if`) | `[34m]` | Blue |
| Function/builtin names | `[36m]` | Cyan |
| Named function definitions | `[33m]` | Yellow |
| String literals | `[31m]` | Red |
| Inline code references | `[94m]` | Bright blue |
| Regular code/identifiers | Default | Terminal default |

---

## 12. ANSI Color Palette

### 12.1 Complete Palette

| Purpose | ANSI Code | Color Name | Approximate Hex |
|---------|-----------|-----------|-----------------|
| Logo (filled blocks) | `[91m]` | Bright red | #FF5555 |
| Logo background | `[40m]` | Black bg | #000000 |
| User message background | `[100m]` | Bright black (dark gray) bg | #555555 |
| User message text | `[97m]` | Bright white | #FFFFFF |
| AI text response prefix | `[97m]` | Bright white | #FFFFFF |
| AI tool call prefix | `[92m]` | Bright green | #55FF55 |
| Response title | `[1;3;4m]` | Bold+italic+underline | (style, not color) |
| Section headers | `[1m]` | Bold | (weight only) |
| Code: keywords | `[34m]` | Blue | #5555FF |
| Code: builtins/functions | `[36m]` | Cyan | #55FFFF |
| Code: function names | `[33m]` | Yellow | #FFFF55 |
| Code: string literals | `[31m]` | Red | #FF5555 |
| Inline code | `[94m]` | Bright blue | #5555FF (bright) |
| Bold text | `[1m]` | Bold | (weight only) |
| Separators | `[2m][37m]` | Dim white | ~50% opacity |
| Status: username | `[1m][32m]` | Bold green | #55FF55 |
| Status: timestamp | `[34m]` | Blue | #5555FF |
| Status: path/dir | `[37m]` | White | #AAAAAA |
| Status: git branch | `[32m]` | Green | #55FF55 |
| Status: dirty git | `[31m]` | Red | #FF5555 |
| Status: errors/failures | `[91m]` | Bright red | #FF5555 |
| Spinner characters | Default | Terminal default | -- |
| Loading verb | Default | Terminal default | -- |
| Dim/secondary text | `[2m]` | Dim/faint | 50% opacity |
| Interruption text | `[37m]` | White | #AAAAAA |
| Tool expand hints | `[37m]` | White/dim | #AAAAAA |
| Slash command selected | `[94m][7m]` | Bright blue + reverse | #5555FF reverse |
| Cursor | `[7m] [0m]` | Reverse video block | -- |

### 12.2 Raw Escape Code Reference

**User message block:**
```
[37m[100m❯ [97mMessage text here                              [39m[49m
[97m[100m                                                   [39m[49m
```

**AI response (text):**
```
[97m⏺[39m Response text here
```

**AI response (tool use):**
```
[92m⏺[39m Read [1m1[0m file [37m(ctrl+o to expand)[39m
```

**Section header:**
```
[1mHeader Text[0m
```

**Response title:**
```
[97m⏺[39m [1;3;4mTitle Text[0m
```

**Code block (JavaScript):**
```
[34mfunction[33m name(args) [39m{
    [34mreturn[39m value.method([31m"string"[39m);
  }
```

**Inline code:**
```
Use [94mcommandName[39m to do something
```

**Separator line:**
```
[2m[37m────────────────────────────────────────────────[0m
```

**Input prompt with cursor:**
```
❯ [7m [0m
```

**Loading spinner:**
```
✶ Leavening…
  ⎿  Tip: [tip text]
```

**Interrupt message:**
```
  ⎿  [37mInterrupted · What should Claude do instead?[39m
```

**Exit and resume:**
```
[37m[100m❯ [97m/exit [39m[49m
[97m  ⎿  [39mGoodbye!

[2mResume this session with:[0m
[2mclaude --resume [session-uuid][0m
```

---

## 13. Implementation Difficulty Tiers

All features grouped from simplest to most complex, suitable for sequencing GitHub tickets.

---

### Tier 1: Foundational (Single-component, no interaction state)

These are pure rendering components with no interactive behavior. Implement these first to establish the visual foundation.

| Feature | Description | Source |
|---------|-------------|--------|
| Logo / header block | ASCII logo + version + model + dir. 3 lines. Fixed ANSI colors. | chat-streaming, startup-navigation, status-errors |
| Separator lines | `─` (U+2500), dim white, full terminal width. Top+bottom of input area. | chat-streaming |
| Input prompt character | `❯` with cursor (`[7m [0m`). No behavior, just rendering. | chat-streaming |
| User message styling | Gray background (`[100m`), bright white text, `❯` prefix, full-width, trailing blank line. | chat-streaming |
| AI response prefix | `⏺` prefix, bright white for text, bright green for tools. | chat-streaming |
| Tree connector | `⎿` at 2-space indent. Connects results to parent. | chat-streaming |
| Blank line spacing | Exact spacing: blank line between each message type. | chat-streaming, tool-use-diffs |
| Dim/secondary text | `[2m]` for session resume message, hints. | chat-streaming |
| `✔✘△` status icons | Success/failure/warning glyphs with green/red/yellow colors. | startup-navigation |

---

### Tier 2: Core Rendering (Visual components with state)

These require tracking state or rendering multiple states but no complex interaction.

| Feature | Description | Source |
|---------|-------------|--------|
| Status bar | Bottom-fixed. 7 components: user@host, time, dir, branch, dirty, alerts, mode. | all UX files |
| Status bar: permission mode | Second line: `⏸` or `⏵⏵` with cycle hint. Three states. | startup-navigation, status-errors |
| Status bar: MCP alerts | `N MCP servers failed · /mcp` in bright red. | chat-streaming, startup-navigation |
| Spinner animation | 6 Unicode chars cycling. Whimsical verb (random pool). Duration counter. Token counter. | chat-streaming, status-errors |
| Tip display below spinner | `  ⎿  Tip: [text]`. Shown during loading. | chat-streaming |
| Loading state verbs | Pool of 20+ whimsical verbs. Random selection per request. | chat-streaming, status-errors |
| "Worked for N time" completion | `✻ Worked for 1m 0s` shown after response completes. | status-errors |
| Markdown: bold text | `[1m]` bold only. | chat-streaming |
| Markdown: inline code | `[94m]` bright blue. No backticks. | chat-streaming |
| Markdown: headers (H1/H2/H3) | H1: `[1;3;4m]` bold+italic+underline; H2/H3: `[1m]` bold. | chat-streaming |
| Markdown: bulleted lists | `- ` prefix, 2-space indent, supports inline formatting. | chat-streaming |
| Markdown: tables | Unicode box-drawing chars. Single-line borders. | chat-streaming |
| Tool call: collapsed display | `⏺ Read 1 file (ctrl+o to expand)`. Per-tool format. | tool-use-diffs, status-errors |
| Tool call: in-progress state | Trailing `…` on verb: `⏺ Reading 1 file…`. | tool-use-diffs |
| Tool result: summary text | `  ⎿  Added 1 line`, `  ⎿  Wrote 2 lines to utils.js`. | tool-use-diffs |
| Code block: syntax highlighting | Blue keywords, cyan builtins, yellow fn names, red strings. No border. | chat-streaming |
| Interruption message | `  ⎿  Interrupted · What should Claude do instead?` inline under user msg. | chat-streaming, status-errors |
| Exit / goodbye message | `  ⎿  Goodbye!` and dim resume instruction. | chat-streaming, tool-use-diffs |
| Bash mode prefix display | Shows `\!` in history, wraps as `⏺ Bash(command)`. | status-errors |
| Session name in separator | Right-aligned name in top separator line. | startup-navigation |
| Welcome screen (box layout) | Two-column boxed welcome with logo, tips, recent activity. | tool-use-diffs |

---

### Tier 3: Core Interaction (Interactive UI components)

These require keyboard handling and interactive state.

| Feature | Description | Source |
|---------|-------------|--------|
| Slash command autocomplete | `/` triggers dropdown. Filters as you type. Name+description per item. Bright blue + reverse-video selection. | chat-streaming, startup-navigation, tool-use-diffs |
| File picker (`@`) | `@` triggers file list. `+` prefix for files/dirs. Arrow navigation. | startup-navigation |
| Multi-line input | `\Enter` for newline, continuation indent, `Ctrl+G` for external editor. | chat-streaming, startup-navigation |
| Escape handling | First Escape: "Esc to clear again" in status bar. Second Escape: clears. During response: interrupt. In dialog: dismiss. | status-errors |
| Ctrl+O toggle | Switches between collapsed and detailed transcript view. | tool-use-diffs, status-errors |
| Ctrl+E toggle | Show/collapse all previous messages. Counter: `ctrl+e to show N previous messages`. | tool-use-diffs |
| Shift+Tab permission cycling | Rotates Default → Accept Edits → Plan. Status bar updates. | startup-navigation, status-errors, tool-use-diffs |
| Permission prompt: file edit | Diff view + Yes/Yes-all-session/No options + Tab to amend. | tool-use-diffs |
| Permission prompt: bash | Command + description + Yes/No + ctrl+e to explain. | tool-use-diffs, status-errors |
| Permission prompt: read | Read path + Yes/Yes-scope/No options. | tool-use-diffs, status-errors |
| Dialog dismissed feedback | `⎿  [Command] dialog dismissed` or `⎿  Kept [model] as Default`. | startup-navigation |
| `❯` cursor navigation | Up arrow for history, numbered shortcuts in dialogs. | startup-navigation |
| `✔` current selection marker | Shown in numbered lists next to active option. | startup-navigation |

---

### Tier 4: Advanced Dialogs (Complex multi-component dialogs)

Full dialog implementations with tabs, search, and scroll.

| Feature | Description | Source |
|---------|-------------|--------|
| `/help` dialog | 3-tab dialog. Tab 1: shortcut reference. Tab 2: scrollable command list. Tab 3: custom commands. Tab nav with ←/→. | startup-navigation, status-errors |
| `/status` dialog | 3-tab dialog (Status/Config/Usage). Config tab: searchable key-value list. Usage tab: block progress bars. | startup-navigation, status-errors |
| `/model` dialog | Numbered list, effort slider (`▌▌▌`), ←/→ for effort, ✔ for current. | startup-navigation, status-errors |
| `/theme` dialog | 6 options, live diff preview with syntax highlighting, `╌` separator. | startup-navigation |
| `/resume` dialog | Searchable scrollable list. `⌕ Search…` box. Per-session: text + time + branch + size. Ctrl shortcuts. | startup-navigation, status-errors |
| `/permissions` dialog | 4-tab dialog (Allow/Ask/Deny/Workspace). Searchable. Numbered list. "Add a new rule…" first item. | startup-navigation, status-errors |
| `/mcp` dialog | Grouped list (User/claude.ai/Built-in). Per-server: name + ✔/✘/△ + status text. Config path shown. | startup-navigation, status-errors |
| `/agents` dialog | Boxed dialog (`╭──╮` border). Two groups. Per-agent: name + model. Create new option. | startup-navigation |
| `/context` inline | 10x10 grid of `⛁⛀⛶⛝` icons. Category breakdown on right. Inline output (not dialog). | status-errors |
| `/stats` dialog | 2-tab dialog. Overview: heatmap `░▒▓█` + key stats + fun fact. Models: ASCII line chart + per-model breakdown. | status-errors |
| `/config` (inside `/status`) | Searchable settings list. Key-value pairs. 23+ settings. Fuzzy filter with `⌕`. | startup-navigation, status-errors |
| Diff viewer (permission flow) | Unified diff in `╌` bordered box. Line numbers. `+`/`-` prefixes with colors. "Edit file"/"Create file" header. | tool-use-diffs |
| Destructive op prompt | `☐ Confirm` icon. Claude-generated question. 4 options including "Type something." and "Chat about this." | tool-use-diffs |
| `/rewind` dialog | Scrollable checkpoint list. Per-turn: first message text + code change info. `❯ (current)` marker. | startup-navigation, status-errors |

---

### Tier 5: Polish and Advanced Features

These require either complex state management, cross-cutting concerns, or are lower-priority quality-of-life features.

| Feature | Description | Source |
|---------|-------------|--------|
| Tool expand/collapse (Ctrl+O expanded view) | Full paths, tool params, result counts, hook count, right-aligned timestamp. | tool-use-diffs |
| Long output truncation | `... +N lines (ctrl+o to expand)` with accurate line count. | tool-use-diffs, status-errors |
| Auto-accept mode indicator | `⏵⏵ accept edits on (shift+tab to cycle)` in status bar. Auto-skip permission dialogs. | tool-use-diffs |
| Tab to amend in permission prompts | Opens tool call for editing before approval. | tool-use-diffs, status-errors |
| Ctrl+E to explain in bash permission | Opens explanation of bash command. | tool-use-diffs, status-errors |
| Agent subagent display | `⏺ agentname(task description)` with nested tool calls. `ctrl+b ctrl+b to run in background`. | startup-navigation |
| Background task mode (`&` prefix) | `&` routes to background task queue. `/tasks` dialog to manage. | startup-navigation |
| Context usage grid (`/context`) | 10x10 grid with `⛁⛀⛶⛝` icons. Category breakdown. Accurate token counting. | status-errors |
| Stats heatmap | GitHub-style activity grid using `░▒▓█`. Day/month labels. | status-errors |
| Stats ASCII line chart | Y-axis with token counts. X-axis with dates. Multiple model series with `●` legend. | status-errors |
| Progress bars (usage) | `█` filled + `▌` half-filled for usage meters. Percentage + reset time. | startup-navigation |
| Effort slider in model picker | `▌▌▌` blocks, adjustable with ←/→. Three levels. | startup-navigation, status-errors |
| Session name in top separator | Right-aligns in `─────── name ──` format. Updated by `/rename`. | startup-navigation |
| `/fork` inline output | Shows UUID of original + exact `claude -r UUID` command. | status-errors |
| `/compact` with spinner | Distinct `✢` spinner during compaction. `ctrl+o to see full summary` hint after. | status-errors |
| Double Escape clear safety | Two-Escape pattern with intermediate status bar hint. | status-errors |
| Ctrl+S stash prompt | Save current input without submitting. | startup-navigation |
| Ctrl+V paste image | Paste image directly into input. | startup-navigation |
| Meta+P model switch shortcut | Quick model selection without `/model`. | startup-navigation |
| Meta+O fast mode toggle | Quick fast mode toggle without `/fast`. | startup-navigation |
| Ctrl+Shift+- undo | Undo last code change (rewind integration). | startup-navigation |
| Post-hook display | `1 PostToolUse hook ran` shown in expanded view. | tool-use-diffs |
| Right-aligned timestamp in expanded view | `09:09 PM claude-opus-4-6` shown on right margin of expanded tool calls. | tool-use-diffs |
| Token metrics during thinking | `↓ 1.8k tokens` counter. Duration as `2s`, `37s`, `1m 20s`. | status-errors |
| `⚠` warning indicators | In `/status`, `/doctor` output for non-fatal issues. | startup-navigation |
| Colorblind theme support | 6 themes including 2 colorblind-friendly variants. | startup-navigation |
| `ctrl+t` syntax theme toggle | Toggle syntax highlighting within theme picker. | startup-navigation |
| Keybindings customization | `/keybindings` opens config file. | startup-navigation |
| Tips pool | Random tip shown during loading. Rotating from a pool of helpful suggestions. | chat-streaming, status-errors |
| Fun context comparison | "You've used ~27x more tokens than Don Quixote" in `/stats`. | status-errors |
| `r` key cycle in stats | Cycles between All time / Last 7 days / Last 30 days. | status-errors |
| `ctrl+s to copy` in stats | Copies stats to clipboard. | status-errors |
| Search in `/resume` | `Ctrl+A` all projects, `Ctrl+B` toggle branch, `Ctrl+W` worktrees, `Ctrl+V` preview, `Ctrl+R` rename. | status-errors |
| `/color` prompt bar coloring | 8 colors (red, blue, green, yellow, purple, orange, pink, cyan). Affects current session only. | startup-navigation, status-errors |
| MCP source path display | `/mcp` shows config source file path per group. | status-errors |
| `※` note indicator | Used in `/mcp` for debug tip. | status-errors |
| Tree connector in `/doctor` | `└` connectors for hierarchical output (different from `⎿`). | startup-navigation |

---

### Summary by Tier

| Tier | Count | Tickets to create |
|------|-------|------------------|
| 1: Foundational | 9 features | 9 tickets |
| 2: Core Rendering | 19 features | ~12 tickets (group related) |
| 3: Core Interaction | 13 features | ~10 tickets |
| 4: Advanced Dialogs | 14 features | 14 tickets |
| 5: Polish | 35+ features | ~20 tickets (group small ones) |
| **Total** | **~90 features** | **~65 tickets** |

---

## Appendix: Cross-Competitor Patterns

### From Crush (Charmbracelet / Bubble Tea)

- Session persistence via SQLite
- Multiple concurrent project contexts (session list)
- Permission system with per-tool gates; `--yolo` bypass flag
- Sub-agent delegation (read-only Task agents)
- Event-driven pub/sub architecture decoupling TUI from agent state
- CPU-heavy Bubble Tea rendering in long sessions (performance concern)
- `agent_tool` as `ParallelAgentTool` for concurrent sub-agent invocation

### From OpenCode (TypeScript + Zig)

- Client/server architecture: TUI ↔ HTTP/SSE ↔ Backend
- Git-based snapshotting (`git add . && write-tree`) for rollback without permanent commits
- Auto-compact at 95% context: summarize older messages with a smaller model
- Per-agent model overrides: cheap model for titles, strong model for coding
- `@` mention for subagent invocation in input
- Primary agents (Build/Plan) vs Subagents (General/Explore)
- SSE `/sse` endpoint for multiple simultaneous clients

### From Pi (TypeScript/Bun + Rust)

- Hashline edits: content-hash anchored line editing (eliminates whitespace ambiguity)
- TTSR (Time Traveling Streamed Rules): zero-token context injections triggered by pattern match
- Session tree structure (JSONL with `id`+`parentId`): branching conversations, `/fork`, `/tree`
- Radical minimalism: 4 core tools, <1k token system prompt
- Mid-session model switching with cross-provider context transformation
- Extension system with 25+ lifecycle hooks for total behavioral override
- `oh-my-pi` fork: up to 100 concurrent background subagents with isolation via git worktrees or fuse-overlay

### Patterns Worth Adopting in go-agent-harness TUI

| Pattern | Source | Priority |
|---------|--------|----------|
| Diff-viewer permission prompt | Claude Code | P0 — core interaction |
| Collapsible tool output (Ctrl+O) | Claude Code | P0 — core rendering |
| Spinner with whimsical verbs | Claude Code | P1 — polish but high impact |
| `⏺`/`⎿` visual hierarchy | Claude Code | P0 — foundational |
| Session resume with search | Claude Code | P1 — core UX |
| Git-based snapshots | OpenCode | P2 — backend feature |
| Auto-compact at threshold | OpenCode/Crush | P1 — context management |
| Session branching (/fork) | Claude Code / Pi | P2 — advanced |
| Sub-agent display with background option | Claude Code | P2 — advanced |
| Activity heatmap in stats | Claude Code | P3 — polish |
| Hashline edits | Pi (omp) | P3 — edit tool quality |
| Per-agent model selection | OpenCode | P2 — model management |
