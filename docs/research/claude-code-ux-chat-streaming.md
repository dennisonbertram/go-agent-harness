# Claude Code CLI: Chat Interaction & Streaming UX Research

Research conducted on Claude Code v2.1.55, Opus 4.6, Claude Max plan.
Captured via tmux session with `capture-pane` on macOS.

---

## 1. Startup Screen

When Claude Code launches, it displays a branded banner followed by the input area.

### ASCII Capture

```
 ▐▛███▜▌   Claude Code v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/Develop/claude-tauri-boilerplate

────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
  dennisonbertram@Mac [21:06:00] [~/Develop/claude-tauri-boilerplate]  [main *] *]  Claude in Chrome enabled · /chrome
```

### Styling Details

| Element | ANSI Code | Visual |
|---------|-----------|--------|
| Logo blocks | `[91m` (bright red) with `[40m` (black bg) for filled blocks | Red diamond/block pattern |
| "Claude Code" | `[1m` (bold) | Bold white |
| Version | `[37m` (white) | `v2.1.55` |
| Model info | `[37m` (white) | `Opus 4.6 · Claude Max` |
| Working dir | `[37m` (white) | `~/Develop/...` |

### Banner Layout

```
 ▐▛███▜▌   [Bold]Claude Code[/Bold] v2.1.55
▝▜█████▛▘  Opus 4.6 · Claude Max
  ▘▘ ▝▝    ~/working/directory
```

The logo is a stylized diamond/gem shape made of Unicode block characters. It appears in bright red (`[91m`), with the filled center blocks having a black background (`[40m`).

---

## 2. Input Area

### Layout Structure

The input area is sandwiched between two horizontal separator lines:

```
────────────────────────────────────────────────────────────────────────
❯ [cursor]
────────────────────────────────────────────────────────────────────────
  [status bar content]
```

### Separator Lines

- Character: `─` (box-drawing horizontal line, U+2500)
- Styling: `[2m[37m` (dim + white)
- Full terminal width (fills the entire row)

### Prompt Character

- Symbol: `❯` (U+276F, heavy right-pointing angle quotation mark)
- No special color on the prompt character itself in the input area
- Followed by a space, then the cursor

### Cursor

- Rendered as a reverse-video space: `[7m [0m` (white block cursor)
- Positioned after the `❯ ` prompt prefix

### Text Input Behavior

- Text appears inline after `❯ ` as you type
- Long text wraps to the next line with 2-space indent for continuation lines:

```
────────────────────────────────────────────────────────────────────────
❯ Show me a Python hello world with explanation. Include a code block,
  a header, a bulleted list, bold text, and inline code.
────────────────────────────────────────────────────────────────────────
```

### Multi-line Input

- Pressing `Enter` in the input area creates a newline (does NOT submit)
- Pressing `Enter` on an empty line submits the message
- This means the submission gesture is: type message, then press Enter twice (once for newline, once on empty line to submit)
- `Shift+Enter` appears to submit immediately (based on testing, it may behave differently depending on terminal emulator)

### Keyboard Shortcuts

- `Ctrl+U`: Clear the current input line
- `Ctrl+G`: Opens the input in `nano` for multi-line editing (hint appears in bottom right: `ctrl+g to edit in nano`)
- `Escape`: When a response is streaming, interrupts the generation

### No Placeholder Text

The input area shows no placeholder/ghost text. It is simply `❯ ` with a cursor. There is no "Type a message..." or similar hint.

---

## 3. Status Bar

The status bar sits below the bottom separator line and contains contextual information.

### Layout

```
  dennisonbertram@Mac [21:05:47] [~/Develop/claude-tauri-boilerplate]  [main *] *]  Claude in Chrome enabled · /chrome
```

### Elements (left to right)

| Element | ANSI Code | Description |
|---------|-----------|-------------|
| Username | `[1m[32m` (bold green) | `dennisonbertram@Mac` |
| Timestamp | `[34m` (blue) | `[21:05:47]` in brackets |
| Working dir | `[37m` (white) | `[~/Develop/...]` in brackets |
| Git branch | `[32m` (green) | ` [main` with branch icon |
| Dirty indicator | `[31m` (red) | `*` if uncommitted changes |
| Right-aligned info | `[91m` (bright red) for errors | MCP status, tips |

### Right-side Status Messages

