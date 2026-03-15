# Claude Code UX Research: Status Bar, Error Handling, Task Management & Advanced Features

Research conducted on Claude Code v2.1.55 running on macOS, captured via tmux ASCII screen captures.

---

## Table of Contents

1. [Startup Screen & Header](#1-startup-screen--header)
2. [Status Bar (Bottom Bar)](#2-status-bar-bottom-bar)
3. [Thinking / Loading States](#3-thinking--loading-states)
4. [Model Selection (`/model`)](#4-model-selection-model)
5. [Context Usage Visualization (`/context`)](#5-context-usage-visualization-context)
6. [Compact Mode (`/compact`)](#6-compact-mode-compact)
7. [Clear History (`/clear`)](#7-clear-history-clear)
8. [Session Management (`/resume`)](#8-session-management-resume)
9. [Fork Conversation (`/fork`)](#9-fork-conversation-fork)
10. [Rewind (`/rewind`)](#10-rewind-rewind)
11. [Stats Dashboard (`/stats`)](#11-stats-dashboard-stats)
12. [Configuration Panel (`/config`)](#12-configuration-panel-config)
13. [Permissions Management (`/permissions`)](#13-permissions-management-permissions)
14. [MCP Server Management (`/mcp`)](#14-mcp-server-management-mcp)
15. [Fast Mode (`/fast`)](#15-fast-mode-fast)
16. [Plan Mode (`/plan`)](#16-plan-mode-plan)
17. [Permission Mode Cycling](#17-permission-mode-cycling)
18. [Export Conversation (`/export`)](#18-export-conversation-export)
19. [Color Customization (`/color`)](#19-color-customization-color)
20. [Extra Usage (`/extra-usage`)](#20-extra-usage-extra-usage)
21. [Bash Mode (`!` prefix)](#21-bash-mode--prefix)
22. [Help System (`/help`)](#22-help-system-help)
23. [Interruption UX](#23-interruption-ux)
24. [Error Handling](#24-error-handling)
25. [Permission Prompts](#25-permission-prompts)
26. [Tool Call Display](#26-tool-call-display)
27. [Input Behavior](#27-input-behavior)
28. [Cost & Usage Tracking](#28-cost--usage-tracking)
29. [Complete Slash Command List](#29-complete-slash-command-list)
30. [GUI Implementation Notes](#30-gui-implementation-notes)

---

## 1. Startup Screen & Header

When Claude Code launches, it displays a branded ASCII art logo, version info, model name, subscription tier, and working directory.

### Raw Capture

```
 ▐▛███▜▌   Claude Code v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/claude-tauri-boilerplate
```

### Analysis

| Element | Value | Position |
|---------|-------|----------|
| Logo | ASCII art penguin (`▐▛███▜▌` / `▝▜█████▛▘` / `▘▘ ▝▝`) | Left side |
| Version | `Claude Code v2.1.55` | Right of logo, line 1 |
| Model + Tier | `Opus 4.6 · Claude Max` | Right of logo, line 2 |
| Working directory | `~/Develop/claude-tauri-boilerplate` | Right of logo, line 3 |

The separator between model and tier is a middle dot (`·`). The header appears once at the start of a session and stays in the scrollback.

---

## 2. Status Bar (Bottom Bar)

The status bar is a persistent single line fixed at the very bottom of the terminal. It contains multiple pieces of contextual information.

### Idle State (Default)

```
  dennisonbertram@Mac [21:00:18] [~/Develop/claude-tauri-boilerplate]  [main *] *]
```

### Idle State with MCP Failure

```
  dennisonbertram@Mac [21:00:18] [~/Develop/claude-tauri-boilerplate]  [main *] *]          1 MCP server failed · /mcp
```

### Plan Mode Active

```
  dennisonbertram@Mac [21:09:54] [~/Develop/claude-tauri-boilerplate]  [main *] *]
  ⏸ plan mode on (shift+tab to cycle)
```

### Accept Edits Mode Active

```
  dennisonbertram@Mac [21:10:04] [~/Develop/claude-tauri-boilerplate]  [main *] *]
  ⏵⏵ accept edits on (shift+tab to cycle)
```

### Analysis

| Component | Description | Position |
|-----------|-------------|----------|
| Username@Host | `dennisonbertram@Mac` | Left |
| Timestamp | `[21:00:18]` in brackets, 24h format | After username |
| Working directory | `[~/Develop/claude-tauri-boilerplate]` | After timestamp |
| Git branch | `[main *]` with dirty indicator | After directory |
| MCP status | `1 MCP server failed · /mcp` | Right-aligned |
| Permission mode | `⏸ plan mode on` or `⏵⏵ accept edits on` | Second line, left |
| Mode toggle hint | `(shift+tab to cycle)` | After mode indicator |

The status bar updates contextually:
- Shows MCP failures with a count and a `/mcp` shortcut hint
- Shows the active permission mode when not in default mode
- Shows `Esc to clear again` after pressing Escape on input
- During `/help`, `/model`, `/stats` dialogs, the status bar is replaced by dialog-specific footer hints

---

## 3. Thinking / Loading States

During extended thinking, Claude shows an animated spinner with a descriptive verb and timing information.

### Thinking Phases

```
✻ Determining… (thought for 2s)       # Early thinking
· Determining… (37s · ↓ 1.8k tokens)  # Extended thinking with metrics
✳ Determining… (56s · ↓ 2.8k tokens)  # Still thinking, symbol changed
✽ Determining… (1m 20s · ↓ 4.2k tokens)  # Over a minute
```

### Spinner Symbols (Cycling)

The spinner cycles through these Unicode asterisk variants:
- `✻` (heavy asterisk)
- `·` (middle dot)
- `✳` (eight-spoked asterisk)
- `✽` (heavy teardrop-spoked asterisk)
- `✻` (repeats)

### Fun Thinking Verbs

The thinking state randomly selects whimsical verbs instead of just "Thinking...":

| Captured Verbs |
|---------------|
| `Determining…` |
| `Waddling…` |
| `Beboppin'…` |
| `Razzmatazzing…` |
| `Flambéing…` |

### Thinking Metrics Format

```
<spinner> <verb>… (<duration> · ↓ <token_count> tokens)
```

- Duration: starts as `2s`, then `37s`, then `1m 20s`
- Token count: `1.8k tokens`, `2.8k tokens`, `4.2k tokens`
- The `↓` arrow indicates incoming/generated tokens

### Response Completion Indicator

After a response completes, a timing summary appears:

```
✻ Worked for 1m 0s
```

### During Tool Calls

```
⏺ Searching for 3 patterns… (ctrl+o to expand)
  ⎿  "packages/shared/**/*"
```

Shows the current tool operation with a hint to expand details.

### Tips During Loading

During extended thinking, tips can appear:
```
· Flambéing…
  ⎿  Tip: Run /install-github-app to tag @claude right from your Github issues and PRs
```

---

## 4. Model Selection (`/model`)

### Raw Capture

```
────────────────────────────────────────────────────────────────────────
 Select model
 Switch between Claude models. Applies to this session and future
 Claude Code sessions. For other/previous model names, specify with
 --model.

 ❯ 1. Default (recommended) ✔  Opus 4.6 · Most capable for complex work
   2. Opus (1M context)        Opus 4.6 with 1M context · Billed as
                               extra usage · $10/$37.50 per Mtok
   3. Sonnet                   Sonnet 4.6 · Best for everyday tasks
   4. Sonnet (1M context)      Sonnet 4.6 with 1M context · Billed as
                               extra usage · $6/$22.50 per Mtok
   5. Haiku                    Haiku 4.5 · Fastest for quick answers

 ▌▌▌ High effort (default) ← → to adjust

 Use /fast to turn on Fast mode (Opus 4.6 only).

 Enter to confirm · Esc to exit
```

### Analysis

| Feature | Detail |
|---------|--------|
| List style | Numbered (1-5), vertical list with arrow cursor `❯` |
| Current selection | Checkmark `✔` next to active model |
| Model info | Name + description + pricing where applicable |
| Effort slider | `▌▌▌` blocks with `← →` hint to adjust |
| Effort levels | Low, Medium, High (3 levels) |
| Footer | Keybinding hints: `Enter to confirm · Esc to exit` |
| Modal type | Takes over the bottom portion of the screen (overlay) |
| After dismiss | Shows `Kept model as Default (recommended)` |

### Available Models (Claude Max)

1. **Default (recommended)** - Opus 4.6, most capable
2. **Opus (1M context)** - Opus 4.6 with extended context, extra usage pricing
3. **Sonnet** - Sonnet 4.6, everyday tasks
4. **Sonnet (1M context)** - Sonnet 4.6 with extended context, extra usage pricing
5. **Haiku** - Haiku 4.5, fastest

---

## 5. Context Usage Visualization (`/context`)

### Raw Capture

```
❯ /context
  ⎿  Context Usage
     ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   claude-opus-4-6 · 41k/200k tokens (20%)
     ⛁ ⛀ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   Estimated usage by category
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   ⛁ System prompt: 4.5k tokens (2.2%)
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   ⛁ System tools: 18.7k tokens (9.4%)
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   ⛁ Custom agents: 987 tokens (0.5%)
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   ⛁ Memory files: 6.1k tokens (3.1%)
     ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶   ⛁ Skills: 4.3k tokens (2.2%)
     ⛶ ⛶ ⛶ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝   ⛁ Messages: 6.9k tokens (3.4%)
     ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝   ⛶ Free space: 125k (62.7%)
                           ⛝ Autocompact buffer: 33k tokens (16.5%)
```

### Analysis

| Feature | Detail |
|---------|--------|
| Display type | 10x10 grid of Unicode symbols |
| Grid symbols | `⛁` (filled/used), `⛀` (partially used), `⛶` (free), `⛝` (autocompact buffer) |
| Header | Model name, current/max tokens, percentage |
| Categories | System prompt, System tools, Custom agents, Memory files, Skills, Messages, Free space, Autocompact buffer |
| Category format | `⛁ Category: Xk tokens (Y%)` |
| Followed by | MCP tools list, Custom agents, Memory files, Skills (with token counts per item) |

### Token Budget Breakdown (Example)

| Category | Tokens | Percentage |
|----------|--------|------------|
| System prompt | 4.5k | 2.2% |
| System tools | 18.7k | 9.4% |
| Custom agents | 987 | 0.5% |
| Memory files | 6.1k | 3.1% |
| Skills | 4.3k | 2.2% |
| Messages | 6.9k | 3.4% |
| Free space | 125k | 62.7% |
| Autocompact buffer | 33k | 16.5% |
| **Total** | **200k** | **100%** |

---

## 6. Compact Mode (`/compact`)

### During Compaction

```
❯ /compact

✢ Compacting conversation…
```

### After Compaction

```
✻ Conversation compacted (ctrl+o for history)

❯ /compact
  ⎿  Compacted (ctrl+o to see full summary)
```

### Analysis

- Uses `✢` symbol during compaction (distinct from the thinking spinners)
- Shows "Compacting conversation..." as the active state
- After completion, shows "Conversation compacted" with a ctrl+o hint
- The inline summary shows "Compacted (ctrl+o to see full summary)"
- The full conversation history is replaced with a summary that preserves key context
- `ctrl+o` expands the summary to show the full compacted text

---

## 7. Clear History (`/clear`)

```
❯ /clear
  ⎿  (no content)
```

### Analysis

- Immediately clears all conversation history
- Shows `(no content)` as confirmation
- The header remains visible (version, model, directory)
- MCP server status returns to the status bar
- No confirmation prompt - it executes immediately

---

## 8. Session Management (`/resume`)

### Raw Capture

```
Resume Session (1 of 15)
╭──────────────────────────────────────────────────────────────────╮
│ ⌕ Search…                                                       │
╰──────────────────────────────────────────────────────────────────╯
  current worktree

❯ Run git status and show me the output
  8 seconds ago · main · 28.4KB

  read the file CLAUDE.md
  30 seconds ago · main · 81.3KB

  /exit
  1 minute ago · main · 1.8KB

  please use context7 to understand all the features...
  2 minutes ago · worktree-agent-a61dddc0 · 1.7MB

  What is 2+2? Answer briefly.
  2 minutes ago · main · 41.9KB

  Hi
  3 minutes ago · main · 9.6KB
```

### Footer

```
Ctrl+A to show all projects · Ctrl+B to toggle branch · Ctrl+W to
show all worktrees · Ctrl+V to preview · Ctrl+R to rename · Type to
search · Esc to cancel
```

### Analysis

| Feature | Detail |
|---------|--------|
| Header | `Resume Session (X of Y)` with total count |
| Search | `⌕ Search…` text input at top with border box |
| Session items | First message preview (truncated if long) |
| Metadata per session | Relative time (`8 seconds ago`) + branch (`main`) + size (`28.4KB`) |
| Worktree indicator | `current worktree` label above current worktree sessions |
| Arrow cursor | `❯` on selected item |
| Navigation | Up/Down arrows, Type to search, Enter to select |
| Extra controls | Ctrl+A (all projects), Ctrl+B (toggle branch), Ctrl+W (worktrees), Ctrl+V (preview), Ctrl+R (rename) |
| Cancel | Escape |
| After cancel | Shows `Resume cancelled` |

---

## 9. Fork Conversation (`/fork`)

```
❯ /fork
  ⎿  Forked conversation. You are now in the fork.
     To resume the original: claude -r f9166a61-3d88-40b9-8e43-86ed46556fdf
```

### Analysis

- Immediately creates a fork (no confirmation needed)
- Shows UUID of the original session for resuming later
- Provides the exact command to resume the original: `claude -r <uuid>`
- The fork maintains all conversation history up to that point
- New messages go into the fork only

---

## 10. Rewind (`/rewind`)

### Raw Capture

```
────────────────────────────────────────────────────────────────────
 Rewind

 Restore the code and/or conversation to the point before…

   Read the file /tmp/this-file-does-not-exist-at-all.txt...
   No code changes

   /plan
   No code changes

   /export
   No code changes

   \!echo hello
   No code changes

   /color
   No code changes

   Run this command: cat /nonexistent/file/path
   No code changes

 ❯ (current)

 Enter to continue · Esc to exit
```

### Analysis

| Feature | Detail |
|---------|--------|
| Title | "Rewind" at top of dialog |
| Description | "Restore the code and/or conversation to the point before..." |
| List items | Each conversation turn shown as a selectable point |
| Code change indicator | `No code changes` when no files were modified |
| Current position | `❯ (current)` marks the current point |
| Navigation | Arrow keys, Enter to confirm, Escape to cancel |
| Behavior | Reverts both conversation state AND code changes to the selected point |

---

## 11. Stats Dashboard (`/stats`)

### Overview Tab

```
 ──────────────────────────────────────────────────────────────────
    Overview   Models  (←/→ or tab to cycle)

      Mar Apr May Jun Jul Aug Sep Oct Nov Dec Jan Feb Mar
      ···········································░░░▒█▓▓██
  Mon ···········································▒▓░░▓██░█
      ···········································▒▒░▒▓▒█▓█
  Wed ··········································░█░▓░▒▒█▒█
      ··········································▓▒▒▒░▓▓▒░█
  Fri ··········································░░░▓█▓▒█▓▓
      ··········································▒▒▒▓▓█·█▓█

      Less ░ ▒ ▓ █ More

  All time · Last 7 days · Last 30 days

  Favorite model: Opus 4.5        Total tokens: 14.4m

  Sessions: 863                   Longest session: 8d 8h 42m
  Active days: 67/68              Longest streak: 45 days
  Most active day: Mar 8          Current streak: 21 days

  You've used ~27x more tokens than Don Quixote

  Esc to cancel · r to cycle dates · ctrl+s to copy
```

### Models Tab

```
 ──────────────────────────────────────────────────────────────────
    Overview   Models  (←/→ or tab to cycle)

  Tokens per Day
    919k ┼       ╭╮
    804k ┤       ││
    690k ┤       ││
    575k ┤       ││╭╮
    460k ┤ ╭╮    ││││╭╮
    345k ┼╮││   ╭╯│││││
    230k ┤││╰╮ ╭╯ ╰╮│││     ╭╮
    115k ┤╭╯ │╭╯╮ ╭│╰╯╰╮╭╮╭╮╭╮ ╭╮ ╭╮ ╭╮╭───╮╭──╮╭╮ ╭───╮╭╮ ╭╮
       0 ┼──────────────╯╰╯╰╯╰─╯╰─╯╰─╯╰╯───╰╯──╰╯╰─╯───╰╯╰─╯╰
          Jan 21         Feb 5          Feb 21         Mar 8
  ● Opus 4.5 · ● Sonnet 4.5 · ● Opus 4.6

  All time · Last 7 days · Last 30 days

  ● Opus 4.5 (59.9%)                      ● Opus 4.6 (17.9%)
    In: 2.9m · Out: 5.7m                    In: 906.2k · Out: 1.7m
  ● Sonnet 4.5 (18.1%)                    ● Sonnet 4.6 (4.0%)
    In: 518.4k · Out: 2.1m                  In: 50.7k · Out: 518.2k

    ↓ 1-4 of 5 models (↑↓ to scroll)

  Esc to cancel · r to cycle dates · ctrl+s to copy
```

### Analysis

| Feature | Detail |
|---------|--------|
| Tabs | `Overview` and `Models`, switched with ←/→ or Tab |
| Activity heatmap | GitHub-style contribution grid using block characters (`░ ▒ ▓ █`) |
| Heatmap rows | Labeled by day of week (Mon, Wed, Fri) |
| Heatmap columns | Labeled by month |
| Time periods | `All time · Last 7 days · Last 30 days` (cycle with `r`) |
| Key stats | Sessions, Active days, Longest session, Longest streak, Current streak, Most active day |
| Fun fact | "You've used ~27x more tokens than Don Quixote" |
| Token chart | ASCII line chart with y-axis labels |
| Model breakdown | Colored dots (●) per model with usage percentages |
| Model details | Input/Output token counts per model |
| Footer | `Esc to cancel · r to cycle dates · ctrl+s to copy` |

---

## 12. Configuration Panel (`/config`)

### Raw Capture

```
 ──────────────────────────────────────────────────────────────────
  Settings:  Status   Config   Usage  (←/→ or tab to cycle)

  Configure Claude Code preferences

  ╭─────────────────────────────────────────────────────────────╮
  │ ⌕ Search settings...                                        │
  ╰─────────────────────────────────────────────────────────────╯

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
    Use custom API key: DpolnG-HTpw-btKS3wAA  false

  Type to filter · Enter/↓ to select · Esc to clear
```

### Analysis

| Feature | Detail |
|---------|--------|
| Tabs | `Status`, `Config`, `Usage` (3 tabs) |
| Search | Fuzzy search input at top with `⌕` icon |
| Layout | Two-column: setting name (left) + current value (right) |
| Selection | Arrow cursor `❯` on focused item |
| Toggle values | `true`/`false`, named values like `Dark mode (ANSI colors only)` |
| Total settings | 23+ visible settings |
| Footer | `Type to filter · Enter/↓ to select · Esc to clear` |
| After dismiss | Shows `Config dialog dismissed` |

### Settings List

| Setting | Default Value |
|---------|---------------|
| Auto-compact | true |
| Show tips | true |
| Reduce motion | false |
| Thinking mode | true |
| Fast mode | false |
| Rewind code (checkpoints) | true |
| Verbose output | false |
| Terminal progress bar | true |
| Default permission mode | Default |
| Respect .gitignore | true |
| Auto-update channel | disabled |
| Theme | Dark mode (ANSI colors only) |
| Notifications | Auto |
| Output style | default |
| Language | Default (English) |
| Editor mode | normal |
| Show PR status footer | true |
| Model | Default (recommended) |
| Auto-connect to IDE | false |
| Claude in Chrome | true |
| Teammate mode | auto |
| Remote Control | default |
| Custom API key | false |

---

## 13. Permissions Management (`/permissions`)

### Raw Capture

```
 ──────────────────────────────────────────────────────────────────
 Permissions:  Allow   Ask   Deny   Workspace  (←/→ or tab to cycle)

 Claude Code won't ask before using allowed tools.
 ╭────────────────────────────────────────────────╮
 │ ⌕ Search…                                      │
 ╰────────────────────────────────────────────────╯

 ❯ 1.  Add a new rule…
   2.  mcp__claude-in-chrome__computer
   3.  mcp__claude-in-chrome__find
   4.  mcp__claude-in-chrome__form_input
   5.  mcp__claude-in-chrome__get_page_text
   6.  mcp__claude-in-chrome__gif_creator
   7.  mcp__claude-in-chrome__javascript_tool
   8.  mcp__claude-in-chrome__navigate
   9.  mcp__claude-in-chrome__read_console_messages
 ↓ 10. mcp__claude-in-chrome__read_network_requests

 Press ↑↓ to navigate · Enter to select · Type to search · Esc to cancel
```

### Analysis

| Feature | Detail |
|---------|--------|
| Tabs | `Allow`, `Ask`, `Deny`, `Workspace` (4 categories) |
| Description | Context-sensitive: "Claude Code won't ask before using allowed tools" |
| Search | `⌕ Search…` text input |
| List items | Numbered, scrollable with `↓` indicator for more |
| First item | Always `Add a new rule…` |
| Tool format | Full tool name including MCP prefix |
| Footer | `Press ↑↓ to navigate · Enter to select · Type to search · Esc to cancel` |

---

## 14. MCP Server Management (`/mcp`)

### Raw Capture

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
 https://code.claude.com/docs/en/mcp for help

 ↑↓ to navigate · Enter to confirm · Esc to cancel
```

### Analysis

| Feature | Detail |
|---------|--------|
| Title | "Manage MCP servers" with total count |
| Grouping | Three sections: User MCPs, claude.ai, Built-in MCPs |
| Status indicators | `✔ connected` (green), `✘ failed` (red), `△ needs authentication` (yellow) |
| Config source | User MCPs show path: `(/Users/.../.claude.json)` |
| Help links | Debug command hint + documentation URL |
| Footer | `↑↓ to navigate · Enter to confirm · Esc to cancel` |

### Status Icons

| Icon | Meaning | Visual |
|------|---------|--------|
| `✔` | Connected successfully | Green |
| `✘` | Connection failed | Red |
| `△` | Needs authentication | Yellow/warning |

---

## 15. Fast Mode (`/fast`)

### Raw Capture

```
 ↯ Fast mode (research preview)
 High-speed mode for Opus 4.6. Billed as extra usage at a premium
 rate. Separate rate limits apply.

   Fast mode  OFF  $30/$150 per Mtok

 Learn more: https://code.claude.com/docs/en/fast-mode

 Tab to toggle · Enter to confirm · Esc to cancel
```

### Analysis

| Feature | Detail |
|---------|--------|
| Icon | `↯` (downward zigzag arrow) |
| Label | "research preview" |
| Toggle | `OFF` / `ON` visual toggle |
| Pricing | `$30/$150 per Mtok` (input/output) |
| Documentation link | Inline URL |
| Controls | Tab to toggle, Enter to confirm, Esc to cancel |
| After dismiss | Shows `Kept Fast mode OFF` or `Enabled Fast mode` |

---

## 16. Plan Mode (`/plan`)

```
❯ /plan
  ⎿  Enabled plan mode
```

Status bar changes to:
```
  ⏸ plan mode on (shift+tab to cycle)
```

### Analysis

- Simple toggle - no dialog, just enables immediately
- `⏸` (pause symbol) icon in the status bar
- Plan mode prevents Claude from making code changes
- Claude will only describe what it would do
- Shift+Tab cycles through permission modes

---

## 17. Permission Mode Cycling

Pressing `Shift+Tab` cycles through three modes:

| Mode | Status Bar Display | Icon | Behavior |
|------|-------------------|------|----------|
| Default | (no indicator) | none | Normal - asks for permission |
| Plan | `⏸ plan mode on (shift+tab to cycle)` | `⏸` | Read-only, no changes |
| Accept Edits | `⏵⏵ accept edits on (shift+tab to cycle)` | `⏵⏵` | Auto-approve file edits |

The cycle order is: Default → Accept Edits → Plan → Default

---

## 18. Export Conversation (`/export`)

```
 Export Conversation
 Select export method:

 ❯ 1. Copy to clipboard  Copy the conversation to your system clipboard
   2. Save to file       Save the conversation to a file in the
                          current directory

 Esc to cancel
```

### Analysis

- Two export options: clipboard or file
- File saves to the current working directory
- Numbered list with descriptions
- Simple Esc to cancel
- After cancel: `Export cancelled`

---

## 19. Color Customization (`/color`)

```
❯ /color
  ⎿  Please provide a color. Available colors: red, blue, green,
     yellow, purple, orange, pink, cyan
```

### Analysis

- Takes a color argument: `/color blue`
- Available colors: red, blue, green, yellow, purple, orange, pink, cyan
- Changes the prompt bar border color for the current session
- If called without argument, lists available colors

---

## 20. Extra Usage (`/extra-usage`)

```
 Starting new login following /extra-usage. Exit with Ctrl-C to use
 existing account.

 Select login method:

 ❯ 1. Claude account with subscription · Pro, Max, Team, or Enterprise
   2. Anthropic Console account · API usage billing
   3. 3rd-party platform · Amazon Bedrock, Microsoft Foundry, or Vertex AI
```

### Analysis

- Full-screen login flow that takes over the terminal
- Three login methods with descriptions
- `Ctrl-C` to exit and keep existing account
- This is a modal flow that blocks all other interaction until completed or cancelled

---

## 21. Bash Mode (`!` prefix)

### Raw Capture

```
❯ \!echo hello

⏺ Bash(echo hello)
  ⎿  hello

✻ Waddling…
```

After completion:
```
⏺ Done.
```

### Analysis

- Prefix a message with `!` to run a bash command directly
- The `!` is displayed as `\!` (escaped) in the chat history
- Command is wrapped in `Bash()` tool call format
- Output is shown in the collapsible `⎿` format
- Claude then processes/summarizes the output
- Equivalent to asking "run this command" but faster

---

## 22. Help System (`/help`)

### General Tab

```
 ──────────────────────────────────────────────────────────────────
  Claude Code v2.1.55  general   commands   custom-commands
  (←/→ or tab to cycle)

  Claude understands your codebase, makes edits with your
  permission, and executes commands — right from your terminal.

  Shortcuts
  ! for bash mode           double tap esc to clear input
  / for commands            shift + tab to auto-accept edits
  @ for file paths          ctrl + o for verbose output
  & for background          ctrl + t to toggle tasks
                            \⏎ for newline
  ctrl + shift + - to undo
  ctrl + z to suspend
  ctrl + v to paste images
  meta + p to switch model
  meta + o to toggle fast mode
  ctrl + s to stash prompt
  ctrl + g to edit in $EDITOR
  /keybindings to customize

 For more help: https://code.claude.com/docs/en/overview

 Esc to cancel
```

### Commands Tab (Full List Captured)

Browsable list with description for each command:

```
  /add-dir         Add a new working directory
  /agents          Manage agent configurations
  /chrome          Claude in Chrome (Beta) settings
  /clear           Clear conversation history and free up context
  /color           Set the prompt bar color for this session
  /compact         Clear conversation history but keep a summary...
  /config          Open config panel
  /context         Visualize current context usage as a colored grid
  /copy            Copy Claude's last response to clipboard as markdown
  /desktop         (not fully captured)
  /export          Export the current conversation to a file or clipboard
  /extra-usage     Configure extra usage to keep working when limits are hit
  /fast            Toggle fast mode (Opus 4.6 only)
  /feedback        Submit feedback about Claude Code
  /fork            Create a fork of the current conversation at this point
  /help            Show help and available commands
  /hooks           (not fully captured)
  /install-github-app  Set up Claude GitHub Actions for a repository
  /install-slack-app   Install the Claude Slack app
  /keybindings     Open or create your keybindings configuration file
  /login           Sign in with your Anthropic account
  /logout          Sign out from your Anthropic account
  /mcp             Manage MCP servers
  /memory          (not fully captured)
  /passes          Share a free week of Claude Code with friends...
  /permissions     Manage allow & deny tool permission rules
  /plan            Enable plan mode or view the current session plan
  /plugin          Manage Claude Code plugins
  /pr-comments     Get comments from a GitHub pull request
  /privacy-settings View and update your privacy settings
  /release-notes   (not fully captured)
  /resume          Resume a previous conversation
  /review          Review a pull request
  /rewind          Restore the code and/or conversation to a previous point
  /sandbox         ◯ sandbox disabled (⏎ to configure)
  /security-review Complete a security review of pending changes
  /skills          List available skills
  /stats           (not fully captured)
```

### Complete Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `!` | Bash mode (run command directly) |
| `/` | Command menu |
| `@` | File path autocompletion |
| `&` | Background task |
| `Double Escape` | Clear input |
| `Shift+Tab` | Cycle permission modes (auto-accept edits) |
| `Ctrl+O` | Toggle verbose output / expand collapsed content |
| `Ctrl+T` | Toggle task display |
| `\Enter` | Insert newline (instead of submitting) |
| `Ctrl+Shift+-` | Undo |
| `Ctrl+Z` | Suspend Claude (like any terminal app) |
| `Ctrl+V` | Paste images |
| `Meta+P` | Switch model |
| `Meta+O` | Toggle fast mode |
| `Ctrl+S` | Stash prompt |
| `Ctrl+G` | Edit prompt in `$EDITOR` |
| `Ctrl+C` | Interrupt / cancel |
| `Escape` | Cancel / dismiss / interrupt response |
| `/keybindings` | Customize keybindings |

---

## 23. Interruption UX

### Escape During Response

```
❯ Write a very long essay about the history of computing...
  ⎿  Interrupted · What should Claude do instead?
```

### Analysis

- **Escape** during streaming: Immediately stops generation
- Shows `Interrupted · What should Claude do instead?` inline
- The prompt is ready for a new message immediately
- The partial response is NOT shown - it's discarded
- The user can type a new instruction or continue

### Ctrl+C Behavior

- During input: Clears the current input field
- During response: Same as Escape - interrupts the response
- At empty prompt: Does nothing (doesn't exit Claude)

### Key Differences from Escape

| Scenario | Escape | Ctrl+C |
|----------|--------|--------|
| During response | Interrupt + "What should Claude do instead?" | Same behavior |
| During input (with text) | First press: shows "Esc to clear again" | Clears input |
| During input (empty) | No effect | No effect |
| In a dialog | Closes/cancels the dialog | May close or have no effect |

---

## 24. Error Handling

### Tool Call Error (File Not Found)

```
❯ Read the file /tmp/this-file-does-not-exist-at-all.txt...

⏺ Read 1 file (ctrl+o to expand)

⏺ That file doesn't exist at /tmp/this-file-does-not-exist-at-all.txt.
```

### Analysis

- Tool errors are NOT displayed as error messages with special formatting
- Instead, the tool executes (or attempts to), and Claude interprets the error
- The error is communicated as natural language in Claude's response
- No red error banners, no error codes, no stack traces shown to the user
- The tool call itself shows in the collapsed `⎿` format with `ctrl+o to expand`
- This is a deliberate design choice: errors are handled conversationally

### Nested Session Error

```
Error: Claude Code cannot be launched inside another Claude Code session.
Nested sessions share runtime resources and will crash all active sessions.
To bypass this check, unset the CLAUDECODE environment variable.
```

### Analysis of System Errors

- System-level errors (not tool errors) are displayed as plain text
- No colored error boxes or special formatting
- Include actionable instructions when possible
- Appear before the Claude interface loads

---

## 25. Permission Prompts

### Bash Command Permission

```
────────────────────────────────────────────────────────────────────
 Bash command

   cat /nonexistent/file/path
   Cat a nonexistent file path

 Do you want to proceed?
 ❯ 1. Yes
   2. No

 Esc to cancel · Tab to amend · ctrl+e to explain
```

### File Read Permission

```
────────────────────────────────────────────────────────────────────
 Read file

  Read(/tmp/this-file-does-not-exist-at-all.txt)

 Do you want to proceed?
 ❯ 1. Yes
   2. Yes, allow reading from tmp/ during this session
   3. No

 Esc to cancel · Tab to amend
```

### Analysis

| Feature | Detail |
|---------|--------|
| Title | Tool type: "Bash command", "Read file", etc. |
| Command preview | Shows the exact command or path |
| Description | AI-generated summary of what the command does |
| Options | Yes, Yes + scope expansion, No |
| Scope expansion | "allow reading from X/ during this session" |
| Additional controls | `Tab to amend` (modify the command), `ctrl+e to explain` (explain what it does) |
| Cancel | Escape |
| Separator | Horizontal line (`────`) above the dialog |

### Key UX Detail: `Tab to amend`

Users can press Tab to modify the proposed tool call before approving it. This is a powerful feature that allows:
- Editing the bash command before running
- Changing file paths
- Adding flags or arguments

---

## 26. Tool Call Display

### Collapsed (Default)

```
⏺ Bash(git status)
  ⎿  On branch main
     Your branch is up to date with 'origin/main'.
     … +15 lines (ctrl+o to expand)
```

### In-Progress

```
⏺ Searching for 3 patterns… (ctrl+o to expand)
  ⎿  "packages/shared/**/*"
```

### Read File (Collapsed)

```
⏺ Read 1 file (ctrl+o to expand)
```

```
⏺ Reading 1 file… (ctrl+o to expand)
  ⎿  /tmp/this-file-does-not-exist-at-all.txt
```

### Analysis

| State | Format |
|-------|--------|
| In-progress | `⏺ <Verb>ing N <noun>… (ctrl+o to expand)` |
| Completed | `⏺ <Verb>ed N <noun> (ctrl+o to expand)` |
| With preview | `⎿  <first few lines>` then `… +N lines (ctrl+o to expand)` |
| Tool name | `Bash(command)`, `Read(path)`, etc. |

The `⏺` (filled circle) is the standard prefix for all Claude response blocks (both text and tool calls). Tool calls are differentiated by their `ToolName(args)` format.

---

## 27. Input Behavior

### Input Area

```
────────────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────────────
```

### Key Behaviors

| Behavior | Detail |
|----------|--------|
| Submit | Enter sends the message (single line) |
| Newline | `\Enter` (backslash + Enter) for multiline input |
| Empty submit | Enter on empty input does nothing (no empty messages) |
| Clear input | Escape (first press shows "Esc to clear again", second press clears) or Ctrl+C |
| Prompt character | `❯` (right-pointing triangle) |
| Borders | Horizontal lines above and below the input area |
| File paths | `@` triggers autocomplete for file paths |
| Commands | `/` triggers command autocomplete |
| Bash mode | `!` at start enables direct command execution |
| Background | `&` at start runs task in background |
| Multi-line display | Input expands vertically for multi-line content |
| External editor | `Ctrl+G` opens `$EDITOR` for complex input |
| Stash prompt | `Ctrl+S` saves current input for later |

### Double Escape Clear Pattern

1. First Escape: Shows `Esc to clear again` in status bar
2. Second Escape: Clears the input
3. This two-step pattern prevents accidental clearing

---

## 28. Cost & Usage Tracking

### Where Cost is Displayed

1. **No per-message cost display** - Claude Code (with subscription) does not show per-message costs
2. **`/context`** - Shows token usage as a grid visualization with category breakdown
3. **`/stats`** - Shows total token usage, model breakdown, daily charts
4. **Model selector** - Shows pricing for premium models (e.g., `$10/$37.50 per Mtok`)
5. **Fast mode** - Shows pricing (`$30/$150 per Mtok`)
6. **Thinking indicator** - Shows token count during extended thinking (`↓ 4.2k tokens`)

### Token Metrics Format

| Location | Format |
|----------|--------|
| Context grid | `41k/200k tokens (20%)` |
| Category breakdown | `4.5k tokens (2.2%)` |
| Thinking indicator | `↓ 1.8k tokens` |
| Stats overview | `Total tokens: 14.4m` |
| Stats models | `In: 2.9m · Out: 5.7m` |
| Skills/tools | `138 tokens` per item |

### API Key Users vs Subscription

- Subscription users (Claude Max): No per-message cost shown
- API key users: Cost likely shown per request (not confirmed in this session)
- Extra usage features show explicit pricing

---

## 29. Complete Slash Command List

Captured from `/help` commands tab:

| Command | Description |
|---------|-------------|
| `/add-dir` | Add a new working directory |
| `/agents` | Manage agent configurations |
| `/chrome` | Claude in Chrome (Beta) settings |
| `/clear` | Clear conversation history and free up context |
| `/color` | Set the prompt bar color for this session |
| `/compact` | Clear conversation history but keep a summary |
| `/config` | Open config panel |
| `/context` | Visualize current context usage as a colored grid |
| `/copy` | Copy Claude's last response to clipboard as markdown |
| `/desktop` | Desktop app settings |
| `/export` | Export the current conversation to a file or clipboard |
| `/extra-usage` | Configure extra usage when limits are hit |
| `/fast` | Toggle fast mode (Opus 4.6 only) |
| `/feedback` | Submit feedback about Claude Code |
| `/fork` | Create a fork of the current conversation |
| `/help` | Show help and available commands |
| `/hooks` | Manage hooks |
| `/install-github-app` | Set up Claude GitHub Actions |
| `/install-slack-app` | Install the Claude Slack app |
| `/keybindings` | Open keybindings configuration |
| `/login` | Sign in with Anthropic account |
| `/logout` | Sign out |
| `/mcp` | Manage MCP servers |
| `/memory` | Manage memory files |
| `/passes` | Share free Claude Code passes |
| `/permissions` | Manage allow & deny tool permission rules |
| `/plan` | Enable plan mode |
| `/plugin` | Manage Claude Code plugins |
| `/pr-comments` | Get comments from a GitHub PR |
| `/privacy-settings` | View and update privacy settings |
| `/release-notes` | View release notes |
| `/resume` | Resume a previous conversation |
| `/review` | Review a pull request |
| `/rewind` | Restore code/conversation to a previous point |
| `/sandbox` | Configure sandbox mode |
| `/security-review` | Security review of pending changes |
| `/skills` | List available skills |
| `/stats` | View usage statistics |

---

## 30. GUI Implementation Notes

### Status Bar Implementation

The desktop app needs a persistent status bar with these dynamic components:
- **Left**: User identity, timestamp, working directory, git branch
- **Right**: MCP status (with failure count), permission mode indicators
- **Second line** (conditional): Permission mode (`⏸ plan mode on`, `⏵⏵ accept edits on`)
- The status bar should update in real-time as mode changes occur

### Thinking State Animation

- Implement a cycling spinner with multiple Unicode symbols
- Display duration timer (seconds, then minutes:seconds)
- Show token count during extended thinking
- Use fun/whimsical verbs randomly selected from a pool
- The completion indicator should show total time: `✻ Worked for Xm Ys`

### Dialog System

Most slash commands open modal dialogs with consistent patterns:
- **Tabbed dialogs**: Model selector, Config, Stats, Permissions (`←/→ or tab to cycle`)
- **List dialogs**: Resume, Rewind, MCP, Permissions (`↑↓` navigation, `❯` cursor)
- **Search dialogs**: Config, Permissions, Resume (have `⌕ Search…` input)
- **Toggle dialogs**: Fast mode, Plan mode (simple on/off)
- **Confirmation dialogs**: Permission prompts (Yes/No with scope options)
- All dialogs share: `Esc to cancel` and `Enter to confirm`

### Permission System UI

Three permission modes (cycled with Shift+Tab):
1. Default - asks before potentially dangerous actions
2. Plan mode - read-only, no changes allowed
3. Accept edits - auto-approve file modifications

Permission prompts should include:
- Tool name and type
- AI-generated description of what the action does
- Scope expansion option ("allow for this session")
- Tab-to-amend for modifying the proposed action
- Ctrl+E for explanation

### Context Visualization

The 10x10 grid visualization uses Unicode symbols:
- `⛁` = filled (used space)
- `⛀` = partially filled
- `⛶` = empty (free space)
- `⛝` = autocompact buffer
- Each grid position represents ~2% of total context

### Session Management

The resume picker needs:
- Search functionality
- Relative time display
- Branch name per session
- Session size (KB/MB)
- Worktree grouping
- Preview capability (Ctrl+V)
- Rename capability (Ctrl+R)

### Error Handling Philosophy

Claude Code handles errors conversationally rather than with error UI:
- Tool errors are interpreted by Claude and explained in natural language
- No red error banners, error codes, or stack traces
- System errors (before Claude loads) are plain text with instructions
- The user never sees raw error output unless they expand tool calls

### Interruption Model

Two interruption mechanisms:
1. **Escape**: Stops response, shows "What should Claude do instead?"
2. **Ctrl+C**: During input clears text, during response same as Escape
- Partial responses are discarded on interrupt
- The input is immediately ready for the next message

### Input Model

- Single Enter submits (not Shift+Enter like many chat apps)
- `\Enter` for newline (unusual pattern - needs clear documentation)
- Double Escape to clear (safety against accidental clearing)
- Empty submits are silently ignored (no error message)
- `@` for file paths, `/` for commands, `!` for bash, `&` for background
