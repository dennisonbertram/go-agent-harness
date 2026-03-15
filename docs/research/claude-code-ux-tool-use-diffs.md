# Claude Code UX: Tool Use, File Operations, and Diff Display

Research conducted on Claude Code v2.1.55 running on macOS (Darwin 24.1.0, ARM64).
Model: Opus 4.6 via Claude Max subscription.

---

## 1. Overall Visual Architecture

Claude Code uses a consistent visual language for all tool interactions built around three core elements:

1. **Filled circle icon** (`⏺`) -- marks each tool call and each response paragraph
2. **Tree connector** (`⎿`) -- connects tool results/details beneath the tool call
3. **User prompt indicator** (`❯`) -- marks user input messages

The conversation scrolls vertically with each turn displayed as:
```
❯ [User message]

⏺ [Tool call - collapsed]

⏺ [Response text from Claude]
```

Two display modes are available, toggled with `Ctrl+O`:
- **Collapsed mode** (default): Tool calls show summary text like "Read 1 file"
- **Expanded/detailed mode**: Shows full tool parameters, file paths, timestamps, hook status

---

## 2. Tool Call Display

### 2.1 Collapsed View (Default)

Each tool category has its own collapsed display format:

#### Read Tool
```
⏺ Read 1 file (ctrl+o to expand)
```
When multiple files are read:
```
⏺ Read 3 files (ctrl+o to expand)
```

#### Edit/Update Tool
```
⏺ Update(test.js)
```
Shown with the filename in parentheses. After approval, the result appears beneath:
```
⏺ Update(test.js)
  ⎿  Added 1 line
      1  function hello() { return "world"; }
      2 +function goodbye() { return "farewell"; }
```

#### Write Tool (New File)
```
⏺ Write(utils.js)
```
After approval:
```
⏺ Write(utils.js)
  ⎿  Wrote 2 lines to utils.js
      1 function add(a, b) { return a + b; }
      2 function subtract(a, b) { return a - b; }
```

#### Bash Tool
```
⏺ Bash(echo 'hello from bash' && date && uname -a)
  ⎿  hello from bash
     Sat Mar 14 21:04:57 EDT 2026
     Darwin Mac 24.1.0 Darwin Kernel Version 24.1.0: ...
```
The command is shown in parentheses. Output is indented beneath the tree connector.

#### Search Tools (Grep/Glob)
Both Grep and Glob show as:
```
⏺ Searched for 1 pattern (ctrl+o to expand)
```

### 2.2 Expanded/Detailed View (Ctrl+O)

When toggled to expanded mode, tool calls show:
- **Full absolute file paths** instead of just filenames
- **Tool name with parameters**: `Read(config.js)`, `Search(pattern: "*.js", path: "/private/tmp/...")`
- **Result counts**: `Read 3 lines`, `Found 4 lines`, `Found 3 files`
- **Post-tool hooks**: `1 PostToolUse hook ran`
- **Timestamps** on the right margin: `09:09 PM claude-opus-4-6`
- **Truncated output** for long results: `... +6 lines (ctrl+o to expand)`

Example expanded Bash tool call:
```
⏺ Bash(cat /etc/hosts)
  ⎿  ##
     # Host Database
     #
     ... +6 lines (ctrl+o to expand)
```

Example expanded Search (Glob) tool call:
```
⏺ Search(pattern: "*.js", path: "/private/tmp/claude-ux-test")
  ⎿  Found 3 files (ctrl+o to expand)
```

Example expanded Read tool call:
```
⏺ Read(config.js)
  ⎿  Read 3 lines
```

### 2.3 History Navigation

- `Ctrl+O` -- toggles between collapsed and detailed transcript view
- `Ctrl+E` -- shows/collapses all previous messages
- Bottom indicator when in detailed mode: `Showing detailed transcript · ctrl+o to toggle · ctrl+e to collapse`
- When messages are hidden: `ctrl+e to show 108 previous messages`

---

## 3. File Edit Display (Diffs)

### 3.1 Permission Prompt (Before Edit)

When Claude wants to edit a file, a full-width permission dialog appears:

```
────────────────────────────────────────────────────────────────
 Edit file
 test.js
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 1 -function hello() { return "world"; }
 2 -function goodbye() { return "farewell"; }
 1 +function hello(name) { return "Hello, " + name + "!"; }
 2 +function farewell() { return "farewell"; }
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Do you want to make this edit to test.js?
 ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No

 Esc to cancel · Tab to amend
────────────────────────────────────────────────────────────────
```

### 3.2 Diff Format Details

- **Unified diff style** -- additions and deletions shown inline (not side-by-side)
- **Line numbers** shown on the left: old lines have old numbers, new lines have new numbers
- **Deletions** prefixed with `-` (displayed in red/removal color in the terminal)
- **Additions** prefixed with `+` (displayed in green/addition color in the terminal)
- **Unchanged lines** shown with just the line number (no prefix)
- **Bordered** with dashed separator lines (`╌╌╌`) above and below the diff
- **Header** shows "Edit file" and filename on separate lines above the diff
- **Full-width** spanning the terminal width