The right side of the status bar shows contextual info:
- MCP server status: `4 MCP servers failed · /mcp` (in red)
- Feature notifications: `Claude in Chrome enabled · /chrome`
- Keyboard hints: `ctrl+g to edit in nano`

---

## 4. User Messages (After Sending)

Once submitted, the user message is displayed with a distinct style.

### ASCII Capture

```
❯ What is 2+2? Answer briefly.
```

### Styling Details

- **Prefix**: `❯ ` (same angle bracket as input)
- **Background**: Full-width gray background (`[100m` = bright black/dark gray bg)
- **Text color**: `[97m` (bright white) for the message text
- **Prompt color**: `[37m` (regular white) for the `❯` on gray bg
- Long messages wrap with 2-space indent, entire block has the gray background:

```
[gray bg] ❯ Show me a Python hello world with explanation. Include a code block, a header, a bulleted list, [/bg]
[gray bg]   bold text, and inline code.                                                                      [/bg]
[gray bg]                                                                                                    [/bg]
```

Note the trailing blank line with gray background -- user message blocks include a blank line at the bottom.

### Key Visual Distinction

User messages are clearly distinguished from AI responses by:
1. The `❯` prefix (vs `⏺` for AI)
2. Gray background spanning the full terminal width
3. Bright white text on gray

---

## 5. Loading / Thinking Indicator

When Claude is processing a request (before text starts streaming), a spinner animation appears.

### ASCII Captures (sequential)

```
✶ Twisting…
  ⎿  Tip: Run Claude Code locally or remotely using the Claude desktop app: clau.de/desktop
```

```
· Twisting…
  ⎿  Tip: Run Claude Code locally or remotely using the Claude desktop app: clau.de/desktop
```

```
✻ Twisting…
  ⎿  Tip: Run Claude Code locally or remotely using the Claude desktop app: clau.de/desktop
```

### Spinner Characters

The spinner cycles through various Unicode star/dot characters:
- `✶` (six-pointed star)
- `·` (middle dot)
- `✻` (teardrop-spoked asterisk)
- `✽` (heavy teardrop-spoked asterisk)
- `✳` (eight-spoked asterisk)
- `✢` (four-teardrop-spoked asterisk)

These appear to cycle through a predefined set at a fixed interval (roughly every 100-200ms based on observation).

### Loading Verb

Each request gets a whimsical/creative loading verb that stays constant throughout the loading phase:
- "Twisting..."
- "Leavening..."
- "Drizzling..."
- "Sauteing..."
- "Channeling..."

These appear to be randomly selected from a pool of creative verbs.

### Tip Display

Below the spinner, a tip is displayed:
```
  ⎿  Tip: [tip text]
```
- Uses `⎿` (left square bracket lower corner, U+239F) as a continuation/tree branch character
- Tips are contextual/random helpful suggestions
- Examples observed:
  - "Tip: Name your conversations with /rename to find them easily in /resume later"
  - "Tip: Run Claude Code locally or remotely using the Claude desktop app: clau.de/desktop"

---

## 6. AI Response Rendering

### Response Prefix

Every AI response block starts with:
```
⏺ [response text]
```

- **Symbol**: `⏺` (U+23FA, black circle for record)
- **Color**: `[97m` (bright white) for text responses
- **Color**: `[92m` (bright green) for tool use indicators

### Text Streaming Behavior

Based on tmux captures at different time intervals, the response appears to render as a complete block once available, rather than showing character-by-character streaming in the visible area. The TUI (built with Ink/React for terminals) re-renders the entire response view on each update.

Key observations:
- During the loading phase (spinner), no text is visible
- Text appears as a block once the first tokens arrive
- The view scrolls down as more content streams in
- The response grows from top to bottom as tokens arrive
- There is no visible cursor or typing animation in the response area

### Response Layout

Responses are indented 2 spaces from the left margin (after the `⏺` prefix on the first line):

```
⏺ First line of response
  Continuation lines are indented 2 spaces
  to align with the text after the prefix
```

---

## 7. Markdown Rendering

Claude Code renders markdown with terminal-native styling (ANSI escape codes), not as raw markdown text.

### Headers

#### Top-level Section Headers (H1 equivalent)

```
⏺ TCP vs UDP
```
- Styling: `[1m` (bold only)
- No special indentation, appears on the `⏺` line

#### Subsection Headers (H2/H3 equivalent)

When rendered as part of response body:
```
  Connection Model
```
- Styling: `[1m` (bold)
- Indented 2 spaces (aligned with body text)

#### Response Title Headers

The first line after `⏺` can be a styled title:
```
⏺ Python Hello World
```
- Styling: `[1;3;4m` (bold + italic + underline)
- This appears to be for the main title/header of the response

### Code Blocks

Code blocks are rendered inline with syntax highlighting but WITHOUT box borders:

**Python example:**
```
  print("Hello, World!")
```
With ANSI codes:
- `print` → `[36m` (cyan) -- function/builtin
- `"Hello, World!"` → `[31m` (red) -- string literal

**JavaScript example:**
```
  function reverseString(str) {
    return str.split("").reverse().join("");
  }
```
With ANSI codes:
- `function` → `[34m` (blue) -- keyword
- `reverseString(str)` → `[33m` (yellow) -- function name
- `""` → `[31m` (red) -- string literal
- `return` → `[34m` (blue) -- keyword

### Key observation about code blocks

- **No border or background**: Code blocks do NOT have a visible border, box, or distinct background color in the terminal
- **No language label**: No label showing "python" or "javascript" above the code
- **Syntax highlighting colors follow a consistent scheme**:
  - Keywords: blue (`[34m`)
  - Function/builtin names: cyan (`[36m`)
  - Named functions: yellow (`[33m`)
  - Strings: red (`[31m`)
  - Regular code: default color
- **Indentation**: Code blocks are indented 2 spaces from left margin (same as body text)

### Inline Code

```
  Run it with python hello.py and you'll see Hello, World! printed
```
- Styling: `[94m` (bright blue) for inline code references
- No backtick rendering, no background color
- Examples: `print()`, `send()`, `recv()`, `python hello.py`

### Bold Text

```
  TCP is connection-oriented
```
- Styling: `[1m` (bold)
- No color change, just weight

### Italic Text

Not observed standalone. When combined with bold for headers: `[1;3;4m` (bold+italic+underline).

### Lists (Bulleted)

```
  - print() is a built-in function that outputs text to the console
  - The string "Hello, World!" is passed as an argument to the function
```
- Prefix: `- ` (dash + space)
- Indented 2 spaces from left margin
- No special color on the bullet character
- List items can contain inline code (bright blue) and bold text

### Tables

Tables render using Unicode box-drawing characters:

```
  ┌────────────────────┬──────────────────────┬─────────────────┐
  │      Feature       │         TCP          │       UDP       │
  ├────────────────────┼──────────────────────┼─────────────────┤
  │ Connection         │ Required (handshake) │ None            │
  ├────────────────────┼──────────────────────┼─────────────────┤
  │ Reliability        │ Guaranteed delivery  │ Best-effort     │
  └────────────────────┴──────────────────────┴─────────────────┘
```

- Uses single-line box-drawing characters: `┌ ─ ┬ ┐ │ ├ ┼ ┤ └ ┴ ┘`
- No special color on table borders (default terminal color)
- Content is padded within cells
- Headers appear to have the same styling as data cells (no bold header row observed)

---

## 8. Tool Use Display

When Claude uses tools (e.g., reading files), the tool invocation is displayed as a collapsible summary.

### ASCII Capture

```
⏺ Read 1 file (ctrl+o to expand)

⏺ CLAUDE.md
  apps/
  docs/
  ...
```

### Styling Details

- **Tool action prefix**: `[92m⏺[39m` -- bright GREEN circle (distinct from the bright WHITE circle for text responses)
- **Tool description**: "Read **1** file" -- with the count in bold `[1m`
- **Expand hint**: `(ctrl+o to expand)` in gray/dim text `[37m]`
- **Result**: Appears as a separate `⏺` block below (bright white prefix)

### Tool Use vs Text Response Visual Distinction

| Type | Prefix Color | Meaning |
|------|-------------|---------|
| `[97m⏺[39m` (bright white) | Text response content |
| `[92m⏺[39m` (bright green) | Tool use / action taken |

### Collapsed by Default

Tool use details are collapsed by default. The user sees only the summary (e.g., "Read 1 file"). Pressing `Ctrl+O` expands to show the full tool input/output.

---

## 9. Conversation Flow

### Message Sequence

Messages appear in chronological order, scrolling the terminal:

```
❯ [user message 1]                      <- gray background

⏺ [AI response 1]                       <- no background

❯ [user message 2]                      <- gray background

⏺ [AI response 2]                       <- no background
```