### 3.3 Diff for New File (Write/Create)

When creating a new file, the header changes:

```
────────────────────────────────────────────────────────────────
 Create file
 utils.js
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
  1 function add(a, b) { return a + b; }
  2 function subtract(a, b) { return a - b; }
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Do you want to create utils.js?
 ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No

 Esc to cancel · Tab to amend
────────────────────────────────────────────────────────────────
```

Key differences from "Edit file":
- Header says "Create file" instead of "Edit file"
- All lines are new (no `-` prefix, no `+` prefix, just line numbers)
- Permission question says "Do you want to create..." instead of "Do you want to make this edit to..."

### 3.4 Post-Approval Diff Display

After approval, the tool call in the conversation shows a collapsed summary:

For edits:
```
⏺ Update(test.js)
  ⎿  Added 2 lines, removed 2 lines
      1 -function hello() { return "world"; }
      2 -function goodbye() { return "farewell"; }
      1 +function hello(name) { return "Hello, " + name + "!"; }
      2 +function farewell() { return "farewell"; }
```

For new files:
```
⏺ Write(utils.js)
  ⎿  Wrote 2 lines to utils.js
      1 function add(a, b) { return a + b; }
      2 function subtract(a, b) { return a - b; }
```

Summary text patterns:
- `Added 1 line`
- `Added 2 lines, removed 2 lines`
- `Wrote 2 lines to utils.js`
- `Wrote 5 lines to index.js`

---

## 4. Permission Prompts

### 4.1 File Edit Permission

Appears for every file edit (Edit tool) and file create (Write tool).

```
 Do you want to make this edit to test.js?
 ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No

 Esc to cancel · Tab to amend
```

Options:
| Option | Behavior |
|--------|----------|
| **Yes** | Approve this single edit |
| **Yes, allow all edits during this session (shift+tab)** | Auto-approve all future edits in this session |
| **No** | Reject this edit |
| **Esc** | Cancel (same as No) |
| **Tab** | Opens the edit for amendment (user can modify before approving) |

### 4.2 Bash Command Permission

Appears for Bash commands that access resources outside the working directory or perform sensitive operations.

```
────────────────────────────────────────────────────────────────
 Bash command

   cat /etc/hosts
   Display contents of /etc/hosts

 Do you want to proceed?
 ❯ 1. Yes
   2. Yes, allow reading from etc/ from this project
   3. No

 Esc to cancel · Tab to amend · ctrl+e to explain
────────────────────────────────────────────────────────────────
```

Key differences from file edit permissions:
- **Header** says "Bash command" instead of "Edit file"
- Shows the **command** and a **description** of what it does
- The "always allow" option is **context-specific**: "Yes, allow reading from etc/ from this project"
- Has an additional `ctrl+e to explain` option to get more details

Note: Simple read-only bash commands within the working directory (like `echo`, `date`) run WITHOUT any permission prompt.

### 4.3 Auto-Approve Mode

After selecting "Yes, allow all edits during this session":
- The bottom status bar shows: `⏵⏵ accept edits on (shift+tab to cycle)`
- All subsequent file edits are auto-approved without a permission prompt
- The `shift+tab` key cycles through approval modes
- Tool calls still appear in the conversation but without the dialog

### 4.4 Destructive Operation Prompts

When Claude interprets a request as destructive (e.g., file deletion), it may present its own confirmation dialog BEFORE using a tool:

```
────────────────────────────────────────────────────────────────
 ☐ Confirm

Do you want to permanently delete /tmp/claude-ux-test/config.js?

❯ 1. Yes, delete it
     Permanently remove config.js
  2. No, keep it
     Cancel the deletion
  3. Type something.
────────────────────────────────────────────────────────────────
  4. Chat about this

Enter to select · ↑/↓ to navigate · Esc to cancel
```

This is a Claude-generated question (not a tool permission), with:
- A checkbox icon (`☐`)
- Multiple options with descriptions
- "Type something" option for custom response
- "Chat about this" option to discuss instead of acting

After the user responds, the result is logged as:
```
⏺ User answered Claude's questions:
  ⎿  · Do you want to permanently delete /tmp/claude-ux-test/config.js? → No, keep it
```

---

## 5. Bash Command Display

### 5.1 Command + Output Layout

```
⏺ Bash(echo 'hello from bash' && date && uname -a)
  ⎿  hello from bash
     Sat Mar 14 21:04:57 EDT 2026
     Darwin Mac 24.1.0 Darwin Kernel Version 24.1.0: ...
```