### Visual Separators

- **No explicit separator lines between messages**: Messages are separated by whitespace (blank lines)
- **User messages** have a blank line after the gray background block
- **AI responses** end with a blank line before the next element
- The only horizontal line separators are around the **input area** (top and bottom of the input box)

### Scrolling

- The conversation scrolls naturally as content grows
- Older messages scroll up and remain in the terminal scrollback buffer
- The input area stays fixed at the bottom of the screen
- The status bar stays fixed below the input area

### Spacing Pattern

```
❯ [user msg]                             <- 1 blank line of gray bg after text
[blank line]                              <- separation
⏺ [response title]                      <- response block
[blank line]                              <- within response for paragraph breaks
  [response body]                         <- indented content
[blank line]                              <- separation before next message
❯ [next user msg]                        <- next user message
```

---

## 10. Interruption UX

### Escape to Interrupt

Pressing `Escape` during response generation interrupts it immediately.

### Interrupted State Display

```
❯ Write a detailed essay about the history of the internet...
  ⎿  Interrupted · What should Claude do instead?
```

- The `⎿` tree-branch character connects the interruption notice to the user message
- Text: "Interrupted" followed by " · What should Claude do instead?"
- Styling: `[37m` (white) for the interruption text
- The input area becomes active again for the user to provide new direction
- No partial response is shown -- the response is fully discarded on interrupt

---

## 11. Exit / Session End

### `/exit` Command

```
❯ /exit
  ⎿  Goodbye!
```

### Post-Exit Display

After the Claude TUI exits, the terminal shows:

```
Resume this session with:
claude --resume 6fea0837-3902-4e2c-98f7-6f9d6a961c7d
```

- Resume text is in dim styling: `[2m` (dim/faint)
- The session UUID is included for resumption
- The shell prompt returns to normal after this

---

## 12. Slash Command Autocomplete

When typing `/` in the input area, a dropdown autocomplete appears.

### ASCII Capture

```
────────────────────────────────────────────────────────────────────────
❯ /cost
  /cost
────────────────────────────────────────────────────────────────────────
```

### Behavior

- Typing `/` triggers a dropdown/popup showing matching commands
- The selected/matching command appears highlighted below the input: `[94m/cost[7m[39m` (bright blue + reverse video for selection)
- Commands shown include: `/cost`, `/exit`, `/context`, `/resume`, `/extra-usage`, etc.
- Pressing `Enter` selects the highlighted command
- Pressing `Escape` dismisses the autocomplete

### Autocomplete Styling

- Input text with `/`: `[94m` (bright blue) -- the slash prefix gets colored
- Dropdown items: shown as a list below the input
- Selected item: `[7m` (reverse video) highlight

---

## 13. Cost / Token Display

### In-Session

There is **no persistent token count or cost display** visible in the default Claude Code UI. The status bar does not show token usage, cost, or rate limit information.

### The `/cost` Command

The `/cost` slash command is a built-in CLI command. When invoked, it is supposed to display token counts and costs directly in the CLI output. However, in our testing with Claude Max (subscription plan), the AI model itself does not have access to this data -- it is handled by the CLI infrastructure.

---

## 14. MCP Server Status

MCP (Model Context Protocol) server connection status is shown in the status bar.

### Success

```
  Claude in Chrome enabled · /chrome
```

### Failure

```
  4 MCP servers failed · /mcp
```
- Failure count in red: `[91m` (bright red)
- Slash command hint for more info

---

## 15. Color Palette Summary

| Purpose | ANSI Code | Color | Hex Approximate |
|---------|-----------|-------|-----------------|
| Logo | `[91m` | Bright red | #FF5555 |
| User message bg | `[100m` | Bright black (dark gray) bg | #555555 |
| User message text | `[97m` | Bright white | #FFFFFF |
| AI response prefix | `[97m` (text) / `[92m` (tool) | Bright white / Bright green | #FFFFFF / #55FF55 |
| Headers (title) | `[1;3;4m` | Bold + italic + underline | (weight/style, not color) |
| Headers (section) | `[1m` | Bold | (weight only) |
| Code: keywords | `[34m` | Blue | #5555FF |
| Code: builtins | `[36m` | Cyan | #55FFFF |
| Code: function names | `[33m` | Yellow | #FFFF55 |
| Code: strings | `[31m` | Red | #FF5555 |
| Inline code | `[94m` | Bright blue | #5555FF |
| Bold text | `[1m` | Bold | (weight only) |
| Separators | `[2m[37m` | Dim white | #888888 |
| Status: username | `[1m[32m` | Bold green | #55FF55 |
| Status: time | `[34m` | Blue | #5555FF |
| Status: path | `[37m` | White | #AAAAAA |
| Status: git branch | `[32m` | Green | #55FF55 |
| Status: dirty | `[31m` | Red | #FF5555 |
| Status: errors | `[91m` | Bright red | #FF5555 |
| Spinner | (no specific color) | Default | -- |
| Loading verb | (no specific color) | Default | -- |
| Dim/secondary text | `[2m` | Dim/faint | (50% opacity) |
| Interrupt text | `[37m` | White | #AAAAAA |