- Tool name: `Bash` (capitalized)
- Command shown in parentheses after tool name
- Output indented beneath the `⎿` tree connector
- Multiple lines of output use consistent indentation

### 5.2 Long Output Truncation

In expanded view, long bash output is truncated:
```
⏺ Bash(cat /etc/hosts)
  ⎿  ##
     # Host Database
     #
     ... +6 lines (ctrl+o to expand)
```

The `+N lines` count tells the user how many lines are hidden.

### 5.3 Permission Tiers

Bash commands follow a tiered permission model:
1. **No permission needed**: Simple read-only commands within the working directory (`echo`, `date`, `uname`)
2. **Permission required**: Commands accessing resources outside the working directory (`cat /etc/hosts`)
3. **Destructive commands**: Claude itself may refuse or ask for confirmation before even calling the tool

---

## 6. Search Results Display

### 6.1 Grep Tool

Collapsed view:
```
⏺ Searched for 1 pattern (ctrl+o to expand)
```

Expanded view:
```
⏺ Search(pattern: "return", path: "/private/tmp/claude-ux-test")
  ⎿  Found 4 lines (ctrl+o to expand)
```

### 6.2 Glob Tool

Collapsed view:
```
⏺ Searched for 1 pattern (ctrl+o to expand)
```

Expanded view:
```
⏺ Search(pattern: "*.js", path: "/private/tmp/claude-ux-test")
  ⎿  Found 3 files (ctrl+o to expand)
```

Both use `Search()` as the tool name in expanded view, differentiated only by parameters.

### 6.3 Key Observation

Search results (both Grep and Glob) are NOT shown directly in the tool call output in collapsed mode. Instead, Claude summarizes the results in its response text below. The raw results are only visible in the expanded detailed transcript view, and even there they are often truncated.

---

## 7. Progress Indicators

### 7.1 Thinking/Processing Spinner

While Claude is processing (thinking, waiting for API response), a spinner appears with fun rotating status messages:

```
✶ Vibing...
```
```
· Frolicking...
```

Observed spinner messages:
- `Vibing...` (with sparkle icon `✶`)
- `Frolicking...` (with dot icon `·`)

The icon rotates between different characters to create animation.

### 7.2 Tool In-Progress State

When a tool is actively executing (before completion), it shows:

```
  Reading 1 file... (ctrl+o to expand)
  ⎿  config.js
```

Note the trailing ellipsis (`...`) indicating active operation, and the partial results appearing beneath as they load.

### 7.3 No Progress Bars

There are no progress bars for any operations. The UI uses only:
- Rotating spinner text for thinking
- Incremental display of results as they arrive
- Collapsed summaries after completion

---

## 8. Visual Hierarchy and Layout

### 8.1 Message Spacing

Each element is separated by blank lines:
```
❯ [User message]
                        ← blank line
⏺ [Tool call]
  ⎿  [Tool result]
                        ← blank line
⏺ [Response text]
                        ← blank line
❯ [Next user message]
```

### 8.2 Indentation Rules

- **User messages** (`❯`): No indentation, flush left
- **Tool calls** (`⏺`): No indentation, flush left
- **Tool results** (`⎿`): Indented 2 spaces under the tool call
- **Tool result content**: Indented 4-6 spaces under the `⎿`
- **Response text**: Indented 2 spaces (aligned with tool content)
- **Diff lines**: Indented 6 spaces with line numbers right-aligned

### 8.3 Icons and Symbols

| Symbol | Usage |
|--------|-------|
| `❯` | User prompt / input indicator |
| `⏺` | Tool call or response paragraph marker (filled circle) |
| `⎿` | Tree connector (results beneath parent) |
| `✶` | Thinking spinner (sparkle variant) |
| `·` | Thinking spinner (dot variant) |
| `⏵⏵` | Auto-approve mode indicator (double play) |
| `☐` | Confirmation dialog checkbox |
| `+` | Diff: added line |
| `-` | Diff: removed line |
| `╌` | Diff: separator line (dashed) |
| `─` | Section separator (solid) |

### 8.4 Status Bar (Bottom)

The bottom of the screen shows a persistent status bar with:
```
  dennisonbertram@Mac [21:10:37] [/private/tmp/claude-ux-test]    1 MCP server failed · /mcp
  ⏵⏵ accept edits on (shift+tab to cycle)
```

Components:
- **Username@hostname** + **timestamp** + **working directory**
- **MCP status**: e.g., "1 MCP server failed"
- **Auto-approve mode**: "accept edits on (shift+tab to cycle)"
- **Input hints**: "ctrl+g to edit in nano"

### 8.5 Welcome Screen

```
╭─── Claude Code v2.1.55 ────────────────────────────────────╮
│                           │ Tips for getting started        │
│    Welcome back Dennison! │ Run /init to create CLAUDE.md   │
│                           │ ────────────────────────────    │
│                           │ Recent activity                 │
│          ▐▛███▜▌          │ No recent activity              │
│         ▝▜█████▛▘         │                                 │
│           ▘▘ ▝▝           │                                 │
│    Opus 4.6 · Claude Max  │                                 │
│  /private/tmp/claude-test │                                 │
╰────────────────────────────────────────────────────────────╯
```

Contains:
- Version number top-left
- ASCII art logo (abstract shape)
- Model name and subscription tier
- Working directory
- Tips and recent activity on the right side
- All wrapped in a box-drawing character border

### 8.6 Exit Messages

```
❯ /exit
  ⎿  Catch you later!
```

After exit, a resume command is shown:
```
Resume this session with:
claude --resume 9ee69993-895f-4a6b-bbce-2726e5ebd21e
```

---

## 9. Multi-File Operations

### 9.1 Sequential Edit Queue

When Claude needs to edit multiple files, it queues them and presents each one for approval:

```
⏺ Update(config.js)    ← Shows first
                         ← Permission dialog appears
                         ← User approves

⏺ Update(test.js)      ← Shows second
                         ← Permission dialog appears
                         ← User approves

⏺ Update(utils.js)     ← Shows third
                         ← Permission dialog appears
                         ← User approves
```

If the user selects "Yes, allow all edits during this session", the remaining edits are auto-approved and the entire sequence completes without further prompts.

### 9.2 Multi-File Read

Multiple reads are batched and shown as a single collapsed entry:
```
⏺ Read 3 files (ctrl+o to expand)
```

In expanded view, each read is shown separately:
```
⏺ Read(config.js)
  ⎿  Read 2 lines

⏺ Read(test.js)
  ⎿  Read 3 lines

⏺ Read(utils.js)
  ⎿  Read 3 lines
```

---

## 10. Slash Commands and Autocomplete

### 10.1 Command Picker

Typing `/` in the input area triggers an autocomplete dropdown:
```
❯ /exit
  /exit                    Exit the REPL
  /extra-usage             Configure extra usage...
  /context                 Visualize current context usage...
  /video-editor            Edit videos...
  /steal-react-component   Extract and reconstruct...
  /lgrep-search            Local semantic code search...
```

Each entry shows:
- **Command name** (left-aligned)
- **Description** (right-aligned, truncated with `...` if too long)

### 10.2 Input Controls

At the input prompt:
- `Enter` -- submit single-line message
- `Shift+Enter` -- newline in multi-line input
- `Ctrl+G` -- open message in nano editor
- `Ctrl+C` -- cancel current operation
- `Esc` -- cancel/dismiss dialogs

---

## 11. Implementation Implications for Tauri Desktop App

### 11.1 Critical Components to Build

1. **Diff viewer component** -- unified diff with line numbers, `+`/`-` prefixes, green/red coloring, dashed borders
2. **Permission dialog component** -- modal with Yes/Allow All/No options, keyboard shortcuts
3. **Tool call display component** -- collapsible with two modes (summary vs detailed)
4. **Thinking spinner** -- animated text with rotating fun messages
5. **Tree connector layout** -- `⎿` visual hierarchy for nesting results under tool calls
6. **Status bar** -- persistent bottom bar with session info, auto-approve state, timestamps

### 11.2 Diff Display Mapping

| CLI Element | GUI Equivalent |
|-------------|---------------|
| `+` prefix | Green highlighted line with `+` gutter icon |
| `-` prefix | Red highlighted line with `-` gutter icon |
| Line numbers | Gutter column (dual: old/new for edits) |
| `╌╌╌` borders | CSS border or divider component |
| "Edit file" / "Create file" header | Card header with filename |

### 11.3 Collapsible Tool Calls

The collapsed/expanded toggle (`Ctrl+O`) maps to a disclosure triangle or accordion component in the GUI. Key design:
- Default to collapsed (clean, scannable)
- Click to expand (shows parameters, timestamps, raw results)
- Keyboard shortcut for power users

### 11.4 Permission Flow

The permission dialog should be:
- **Modal** -- blocks further interaction until resolved
- **Full-width** -- shows the complete diff for context
- **Keyboard-first** -- Enter to approve, Esc to cancel, number keys for options
- **Persistent state** -- "allow all edits" mode persists for the session with visual indicator

### 11.5 Auto-Approve Mode

The `⏵⏵ accept edits on` indicator suggests a toggle-able mode that should be:
- Visible in the status bar at all times when active
- Cyclable between modes (shift+tab equivalent: click or keyboard shortcut)
- Modes: ask for each edit, allow all edits, maybe allow specific tool categories

---

## 12. Raw Captures Reference

All raw terminal captures were taken from a live Claude Code v2.1.55 session. The key captures documented above show the actual character-by-character rendering of each UI element, including Unicode box-drawing characters, tree connectors, and diff formatting.