---

## 16. Implications for Desktop GUI Implementation

### Must-Have UX Elements

1. **Distinct user/AI message styling**: Gray background for user, no background for AI. Different prefix icons.
2. **Streaming response**: Show a spinner/loading indicator while waiting, then stream text as it arrives.
3. **Markdown rendering**: Render headers (bold/underline), code blocks (syntax-highlighted), inline code, bold, lists, and tables.
4. **Tool use collapse**: Show tool invocations as collapsible summaries with expand/collapse toggle.
5. **Interrupt capability**: Allow user to press Escape (or click a button) to stop generation mid-stream.
6. **Input area**: Fixed at bottom, supports multi-line input, `Enter` to send.
7. **Status bar**: Show model info, git status, MCP status.
8. **Session resume**: Provide a way to resume previous sessions.

### Differences for Desktop GUI

- **No need for ANSI codes**: Use CSS for all styling (colors, bold, italic, underline).
- **Code blocks can have visible borders**: The CLI has no borders, but a GUI should use bordered/shadowed code blocks with language labels and copy buttons.
- **Richer tables**: GUI can render proper HTML tables with borders and alternating row colors.
- **Syntax highlighting**: Use a proper syntax highlighting library (e.g., Prism, Shiki) instead of ANSI-approximated colors.
- **Scroll behavior**: Implement smooth scrolling with auto-scroll-to-bottom during streaming, but allow user scroll-back without fighting auto-scroll.
- **Loading animation**: Can be more sophisticated than cycling Unicode chars -- consider a pulsing dot, animated gradient, or skeleton screen.
- **Token/cost display**: Could be shown persistently in the header or status bar if the SDK provides the data.

### Character/Symbol Reference

| CLI Symbol | Unicode | Purpose | GUI Equivalent |
|-----------|---------|---------|----------------|
| `❯` | U+276F | User message prefix | User avatar or colored label |
| `⏺` | U+23FA | AI response prefix | AI avatar or bot icon |
| `⎿` | U+239F | Tree branch / continuation | Indented nested block |
| `─` | U+2500 | Horizontal separator | CSS border or hr |
| `✶✻✽✳✢·` | Various | Spinner animation | CSS spinner or loading dots |
| `┌─┬┐│├┼┤└┴┘` | Box-drawing | Table borders | HTML table |

---

## 17. Raw Escape Code Reference

### User Message Block
```
[37m[100m❯ [97mMessage text here                                          [39m[49m
[97m[100m                                                              [39m[49m
```

### AI Response (Text)
```
[97m⏺[39m Response text here
```

### AI Response (Tool Use)
```
[92m⏺[39m Read [1m1[0m file [37m(ctrl+o to expand)[39m
```

### Section Header
```
[1mHeader Text[0m
```

### Response Title
```
[97m⏺[39m [1;3;4mTitle Text[0m
```

### Code Block (JavaScript)
```
[34mfunction[33m name(args) [39m{
    [34mreturn[39m value.method([31m"string"[39m);
  }
```

### Inline Code
```
Use [94mcommandName[39m to do something
```

### Separator Line
```
[2m[37m────────────────────────────────────────────────[0m
```

### Input Prompt with Cursor
```
❯ [7m [0m
```

### Loading Spinner
```
✶ Leavening…
  ⎿  Tip: [tip text]
```

### Interrupt Message
```
  ⎿  [37mInterrupted · What should Claude do instead?[39m
```

### Exit and Resume
```
[37m[100m❯ [97m/exit [39m[49m
[97m  ⎿  [39mGoodbye!

[2mResume this session with:[0m
[2mclaude --resume [session-uuid][0m
```
