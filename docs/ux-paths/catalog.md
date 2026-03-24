# UX Path Catalog: go-agent-harness CLI/TUI

Generated: 2026-03-23
Total Stories: 136
Coverage: 16 / 16 feature areas (100%)

## Summary

| Type | Count |
|------|-------|
| Short | 68 |
| Medium | 48 |
| Long | 20 |

## Coverage Matrix

| Feature Area | Stories | Gaps |
|-------------|---------|------|
| First Launch & Chat | 12 | None |
| Tool Execution Flow | 12 | None |
| Permission & Safety Controls | 12 | None |
| Model & Provider Selection | 12 | None |
| Profile Selection & Isolation | 12 | None |
| Conversation Management | 12 | None |
| Planning Mode (Extended Thinking) | 10 | None |
| Cost & Context Awareness | 10 | None |
| Slash Commands & Autocomplete | 10 | None |
| Error Recovery & Interrupts | 12 | None |
| Keyboard-Driven Navigation | 12 | None |
| Session Resumption & Export | 10 | None |

## Story Dependency Graph

STORY-001 (Minimal First Launch)
├── STORY-002 (Sending the First Message)
│   ├── STORY-003 (Watching Streamed Response)
│   │   └── STORY-004 (Run Completes — Cost Update)
│   │       └── STORY-005 (Multi-Turn ConversationID Linkage)
│   │           ├── STORY-006 (Typing /help on First Launch)
│   │           ├── STORY-007 (Long First Session)
│   │           └── STORY-061 (Establishing Multi-Turn Identity)
│   └── STORY-008 (First Message When Server Unreachable)
├── STORY-009 (Multi-Line Input)
├── STORY-010 (Input History Navigation)
├── STORY-011 (Status Bar Transient Message)
└── STORY-012 (Cold Start with Persisted API Keys)

STORY-013 (Tool Call Appear and Resolve) — requires STORY-002
├── STORY-014 (Streaming Output Tail Replace)
├── STORY-015 (Expanding Tool Call ctrl+o)
├── STORY-016 (Tool Call Error in Red)
├── STORY-017 (Elapsed Timer Slow Tool)
├── STORY-018 (Diff Tool Syntax Highlight)
├── STORY-019 (Sequential Tool Calls)
├── STORY-020 (Nested Tool Calls Subagent)
├── STORY-021 (File Operation Summary Line)
├── STORY-022 (Interrupt During Tool Call)
├── STORY-023 (Reviewing Completed Tool Calls)
└── STORY-024 (Permission Prompt Blocking Tool) — links to STORY-025+

STORY-025 (Approving a Single File Write) — requires STORY-013
├── STORY-026 (Denying a Bash Command)
├── STORY-027 (Granting Session-Wide Permission)
├── STORY-028 (Amending Resource Path)
├── STORY-029 (Ctrl+U to Clear Amend Input)
├── STORY-030 (Permission vs Interrupt Banner)
├── STORY-031 (Approval Blocks All Input)
├── STORY-032 (Reviewing Session Permissions)
├── STORY-033 (Approval Timeout)
├── STORY-034 (Full-Auto Profile Bypasses Prompts)
├── STORY-035 (Deny Produces Tool Error)
└── STORY-036 (Unknown/Generic Tool Prompt)

STORY-037 (Basic Model Switch) — requires STORY-001
├── STORY-038 (Starring a Model)
├── STORY-039 (Cross-Provider Search)
├── STORY-040 (Unconfigured Provider → Keys)
├── STORY-041 (Setting/Changing Provider API Key)
├── STORY-042 (Selecting Gateway Direct vs OpenRouter)
├── STORY-043 (Configuring Reasoning Effort)
├── STORY-044 (OpenRouter Expanded Catalog)
├── STORY-045 (Config Panel Keyboard Navigation)
├── STORY-046 (Unstarring a Model)
├── STORY-047 (First-Time Setup No Models)
└── STORY-048 (Esc Safe Dismissal Model Overlay)

STORY-049 (Opening Profile Picker First Time) — requires STORY-001
├── STORY-050 (Selecting Built-in Profile)
├── STORY-051 (Browsing Long Profile List)
├── STORY-052 (Dismissing Picker Without Selecting)
├── STORY-053 (Profile Load Failure)
├── STORY-054 (No Profiles Configured Empty State)
├── STORY-055 (Project-Level Profile Override)
├── STORY-056 (Container-Isolated Profile)
├── STORY-057 (Switching Profiles Mid-Session)
├── STORY-058 (--list-profiles CLI Flag)
├── STORY-059 (Reading Profile Metadata)
└── STORY-060 (Worktree-Isolated Profile)

STORY-061 (Clearing Conversation) — requires STORY-005
├── STORY-062 (Multi-Turn ConversationID)
├── STORY-063 (Exporting Transcript)
├── STORY-064 (Recovering Message with History)
├── STORY-065 (Copying Last Assistant Response)
├── STORY-066 (Export After Multi-Turn)
├── STORY-067 (Clear Between Independent Tasks)
├── STORY-068 (Navigating Long History)
├── STORY-069 (Transcript Accumulation Across Turns)
├── STORY-070 (Export Failure Graceful)
├── STORY-071 (What /clear Does Not Reset)
└── STORY-072 (Draft Abandonment and Recall)

STORY-073 (Toggling Plan Mode) — requires STORY-002
├── STORY-074 (Reviewing Multi-Step Plan)
├── STORY-075 (Rejecting a Plan)
├── STORY-076 (Thinking Bar vs Plan Overlay)
├── STORY-077 (Plan Overlay Blocks Input)
├── STORY-078 (Expanding Tool Call ctrl+o — Planning)
├── STORY-079 (Cancelling Run While Plan Pending)
├── STORY-080 (Plan Overlay Full Lifecycle)
├── STORY-081 (Thinking Bar Without Plan Mode)
└── STORY-082 (Narrow Terminal Plan Overlay)

STORY-083 (Status Bar Cost Counter Real Time) — requires STORY-004
├── STORY-084 (Per-Run vs Session-Total Cost)
├── STORY-085 (Opening Context Grid)
├── STORY-086 (Opening Stats Panel)
├── STORY-087 (Stats to Decide on /clear)
├── STORY-088 (Context Bar Progress Toward Full)
├── STORY-089 (Transient Status Bar Messages)
├── STORY-090 (Comparing Costs Across Time)
├── STORY-091 (First Run Cost Hidden to Visible)
└── STORY-092 (Closing and Reopening Stats)

STORY-093 (/ Opens Autocomplete Dropdown) — requires STORY-001
├── STORY-094 (Filtering Commands by Typing)
├── STORY-095 (Navigating with Arrow Keys)
├── STORY-096 (Accepting with Enter)
├── STORY-097 (Accepting with Tab)
├── STORY-098 (Dismissing with Escape)
├── STORY-099 (Partial Command Tab Completion)
├── STORY-100 (History Up/Down While Autocomplete Open)
├── STORY-101 (Autocomplete Reopens After Editing)
└── STORY-102 (Quitting with /quit)

STORY-103 (Two-Stage Ctrl+C Interrupt) — requires STORY-002
├── STORY-104 (Esc as Single-Press Cancel)
├── STORY-105 (Run Failure via run.failed)
├── STORY-106 (SSE Stream Error Polling)
├── STORY-107 (Tool Call Error in Viewport)
├── STORY-108 (Export Failure Status Bar)
├── STORY-109 (Unknown Slash Command)
├── STORY-110 (Server Unreachable at Launch)
├── STORY-111 (Recovery After Interrupt)
├── STORY-112 (In-Progress Tool Calls at Interrupt)
├── STORY-113 (Ctrl+C No Active Run Quits)
└── STORY-114 (Cascading Esc Priority)

STORY-115 (Paging Through Long Conversation) — requires STORY-005
├── STORY-116 (Line-by-Line Scroll)
├── STORY-117 (Copying Last Assistant Response)
├── STORY-118 (Help Dialog Tab Navigation)
├── STORY-119 (Model Switcher Vim Keys)
├── STORY-120 (Starring Frequently Used Model)
├── STORY-121 (API Keys via Keyboard)
├── STORY-122 (Esc to Unwind Nested Overlay)
├── STORY-123 (Interrupting Active Run Keyboard)
├── STORY-124 (Multi-Line Prompt Without Submitting)
├── STORY-125 (Stats Panel Period Toggle r)
└── STORY-126 (Opening External Editor)

STORY-127 (Resuming Past Session) — requires STORY-005
├── STORY-128 (Empty State Session Picker)
├── STORY-129 (Multi-Turn conversationID Linkage)
├── STORY-130 (Exporting Transcript /export)
├── STORY-131 (Locating Exported File)
├── STORY-132 (Input History Within Session)
├── STORY-133 (Export vs Resumption Concepts)
├── STORY-134 (Long Session List Scroll)
├── STORY-135 (Exporting Empty Transcript)
└── STORY-136 (Aborting Session Picker with Escape)

## All Stories

### First Launch & Chat

## STORY-001: Minimal First Launch — Empty Chat State

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user, first time running the CLI
**Goal**: Verify the TUI opens and is ready to accept input
**Preconditions**: `harnessd` is running at `http://localhost:8080`. `harnesscli` binary is installed. No persisted config file exists.

### Steps
1. User runs `harnesscli --tui` in the terminal → The BubbleTea TUI initializes; terminal is taken over full-screen. The viewport is empty (no conversation history). The input area at the bottom shows the `❯` prompt symbol and a blinking cursor. The status bar at the bottom shows the default model name and `$0.0000`.
2. User reads the status bar → Sees the active model name (e.g. `gpt-4o`) on the left side and `$0.0000` on the right side, confirming no cost has been incurred yet.
3. User reads the viewport → The viewport is blank; no welcome message, no previous turns.
4. User places focus in the input area (it is focused by default) → Cursor is ready. The `❯` symbol is visible to the left of the input field.

### Variations
- With `--model gpt-4-turbo` flag: the status bar shows `gpt-4-turbo` immediately instead of the default model.
- With a persisted config that has a starred model: `Init()` replays pending API keys via a batched `tea.Cmd` before the first render.

### Edge Cases
- `harnessd` not running: the TUI still opens and displays the empty state. The error only surfaces when the user attempts to send a message (POST to `/v1/runs` fails, `RunFailedMsg` rendered in viewport).
- Terminal smaller than minimum dimensions: the layout still renders; `WindowSizeMsg` propagates width/height to all components at startup.

---

## STORY-002: Sending the First Message

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user
**Goal**: Submit a prompt and see a response begin streaming
**Preconditions**: TUI is open, viewport is empty, input area is focused, `harnessd` is running with a configured model.

### Steps
1. User types `Hello, what can you do?` into the input area → Characters appear at the cursor position to the right of the `❯` symbol.
2. User presses `Enter` → Input text is submitted. The input area clears. A POST is made to `/v1/runs` with `{"prompt": "Hello, what can you do?"}` (no `conversation_id` on the first message). A `RunStartedMsg{RunID: "run-abc123"}` is emitted.
3. The `runActive` flag becomes `true` → The input area becomes non-interactive while the run is in flight.
4. The user message bubble appears in the viewport → The `messagebubble` component renders the user turn with role-appropriate styling.
5. The SSE bridge connects to `/v1/runs/run-abc123/events` → The TUI begins receiving events on the channel (capacity 256).

### Variations
- User presses `Shift+Enter` instead of `Enter`: inserts a newline into the input rather than submitting — useful for multi-line prompts.
- Empty input and `Enter`: no submission occurs (the empty string is not posted).

### Edge Cases
- Network timeout on POST to `/v1/runs`: `RunFailedMsg` is received, error text is appended to viewport, `runActive` is cleared.
- Server returns HTTP 4xx/5xx: `RunFailedMsg{Error: "start run: HTTP 503"}` is rendered in the viewport.

---

## STORY-003: Watching the Streamed Assistant Response

**Type**: medium
**Topic**: First Launch & Chat
**Persona**: New user
**Goal**: Understand how text streams into the viewport in real time
**Preconditions**: First message has been submitted, `RunStartedMsg` received, SSE bridge active.

### Steps
1. First `assistant.text.delta` SSE event arrives → The bridge decodes it and sends `AssistantDeltaMsg{Delta: "Sure"}` to the TUI update loop.
2. The model's `responseStarted` flag is `false` → On this first delta, the `messagebubble` component renders a new assistant bubble in the viewport and sets `responseStarted = true`. The text `Sure` appears in the chat.
3. Subsequent `assistant.text.delta` events arrive in rapid succession → Each new `AssistantDeltaMsg` calls `ReplaceTailLines` on the viewport, updating the growing assistant bubble in place rather than appending new lines. The text accumulates smoothly.
4. The viewport auto-scrolls to keep the latest text visible → The user sees the response forming word-by-word without needing to scroll manually.
5. `assistant.text.done` SSE event arrives → The delta accumulation stops. `lastAssistantText` holds the complete response. `responseStarted` is reset to `false`.
6. The final rendered message bubble shows the complete, glamour-rendered markdown response.
7. The `runActive` flag is still `true` (the run has not yet completed; the harness may execute tools after the text response).

### Variations
- Response contains markdown (code blocks, bullet lists): glamour renders them with syntax highlighting and list indentation.
- Response is very long: the viewport scrolls; user can use `pgup`/`pgdn` or `up`/`down` to navigate.

### Edge Cases
- SSE channel backpressure: if the TUI render loop falls more than 256 messages behind, excess events are dropped and `SSEDropMsg` is emitted. The assistant text may skip a few delta characters but will complete correctly on `assistant.text.done`.
- Zero-length delta: no visible change; `AppendChunk` is a no-op for empty strings.

---

## STORY-004: Run Completes — Status Bar Cost Update

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user
**Goal**: See cumulative cost update in the status bar after a run finishes
**Preconditions**: Assistant response has fully streamed; one or more `usage.delta` SSE events have arrived.

### Steps
1. `usage.delta` SSE event arrives during the run (e.g. `{"input_tokens": 120, "output_tokens": 85, "cost_usd": 0.000312}`) → `UsageDeltaMsg` is dispatched to the TUI update loop.
2. `cumulativeCostUSD` is incremented by `0.000312` → The status bar re-renders showing the updated cost (e.g. `$0.0003`).
3. `run.completed` SSE event arrives → `SSEDoneMsg{EventType: "run.completed"}` is sent. `RunCompletedMsg` is emitted. `runActive` is set to `false`. The SSE bridge channel is closed.
4. The input area becomes interactive again → Cursor reappears in the input area. The `❯` prompt symbol is visible and focused.
5. User reads the status bar → Model name unchanged. Cost reflects the total for this run (e.g. `$0.0003`).

### Variations
- Multiple `usage.delta` events per run: each increments `cumulativeCostUSD` additively. The status bar shows the running total.
- Zero-cost run (no usage events): status bar remains at `$0.0000`.

### Edge Cases
- `cost_usd` field missing from `usage.delta` payload: only token counts update; cost stays unchanged.
- `run.failed` instead of `run.completed`: `RunFailedMsg` is rendered in the viewport; cost still reflects whatever `usage.delta` events arrived before failure.

---

## STORY-005: Multi-Turn Conversation — ConversationID Linkage

**Type**: medium
**Topic**: First Launch & Chat
**Persona**: Developer testing multi-turn context retention
**Goal**: Send a follow-up message and confirm it is linked to the same conversation
**Preconditions**: First message completed successfully. `conversationID` is stored as `"run-abc123"` (the first run's ID). Input area is focused.

### Steps
1. User reads the viewport → First user message and first assistant response are visible. Status bar shows cost from first run (e.g. `$0.0003`).
2. User types a follow-up: `Can you give me a code example?` → Characters appear in input area.
3. User presses `Enter` → Input submits. The TUI posts to `/v1/runs` with `{"prompt": "Can you give me a code example?", "conversation_id": "run-abc123"}`. The `conversation_id` field is non-empty because `conversationID` was set by the first run.
4. A new `RunStartedMsg{RunID: "run-def456"}` arrives → `runActive` is set to `true`. `RunID` is updated to `"run-def456"` but `conversationID` remains `"run-abc123"` (it does not change after the first run).
5. The SSE bridge connects to `/v1/runs/run-def456/events` → Second run streams normally.
6. User message bubble and assistant response bubble for the second turn appear in the viewport below the first turn.
7. `run.completed` arrives for the second run → `runActive` cleared. Status bar cost increments again.
8. User reads the viewport → Both turns (two user bubbles, two assistant bubbles) are visible in scroll order.

### Variations
- Third and fourth messages: `conversation_id` remains `"run-abc123"` throughout the session, grouping all turns under one conversation on the server side.
- `/clear` during session: `conversationID` is reset to `""`. The next message starts a new conversation.

### Edge Cases
- Race condition (user sends second message before first run's `RunCompletedMsg`): the input area is non-interactive while `runActive` is `true`, preventing this scenario.

---

## STORY-006: Typing `/help` on First Launch

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user exploring available commands
**Goal**: Discover slash commands and keyboard shortcuts
**Preconditions**: TUI is open, viewport empty, no run active.

### Steps
1. User types `/` into the input area → The slash-command autocomplete dropdown (`slashcomplete.Model`) appears above the input area showing all registered commands: `/clear`, `/context`, `/export`, `/help`, `/keys`, `/model`, `/profiles`, `/quit`, `/stats`, `/subagents`.
2. User types `h` → The dropdown filters to commands matching `h`: only `/help` remains.
3. User presses `Enter` (or `Tab`) → `/help` is accepted. Because it is the only match, it auto-executes. The help dialog overlay (`helpdialog.Model`) opens full-screen.
4. Help dialog shows the **Commands** tab by default → The user reads the list of slash commands with their descriptions.
5. User presses `Tab` or `l` → The **Keybindings** tab becomes active. The user reads the keyboard shortcut list.
6. User presses `Tab` again → The **About** tab becomes active.
7. User presses `Esc` → The help overlay closes. Focus returns to the input area.

### Variations
- User types `/help` in full and presses `Enter`: same result — the command dispatches via `CommandRegistry`.
- User presses `ctrl+h` or `?` without typing: help dialog opens directly (keyboard shortcut path, no dropdown).

### Edge Cases
- Overlay already open when `/help` is typed: the previous overlay closes first before the help dialog opens (mutually exclusive overlay state).

---

## STORY-007: Long First Session — Multi-Turn with Cost Accumulation

**Type**: long
**Topic**: First Launch & Chat
**Persona**: Developer using the TUI for extended work
**Goal**: Conduct a full multi-turn working session and observe cost tracking throughout
**Preconditions**: `harnessd` running, model configured with API key. TUI launched with `harnesscli --tui`.

### Steps
1. TUI opens → Viewport empty. Status bar shows model name and `$0.0000`. Input area focused with `❯` cursor.
2. User types `I need to debug a Go HTTP handler that returns 500 errors.` and presses `Enter` → POST to `/v1/runs` with no `conversation_id`. `RunStartedMsg{RunID: "run-0001"}` received. `conversationID` set to `"run-0001"`.
3. User message bubble renders in viewport.
4. `assistant.text.delta` events stream in → Assistant response appears token-by-token. Viewport auto-scrolls.
5. `usage.delta` arrives → Status bar updates: `$0.0008`.
6. `run.completed` arrives → `runActive` cleared. Input area re-activated.
7. User types `Can you look at the error handling pattern in the standard library?` and presses `Enter` → POST includes `conversation_id: "run-0001"`. New run `"run-0002"` started.
8. Second assistant response streams in → Additional text appended below. Viewport auto-scrolls.
9. `usage.delta` arrives → Status bar: `$0.0019` (cumulative across both runs).
10. `run.completed` for second run → Input area re-activated.
11. User presses `pgup` → Viewport scrolls up. User can read the first response.
12. User presses `pgdn` → Viewport scrolls back to bottom.
13. User types `Show me how to write a test for this.` → Third run started with `conversation_id: "run-0001"`.
14. Third assistant response streams. Code block rendered by glamour with syntax highlighting.
15. `usage.delta` arrives → Status bar: `$0.0031`.
16. `run.completed` for third run.
17. User presses `ctrl+s` → Last assistant response (the test code) is copied to clipboard. Status bar flashes `Copied` for 3 seconds, then reverts.
18. User types `/context` → Context grid overlay opens. Progress bar shows current token usage as a percentage of 200k-token context window (e.g. `3.2%` after three turns).
19. User presses `Esc` → Context grid overlay closes.
20. User types `/stats` → Stats panel overlay opens. Activity heatmap shows today's run count (3) and cumulative cost (`$0.0031`).
21. User presses `r` → Heatmap toggles from "week" view to "month" view.
22. User presses `r` again → Toggles to "year" view.
23. User presses `Esc` → Stats overlay closes.
24. User types a fourth message and continues working.

### Variations
- User opens `/model` mid-session to switch to a different model: the next run uses the new model. `conversationID` is preserved; conversation context on the server side is maintained.
- User opens `/export` mid-session: a timestamped markdown file (e.g. `transcript-20260323-142301.md`) is written; status bar confirms path for 3 seconds.

### Edge Cases
- User fills the 200k-token context window: the harness handles this server-side (context compaction or error). The context grid would show 100% before this occurs, warning the user visually.
- Long viewport (hundreds of lines): `pgup`/`pgdn` navigate by half-screen increments. Scrolling performance is handled by the BubbleTea viewport component.

---

## STORY-008: First Message When Server Is Unreachable

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user whose server is not running
**Goal**: Understand what happens when the backend is absent
**Preconditions**: `harnesscli --tui` launched. `harnessd` is NOT running (no process on port 8080).

### Steps
1. TUI opens normally → Viewport empty. Status bar shows model name and `$0.0000`. No indication of server status at startup.
2. User types `Hello` and presses `Enter` → Input clears. POST to `http://localhost:8080/v1/runs` fails immediately (connection refused).
3. `RunFailedMsg{Error: "start run: Post \"http://localhost:8080/v1/runs\": dial tcp [::1]:8080: connect: connection refused"}` is received → Error text is appended to the viewport as an error line.
4. `runActive` is reset to `false` → Input area re-activates. User can try again.
5. User message bubble may or may not have been rendered (it is appended before the run starts) → The viewport shows the user's message and the error below it.

### Variations
- Server at a non-default URL: user can restart with `harnesscli --tui --base-url http://10.0.1.5:8080`.
- Server running but `/v1/runs` returns 503: `RunFailedMsg{Error: "start run: HTTP 503"}` is appended instead.

### Edge Cases
- Intermittent network: subsequent messages after the server comes back online succeed normally. `conversationID` is still `""` so the next successful message starts a fresh conversation.

---

## STORY-009: Using the `❯` Prompt for Multi-Line Input

**Type**: medium
**Topic**: First Launch & Chat
**Persona**: Developer crafting a detailed multi-line prompt
**Goal**: Submit a prompt that spans multiple lines using Shift+Enter
**Preconditions**: TUI open, no run active, input area focused.

### Steps
1. User types `Please analyze the following code:` → Text appears after `❯`.
2. User presses `Shift+Enter` (or `Ctrl+J`) → A newline is inserted into the input area. The cursor moves to the second line. The input area expands vertically to accommodate the additional line.
3. User types `func main() {` → Second line text appears.
4. User presses `Shift+Enter` again → Third line begins.
5. User types `    fmt.Println("hello")` → Third line text appears with indentation.
6. User presses `Shift+Enter` → Fourth line begins.
7. User types `}` → Fourth line text appears.
8. User presses `Enter` → The entire multi-line text is submitted as a single prompt. The POST body contains the full multi-line string.
9. The user message bubble renders the multi-line code in the viewport using the `messagebubble` component.
10. Assistant response streams in as normal.

### Variations
- User uses `ctrl+e` to open editor mode: the prompt is opened in the user's `$EDITOR` for rich editing, then submitted on save/exit.

### Edge Cases
- Paste of multi-line text via terminal: newlines in the pasted content are preserved in the input area. Only an explicit `Enter` on a single line (not inside a paste) submits.

---

## STORY-010: Input History Navigation

**Type**: medium
**Topic**: First Launch & Chat
**Persona**: Power user repeating or refining previous prompts
**Goal**: Retrieve and re-send a previous prompt using keyboard history navigation
**Preconditions**: At least two messages have been sent in the current session. No run is currently active. Input area is focused and empty.

### Steps
1. Input area is empty, cursor at `❯`. User presses `Up` (or `Ctrl+P`) → The most recent submitted prompt appears in the input area (history navigates backward, up to 100 entries).
2. User reads the prompt → It is the last message sent, e.g. `Can you give me a code example?`.
3. User presses `Up` again → The previous message appears: `Hello, what can you do?`.
4. User presses `Down` (or `Ctrl+N`) → Navigates forward; `Can you give me a code example?` reappears.
5. User edits the recalled text: presses `End` to move cursor to end of line, then types ` in Go` → Input now reads `Can you give me a code example? in Go`.
6. User presses `Enter` → The edited prompt is submitted as a new message. The original history entries are unchanged.
7. The new user message bubble renders in the viewport.

### Variations
- User presses `Up` at the beginning of history (oldest entry): navigation stops; no wraparound.
- User presses `Down` past the newest entry: returns to an empty input (the "present" state).
- User presses `Esc` while a recalled prompt is shown: the `Esc` handler checks for a non-empty input and clears it, showing `"Input cleared"` in the status bar for 3 seconds.

### Edge Cases
- Run becomes active while user is browsing history (impossible — input non-interactive during run).
- History cap (100 entries): entries beyond 100 are dropped from the oldest end.

---

## STORY-011: Status Bar Transient Message on Completion

**Type**: short
**Topic**: First Launch & Chat
**Persona**: New user observing UI feedback
**Goal**: Observe that the status bar surfaces context-relevant feedback without permanently displacing model and cost information
**Preconditions**: TUI open, a run has just completed.

### Steps
1. Run completes successfully → `RunCompletedMsg` received. `runActive` cleared. Input area reactivates.
2. User presses `/export` → The transcript export command runs in the background (`transcriptexport` cmd).
3. `ExportTranscriptMsg{FilePath: "/Users/alice/.harness/exports/transcript-20260323-155204.md"}` arrives → The status bar replaces its normal content with the message `Exported: transcript-20260323-155204.md` for 3 seconds.
4. After 3 seconds (`statusTickMsg` fires) → The status bar reverts to showing the model name and cumulative cost. No data is lost.

### Variations
- User types `/clear` during a session with content: status bar shows `"Conversation cleared"` for 3 seconds, then reverts. The viewport is empty and `conversationID` is reset.
- User presses `Esc` on an empty input: status bar shows `"Input cleared"` for 3 seconds even though nothing was in the input (edge case: the message fires regardless because `Esc` on the no-active-run path always evaluates input state).

### Edge Cases
- Two transient messages in quick succession: the second message's `statusMsgExpiry` supersedes the first. The second message is shown for its full 3 seconds from the point it was set.

---

## STORY-012: Cold Start with Persisted API Keys

**Type**: long
**Topic**: First Launch & Chat
**Persona**: Returning developer who has configured API keys in a previous session
**Goal**: Confirm that persisted keys are replayed at startup and a run can proceed without re-entering credentials
**Preconditions**: A previous session stored API key `groq: "gsk-..."` in `~/.config/harnesscli/config.json`. `harnessd` is running.

### Steps
1. User runs `harnesscli --tui` → `tui.New(cfg)` reads `~/.config/harnesscli/config.json`. `pendingAPIKeys` is populated with `{"groq": "gsk-..."}`.
2. `Init()` is called by BubbleTea → Because `pendingAPIKeys` is non-empty, `Init()` returns a non-nil `tea.Cmd` (a batch of `setProviderKeyCmd` calls).
3. The key-replay commands execute → Each POSTs to `/v1/providers/{name}` to set the API key in the running harness server. The server now has the Groq key available.
4. `ProvidersLoadedMsg` or `APIKeySetMsg` arrives → No visible change to the user. The status bar still shows the model name and `$0.0000`. The key is silently configured.
5. TUI is now fully ready → Viewport empty. Input focused. `❯` cursor visible.
6. User selects Groq as model via `/model` → Model switcher opens. User navigates to the Groq provider. At level 1, the Groq model list is shown. The previously set key shows the provider as "configured". User selects a model (e.g. `llama3-70b-8192`).
7. `ModelSelectedMsg{ModelID: "llama3-70b-8192", Provider: "groq"}` received → Status bar updates to show `llama3-70b-8192`.
8. User types a message and presses `Enter` → POST to `/v1/runs` with `model: "llama3-70b-8192"` and `provider_name: "groq"`. No credential errors occur because the key was replayed at startup.
9. Assistant response streams in normally.
10. `usage.delta` received → Status bar cost updates.
11. `run.completed` → Run finishes normally.

### Variations
- Persisted config has keys for multiple providers (e.g. OpenAI + Anthropic + Groq): all are replayed in a batch at `Init()`. All providers are immediately available without further `/keys` interaction.
- Persisted starred models: the `modelswitcher` component initializes with those models starred, visible in the level-1 model list with star indicators.

### Edge Cases
- `~/.config/harnesscli/config.json` is corrupted (invalid JSON): the config load returns an error; `pendingAPIKeys` remains empty; `Init()` returns nil; TUI opens without pre-configured keys. No crash.
- Replayed key is rejected by the server (key expired): `APIKeySetMsg` with an error status arrives; the status bar may show an error message for 3 seconds. The user would need to use `/keys` to re-enter the key.

---

### Tool Execution Flow

## STORY-013: Watching a Tool Call Appear and Resolve

**Type**: short
**Topic**: Tool Execution Flow
**Persona**: Developer using the TUI for the first time after sending a prompt
**Goal**: Understand what the spinning green dot means and see it settle when the tool finishes
**Preconditions**: TUI is open, a run is active, no previous tool calls have appeared in the viewport

### Steps

1. User sends a prompt ("read the main.go file and summarize it") → Run starts; the assistant begins processing; the viewport shows no tool blocks yet.
2. SSE stream delivers `tool.call.started` for `read_file` → A new block appears at the bottom of the viewport: `⏺ ReadFile(main.go)…` — the `⏺` dot renders in bright green, the trailing `…` indicates in-progress state.
3. The timer inside the block begins counting elapsed milliseconds from the moment the block appeared.
4. SSE stream delivers `tool.call.done` for the same call ID → The block transitions: dot dims to faint gray, the `…` suffix is replaced by the elapsed duration, e.g. `⏺ ReadFile(main.go) (0.8s)` — all text dims to faint style.
5. User reads the final collapsed line and understands the tool completed in under a second.

### Variations

- **Long tool name**: If the tool name plus args would exceed the terminal width, the args string is truncated with `…` before the closing `)`.
- **Sub-second completion**: Duration renders as milliseconds, e.g. `(450ms)`.
- **Minute-long tool**: Duration renders as `(1m 12s)`.

### Edge Cases

- **No args**: If `Args` is empty the block falls back to displaying the call ID instead of blank parens.
- **Terminal width under ~25 cols**: `dotPrefixWidth` (2) is subtracted first; args may be completely elided to a single `…`.

---

## STORY-014: Streaming Output Replaces the Tail Line

**Type**: medium
**Topic**: Tool Execution Flow
**Persona**: Developer who wants to monitor what a bash command is printing in real time
**Goal**: See live output grow inside the tool block without the entire viewport jumping
**Preconditions**: A `bash` tool call is running; the tool block is in the running state (green dot, `…` suffix)

### Steps

1. Agent calls `bash` with command `go test ./...` → Block appears: `⏺ BashExec(go test ./...)…`.
2. First `tool.call.output.chunk` arrives with `"Running tests..."` → The viewport calls `ReplaceTailLines` to splice the new content into the last rendered position; the tool block now shows:
   ```
   ⏺ BashExec(go test ./...)…
   ⎿  $ go test ./...
   ⎿  Running tests...
   ```
3. Subsequent chunks arrive as test packages complete → Each chunk replaces the tail lines; the output grows downward inside the block, showing one new `⎿` prefixed line per output line received.
4. After 10 lines are visible, the next chunk causes a truncation hint to appear: `⎿  +3 more lines (ctrl+o to expand)` — the older lines stay but the line count cap (`defaultMaxLines = 10`) prevents unbounded growth.
5. `tool.call.done` arrives → The dot dims and the timing suffix appears: `⏺ BashExec(go test ./...) (4.2s)`.
6. The truncation hint remains visible in the collapsed view, reminding the user that more output is available via `ctrl+o`.

### Variations

- **Tool without Command field set**: No `$ <command>` label line is rendered; output lines start directly below the collapsed header.
- **ANSI color in output**: `StripANSI` removes all CSI escape sequences before rendering so ANSI codes do not corrupt the line display.

### Edge Cases

- **Output exceeds 512 KB**: The accumulator clamps at 512 KB and appends `[output truncated at 512KB]` as a final line before rendering.
- **Duplicate consecutive chunks**: The accumulator's idempotency guard silently drops any chunk that is byte-for-byte identical to the preceding chunk for the same call ID, preventing visual flicker from redelivered SSE events.

---

## STORY-015: Expanding a Running Tool Call with ctrl+o

**Type**: short
**Topic**: Tool Execution Flow
**Persona**: Developer who wants to see the full output of a long-running bash command without waiting for it to finish
**Goal**: Toggle the active tool block from collapsed to expanded mid-run
**Preconditions**: A `bash` tool call is running; the block is collapsed; more than 10 output lines have already arrived

### Steps

1. User sees `⏺ BashExec(go build ./...) (3s)…` with `⎿  +8 more lines (ctrl+o to expand)` hint → Recognizes the truncation hint.
2. User presses `ctrl+o` → `ToggleState.Toggle()` flips the `expanded` flag from `false` to `true` for the active tool call.
3. The block re-renders in expanded mode: the `CollapsedView` header line remains at top, followed by `⎿  $ go build ./...`, then all accumulated output lines with `⎿` prefixes, and finally a `+N more lines` hint only if the result itself exceeds `maxResultLines` (20).
4. Subsequent `tool.call.output.chunk` events continue to update the block in expanded state; new lines appear below the existing ones via `ReplaceTailLines`.
5. User presses `ctrl+o` again → block collapses back to single-line mode.

### Variations

- **Non-bash tool expanded**: The `ExpandedView` renders `Params` key-value lines (from parsed args) followed by result lines using `⎿` tree connectors, then a duration/timestamp footer line.
- **Pressing ctrl+o when no active tool**: No-op; the key is handled by the plan overlay toggle path if plan mode is active.

### Edge Cases

- **Expanded while running**: Timer shows elapsed time updated each tick; when `tool.call.done` arrives the running duration is locked in and the dot dims.

---

## STORY-016: Tool Call Error State Rendered in Red

**Type**: short
**Topic**: Tool Execution Flow
**Persona**: Developer troubleshooting a failing agent run
**Goal**: Immediately identify which tool failed and why, without leaving the viewport
**Preconditions**: A run is active; a tool call has just returned a non-success status

### Steps

1. SSE stream delivers `tool.call.done` with an error payload for call ID `call-abc` (tool: `read_file`) → The block transitions to error state.
2. The `ErrorView` renders in place of the standard collapsed view:
   ```
   ⏺ ReadFile ✗
   ⎿  Error: open /etc/shadow: permission denied
   ⎿  Hint: Check file permissions or run with sudo
   ```
   The `⏺` dot uses faint style; the `✗` suffix renders in pink/red (`#FF5F87`); the `Error:` label and message render in the same error color.
3. If a `Hint` string is present in the error payload, it appears on a second `⎿` tree line in dim/faint style below the error message.
4. User reads the error and hint without needing to scroll or open any panel.

### Variations

- **Long error message**: `wrapText` wraps the error at `width - 12` runes; continuation lines are indented to align with the text after `"Error: "`.
- **No hint**: The hint line is omitted entirely; only the header and error lines render.
- **Tool error without ErrorText**: The `ErrorView` renders `"Error: "` as an empty placeholder line.

### Edge Cases

- **Error in collapsed hint line**: When `State == StateError` and `Hint` is non-empty, the `CollapsedView` also renders a hint line below the header in the standard collapsed path (not just `ErrorView`). The two paths are consistent.

---

## STORY-017: Observing the Elapsed Timer During a Slow Tool Call

**Type**: short
**Topic**: Tool Execution Flow
**Persona**: Developer monitoring an agent that has been running for an unexpectedly long time
**Goal**: Know how long the current tool call has been running without opening any panel
**Preconditions**: A run is active; a single tool call has been in the running state for more than 10 seconds

### Steps

1. `tool.call.started` arrives for `bash` with a long-running database migration command → Block appears with green dot and `…` suffix. Timer starts (`Timer.Start()` records `startTime`).
2. Each UI tick (driven by `SpinnerTickMsg` or a periodic message) re-renders the collapsed header; the timer's `Elapsed()` method computes `time.Since(startTime)` since the timer is still running.
3. At 5 seconds the header reads `⏺ BashExec(psql -c "ALTER TABLE...")…` with the timer value visible in expanded view as `   5.0s` on the footer line (only if expanded).
4. In collapsed view the elapsed time is **not** shown while running (only after completion) — the user presses `ctrl+o` to expand and see `   12.3s` updating in the footer.
5. `tool.call.done` arrives → Timer calls `Stop()`; `endTime` is recorded; `IsRunning()` returns false; `FormatDuration()` now returns the final locked value, e.g. `"15.7s"`; the collapsed header appends ` (15.7s)` in dim style.

### Variations

- **Sub-minute**: Duration renders as `"N.Ns"` (one decimal place).
- **Over one minute**: Duration renders as `"Nm Ns"`, e.g. `"2m 30s"`.
- **Under one second**: Duration renders as `"NNNms"`, e.g. `"450ms"`.

### Edge Cases

- **Timer never started** (call ID seen in `tool.call.done` with no preceding `tool.call.started`): `startTime` is zero; `FormatDuration()` returns `"0ms"`; no timing suffix appears in the collapsed header because the guard `!v.Timer.startTime.IsZero()` fails.

---

## STORY-018: A Diff-Producing Tool Renders with Syntax Highlighting

**Type**: medium
**Topic**: Tool Execution Flow
**Persona**: Developer reviewing code changes made by the agent
**Goal**: See a readable, syntax-highlighted unified diff inside the tool block instead of raw text
**Preconditions**: A run is active; the agent has just used a file-editing tool that returned a unified diff as its result

### Steps

1. Agent calls `edit_file` on `internal/server/http.go` → `tool.call.started` fires; block appears collapsed: `⏺ EditFile(internal/server/http.go)…`.
2. `tool.call.done` arrives; the result string begins with `--- a/internal/server/http.go` → `looksLikeUnifiedDiff()` returns `true`; the model enters the diff rendering path.
3. User presses `ctrl+o` to expand the block → The `diffview.Model` receives the full diff string and the file path; `View()` calls `View{}.Render()` which formats the unified diff with colored `+`/`-` lines using terminal color codes.
4. The expanded block renders:
   ```
   ⏺ EditFile(internal/server/http.go)
   ⎿  path: internal/server/http.go
   ⎿  [diff lines with +/- syntax highlighting]
      0.3s                                   14:45:22
   ```
5. Duration and timestamp appear in the footer line: duration left-aligned, timestamp right-aligned, separated by padding calculated to fill the terminal width.
6. User reads the diff, understands what changed, and presses `ctrl+o` again to collapse.

### Variations

- **Diff detected by `\ndiff --git` mid-string**: `looksLikeUnifiedDiff()` also matches when the diff prefix appears after a leading newline.
- **Diff rendering returns empty string**: Falls through to `ExpandedView` which renders the raw result text without diff highlighting.

### Edge Cases

- **Params present with diff**: Key-value param lines are rendered between the header and the diff block via `renderTreeLine`.
- **Very large diff**: `diffview` has its own `MaxLines` cap (defaulting to `defaultMaxLines`) to prevent the viewport from becoming unusable.

---

## STORY-019: Sequential Tool Calls Accumulate in the Viewport

**Type**: medium
**Topic**: Tool Execution Flow
**Persona**: Developer watching an agent work through a multi-step task (read, analyze, write)
**Goal**: Track the sequence of tool calls and their completion states without losing context
**Preconditions**: A run is active; the agent is executing a plan that involves three consecutive tool calls

### Steps

1. `tool.call.started` for `read_file` (call ID: `call-1`) → First block appears: `⏺ ReadFile(main.go)…` in green.
2. `tool.call.done` for `call-1` → Block transitions: `⏺ ReadFile(main.go) (0.6s)` in dim. Block stays in the viewport.
3. `tool.call.started` for `bash` (call ID: `call-2`) → Second block appends below: `⏺ BashExec(go vet ./...)…` in green. First block remains above it, completed and dimmed.
4. Streaming output chunks arrive for `call-2`; lines appear inside the second block via `ReplaceTailLines`.
5. `tool.call.done` for `call-2` → Second block dims: `⏺ BashExec(go vet ./...) (2.1s)`.
6. `tool.call.started` for `write_file` (call ID: `call-3`) → Third block appears: `⏺ WriteFile(main.go, <content>)…`.
7. `tool.call.done` for `call-3` → Third block dims. All three blocks are now visible in the viewport in their completed (dim, with durations) state.
8. User scrolls up with `pgup` to review earlier blocks.

### Variations

- **Long task with 10+ tool calls**: All blocks accumulate; the viewport becomes scrollable. User navigates with `up`/`down`, `pgup`/`pgdn`.
- **Mixed success and errors**: Completed blocks show durations; errored blocks show `✗` suffix in red; the mixture is immediately scannable.

### Edge Cases

- **Interleaved assistant text**: Between tool call blocks, assistant text deltas appear as `messagebubble` entries; the viewport renders them inline in order of arrival.

---

## STORY-020: Nested Tool Calls from a Subagent

**Type**: long
**Topic**: Tool Execution Flow
**Persona**: Power user running the agent against a complex codebase where the agent spawns a subagent
**Goal**: Understand the parent-child relationship between the outer tool call and the inner tool calls launched by the subagent
**Preconditions**: A run is active; the top-level agent has called `run_agent` (a subagent-spawning tool); the subagent is itself calling tools

### Steps

1. `tool.call.started` for `BashExec` (call ID: `call-root`) at depth 0 → Root block appears: `⏺ BashExec(go test ./...)…`.
2. A nested `tool.call.started` arrives for `ReadFile` (call ID: `call-child`, parent ID: `call-root`) → The `Tree.Add()` method attaches the child node under the root; `RenderTree` re-renders the block:
   ```
   ⏺ BashExec(go test ./...)
     ⎿  ⏺ ReadFile(theme.go)…
   ```
3. A second nested `tool.call.started` (call ID: `call-grandchild`, parent ID: `call-child`) arrives → Grandchild attaches under the child; depth 2 prefix is `"  ⎿    "`:
   ```
   ⏺ BashExec(go test ./...)
     ⎿  ⏺ ReadFile(theme.go)
     ⎿    ⏺ GrepSearch(lipgloss, tui/) ✗
   ```
4. `tool.call.done` for `call-grandchild` with an error → Grandchild renders `✗`; its `ErrorView` is shown inline at depth 2.
5. `tool.call.done` for `call-child` → Child dims to `⏺ ReadFile(theme.go) (0.4s)`.
6. `tool.call.done` for `call-root` → Root dims: `⏺ BashExec(go test ./...) (3.8s)`. The entire subtree is now in completed/error state.
7. User presses `ctrl+o` → The `expanded` map for `call-root` flips; the root expands to `ExpandedView`, showing params and result; child nodes remain in their own toggle state (collapsed by default).

### Variations

- **Single-level nesting only**: Most real runs have depth 0 and depth 1; depth 2 is rarer. The tree supports arbitrary depth via recursive `flattenNode`.
- **Unknown parent ID**: If a child arrives before its parent has been registered, the node is placed at root level as a fallback.

### Edge Cases

- **Replace vs. insert**: `Tree.Add()` with an existing `CallID` triggers `removeFromTree` then re-add, preserving position — handles the case where a `tool.call.started` event is redelivered with updated fields.
- **Width constraint at deep nesting**: At depth 3, prefix width is `"  ⎿      "` (8 runes); on an 80-col terminal the inner content has only 72 cols; args truncation still applies.

---

## STORY-021: File Operation Tool Renders a Summary Line

**Type**: short
**Topic**: Tool Execution Flow
**Persona**: Developer watching the agent modify source files
**Goal**: See a concise human-readable summary of what a file operation tool did, rather than raw result text
**Preconditions**: A run is active; the agent has called `write_file` and the call is now complete

### Steps

1. `tool.call.done` arrives for `write_file` with a result that contains 47 lines of written content → `ParseFileOp("write_file", "/src/handler.go", result)` is called; `countLines(result)` returns 47; `FileOpSummary{Kind: FileOpWrite, FileName: "handler.go", LineCount: 47}` is constructed.
2. `FileOpSummary.Line()` returns `"⎿  Wrote 47 lines to handler.go"` — the `⎿` tree connector is rendered in dim/faint style; the text is plain.
3. The collapsed block shows:
   ```
   ⏺ WriteFile(handler.go, <content>) (0.2s)
   ⎿  Wrote 47 lines to handler.go
   ```
4. For a `read_file` call returning 120 lines: `"⎿  Read 120 lines"` (no filename shown for reads).
5. For an `edit_file` call with `+5` diff lines: `"⎿  Added 5 lines to server.go"`.
6. For an `edit_file` call with no `+` lines: `"⎿  Edited server.go"`.

### Variations

- **Filename exceeds 40 runes**: `truncateFileName` clamps to 39 runes and appends `…`.
- **`str_replace_editor` tool name**: Maps to `FileOpEdit` via `classifyToolName`.

### Edge Cases

- **Unknown tool name** (e.g. `custom_writer`): `ParseFileOp` returns `FileOpSummary{Kind: FileOpUnknown}`; `Line()` returns `""` and no summary line is rendered.
- **Empty result string**: `countLines("")` returns 0; `FileOpWrite` with 0 lines returns `""` — no summary line rendered.

---

## STORY-022: Interrupt During an Active Tool Call

**Type**: medium
**Topic**: Tool Execution Flow
**Persona**: Developer who realizes the agent is running a destructive command and wants to stop it
**Goal**: Cancel the active run cleanly while observing the tool call block transition to a final state
**Preconditions**: A bash tool call is running (`⏺ BashExec(rm -rf ./tmp/*)…`); the user decides to interrupt

### Steps

1. User presses `ctrl+c` → `interruptui.Model` transitions to `Confirm` state; a banner appears above the input area: `"Press Ctrl+C again to stop..."`. The run continues; the tool block keeps streaming.
2. User presses `ctrl+c` again → Banner transitions to `Waiting` state: `"Stopping... (waiting for current tool to finish)"`. An interrupt request is sent to the server via the cancel mechanism.
3. SSE stream delivers `run.failed` with a cancellation error → `RunFailedMsg` fires; error lines are appended to the viewport below the tool block; the run state is cleared.
4. The tool block that was running never receives `tool.call.done` — it remains in running state (green dot, `…` suffix) since no done event arrived. This is a known terminal state for cancelled calls.
5. Status bar shows transient message `"Interrupted"` for 3 seconds.
6. User sees the viewport with: the frozen `⏺ BashExec(rm -rf ./tmp/*)…` block followed by the run-failed error lines. The `interruptui.Model` transitions to `Done` and then hides.

### Variations

- **Pressing `esc` instead of second `ctrl+c`**: `Esc` cancels the interrupt sequence; the banner closes; the run continues. The tool block remains in running state.
- **Tool completes before interrupt is processed**: If `tool.call.done` arrives before the server cancels, the block transitions to completed with duration before the run fails.

### Edge Cases

- **No active run when `ctrl+c` is pressed**: Triggers quit flow immediately (no interrupt banner shown).

---

## STORY-023: Reviewing Completed Tool Calls by Scrolling

**Type**: long
**Topic**: Tool Execution Flow
**Persona**: Developer reviewing what actions the agent took during a completed run
**Goal**: Scroll back through the conversation viewport to inspect past tool call blocks and expand specific ones for full output
**Preconditions**: A run has completed; the viewport contains 8 tool call blocks (a mix of completed and error states); the user is at the bottom of the viewport

### Steps

1. Run ends; assistant final text appears below the tool blocks → User is at the bottom of the viewport looking at the assistant's summary.
2. User presses `pgup` → Viewport scrolls up by half its height; older tool call blocks come into view. Each completed block shows its dim dot, args, and duration; errored blocks show the red `✗` suffix.
3. User reads: `⏺ BashExec(go test ./internal/...) (12.4s)` — notes it took 12 seconds.
4. User presses `ctrl+o` → The most recently active tool call's `expanded` flag toggles.
5. User presses `pgdn` → Scrolls back down; the `⏺ WriteFile(api.go) (0.1s)` block is visible with `⎿  Wrote 120 lines to api.go`.
6. User presses `ctrl+s` → The last assistant response is copied to the clipboard. Status bar shows `"Copied"` for 3 seconds.
7. User types `/export` → Transcript export runs in background; status bar shows `"Exported: transcript-20260323-145832.md"` for 3 seconds.

### Variations

- **Error block with hint**: `⏺ GrepSearch(badpattern) ✗` followed by `⎿  Error: invalid regex: ...` and `⎿  Hint: Check your regex syntax` — full error detail visible without expanding.
- **Diff block in history**: User scrolls to an `edit_file` block; presses `ctrl+o`; sees the syntax-highlighted unified diff with duration and timestamp in the footer.

### Edge Cases

- **Viewport at top**: `pgup` is a no-op; `up`/`ctrl+p` scroll one line at a time and also no-op at the top boundary.
- **Only one tool call**: Viewport height is sufficient to show everything without scrolling.

---

## STORY-024: Permission Prompt Blocking Tool Execution

**Type**: long
**Topic**: Tool Execution Flow
**Persona**: Developer who configured the agent to require explicit approval before writing files
**Goal**: Approve a file-write tool call, observe the block transition from pending to completed
**Preconditions**: The agent is mid-run; a `write_file` call has been proposed; the server is waiting for approval

### Steps

1. SSE stream delivers a permission-required event for `write_file` targeting `src/main.go` → The `permissionprompt.Model` modal appears over the viewport; the tool block `⏺ WriteFile(src/main.go)…` is visible behind the modal but frozen.
2. Modal displays three options: `[ Yes (allow once) ]`, `[ No (deny) ]`, `[ Allow all (this session) ]`.
3. User tabs to `[ Yes (allow once) ]` and presses `Enter` → TUI sends `POST /v1/runs/{id}/approve`; the permission modal closes.
4. The server proceeds with the `write_file` call; `tool.call.output.chunk` events resume → The block updates with live output.
5. `tool.call.done` arrives → Block dims: `⏺ WriteFile(src/main.go) (0.3s)` with `⎿  Wrote 47 lines to main.go`.
6. Run continues; next tool call fires.

### Variations

- **Tab-amend flow**: Before pressing `Enter`, user presses `Tab` to enter amend mode; they edit the resource path from `src/main.go` to `src/main_backup.go`; then confirm — the amended path is sent in the approve request.
- **Deny**: User selects `[ No (deny) ]`; `POST /v1/runs/{id}/deny` is sent; the tool block transitions to error state with an appropriate message.
- **Allow all**: `[ Allow all (this session) ]` suppresses future permission prompts for this session.

### Edge Cases

- **Run cancelled while modal is open**: `ctrl+c` cancels the run; the modal closes; the tool block stays frozen in the running state.
- **Simultaneous tool calls requiring approval**: The current implementation shows one permission prompt at a time.

---

### Permission & Safety Controls

## STORY-025: Approving a Single File Write

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer using the TUI for day-to-day coding assistance
**Goal**: Allow the agent to write a single file without granting blanket permissions
**Preconditions**: TUI running with an active run, server configured with `ApprovalPolicyDestructive`. The agent has decided to write `main.go`.

### Steps

1. Agent decides to write a file → Server runner emits a `tool.approval_required` SSE event with `call_id`, `tool: "WriteFile"`, and `arguments` including the file path. Run status transitions to `RunStatusWaitingForApproval`.
2. TUI receives the SSE event → The `permissionprompt.Model` is instantiated and rendered as a rounded-border modal over the chat viewport. All keyboard input is now consumed by the prompt.
3. Modal displays three options with cursor on `> Yes (allow once)` → User reads "Allow tool: WriteFile" and "Resource: main.go" in the modal header.
4. User presses `Enter` with the cursor on "Yes (allow once)" → `permissionprompt` resolves with `PromptResult{Option: OptionYes}`. The TUI POSTs to `POST /v1/runs/{id}/approve`.
5. Server receives approve request → `ApprovalBroker.Approve(runID)` unblocks the runner. Run status returns to `RunStatusRunning`.
6. Agent proceeds to execute `WriteFile` → A `tool.call.started` block appears in the viewport. The modal disappears and normal input is restored.

### Variations

- **Already approved tool in session**: If the user had previously selected "Allow all (this session)" for `WriteFile`, no prompt appears.
- **Profile with `ApprovalPolicyNone`**: No `tool.approval_required` event is ever emitted; the tool runs without interruption.

### Edge Cases

- **Prompt appears at narrow terminal width (< 40 cols)**: The modal adapts by enforcing a minimum `innerWidth` of 10; option labels and resource path are truncated with `…`.
- **Enter pressed with no Options (empty slice)**: The prompt falls back to `OptionNo` and resolves immediately; a `deny` POST is issued.

---

## STORY-026: Denying a Bash Command

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Security-conscious developer who wants to vet every shell command
**Goal**: Prevent the agent from running an unexpected `rm -rf` command
**Preconditions**: TUI running with `ApprovalPolicyAll` profile active.

### Steps

1. Agent issues a `bash` tool call with arguments `{"command": "rm -rf /tmp/build"}` → `tool.approval_required` SSE event is emitted.
2. Permission prompt modal appears with "Allow tool: bash" and "Resource: rm -rf /tmp/build" → User reads the command text.
3. User presses `Down` arrow once → Cursor moves from "Yes (allow once)" to `> No (deny)`.
4. User presses `Enter` → `permissionprompt` resolves with `PromptResult{Option: OptionNo}`. TUI POSTs to `POST /v1/runs/{id}/deny`.
5. Server `ApprovalBroker.Deny(runID)` returns `approved = false` to the runner → Runner emits `tool.approval_denied` SSE event. Runner constructs a denied tool result JSON: `{"error": {"code": "permission_denied", "message": "tool call denied by operator"}}`.
6. Runner emits `tool.call.completed` with the denial payload → The tool call block in the viewport renders in error state showing `permission_denied`. The agent receives the error as its tool result and responds accordingly.
7. TUI restores normal input.

### Variations

- **Pressing `Esc` instead of navigating to "No"**: `Esc` in selection mode also resolves with `OptionNo`, issuing the same deny POST.

### Edge Cases

- **Approval broker returns error (e.g., context cancelled)**: Runner treats this as a denial with `approval_timeout` code instead of `permission_denied`.
- **Run cancelled (Ctrl+C twice) while prompt is open**: The interrupt confirmation banner is a separate overlay (`interruptui`). If the run is cancelled, the runner's context is cancelled, which causes `ApprovalBroker.Ask` to return an error.

---

## STORY-027: Granting Session-Wide Permission

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer in a flow state who trusts the agent's file-editing actions for the current session
**Goal**: Stop seeing repeated permission prompts for `WriteFile` during a single session
**Preconditions**: TUI running with `ApprovalPolicyDestructive`. Agent is performing a multi-file refactor and will write many files.

### Steps

1. Agent calls `WriteFile` for the first file → Permission prompt appears: "Allow tool: WriteFile", "Resource: src/foo.go".
2. User reads the prompt and decides to trust all writes this session → User presses `Down` twice to reach `> Allow all (this session)`.
3. User presses `Enter` → `permissionprompt` resolves with `PromptResult{Option: OptionAllowAll}`. TUI POSTs `POST /v1/runs/{id}/approve`.
4. TUI records the session-wide grant locally → All future `tool.approval_required` events for `WriteFile` during this session are automatically approved without showing the modal.
5. Agent writes the second file `src/bar.go` → No permission prompt appears.
6. Agent writes a third file `src/baz.go` → Again no prompt. The refactor completes uninterrupted.

### Variations

- **Starting a new TUI session**: Session-wide grants are in-memory only and do not persist; the first `WriteFile` in the new session will prompt again.
- **Different tool in same session**: "Allow all (this session)" for `WriteFile` does not suppress prompts for `bash` or `ReadFile`.

### Edge Cases

- **Run fails partway through**: Subsequent runs in the same TUI session retain the session-wide grant (it is session-scoped, not run-scoped).

---

## STORY-028: Amending a Resource Path Before Approving

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer who wants to redirect a file write to a safer path
**Goal**: Approve the action but change the target file path before confirming
**Preconditions**: Permission prompt is open showing "Allow tool: WriteFile", "Resource: /etc/hosts".

### Steps

1. Permission prompt appears with `> Yes (allow once)` as the default cursor position → User sees the resource path `/etc/hosts` and is concerned.
2. User presses `Tab` → The prompt enters amend mode. The footer changes from "[Tab to amend path]" to "Amend path: _" (a text cursor). The `amended` buffer starts empty.
3. User types `~/scratch/hosts-copy.txt` character by character → Each keystroke updates `m.amended`; the "Resource:" line in the modal header shows the typed text live.
4. User presses `Enter` to confirm the amendment → `IsAmending()` returns to `false`. The modal returns to selection mode displaying the amended path.
5. User presses `Enter` again to confirm the option → `permissionprompt` resolves with `PromptResult{Option: OptionYes, Amended: "~/scratch/hosts-copy.txt"}`. TUI POSTs `POST /v1/runs/{id}/approve` with the amended resource.
6. Server approves and the tool runs with the amended path → The tool block shows the corrected path in the viewport.

### Variations

- **User changes their mind mid-amendment and presses `Esc`**: In amend mode, `Esc` cancels the amendment, clears `m.amended`, and returns to selection mode.
- **User uses Backspace to correct a typo in amend mode**: Each `Backspace`/`Delete` removes the last rune from `m.amended` (UTF-8 safe rune slicing).

### Edge Cases

- **Terminal too narrow to show the full amended path**: The `truncate()` helper clips the line to `innerWidth - 1` runes and appends `…`.
- **User presses `Tab` again while already in amend mode**: Falls through without action (no nested amend mode).

---

## STORY-029: Clearing Amend Input with Ctrl+U

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer who started typing an amended path but wants to start over
**Goal**: Clear the entire in-progress amend input in one keystroke rather than pressing Backspace repeatedly
**Preconditions**: Permission prompt is open in amend mode; user has typed a partial path `~/wrong/path/to/`.

### Steps

1. Permission prompt is in amend mode → "Amend path: ~/wrong/path/to/_" is shown in the footer.
2. User realizes the path prefix is entirely wrong → User presses `Ctrl+U`.
3. The amend buffer is cleared (`m.amended = ""`) → Footer shows "Amend path: _" with an empty input field.
4. User types the correct path `~/correct/target.go` → Each character accumulates in `m.amended`.
5. User presses `Enter` to confirm the amendment → Returns to selection mode with the corrected path visible.
6. User presses `Enter` to approve → Prompt resolves with `PromptResult{Option: OptionYes, Amended: "~/correct/target.go"}`.

### Variations

- **Ctrl+U pressed when amend buffer is already empty**: No visible change; the footer continues showing "Amend path: _".

### Edge Cases

- **Ctrl+U pressed in selection mode (not amend mode)**: `Ctrl+U` is only handled in `updateAmending`. In selection mode, the key falls through unhandled.

---

## STORY-030: Distinguishing Permission Prompt from Interrupt Banner

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer new to the harness TUI, unsure of the different modal surfaces
**Goal**: Understand which overlay is which and how to interact with each correctly
**Preconditions**: TUI is active with a run in progress.

### Steps

1. Developer presses `Ctrl+C` once during an active run → The **interrupt banner** appears: a yellow-bordered box with "⚠  Press Ctrl+C again to stop, or Esc to continue". This is `interruptui.StateConfirm`. The banner appears above the input area, not as a full viewport takeover.
2. Developer presses `Esc` → `interruptui` returns to `StateHidden`. The run continues uninterrupted.
3. Shortly after, the agent calls a mutating tool → The **permission prompt** appears as a rounded-border modal with three selectable options and a "[Tab to amend path]" footer. Visually distinct from the interrupt banner.
4. Developer notices the visual difference → Interrupt banner: yellow border, warning icon, one-line, dismissible with `Esc`. Permission prompt: neutral rounded border, tool/resource header, numbered options.
5. Developer selects "Yes (allow once)" and presses `Enter` → Prompt resolves. Normal input restored.

### Variations

- **Ctrl+C pressed twice rapidly when a permission prompt is already open**: Both the interrupt banner transition and permission prompt timeout can occur simultaneously depending on timing.

### Edge Cases

- **Pressing `Ctrl+C` in Confirm state**: `interruptui.Confirm()` transitions to `StateWaiting` and the text changes to "Stopping… (waiting for current tool to finish)".
- **Run completes before second `Ctrl+C`**: The `interruptui` banner returns to `StateHidden` once the `run.completed` SSE event arrives.

---

## STORY-031: Approval Prompt Blocking All Input

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer who tries to type a message while the agent is waiting for approval
**Goal**: Understand that the permission prompt is a hard gate — no other interaction proceeds until it is resolved
**Preconditions**: A `tool.approval_required` event has arrived and the permission prompt modal is active.

### Steps

1. Permission prompt modal is displayed → `permissionprompt.IsActive()` returns `true`. The modal holds the BubbleTea update loop's exclusive attention.
2. User tries to type a new message in the input area → All key events are routed to the `permissionprompt.Update()` handler first. Printable characters in selection mode are not handled, so they fall through silently.
3. User tries `/help` or any slash command → Same routing: slash and subsequent characters are consumed by the prompt as unhandled keys. The slash-complete dropdown does not open.
4. User presses `Up` or `Down` → These are handled by `updateSelecting`, moving the cursor between the three options.
5. User presses `Enter` to resolve → The prompt resolves; normal input routing resumes.

### Variations

- **User presses `Esc` while in selection mode**: Resolves with `OptionNo` (deny). Normal input returns immediately.

### Edge Cases

- **Long-running approval with user idle**: The harness runner uses `AskUserTimeout` as a deadline. If the user does not respond before the timeout, `ApprovalBroker.Ask` returns an error, and the runner emits `tool.approval_denied` with `code: "approval_timeout"`.

---

## STORY-032: Reviewing Session Permissions via /permissions Panel

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer mid-session who wants to audit which tools have been granted or denied
**Goal**: View and manage the accumulated permission rules for the current session
**Preconditions**: Several permission decisions have been made: `WriteFile` allowed permanently (Allow all), `bash` denied once, `ReadFile` allowed once.

### Steps

1. User types `/permissions` and accepts from the slash-complete dropdown → The `permissionspanel.Model` opens, displaying a scrollable list of `PermissionRule` entries.
2. Panel shows three rows:
   - `  ✓ WriteFile  permanent`
   - `  ✗ bash  once`
   - `  ✓ ReadFile  once`
3. User presses `j` or `Down` → Selection cursor moves down through rows.
4. User selects the `bash` deny row and presses `t` (toggle) → `ToggleSelected()` flips `Allowed` from `false` to `true`. The row now shows `✓ bash  once`.
5. User selects the `ReadFile` allow row and presses `d` (delete) → `RemoveSelected()` removes the rule.
6. User presses `Esc` → Panel closes. Remaining rules are preserved in memory.

### Variations

- **Panel opened when no rules exist**: The panel renders "No permission rules active" in dimmed style.
- **Rules list has wrap-around navigation**: `SelectUp()` at the top wraps to the last entry; `SelectDown()` at the bottom wraps to the first.

### Edge Cases

- **Toggling the only rule**: Works normally.
- **Deleting the last rule**: `RemoveSelected()` leaves an empty slice; panel shows "No permission rules active".

---

## STORY-033: Approval Timeout — User Steps Away

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer who left the terminal while a permission prompt was waiting
**Goal**: Understand what happens when the approval deadline passes without a response
**Preconditions**: Permission prompt is active for a `WriteFile` call. The harness runner's `AskUserTimeout` is configured (e.g., 2 minutes).

### Steps

1. Permission prompt modal is displayed → TUI shows the modal. User steps away from terminal without responding.
2. Two minutes pass → On the server side, `ApprovalBroker.Ask` times out. It returns an error to the runner.
3. Runner detects `approvalErr != nil` → Runner sets status back to `RunStatusRunning`. Runner emits `tool.approval_denied` SSE event with `reason: "approval timeout"`.
4. Runner constructs denied tool result with `code: "approval_timeout"` → Emits `tool.call.completed` with the error payload.
5. TUI receives both SSE events → The tool block in the viewport updates to show an error state: "Error: approval_timeout". The permission prompt should be dismissed as the run continues.
6. Agent receives the timeout error as tool output → Agent responds, possibly retrying the operation or abandoning it.

### Variations

- **`AskUserTimeout` is zero / very short**: Prompts fail almost immediately; effectively disables interactive approval.

### Edge Cases

- **User returns and presses `Enter` on the prompt after the timeout**: The server returns a 404 or error (no pending approval for that run ID). The TUI should handle the error gracefully.

---

## STORY-034: Full-Auto Profile Bypasses All Prompts

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Operator setting up an automated pipeline run
**Goal**: Confirm that a profile with `ApprovalPolicyNone` produces zero permission interruptions
**Preconditions**: A profile named `ci-auto` has been created with `approval: "none"`. The developer selects it via `/profiles` before starting a run.

### Steps

1. Developer types `/profiles` → Profile picker opens. Developer navigates to `ci-auto` and presses `Enter`.
2. Status bar (or next-run indicator) reflects the selected profile.
3. Developer submits a prompt that causes the agent to call several mutating tools → Multiple `tool.call.started` blocks appear in the viewport.
4. No `tool.approval_required` SSE events are emitted → The runner checks `needsApproval` and with `ApprovalPolicyNone`, the condition is never true.
5. All tool calls execute and complete → Viewport shows tool blocks transitioning to completed state. No permission modal ever appears.

### Variations

- **Switching back to a `permissions` profile on the next run**: The next prompt submission uses the newly selected profile and resumes prompting for destructive tools.

### Edge Cases

- **Profile `approval` field missing**: Defaults to `ApprovalPolicyNone` (the zero value for the config). Behavior is identical to the full-auto case.

---

## STORY-035: Deny Produces Tool Error Visible in Viewport

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer debugging why an agent task stalled
**Goal**: Trace the visible effect of a denial from the permission prompt through to the tool call block
**Preconditions**: User denied a tool call via the permission prompt earlier in the session. The tool call block is visible in the viewport.

### Steps

1. User denied `bash` via the permission prompt → `POST /v1/runs/{id}/deny` was sent. Server emitted `tool.approval_denied` then `tool.call.completed` with `{"error": {"code": "permission_denied", "message": "tool call denied by operator"}}`.
2. TUI processed the `tool.call.completed` SSE event → The `tooluse.Model` for that call ID transitioned to error state.
3. User scrolls up in the viewport → The `bash` tool block shows the error text in red: "Error: permission_denied — tool call denied by operator".
4. User can see this in context → Other tool calls before and after show green completed state; only the denied one is red.
5. Agent's next message is also visible → The assistant, having received the error as its tool result, responded with an explanation or alternative approach.

### Variations

- **Timeout denial (approval_timeout)**: The error code differs (`approval_timeout`) but the visual treatment in the tool block is identical — red error state.

### Edge Cases

- **User expands the tool block with `Ctrl+O`**: The full error JSON is shown in the expanded view, confirming the exact `code` and `message` fields.

---

## STORY-036: Permission Prompt with Unknown/Generic Tool

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer using a custom or third-party tool registered with the harness
**Goal**: Confirm that the permission prompt renders correctly even for tool names the TUI has no specific knowledge of
**Preconditions**: A custom tool named `PushToRegistry` is registered. The server is configured with `ApprovalPolicyAll`.

### Steps

1. Agent calls `PushToRegistry` with arguments referencing a container image → `tool.approval_required` emitted with `tool: "PushToRegistry"`.
2. Permission prompt modal appears → Header shows "Allow tool: PushToRegistry" and "Resource: registry.internal/myapp:latest". All three options are present.
3. User verifies the display is correct → The tool name renders verbatim; the full three-option list is shown.
4. User selects "No (deny)" and presses `Enter` → Prompt resolves; deny POST sent; tool fails with `permission_denied`.

### Variations

- **Tool name is very long (> innerWidth runes)**: The header line is truncated with `…` by the `truncate()` helper.
- **Resource path is a URL**: The resource field renders as-is; no URL-specific formatting is applied.

### Edge Cases

- **Tool name is empty string**: The header reads "Allow tool: " with no name. The prompt still functions correctly.
- **Options slice is nil (not populated by caller)**: Prompt shows "(no options available — press Esc to dismiss)". The fallback Enter resolves as `OptionNo`.

---

### Model & Provider Selection

## STORY-037: Basic Model Switch via Provider Browser

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to try a different model mid-session
**Goal**: Switch from the current OpenAI model to Claude Sonnet without leaving the TUI
**Preconditions**: TUI is running with `gpt-4.1` as the active model; Anthropic API key is configured; no overlay is open

### Steps

1. User types `/model` in the input area and presses Enter → The model overlay opens at Level 0, showing the provider list. The overlay shows "Loading models..." while fetching.
2. Providers arrive; each row shows a model count `(N)` right-aligned and a `●` (configured) or `○` (unconfigured) indicator → Anthropic shows `●` because its API key is set.
3. User presses `k` (or Up) to move the cursor to Anthropic → The Anthropic row becomes highlighted.
4. User presses Enter → The overlay transitions to Level 1, showing the Anthropic model list: Claude Haiku 4.5, Claude Opus 4.6, Claude Sonnet 4.6. A breadcrumb header reads `< Back  [Anthropic]`.
5. User presses `j` (or Down) to navigate to Claude Sonnet 4.6.
6. User presses Enter → The config panel (Level 2) opens for Claude Sonnet 4.6, showing: model name + "Anthropic" provider line, Gateway section with `▶ Direct` selected, API Key section showing `● configured` in green.
7. User presses Enter to confirm with the current gateway and key settings → The overlay closes; the status bar at the bottom now reads `claude-sonnet-4-6  $0.00`; the model has been switched.

### Variations

- **Tab-complete shortcut**: User types `/mo` — the dropdown filters to `/model` as the only match and auto-executes on Tab.
- **Already on Anthropic**: Opening `/model` pre-positions the provider cursor on Anthropic and the model cursor on the current model.

### Edge Cases

- **Fetch fails**: If `GET /v1/models` returns an HTTP error, the overlay shows the error string in red with `esc cancel` as the only available footer action.
- **Provider list empty**: The overlay shows "No providers available" in dim text.

---

## STORY-038: Starring a Model for Quick Access

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who switches between two models frequently
**Goal**: Star Claude Opus 4.6 so it appears at the top of any future model list
**Preconditions**: TUI is running; `/model` overlay is open at Level 1 on the Anthropic model list; Claude Opus 4.6 is not yet starred

### Steps

1. User navigates to Claude Opus 4.6 with `j`/`k` → The row highlights.
2. User presses `s` → The row immediately gains a gold `★` prefix; the entry is moved to the top of the visible list (starred models sort first within any provider view); the `StarredIDs()` list is written to the persistent config file.
3. User presses Esc to close the overlay without selecting a model → The overlay closes; the config file retains the star; the current model is unchanged.

### Variations

- **Unstarring**: If Claude Opus 4.6 was already starred, pressing `s` removes the `★` and moves the model back to its alphabetical position.
- **Star during search**: With Claude Opus 4.6 highlighted in the cross-provider flat view, pressing `s` stars it in the same way.

### Edge Cases

- **No visible models**: If the search query matches nothing, pressing `s` is a no-op.
- **Cursor out of range**: The toggle guard `m.Selected >= len(visible)` prevents a panic if the model list is empty.

---

## STORY-039: Cross-Provider Search from Any Level

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who knows the model name but not which provider hosts it
**Goal**: Find and select "grok" without navigating the provider hierarchy
**Preconditions**: TUI running; `/model` overlay is open at Level 0 (provider list); no search is active

### Steps

1. User types `g` → The overlay switches to the flat cross-provider search view; visible models are filtered to any whose display name contains "g" (case-insensitive).
2. User continues typing `r`, `o`, `k` → The filter narrows to "Grok 3 Mini" and "Grok 4.1 Fast [R]" from xAI; both rows show a `[xAI]` provider prefix in dim text.
3. User navigates to Grok 3 Mini with `j`/`k` → The row highlights.
4. User presses Enter → The config panel opens for Grok 3 Mini.
5. User confirms with Enter → Model switches to grok-3-mini; overlay closes; status bar updates.

### Variations

- **Search from Level 1**: If the user is already browsing inside a provider and starts typing, the search activates globally — results span all providers.

### Edge Cases

- **No matches**: Shows "No models match" in dim text; Backspace progressively removes characters; pressing Esc clears the search and returns to the previous browse level.
- **Backspace to empty**: Deleting the entire search query restores the previous browse level view.

---

## STORY-040: Unconfigured Provider Redirects to Keys Overlay

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer trying a new provider for the first time
**Goal**: Select a Groq model even though no Groq API key has been set
**Preconditions**: TUI running; Groq API key is NOT configured; `/model` overlay is open at Level 0; Groq shows `○` (unconfigured) indicator

### Steps

1. User navigates to Groq and presses Enter → The overlay drills into Level 1, showing Groq models; both rows display `(unavailable)` in muted text.
2. User selects QwQ 32B and presses Enter → The config panel (Level 2) opens; the API Key section shows `○ not set` in dim style.
3. User attempts to confirm with Enter → The TUI detects the provider is unconfigured; closes the model overlay and opens the `/keys` overlay, pre-positioning the cursor on the Groq row.
4. The `/keys` overlay is now active, showing "Groq  GROQ_API_KEY  ○ unset".
5. User presses Enter → The overlay transitions to key input mode for Groq; an input field `> █` appears.
6. User types the API key value and presses Enter → The TUI fires `PUT /v1/providers/groq/key`; on success the Groq row updates to show `● set`.
7. User presses Esc to return to the provider list in the `/keys` overlay, then Esc again to close the overlay → The user re-opens `/model` to complete the switch.

### Variations

- **Configure key in config panel directly**: The user focuses the API Key section in the config panel and presses K to enter inline key input mode within the config panel itself.

### Edge Cases

- **Key set fails**: If the PUT returns a non-2xx status, the provider row remains `○ unset`.
- **OpenRouter key**: Setting the OpenRouter key configures routing for all providers via OpenRouter.

---

## STORY-041: Setting and Changing a Provider API Key via /keys

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Operator who needs to rotate an API key for an already-configured provider
**Goal**: Replace the existing Anthropic API key with a new one
**Preconditions**: TUI running; `/keys` command typed; Anthropic API key is currently configured (shows `● set`)

### Steps

1. User types `/keys` and presses Enter → The `/keys` overlay opens, listing all known providers with their configured status.
2. User navigates with `j`/`k` to position the cursor on Anthropic → The row highlights.
3. User presses Enter → Input mode activates for Anthropic; the env var `ANTHROPIC_API_KEY` is shown; the input field appears empty (the existing key value is never pre-filled for security).
4. User types the new API key value → Characters accumulate in the input field.
5. (Optional) User presses Ctrl+U to clear the input if they mistyped → The input field clears to empty.
6. User presses Enter → The TUI fires `PUT /v1/providers/anthropic/key`; on success the overlay shows Anthropic as `● set`.
7. User presses Esc → Returns to the provider list; pressing Esc again closes the `/keys` overlay entirely.

### Variations

- **Add a new provider key**: Same flow, but the target provider shows `○ unset` before step 3; after step 6 it transitions to `● set`.
- **Via status bar hint**: If the user tries to send a run with an unconfigured model, the run fails with a clear backend error; the user can then open `/keys` to fix the issue.

### Edge Cases

- **Empty key submission**: Pressing Enter with an empty input still fires the PUT; the server may accept or reject an empty key depending on its validation.
- **Esc during input mode**: Pressing Esc in input mode exits to the provider list view (does NOT close the whole overlay); the user must press Esc a second time to close.

---

## STORY-042: Selecting Gateway — Direct vs OpenRouter

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to route all traffic through OpenRouter to use a single API key
**Goal**: Switch the active gateway from Direct to OpenRouter
**Preconditions**: TUI running; gateway is currently set to Direct; OpenRouter API key is configured; `/model` overlay is open; user has navigated to a model and entered the config panel (Level 2)

### Steps

1. The config panel is open for GPT-4.1 (OpenAI); the Gateway section shows two options: `▶ Direct  Each model's native provider` and `  OpenRouter  Route all via openrouter.ai`.
2. User presses `←`/`→` to move the cursor from Direct to OpenRouter → The cursor (`▶`) moves to the OpenRouter row.
3. User presses Enter to confirm → The gateway is saved as `"openrouter"`; the TUI emits `GatewaySelectedMsg{Gateway: "openrouter"}`; the overlay closes; the TUI fires `fetchOpenRouterModelsCmd` to load the live OpenRouter model catalog.
4. Future runs use OpenRouter slugs (e.g. `openai/gpt-4.1`) and pass `provider_name: "openrouter"` in the `POST /v1/runs` body.

### Variations

- **Gateway overlay via /provider command**: Also accessible by typing `/provider` which opens the standalone "Routing Gateway" overlay.
- **Switching back to Direct**: Same flow; the user selects Direct; the TUI switches back to the server model list from `GET /v1/models`.

### Edge Cases

- **OpenRouter key not set**: The overlay still allows selecting OpenRouter; `fetchOpenRouterModelsCmd` makes an unauthenticated request; rate limits may apply.
- **OpenRouter fetch fails**: `ModelsFetchErrorMsg` is emitted; the model switcher shows the error string in red.

---

## STORY-043: Configuring Reasoning Effort for a Reasoning Model

**Type**: medium
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to tune the trade-off between reasoning depth and cost/speed
**Goal**: Set DeepSeek Reasoner to "high" reasoning effort before a complex task
**Preconditions**: TUI running; DeepSeek Reasoner is NOT the current model; DeepSeek API key IS configured

### Steps

1. User opens `/model` → Level 0 provider list appears.
2. User navigates to DeepSeek and presses Enter → Level 1 shows DeepSeek models: DeepSeek Chat and DeepSeek Reasoner; DeepSeek Reasoner shows the `[R]` reasoning badge.
3. User navigates to DeepSeek Reasoner and presses Enter → The config panel (Level 2) opens with three sections: Gateway, API Key, and Reasoning Effort.
4. User presses Down to move section focus to Reasoning Effort → The four options are listed: Default (← current), Low, Medium, High.
5. User presses Down to move the cursor within the Reasoning Effort section to "High" → The `▶` cursor moves to High.
6. User presses Enter to confirm the entire config panel → The TUI saves: reasoning effort = "high"; the next `POST /v1/runs` body includes `"reasoning_effort": "high"`.

### Variations

- **Default effort**: The user skips the Reasoning Effort section and confirms with Enter; the effort field is `""` (empty string = server default).
- **Mid-run change**: The reasoning effort setting persists for all subsequent runs in the session until changed again.

### Edge Cases

- **Non-reasoning model**: Models without `ReasoningMode: true` do not show the Reasoning Effort section in the config panel.
- **Current effort preserved**: If the user previously set "medium" effort and reopens the config panel, the `← current` marker appears next to "Medium".

---

## STORY-044: Discovering and Using a Model from OpenRouter's Expanded Catalog

**Type**: long
**Topic**: Model & Provider Selection
**Persona**: Developer who wants to try a model not in the default list (e.g. a new Mistral model)
**Goal**: Switch to an OpenRouter-exclusive model that is not in the default `DefaultModels` list
**Preconditions**: TUI running with Direct gateway; OpenRouter API key is configured; current model is gpt-4.1

### Steps

1. User opens the config panel for any model via `/model` → navigates to any provider → selects any model → config panel opens.
2. In the Gateway section, the user moves the cursor to OpenRouter and presses Enter to confirm → Gateway switches to "openrouter"; the TUI fires `fetchOpenRouterModelsCmd`; the overlay closes.
3. The TUI fetches `https://openrouter.ai/api/v1/models`; a `ModelsFetchedMsg{Source: "openrouter"}` arrives with the full catalog.
4. User opens `/model` again → Level 0 shows a richer provider list derived from the OpenRouter catalog.
5. User navigates to "mistralai" (or types `mis` to filter) and presses Enter → Level 1 shows Mistral models from OpenRouter.
6. User selects `mistralai/mistral-large` and presses Enter → Config panel opens; Gateway shows OpenRouter selected; API Key shows `● configured` (the OpenRouter key covers all models).
7. User confirms with Enter → The model is set to the OpenRouter slug `mistralai/mistral-large`; the next run posts with `model: "mistralai/mistral-large"` and `provider_name: "openrouter"`.

### Variations

- **Switching back to Direct**: The user re-opens `/model`, changes the gateway to Direct in the config panel, confirms → The TUI re-fetches `GET /v1/models` and restores the default model list.

### Edge Cases

- **OpenRouter fetch slow or times out**: The client has a 10-second timeout; on timeout, `ModelsFetchErrorMsg` is emitted.
- **Model ID not in display name map**: OpenRouter models not in `modelDisplayNames` fall through to using the OpenRouter-supplied `name` field.
- **Stars for OpenRouter models**: A user can star any model in the flat search view; stars persist to config by model ID and survive gateway switches.

---

## STORY-045: Navigating the Config Panel Sections with Keyboard

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Keyboard-centric developer who prefers not to use the mouse
**Goal**: Efficiently configure gateway, API key, and reasoning effort for QwQ 32B in one pass using only keyboard
**Preconditions**: TUI running; Groq is configured; current model is gpt-4.1; `/model` overlay is at Level 0

### Steps

1. User types `qwq` in the search filter → The flat search view narrows to "QwQ 32B [R]" with provider prefix `[Groq]`.
2. User presses Enter → Config panel opens with three sections: Gateway (focused), API Key, Reasoning Effort.
3. User presses `→` to move the gateway cursor from Direct to OpenRouter → The cursor moves to OpenRouter.
4. User presses `←` to move back to Direct → Cursor returns to Direct.
5. User presses Down → Section focus moves to API Key; status shows `● configured`.
6. User presses Down again → Section focus moves to Reasoning Effort; cursor is on "Default (← current)".
7. User presses Down within the Reasoning Effort option list to select "Medium" → The `▶` cursor moves to Medium.
8. User presses Enter → The entire config is confirmed: gateway = Direct, reasoning effort = "medium"; the model switches to `qwen-qwq-32b`.

### Variations

- **Up navigation**: The user can navigate sections in reverse with Up key; wrap-around is not implemented.

### Edge Cases

- **Enter in gateway section**: Pressing Enter while focused on the Gateway section confirms the entire config panel (not just the gateway sub-selection).
- **Key input mode activated accidentally**: If the user is in the API Key section and presses K, they can press Esc to exit input mode without leaving the config panel.

---

## STORY-046: Unstarring a Model that is No Longer Needed

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer cleaning up a cluttered starred model list
**Goal**: Remove the star from GPT-4.1 Mini so it returns to its alphabetical position
**Preconditions**: TUI running; GPT-4.1 Mini is starred (shows `★` in the OpenAI model list); `/model` overlay is open at Level 0

### Steps

1. User navigates to OpenAI and presses Enter → Level 1 shows OpenAI models; GPT-4.1 Mini appears at the top with a gold `★` prefix because it is starred.
2. User verifies the cursor is on GPT-4.1 Mini and presses `s` → The `★` is removed immediately; GPT-4.1 Mini moves to its alphabetical position; the cursor follows the model to its new position; the updated `StarredIDs()` list is written to the config file.
3. User presses Esc to exit to the provider list, then Esc again to close the overlay.

### Variations

- **Unstar during search**: The user types `mini` in the search filter; GPT-4.1 Mini (starred) appears at the top; pressing `s` removes the star.
- **Multiple starred models**: Pressing `s` on one removes only that one; others remain starred.

### Edge Cases

- **Cursor drift after unstar**: After unstarring, the cursor index may point to a different model if the list reorders; the implementation moves the cursor to follow the toggled model by its new index position.

---

## STORY-047: First-Time Setup — No Models Configured

**Type**: long
**Topic**: Model & Provider Selection
**Persona**: New user who just installed the harness and launched the TUI for the first time
**Goal**: Configure an OpenAI API key and select GPT-4.1 to send the first message
**Preconditions**: TUI has just launched; no API keys are configured on the server; the welcome hint is visible in the viewport ("no model configured" state)

### Steps

1. The TUI shows an empty viewport with a first-time welcome hint because `selectedModel == ""`.
2. User types `/model` and presses Enter → The model overlay opens; the TUI fires `GET /v1/models` and `GET /v1/providers`.
3. Models and providers load; all providers show `○` (unconfigured).
4. User navigates to OpenAI and presses Enter → Level 1 shows GPT-4.1 and GPT-4.1 Mini; both rows show `(unavailable)` with a `○` key status indicator.
5. User selects GPT-4.1 and presses Enter → The config panel opens; the API Key section shows `○ not set`.
6. User navigates to the API Key section (press Down) and presses K to activate inline key input mode → The API Key section expands to show an input field `> █`.
7. User types the OpenAI API key and presses Enter → The TUI fires `PUT /v1/providers/openai/key`; on success the API Key section updates to `● configured`.
8. User presses Enter to confirm the config panel → The model switches to `gpt-4.1`; the overlay closes; the status bar updates to `gpt-4.1  $0.00`.
9. User types a prompt and presses Enter → The first run starts; `POST /v1/runs` is called with `model: "gpt-4.1"`, `provider_name: "openai"`.

### Variations

- **Keys first via /keys**: The user opens `/keys` before `/model`, sets the OpenAI key, then returns to `/model` to pick a model.
- **Redirect from model overlay**: If the user tries to confirm a model whose provider is unconfigured, the overlay redirects to `/keys` with the cursor pre-positioned on OpenAI.

### Edge Cases

- **No server running**: If the TUI cannot reach the harness server, `GET /v1/models` fails; the model switcher shows the error in red.
- **Welcome hint timing**: The welcome hint is shown only when `selectedModel == ""` AND the viewport is empty AND no run is active.

---

## STORY-048: Esc Key Priority and Safe Dismissal of the Model Overlay

**Type**: short
**Topic**: Model & Provider Selection
**Persona**: Developer who accidentally opened the model overlay or changed their mind
**Goal**: Close the model overlay cleanly from various intermediate states without making unwanted changes
**Preconditions**: TUI running; `/model` overlay is open

### Steps (from config panel)

1. User is in the config panel (Level 2) with the API key input field active (key input mode).
2. User presses Esc → Key input mode exits; the config panel remains open.
3. User presses Esc again → The config panel closes; returns to Level 1 (model list). No model was changed.
4. User presses Esc again → Returns to Level 0 (provider list); search is cleared if it was active.
5. User presses Esc at Level 0 with no search active → The model overlay closes entirely; the chat input regains focus.

### Steps (from Level 1 with active search)

1. User is at Level 1 with a search query active.
2. User presses Esc → The search query is cleared; the view returns to the Level 1 model list unfiltered.
3. User presses Esc again → Returns to Level 0 (if at Level 1) or closes overlay (if already at Level 0).

### Variations

- **Esc at Level 0 with search**: If the user is at Level 0 with a search query, pressing Esc clears the search first; a second Esc closes the overlay.
- **Esc from /keys**: In the `/keys` overlay, Esc exits key input mode first (if active), then closes the overlay on the second press.

### Edge Cases

- **Search cleared on Esc at Level 1 exit**: When Esc transitions from Level 1 back to Level 0, any active search query is cleared.
- **No-op Esc when no overlay is open**: If the user presses Esc with no overlay open and no active run, the input is cleared if it has content, otherwise nothing happens.

---

### Profile Selection & Isolation

## STORY-049: Opening the Profile Picker for the First Time

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer using the TUI for the first time who wants to understand what profiles are available
**Goal**: Open the profile picker and see the full list of available profiles
**Preconditions**: TUI is running (`harnesscli --tui`), no profile is currently selected, server is reachable

### Steps

1. User types `/` in the input area → Slash command autocomplete dropdown appears.
2. User types `prof` → Dropdown filters to show only `/profiles`.
3. User presses Enter (or Tab) → Autocomplete closes; `/profiles` command executes.
4. TUI sets `activeOverlay = "profiles"` and fires `loadProfilesCmd` against `GET /v1/profiles` → Status bar flashes "Loading profiles..." for the duration of the fetch.
5. Server returns a JSON response containing all profiles → `ProfilesLoadedMsg` is received.
6. Profile picker opens as a rounded-border overlay centered in the terminal → Picker shows "Profiles" as the bold title, followed by up to 10 profile rows; footer shows `↑/↓ navigate  enter select  esc cancel`.
7. User reads the first row: `full` profile — dim metadata columns show name, model, source tier (`built-in`), description.

### Variations

- Typed `/profiles` in full without using autocomplete: Command still executes identically.
- Used keyboard shortcut instead: No dedicated shortcut exists for `/profiles`; the slash command is the only entry point.

### Edge Cases

- Server takes more than a moment to respond: Status bar shows "Loading profiles..." for the entire duration.
- Profile name column exceeds 20 characters: `truncateStr` clips to 19 runes and appends `…`.

---

## STORY-050: Selecting a Built-in Profile and Sending a Run

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run a read-only analysis task without risking file writes
**Goal**: Select the built-in `researcher` profile and send a prompt that will use it
**Preconditions**: TUI is running, server is reachable, `researcher` profile exists as a built-in

### Steps

1. User types `/profiles` and presses Enter → Picker opens.
2. `ProfilesLoadedMsg` arrives with built-in profiles listed → Picker renders; `researcher` entry shows: name=`researcher`, model=`gpt-4.1-mini`, source tier=`built-in`, description=`Read-only analysis, no writes`, tool count=6.
3. User presses `j` or Down arrow twice to move the highlight to `researcher`.
4. User presses Enter → `ProfileSelectedMsg` is emitted with `Entry.Name = "researcher"`; overlay closes; `selectedProfile` is set to `"researcher"`; status bar flashes "Profile: researcher".
5. User types a prompt: `Summarize all Go files in internal/harness/` and presses Enter → `startRunCmd` fires against `POST /v1/runs` with `profile: "researcher"` in the request body.
6. Run starts; server enforces `researcher` profile's tool allowlist — only `read`, `grep`, `glob`, `ls`, `web_search`, `web_fetch` are available to the agent.
7. Agent streams results; viewport renders assistant response normally.

### Variations

- User navigates with `k` / Up arrow: Moves selection up; wraps from the first entry to the last entry in the list.
- User navigates with arrow keys instead of vim keys: `tea.KeyUp` and `tea.KeyDown` produce identical behavior to `k` and `j`.

### Edge Cases

- User selects a profile and then sends multiple prompts: `selectedProfile` persists in memory for the rest of the session.
- User closes TUI and reopens it: `selectedProfile` is not persisted to disk; the new session starts with no profile selected.

---

## STORY-051: Browsing a Long Profile List with Scroll

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has installed many custom profiles at the project and user level
**Goal**: Navigate a list longer than the visible window and find a specific profile
**Preconditions**: 15 or more profiles exist across all tiers; the picker window is open

### Steps

1. User opens `/profiles` → Picker fetches all 15+ profiles; server returns summaries sorted by source tier (project first, then user, then built-in).
2. Picker renders with the first 10 entries visible and a dim footer line: `  ... 5 more`.
3. User holds `j` (Down) and navigates through all 10 visible rows.
4. User presses `j` once more (row 11) → `adjustScroll` shifts the window: `scrollOffset` becomes 1; picker re-renders showing rows 2–11.
5. User continues pressing `j` until reaching the desired profile.
6. User presses Enter → Profile is selected; overlay closes; status bar shows the profile name.

### Variations

- User navigates upward from the top: Wraps to the last entry; scroll window jumps to show the last 10 entries.
- User navigates downward from the bottom: Wraps to the first entry; scroll offset resets to 0.

### Edge Cases

- Exactly 10 profiles exist: No footer appears; no scrolling needed.
- Exactly 11 profiles exist: footer shows "... 1 more"; one `j` press reveals row 11 and hides row 1.

---

## STORY-052: Dismissing the Picker Without Selecting

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer who opened the picker by mistake or changed their mind
**Goal**: Close the profile picker without applying any profile
**Preconditions**: Profile picker is open and populated with entries

### Steps

1. User has opened `/profiles` and the picker shows several profiles → `selectedProfile` is currently `""` (no prior selection).
2. User presses `Esc` → `profilePicker.Close()` is called; `overlayActive` is set to false; `activeOverlay` is cleared to `""`.
3. Picker overlay disappears; main viewport is visible again → `selectedProfile` remains `""` — no profile was applied.
4. User types a new prompt and submits → Run fires without any profile override.

### Variations

- User had a profile already selected from an earlier `/profiles` invocation: Pressing Esc on a new picker session does not clear the previously selected profile.
- User navigates to a row but presses Esc instead of Enter: Esc always cancels without applying anything.

### Edge Cases

- User presses Esc while the picker is loading: Esc still closes it cleanly; no crash or stuck state.

---

## STORY-053: Handling a Profile Load Failure

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer working with a harnessd server that is temporarily down or misconfigured
**Goal**: Understand what happens when the profile list cannot be fetched
**Preconditions**: TUI is running; harnessd is not reachable or returns a non-200 response on `GET /v1/profiles`

### Steps

1. User types `/profiles` and presses Enter → Overlay opens; `loadProfilesCmd` fires.
2. Server is unreachable → HTTP request fails; `ProfilesLoadedMsg{Err: <error>}` is returned.
3. TUI receives `ProfilesLoadedMsg` with non-nil `Err` → `overlayActive` is set to false; overlay closes immediately.
4. Status bar shows `"Load profiles failed: <error message>"` for 3 seconds.
5. Main chat viewport is visible again → User can continue typing prompts normally.

### Variations

- Server returns HTTP 500: Same flow as a network error.
- Server returns malformed JSON: JSON decode fails; same `ProfilesLoadedMsg.Err` path.

### Edge Cases

- User immediately retries `/profiles` after a failure: Another `loadProfilesCmd` is dispatched; if the server has recovered, it succeeds normally.
- Error message is very long: Status bar truncates or wraps according to its own layout.

---

## STORY-054: No Profiles Configured — Empty State

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has just deployed harnessd without any project or user profiles defined
**Goal**: Understand the picker's behavior when no profiles are available
**Preconditions**: No `.harness/profiles/` directory exists; no `~/.harness/profiles/` directory exists; no built-in profiles embedded

### Steps

1. User types `/profiles` → Overlay opens; fetch fires.
2. Server returns `{"profiles": [], "count": 0}` → `ProfilesLoadedMsg{Entries: []}` is received.
3. Picker opens with zero entries → View renders the "Profiles" title, then a center-padded dim line: `No profiles available`, then the footer instructions.
4. User presses `j`, `k`, Up, or Down → All navigation keys are no-ops.
5. User presses Enter → No `ProfileSelectedMsg` is emitted; nothing happens.
6. User presses Esc → Picker closes cleanly; no profile is applied.

### Variations

- Built-in profiles exist but project/user profiles do not: Picker shows only the built-in tier entries.
- Profiles directory exists but is empty: Server returns zero profiles from that tier; built-ins still appear.

### Edge Cases

- Picker opens with entries after previously showing empty state: User opens `/profiles` again after operator adds profile files; `SetEntries` resets selection to index 0 and scroll to 0.

---

## STORY-055: Selecting a Project-Level Profile That Overrides a Built-in

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Operator who has customized the `researcher` profile for their project's specific toolchain
**Goal**: Verify that the project-level `researcher` profile appears in the picker and takes priority over the built-in
**Preconditions**: `.harness/profiles/researcher.toml` exists in the project root with a custom tool set including `bash`; the built-in `researcher.toml` allows only read-only tools

### Steps

1. User types `/profiles` → Picker fetches `GET /v1/profiles`.
2. Server resolves profiles using the three-tier priority order: project-level first, then user-global, then built-in; because a project-level `researcher` exists, the built-in `researcher` is suppressed → Only one `researcher` entry appears.
3. Picker renders the `researcher` entry with `SourceTier = "project"` visibly in the dim metadata column.
4. User selects `researcher` → `selectedProfile = "researcher"`; status bar shows "Profile: researcher".
5. User sends a prompt → Server loads the project-level profile; the expanded tool list (including `bash`) is available to the agent.
6. Agent can call `bash` within the project's researcher profile → The project override is transparently applied.

### Variations

- User-global profile with same name as built-in: `SourceTier = "user"` appears in the picker.
- All three tiers define the same profile name: Project-level always wins.

### Edge Cases

- Operator deletes the project-level file between the picker fetch and the run start: Server loads the user or built-in tier on run start; the picker display becomes stale but no error is shown at selection time.

---

## STORY-056: Selecting a Container-Isolated Profile

**Type**: long
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run an untrusted code-analysis task in a fully isolated container workspace
**Goal**: Select a profile with `isolation_mode = "container"` and observe that the run is executed in a container workspace transparently
**Preconditions**: A profile named `secure-sandbox` exists at user level with `isolation_mode = "container"`, `cleanup_policy = "delete_on_success"`, and a restricted tool list; harnessd is configured with a Docker daemon accessible

### Steps

1. User types `/profiles` → Picker fetches `GET /v1/profiles`; `secure-sandbox` appears with `SourceTier = "user"`.
2. User navigates to `secure-sandbox` and presses Enter → `selectedProfile = "secure-sandbox"`; status bar shows "Profile: secure-sandbox".
3. User types a prompt and presses Enter → `startRunCmd` fires with `profile: "secure-sandbox"` in the request body.
4. Server receives the run request and resolves the `secure-sandbox` profile → `IsolationMode = "container"` is read; harnessd provisions a new Docker container workspace.
5. Container spins up and `harnessd` polls `/healthz` inside it until the harness is ready → This provisioning latency appears in the TUI as a longer-than-usual delay before the first streaming event.
6. Agent run begins inside the container → Tool calls stream normally via SSE; the user does not see any container-specific UI.
7. Run completes successfully → `cleanup_policy = "delete_on_success"` causes the container and its filesystem to be destroyed.
8. Cost and token usage update in the status bar → No indication in the TUI that a container was used (isolation is transparent).

### Variations

- Profile uses `isolation_mode = "worktree"` instead: Server creates a git worktree on the host filesystem; TUI experience is identical.
- Profile uses `isolation_mode = "none"`: No workspace isolation is applied.

### Edge Cases

- Docker daemon is unavailable when the run starts: Server fails to provision the container; `run.failed` SSE event is emitted; TUI appends an error to the viewport.
- Container takes longer than the provisioning timeout: Server times out waiting for harnessd inside the container to become healthy; same `run.failed` path.
- User presses `Ctrl+C` during a container-backed run: Interrupt banner appears; server sends a cancel signal to the container.

---

## STORY-057: Switching Profiles Mid-Session

**Type**: medium
**Topic**: Profile Selection & Isolation
**Persona**: Developer who ran one task with the `reviewer` profile and now wants to run a follow-up with `bash-runner`
**Goal**: Switch the active profile between turns without restarting the TUI
**Preconditions**: TUI is running; a previous run completed with `selectedProfile = "reviewer"`.

### Steps

1. Previous run with `reviewer` profile is complete → `selectedProfile` holds `"reviewer"`.
2. User types `/profiles` → Picker fetches fresh profile list; `reviewer` and `bash-runner` are both visible.
3. User navigates to `bash-runner` → Entry shows: model=`gpt-4.1-mini`, source tier=`built-in`, description=`Script execution, pipeline tasks`, tool count=1 (`bash` only).
4. User presses Enter → `selectedProfile` is updated to `"bash-runner"`; status bar flashes "Profile: bash-runner".
5. User types prompt: `Run the regression test suite and report failures` and presses Enter → New run fires with `profile: "bash-runner"`; agent has only `bash` in its tool allowlist.
6. Agent runs bash commands to execute the tests; results stream into the viewport.

### Variations

- User opens `/profiles` while a run is active: Profile picker can be opened but selecting a profile only affects the next run.

### Edge Cases

- User switches profiles between turns of a multi-turn conversation: Each `startRunCmd` carries the current `selectedProfile` at the time of submission; conversation ID is preserved regardless of profile changes.
- User selects the same profile they already have selected: Status bar still flashes "Profile: reviewer"; no observable difference.

---

## STORY-058: Using `harnesscli --list-profiles` Without the TUI

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Operator writing CI scripts or automation that needs to enumerate available profiles
**Goal**: List all profiles from the command line and exit, without launching the TUI
**Preconditions**: harnessd is running and reachable; at least some profiles exist across tiers

### Steps

1. Operator runs: `harnesscli --list-profiles` → `listProfilesCmd` is called; program issues `GET /v1/profiles`.
2. Server returns profiles from all three tiers in resolution order → Each profile is printed to stdout in a fixed-width tabular format: `Name: <name padded to 30>  | Description: <description padded to 40>  | Model: <model>` → One profile per line.
3. Program exits with code 0.

### Variations

- No profiles are configured anywhere: `listProfilesCmd` prints `"No profiles available"` and exits 0.
- Server is not reachable: Error message printed to stderr; exits with code 1.

### Edge Cases

- Profile has no description: Output shows `(no description)`.
- Profile has no model override: Output shows `(default)` in the model column.
- `--list-profiles` and `--tui` are both specified: `--list-profiles` is checked first; TUI is never launched.

---

## STORY-059: Reading Profile Metadata Before Selecting

**Type**: short
**Topic**: Profile Selection & Isolation
**Persona**: Developer who is unfamiliar with the available profiles and wants to understand each before committing to one
**Goal**: Read and compare the description, model, tool count, and source tier of multiple profiles before selecting
**Preconditions**: Profile picker is open and populated with a mix of project, user, and built-in profiles

### Steps

1. Picker is open; user reads the first row of the list → Format: `  <name padded 20>  <model padded 20>  <source tier padded 10>  <description up to 40 chars>`.
2. User presses `j` to move down the list, reading each row.
3. User wants to see the full description of a profile whose text is truncated at 40 characters → No expand/drill-down exists; user must refer to the profile TOML file or server docs for full details.
4. User navigates to the profile with the right combination of model and tier → Highlight sits on the target row.
5. User presses Enter → Profile is selected; status bar confirms the name; overlay closes.

### Variations

- User wants to know the exact list of allowed tools: Tool count is shown in the picker metadata but individual tool names are not.

### Edge Cases

- Source tier column shows an unexpected value: The picker renders it verbatim; no validation or normalization occurs in the TUI.

---

## STORY-060: Profile Selection with a Worktree-Isolated Profile and BaseRef

**Type**: long
**Topic**: Profile Selection & Isolation
**Persona**: Developer who wants to run a code-review agent on a feature branch without touching their working tree
**Goal**: Select a worktree-isolated profile that targets a specific base branch, observe that the run is executed in an isolated worktree
**Preconditions**: Project has a profile at `.harness/profiles/branch-reviewer.toml` with `isolation_mode = "worktree"`, `base_ref = "main"`, `cleanup_policy = "delete"`, and tools `["read", "grep", "glob", "ls", "git_diff"]`; git repository is available to harnessd

### Steps

1. User types `/profiles` → Picker opens; `branch-reviewer` appears with `SourceTier = "project"`.
2. User selects `branch-reviewer` → `selectedProfile = "branch-reviewer"`; status bar shows "Profile: branch-reviewer".
3. User types: `Review changes on the current branch against main and list all issues` and submits → Run fires with `profile: "branch-reviewer"` in the request.
4. Server reads the profile; `IsolationMode = "worktree"` and `BaseRef = "main"` are resolved → harnessd calls `git worktree add` to create a new worktree based on `main`.
5. Agent run starts in the new worktree → Tool calls execute inside the worktree directory; the agent's view of the filesystem is isolated from the user's working tree.
6. Agent completes the code review and streams findings into the TUI viewport.
7. Run completes → `cleanup_policy = "delete"` causes harnessd to call `git worktree remove` and delete the temporary directory.
8. User can immediately start a new run or switch profiles → No cleanup steps needed in the TUI.

### Variations

- User sets `base_ref = "develop"` in the profile: Worktree is created from `develop` instead of `main`.
- `cleanup_policy = "keep"`: Worktree is preserved after the run for manual inspection.

### Edge Cases

- Git worktree creation fails (e.g., conflicting branch name or dirty index): Server returns a run failure; TUI appends an error to the viewport.
- User's repository has no `main` branch: Worktree creation fails at the server; same error path.
- User opens `/profiles` while a worktree run is active: Selecting a new profile only affects the next run.

---

### Conversation Management

## STORY-061: Clearing a Long Conversation to Start Fresh

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who has been iterating on a problem for 30 minutes and wants a clean slate without quitting the TUI
**Goal**: Reset the viewport, transcript accumulator, and assistant text so the next run starts from an empty state
**Preconditions**: TUI is open with multiple turns of conversation visible in the viewport; no run is currently active

### Steps

1. User types `/clear` in the input area → The slash-complete dropdown opens and shows `clear` as the only match.
2. User presses Enter to accept → The command executes immediately.
3. Viewport is replaced with a fresh empty `viewport.New(width, height)` → All previous message bubbles and tool call blocks disappear.
4. `transcript []TranscriptEntry` is set to `nil` → The in-memory transcript accumulator is empty.
5. `lastAssistantText` is reset to `""` and `responseStarted` is set to `false`.
6. The thinking bar is cleared → No stale reasoning text appears above the input.
7. Status bar shows "Conversation cleared" for 3 seconds, then auto-dismisses.

### Variations

- **Autocomplete shortcut**: User types `/cl` then presses Tab → dropdown auto-executes `/clear`.
- **Clear with active run**: `/clear` is submitted while a run is in progress — the command still clears the transcript and viewport immediately.

### Edge Cases

- **Clear an already-empty conversation**: `/clear` with nothing in the viewport still executes cleanly; status bar shows "Conversation cleared".
- **conversationID is not reset**: After `/clear`, `conversationID` still holds the first run's ID; the next message sent will still pass the same conversation linkage to the server.

---

## STORY-062: Establishing a Multi-Turn Conversation Identity

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer building a multi-step coding task who needs the harness to link successive runs together
**Goal**: Understand how the TUI establishes and propagates the conversationID across turns
**Preconditions**: TUI is open, no prior messages sent in this session (`conversationID` is `""`)

### Steps

1. User types "Scaffold a Go HTTP handler for POST /api/users" and presses Enter → A user-role `TranscriptEntry` is appended.
2. TUI calls `startRunCmd(baseURL, prompt, conversationID="", ...)` → POST to `/v1/runs` with no `conversation_id` field in the JSON body.
3. Server responds with `{"run_id": "run-7f3a9c"}` → `RunStartedMsg{RunID: "run-7f3a9c"}` is emitted.
4. TUI sets `m.conversationID = "run-7f3a9c"` → The first run ID becomes the stable conversation identifier for this session.
5. SSE stream delivers assistant deltas; response appears in the viewport; run completes → An assistant-role `TranscriptEntry` is appended.
6. User types a follow-up: "Now add input validation middleware" and presses Enter → `startRunCmd` is called with `conversationID = "run-7f3a9c"`.
7. POST to `/v1/runs` carries `"conversation_id": "run-7f3a9c"` → Server links this turn to the same conversation.
8. Second run streams and completes → Transcript now has 4 entries: user/assistant/user/assistant.

### Variations

- **No follow-up**: User sends only one message; `conversationID` is set but never used in a subsequent POST.
- **Multiple conversations in one session**: After `/clear` the viewport resets but `conversationID` is preserved.

### Edge Cases

- **First POST fails**: `conversationID` remains `""`; the user can retry and the retry will again POST without a conversation ID.
- **Server returns malformed run_id**: JSON decode fails; `conversationID` is never set.

---

## STORY-063: Exporting a Conversation Transcript to Markdown

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who wants to preserve a conversation for a code review or share findings with a colleague
**Goal**: Write the current session's transcript entries to a timestamped markdown file
**Preconditions**: At least one full turn (user message + assistant response) has completed

### Steps

1. User types `/export` and presses Enter → The slash-complete dropdown auto-executes.
2. `executeExportCommand` takes a snapshot copy of `m.transcript` → A new slice is allocated via `copy()`.
3. `transcriptexport.NewExporter(defaultExportDir())` is constructed → Output directory resolves to `~/Library/Caches/harness/transcripts` on macOS.
4. `exporter.Export(snapshot)` runs in a background `tea.Cmd` → The output directory is created if it does not exist; the file is named `transcript-20260323-114704.md`.
5. Markdown file is written with a `# Conversation Transcript` header and sections for each entry.
6. `ExportTranscriptMsg{FilePath: "..."}` is returned → Status bar shows "Transcript saved to ..." for 3 seconds.

### Variations

- **Tab completion**: User types `/ex` then Tab → single match auto-executes `/export`.
- **Export after /clear**: The transcript slice is `nil`; the export still runs and produces a valid (empty) markdown file.

### Edge Cases

- **Directory not writable**: `ExportTranscriptMsg{FilePath: ""}` is returned; status bar shows "Export failed" for 3 seconds.
- **Export during active run**: The snapshot is taken at the moment `/export` is issued; any assistant deltas after the snapshot are not included.

---

## STORY-064: Recovering a Previous Message with History Navigation

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who typed a long prompt, sent it, and now wants to send a slightly modified version without retyping
**Goal**: Use the up arrow in the input area to recall the last submitted message and edit it before re-sending
**Preconditions**: At least one message has been submitted in the current session; no run is currently active

### Steps

1. Input area is empty and focused → cursor shows after `❯`.
2. User presses the up arrow key → `inputarea` calls `h.Up(currentText)` — since `currentText` is `""`, the empty draft is saved and the most recent history entry is loaded.
3. Input area now shows the previously submitted message text.
4. User edits the text (adds, removes, or changes words).
5. User presses Enter to submit the edited prompt → The edited text is sent; the edited text is pushed to history as a new entry.

### Variations

- **Navigate multiple steps back**: User presses up again after step 3 → the second-most-recent message loads.
- **Return to draft**: After navigating into history, user presses the down arrow → history position moves forward; pressing down past the most recent entry restores the saved empty draft.

### Edge Cases

- **History is empty**: User presses up on the first message of a session → `h.Up` returns the current text unchanged.
- **History cap at 100 entries**: After 100 submitted messages, the 101st push evicts the oldest entry.
- **History does not persist across TUI restarts**: `History` is an in-memory value type on `inputarea.Model`.

---

## STORY-065: Copying the Last Assistant Response to the Clipboard

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who wants to paste the assistant's code snippet or explanation into an editor without selecting text in the terminal
**Goal**: Copy the full accumulated assistant response from the current (or most recently completed) run to the system clipboard
**Preconditions**: At least one run has completed; `lastAssistantText` is non-empty; terminal supports OSC52

### Steps

1. User presses `ctrl+s` → TUI matches `m.keys.Copy` in the key handler.
2. `CopyToClipboard(m.lastAssistantText)` is called → OSC52 escape sequence `\033]52;c;<base64-encoded-text>\a` is written to stdout.
3. Terminal forwards the clipboard write to the OS clipboard.
4. `CopyToClipboard` returns `true` → Status bar shows "Copied!" for 3 seconds, then auto-dismisses.

### Variations

- **After multiple turns**: `lastAssistantText` holds only the most recent run's assistant response; pressing `ctrl+s` copies only the last turn's response.
- **Pressing ctrl+s mid-stream**: If a run is active and the assistant is still streaming, `lastAssistantText` holds what has accumulated so far.

### Edge Cases

- **Headless/CI terminal**: `IsHeadless()` returns true; `CopyToClipboard` returns `false` without writing OSC52 → status bar shows "Copy unavailable".
- **Empty lastAssistantText**: `ctrl+s` copies an empty string; status bar shows "Copied!" but clipboard is empty.
- **Terminal does not support OSC52**: The write succeeds at the OS level, `CopyToClipboard` returns `true`, but the terminal silently ignores the escape sequence.

---

## STORY-066: Exporting After a Multi-Turn Session

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer who has had a 10-turn conversation and wants a complete markdown record of all user and assistant messages
**Goal**: Export all turns accumulated in the session transcript to a single markdown file
**Preconditions**: 10 turns completed; `transcript` has 20 entries (alternating user/assistant); no run is active

### Steps

1. User types `/export` and presses Enter.
2. `executeExportCommand` takes a snapshot: `copy(snapshot, m.transcript)` — 20 entries are copied.
3. Background `tea.Cmd` runs `exporter.Export(snapshot)`.
4. Markdown file is written with all 20 sections in submission order, separated by `---` horizontal rules.
5. `ExportTranscriptMsg{FilePath: "..."}` is received → Status bar shows "Transcript saved to …" for 3 seconds.
6. User opens the file in a text editor → Sees the full conversation readable as a structured markdown document.

### Variations

- **Export mid-conversation**: User exports after 5 turns, continues chatting for 5 more, then exports again → Two files are created.
- **Export-then-clear**: User exports, then runs `/clear` → The exported file is preserved on disk; the in-memory `transcript` is set to `nil`.

### Edge Cases

- **Very long assistant responses**: Each entry's `Content` field is written verbatim; there is no size limit.
- **Concurrent export calls**: Two separate files are written with slightly different timestamps.
- **Tool entries in transcript**: If any tool-role entries are present they render as `## Tool: bash [2:31 PM]`.

---

## STORY-067: Using /clear to Reset Between Independent Tasks

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer who uses a single TUI session for multiple unrelated tasks throughout the day
**Goal**: Clear one task's conversation history before starting a new unrelated task
**Preconditions**: Task A conversation is complete; at least 5 turns are visible; no run is active

### Steps

1. User has finished task A; viewport shows all its turns.
2. User types `/clear` → Dropdown shows `clear`; user presses Enter.
3. `executeClearCommand` runs: viewport is fresh, transcript is nil, lastAssistantText is reset, responseStarted is false, activeAssistantLineCount is reset, thinking bar is cleared.
4. Status bar shows "Conversation cleared" for 3 seconds.
5. Status bar still shows the cumulative cost from task A (cost counter is not reset by `/clear`).
6. User types the first message for task B and sends it.
7. Run POST carries `conversationID` from task A (conversation ID is not reset by `/clear`).

### Variations

- **User wants a true fresh conversation identity**: There is no in-TUI way to reset `conversationID` without restarting the process.

### Edge Cases

- **Viewport scroll offset**: After `/clear`, the new viewport starts at the bottom with zero scroll offset.
- **Tool state maps not cleared**: `toolViews`, `toolTimers`, etc. are not explicitly cleared by `/clear`; they hold references to past tool call data but will be overwritten by new runs.

---

## STORY-068: Navigating Long History Within a Session

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who is refining a prompt through many iterations
**Goal**: Navigate backward through up to 100 history entries using the up arrow, find a specific prior message, and send it again
**Preconditions**: 15 or more messages have been submitted in the current session; input area is focused and empty

### Steps

1. User presses up arrow three times → History position moves from draft (-1) to entries at indices [0], [1], [2] (newest-first order).
2. On the third press, input area shows the message from 3 turns ago.
3. User presses Enter to submit it unchanged → The recalled text is submitted; it is pushed to history as a new entry; history position resets to draft.
4. Input area returns to empty, ready for the next message.

### Variations

- **Modify before sending**: User presses up twice, edits the recalled text, then presses Enter → The modified text is sent; the original entry in history is unchanged.
- **Abandon recall**: User navigates up into history, then presses the down arrow until returning to the draft → No message is submitted.

### Edge Cases

- **Up past oldest entry**: User keeps pressing up after reaching the oldest entry → `h.Up` is a no-op.
- **History after /clear**: `/clear` does not call `h.Clear()` on the input area's history; history entries from before `/clear` remain navigable.

---

## STORY-069: Observing Transcript Accumulation Across Turns

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer or QA engineer who wants to verify that transcript entries are being recorded correctly
**Goal**: Confirm that each user message and assistant response is recorded as a separate `TranscriptEntry` with role, content, and timestamp
**Preconditions**: TUI has just been opened; no messages sent yet; `transcript` is `nil`

### Steps

1. User sends message "What is the capital of France?" → At submit time, a `TranscriptEntry{Role: "user", Content: "What is the capital of France?", Timestamp: <now>}` is appended; transcript length is now 1.
2. `RunStartedMsg{RunID: "run-a1b2"}` arrives → `conversationID` is set; `lastAssistantText` is reset to `""`.
3. SSE stream delivers delta events → Each delta is concatenated into `m.lastAssistantText`.
4. `SSEDoneMsg{EventType: "run.completed"}` arrives → `lastAssistantText` is appended as `TranscriptEntry{Role: "assistant", ...}`; transcript length is now 2.
5. User sends message "And Germany?" → Another user entry appended; transcript length is 3.
6. Run completes with response "Berlin." → Another assistant entry appended; transcript length is 4.
7. User types `/export` → All 4 entries are exported in order.

### Variations

- **No assistant deltas received**: Run completes with zero delta events → `lastAssistantText` is `""`; no assistant entry is appended.

### Edge Cases

- **SSE stream interrupted**: `SSEDoneMsg` never arrives → `lastAssistantText` accumulates but is never flushed to the transcript; the partial response is lost from the transcript.
- **Timestamps are local time**: Two entries at the same clock minute will show the same time string even if seconds differ.

---

## STORY-070: Handling an Export Failure Gracefully

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer working on a read-only filesystem
**Goal**: Attempt to export and receive a clear failure message rather than a silent error
**Preconditions**: `defaultExportDir()` resolves to a directory the current user cannot write to; transcript has content

### Steps

1. User types `/export` and presses Enter.
2. Background `tea.Cmd` runs `exporter.Export(snapshot)`.
3. `os.MkdirAll(outputDir, 0o755)` fails with a permission error → `Export` returns `("", error)`.
4. `ExportTranscriptMsg{FilePath: ""}` is returned to the TUI update loop.
5. The `case ExportTranscriptMsg:` handler checks `msg.FilePath == ""` → calls `m.setStatusMsg("Export failed")`.
6. Status bar shows "Export failed" for 3 seconds, then auto-dismisses.
7. Transcript accumulator is unchanged — no data was lost.

### Variations

- **WriteFile fails but MkdirAll succeeded**: Same flow; `Export` returns an error; status bar shows "Export failed".

### Edge Cases

- **Partial file written**: If `os.WriteFile` fails mid-write, the incomplete file may exist on disk; the TUI does not attempt cleanup.
- **Retry after failure**: User fixes permissions and runs `/export` again → A new snapshot is taken; a new timestamped file is written.

---

## STORY-071: Understanding What /clear Does Not Reset

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who has used `/clear` and is unsure whether the conversation is truly isolated from the previous one
**Goal**: Understand the exact scope of what `/clear` resets versus what it preserves
**Preconditions**: User has had a long session; ran `/clear`; is about to send a new message

### Steps

1. User runs `/clear` → viewport and transcript are wiped; status bar confirms "Conversation cleared".
2. User notices status bar still shows the cumulative cost (e.g. "$0.12") → cost counter is NOT reset by `/clear`.
3. User opens `/context` → Token count displayed is the total since TUI launch, not since the last clear.
4. User opens `/stats` → Usage heatmap shows all runs since TUI launch, including those before the clear.
5. User sends a new message → POST carries the same `conversationID` as before the clear → Server still groups this run with all prior runs in the session.
6. User realizes: `/clear` is a viewport and transcript reset only; it does not create a new conversation identity or reset cost/token counters.

### Variations

- **True session isolation**: To get a genuinely fresh conversation with a new ID, the user must quit (`/quit`) and relaunch `harnesscli --tui`.

### Edge Cases

- **Input history preserved**: The input area's history buffer (up to 100 entries) is not cleared by `/clear`.

---

## STORY-072: Drafting a Message, Abandoning It, and Recalling It Later

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who starts drafting a message, gets interrupted, navigates to a past message to check wording, then returns to the in-progress draft
**Goal**: Use history navigation without losing the partially typed message in progress
**Preconditions**: Input area contains a partially typed message "Refactor the auth module to use"; user has at least 2 prior messages in history

### Steps

1. Input area shows "Refactor the auth module to use" (partial draft; not yet submitted).
2. User presses up arrow → `h.Up("Refactor the auth module to use")` is called; the current text is saved as the draft; the most recent history entry is loaded.
3. User presses up again → second most-recent message loads.
4. User presses down → first most-recent message loads again.
5. User presses down again → `h.Down()` returns the saved draft text "Refactor the auth module to use" and resets `pos` to -1 (AtDraft).
6. Input area shows "Refactor the auth module to use" exactly as left → User can continue editing without retyping.

### Variations

- **Draft abandoned**: User navigates into history and presses Enter on a historical entry instead of returning to draft → The draft "Refactor the auth module to use" is permanently lost.

### Edge Cases

- **Empty draft navigation**: If the input is empty when up is first pressed, an empty string is saved as the draft; returning to draft via down produces an empty input.
- **Draft is never submitted to history**: The saved draft text is held only in `h.draft`; it is not pushed to the history entries slice unless the user eventually submits it.

---

### Planning Mode (Extended Thinking)

## STORY-073: Toggling Plan Mode Before a Run Starts

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who wants the agent to show its plan before executing
**Goal**: Enable plan mode so the next run pauses for approval before acting
**Preconditions**: TUI is open, no active run, plan overlay is in `PlanStateHidden`

### Steps
1. User presses `ctrl+o` with no active tool call and no run in progress → The key matches `m.keys.PlanMode` binding; because `activeToolCallID` is empty, the ExpandTool branch is a no-op; plan mode toggle state is noted for the next submitted run.
2. User types a prompt and presses Enter → Run starts via POST `/v1/runs`; the server is aware of plan mode intent (passed as a run parameter).
3. Server produces a plan before calling any tools → A plan SSE event arrives; `planoverlay.Model.Show(planText)` is called; overlay transitions from `PlanStateHidden` to `PlanStatePending`.
4. Full-screen plan overlay appears → Header shows "📋 Plan Mode" with a yellow "[Awaiting Approval]" badge; plan markdown is rendered in the scroll area; footer hint reads "y approve  n reject  ↑/↓ scroll".
5. User reads the plan and presses `y` → `PlanApprovedMsg{}` is emitted; overlay transitions to `PlanStateApproved`; green "[Approved ✓]" badge replaces the yellow one; agent continues execution.

### Variations
- **Run submitted without plan mode active**: No plan overlay appears; agent proceeds directly to tool execution.

### Edge Cases
- **`ctrl+o` pressed while overlay is open**: The overlay already owns input focus; the keypress is ignored or scrolls the overlay.
- **Plan text is empty string**: `planoverlay.View()` renders the placeholder "(no plan text)"; the overlay is still shown and awaits approval.

---

## STORY-074: Reviewing and Approving a Multi-Step Plan

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Careful developer reviewing an agent's proposed approach to a complex refactor
**Goal**: Read the full plan before the agent makes any file changes
**Preconditions**: Plan overlay is in `PlanStatePending` with a long multi-step plan (more than the visible height allows)

### Steps
1. Plan overlay appears in `PlanStatePending` state → Rounded-border box fills the terminal; header shows "📋 Plan Mode" and "[Awaiting Approval]" badge; footer shows `  ... N more line(s)`.
2. User presses `↓` (down arrow) → `planoverlay.Model.ScrollDown(maxLines)` is called; offset increments by one; next lines of the plan scroll into view.
3. User continues pressing `↓` until all content is visible → When `end >= totalLines`, the "more line(s)" footer disappears.
4. User presses `↑` (up arrow) to re-read the first step → `planoverlay.Model.ScrollUp()` is called; first lines scroll back into view.
5. User presses `y` to approve → `planoverlay.Model.Approve()` transitions state to `PlanStateApproved`; badge turns green ("[Approved ✓]"); `PlanApprovedMsg{}` is emitted; the run continues.

### Variations
- **Plan fits entirely on screen**: No "more line(s)" footer appears; scroll keys are still accepted but have no visible effect.
- **User reads plan then waits**: The overlay stays in `PlanStatePending` indefinitely; no timeout.

### Edge Cases
- **Scrolling past the bottom**: `ScrollDown` clamps at `maxLines - Height`; repeated presses do not advance the offset further.
- **Terminal is resized while overlay is open**: `tea.WindowSizeMsg` updates dimensions; content reflows to the new dimensions without resetting the scroll offset.

---

## STORY-075: Rejecting a Plan and Observing the Outcome

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who disagrees with the agent's proposed approach
**Goal**: Stop the agent from executing a plan that would take the wrong approach
**Preconditions**: Plan overlay is in `PlanStatePending` with a plan the user wants to reject

### Steps
1. Plan overlay is visible with "[Awaiting Approval]" badge → User reads the plan and determines the approach is wrong.
2. User presses `n` → `planoverlay.Model.Reject()` transitions state to `PlanStateRejected`; badge turns red ("[Rejected ✗]"); `PlanRejectedMsg{}` is emitted.
3. TUI model receives `PlanRejectedMsg` → The overlay is hidden; the run is signaled to stop or the server receives a denial via POST `/v1/runs/{id}/deny`; the viewport shows a message indicating the plan was rejected.
4. User is returned to the input area → The user can type a follow-up message explaining what approach they want instead.

### Variations
- **User rejects and immediately sends a correction**: After rejection, the user types "Instead of X, please do Y" and submits; a new run starts with the corrected guidance.

### Edge Cases
- **Reject called when state is not Pending**: `planoverlay.Model.Reject()` is a no-op if state is already `PlanStateApproved`, `PlanStateRejected`, or `PlanStateHidden`.
- **Plan rejected but SSE stream continues sending events**: The TUI should cancel the run; stale SSE tool events arriving after rejection should be dropped.

---

## STORY-076: Distinguishing the Thinking Bar from the Plan Overlay

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer new to the TUI who is confused by two different "thinking" indicators
**Goal**: Understand which part of the UI indicates reasoning in progress versus a plan awaiting approval
**Preconditions**: TUI is open; user has submitted a prompt; model supports extended thinking

### Steps
1. User submits a prompt → Run starts; the `thinkingbar.Model` activates.
2. `assistant.thinking.delta` SSE events arrive → Each delta is appended via `appendThinkingDelta(delta)`; `normalizeThinkingLabel` collapses whitespace and prepends "Thinking: "; the thinking bar renders above the input area as a single line like "Thinking: analyzing the codebase structure...".
3. More `assistant.thinking.delta` events stream in → The label accumulates all delta text; the bar updates in place above the input; it does NOT fill the screen and does NOT show approve/reject controls.
4. Reasoning completes; a plan SSE event arrives → `clearThinkingBar()` is called first; the thinking bar disappears; `planoverlay.Model.Show(planText)` is called; the full-screen plan overlay replaces the main view.
5. User now sees a visually distinct full-screen overlay with a rounded border, header "📋 Plan Mode", state badge "[Awaiting Approval]", and footer hint "y approve  n reject  ↑/↓ scroll".

### Variations
- **Model does not emit thinking deltas**: The thinking bar never activates; only the plan overlay appears if plan mode is active.

### Edge Cases
- **Thinking delta arrives with empty content string**: The `content != ""` guard prevents `appendThinkingDelta` from being called.
- **clearThinkingBar is called before any delta arrives**: `thinkingBar` is reset to `thinkingbar.New()` (inactive); this is safe and idempotent.

---

## STORY-077: Plan Overlay Blocks Input While Pending

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who tries to type a new message while a plan is awaiting approval
**Goal**: Verify that the plan overlay correctly locks out other input until resolved
**Preconditions**: Plan overlay is in `PlanStatePending`; user did not notice the overlay and tries to continue typing

### Steps
1. Plan overlay is in `PlanStatePending` → Full-screen overlay is rendered; input area is obscured or rendered behind the overlay.
2. User types characters → Characters are NOT entered into the input area; keystrokes are routed to the plan overlay's own key handler.
3. User presses a letter key that is not `y` or `n` → The plan overlay ignores unrecognized keys; no state change occurs.
4. User presses `y` or `n` → Approval or rejection is processed; control returns to the main chat view.

### Variations
- **User tries to open `/help` while plan is pending**: Slash-command autocomplete is not active since the input area does not receive keystrokes.
- **User presses `ctrl+c` while plan is pending**: If `runActive` is true, the run is cancelled; `cancelRun()` is called; the plan overlay is hidden.

### Edge Cases
- **Two overlays stacking**: The plan overlay and another overlay cannot both be open simultaneously.
- **Scroll keys while pending**: `↑` and `↓` are handled by the plan overlay's own scroll logic; the conversation history does not scroll.

---

## STORY-078: Expanding an Active Tool Call with ctrl+o (Planning Context)

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who wants to see the full output of a running bash tool call
**Goal**: Expand the truncated tool call output to see all lines
**Preconditions**: A tool call is active (`activeToolCallID` is set); tool output exceeds `MaxLines` and is showing the "+N more lines (ctrl+o to expand)" truncation hint

### Steps
1. A tool call block appears in the viewport with truncated output → The block shows a spinner, elapsed time, the command, and a truncation hint.
2. User presses `ctrl+o` → Because `activeToolCallID != ""`, the `ExpandTool` branch fires; `m.toolExpanded[activeToolCallID]` is toggled to true; `rerenderActiveToolView()` is called.
3. `rerenderActiveToolView` calls `appendToolUseView` with `Expanded = true` → The tool block is re-rendered at the tail of the viewport via `ReplaceTailLines`.
4. User presses `ctrl+o` again → `m.toolExpanded[activeToolCallID]` toggles back to false; `rerenderActiveToolView()` re-renders the collapsed view.

### Variations
- **Tool call has already completed when ctrl+o is pressed**: `activeToolCallID` still points to the last completed call; the expand/collapse toggle still works.
- **No active tool call when ctrl+o is pressed**: `activeToolCallID == ""`; the ExpandTool case is a no-op.

### Edge Cases
- **ctrl+o pressed during plan pending state**: The plan overlay holds modal focus; the ExpandTool branch is not reachable while the plan overlay is the active modal.
- **ctrl+o and PlanMode share the same key binding**: Both `m.keys.PlanMode` and `m.keys.ExpandTool` are bound to `ctrl+o`; the `switch` in `Update` matches `ExpandTool` first when `activeToolCallID != ""`.

---

## STORY-079: Cancelling a Run While Plan Is Pending

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who realizes they want to abort entirely after seeing a plan they cannot fix with just a rejection
**Goal**: Cancel the run completely, not just reject the plan
**Preconditions**: Plan overlay is in `PlanStatePending`; user has decided to abandon the entire run

### Steps
1. Plan overlay is showing "[Awaiting Approval]" → User decides the entire task framing is wrong.
2. User presses `ctrl+c` → The `Quit` key binding matches; because `m.runActive == true` and `m.cancelRun != nil`, the cancellation path fires: `m.cancelRun()` is called, `m.runActive` is set to false, and `setStatusMsg("Interrupted")` schedules a status bar flash.
3. Plan overlay state becomes stale → The overlay is still in `PlanStatePending`; `planoverlay.Model.Hide()` is called; `IsVisible()` returns false.
4. SSE stream is closed by `cancelRun` → No further SSE events arrive; the thinking bar (if active) is cleared; the status bar flashes "Interrupted" for 3 seconds.
5. User is returned to the input area → The full-screen overlay is gone; the user can start a new run with a corrected prompt.

### Variations
- **cancelRun is nil when ctrl+c is pressed**: The nil guard `m.cancelRun != nil` prevents a panic.

### Edge Cases
- **ctrl+c pressed twice**: The first Ctrl+C cancels the run and clears `m.cancelRun`; a second Ctrl+C finds `m.runActive == false`, falls through to `tea.Quit`, and exits the TUI entirely.
- **Plan overlay Hide is not called after cancellation**: If the TUI only sets `m.runActive = false` but forgets to hide the plan overlay, the overlay would remain rendered on screen in `PlanStatePending` state even though the run is dead.

---

## STORY-080: Plan Overlay State Transitions — Full Lifecycle

**Type**: long
**Topic**: Planning Mode (Extended Thinking)
**Persona**: QA engineer validating the complete state machine of the plan overlay
**Goal**: Exercise all four `PlanState` values and all legal transitions in a single session
**Preconditions**: TUI is open; plan overlay starts in `PlanStateHidden`

### Steps

**Phase 1 — Hidden to Pending:**
1. Run starts; server emits a plan event → `planoverlay.Model.Show(planText)` is called; state transitions `PlanStateHidden → PlanStatePending`; `IsVisible()` returns true.

**Phase 2 — Pending to Approved:**
2. User presses `y` → `planoverlay.Model.Approve()` transitions `PlanStatePending → PlanStateApproved`; badge changes from yellow "[Awaiting Approval]" to green "[Approved ✓]"; `PlanApprovedMsg{}` is emitted; overlay is eventually hidden via `.Hide()` → `PlanStateHidden`.

**Phase 3 — Pending to Rejected:**
3. On the next run, a new plan arrives → `planoverlay.Model.Show(newPlanText)` is called again; scroll offset is reset; state goes to `PlanStatePending`.
4. User presses `n` → `planoverlay.Model.Reject()` transitions `PlanStatePending → PlanStateRejected`; badge changes to red "[Rejected ✗]"; `PlanRejectedMsg{}` is emitted; overlay is hidden via `.Hide()`.

**Phase 4 — Idempotent no-op calls:**
5. `Approve()` called on a `PlanStateApproved` model → No-op; state remains `PlanStateApproved`.
6. `Reject()` called on a `PlanStateRejected` model → No-op.
7. `Hide()` called on any state → Always transitions to `PlanStateHidden`; `IsVisible()` returns false; `View()` returns "".

### Variations
- **Show called on an already-Pending model**: The scroll offset is reset and plan text is replaced; the state stays `PlanStatePending`.
- **Show called on an Approved model**: Transitions back to `PlanStatePending`; the user must re-approve the new plan.

### Edge Cases
- **Value semantics guarantee**: All method calls return a new `planoverlay.Model` copy; the original is unchanged; no mutex needed.
- **Concurrent copies in test goroutines**: `TestTUI055_ConcurrentModels` demonstrates 10 goroutines each holding their own `Model` running through the full lifecycle without data races.

---

## STORY-081: Thinking Bar During Extended Reasoning Without Plan Mode

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer using a reasoning-capable model (e.g., o3 or Claude with extended thinking) who did NOT opt into plan mode
**Goal**: Observe streaming reasoning text without the approval gate
**Preconditions**: TUI is open; a reasoning-capable model is selected; plan mode is not activated; run is in progress

### Steps
1. User submits a complex coding question → Run starts; the model enters its reasoning phase.
2. `assistant.thinking.delta` SSE events begin arriving → Each event has `content` field with a reasoning text fragment; `appendThinkingDelta(delta)` is called for each; `m.thinkingText` accumulates all fragments.
3. `normalizeThinkingLabel` processes the accumulated text → Collapses all whitespace; prepends "Thinking: "; produces a single-line label like "Thinking: I need to analyze the function signatures...".
4. `thinkingbar.Model{Active: true, Label: label}` is set → The thinking bar renders above the input area as one line; it is NOT full-screen and does NOT show approve/reject controls.
5. Reasoning completes; `assistant.content.delta` arrives → `clearThinkingBar()` is called; `m.thinkingText` is reset; the thinking bar disappears; the assistant's response begins streaming.

### Variations
- **No thinking deltas are emitted by the model**: The thinking bar remains inactive for the entire run.
- **Reasoning text is very long**: `normalizeThinkingLabel` still produces a single line by collapsing all whitespace.
- **Tool call starts during reasoning**: `handleToolStart` calls `clearThinkingBar()` before setting up the tool call block.

### Edge Cases
- **`assistant.thinking.delta` with content = ""**: The `p.Content != ""` guard prevents `appendThinkingDelta` from being called.
- **Thinking bar and plan overlay coexist**: In practice these are mutually exclusive in timing; the thinking bar clears when a plan event arrives.

---

## STORY-082: Narrow Terminal — Plan Overlay Layout at 80x24

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer running the TUI in a constrained terminal window
**Goal**: Verify the plan overlay renders correctly at minimum practical terminal width
**Preconditions**: Terminal is 80 columns wide and 24 rows tall; plan overlay is in `PlanStatePending`

### Steps
1. Plan overlay is shown at 80x24 → `innerWidth = 80 - 6 = 74` (border 2 + padding 4); `contentHeight = 24 - 2 - 2 = 20`; `visibleLines = 20 - 2 = 18`.
2. The header line renders "📋 Plan Mode" left-aligned, "[Awaiting Approval]" right-aligned within the 74-character inner width.
3. The separator renders as 74 dash characters → Visible as a full-width horizontal rule.
4. Plan content lines fill up to 18 visible rows.
5. The footer hint "  y approve  n reject  ↑/↓ scroll" appears at the bottom.
6. The snapshot `TUI-055-plan-80x24.txt` captures this exact layout.

### Variations
- **120x40 terminal**: `innerWidth = 114`; more plan lines are visible at once.
- **Very small terminal (5x5)**: `innerWidth` is clamped to minimum 10; `visibleLines` is clamped to minimum 1; `TestTUI055_ViewBoundaryDimensions` verifies no panic occurs.

### Edge Cases
- **Plan text contains ANSI escape sequences**: ANSI codes pass through; lipgloss's `Width()` function may miscalculate rendered widths.
- **Width or Height set to 0 or negative**: The `View()` function applies floor defaults (`width = 80` if `Width <= 0`, `height = 20` if `Height <= 0`).

---

### Cost & Context Awareness

## STORY-083: Watching the Status Bar Cost Counter Update in Real Time

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer using the TUI for the first time after a cost surprise in a previous project
**Goal**: Know exactly how much each exchange costs without leaving the conversation
**Preconditions**: TUI is open, no run is active. Status bar shows the active model name. No cost has been incurred yet.

### Steps

1. User types a prompt and presses **Enter** → The run starts; `statusbar.Model.running` becomes `true`; the status bar renders `...` (dimmed) next to the model name.
2. Server streams the first `usage.delta` SSE event with `cumulative_cost_usd: 0.0012` → The TUI calls `m.statusBar.SetCost(0.0012)`, and re-renders the status bar. The cost segment `$0.0012` appears.
3. Server streams a second `usage.delta` event with `cumulative_cost_usd: 0.0048` → Status bar updates in place; `$0.0048` replaces `$0.0012`.
4. Run completes (`SSEDoneMsg`) → The `...` running indicator disappears. The cost figure `$0.0048` remains visible.
5. User types another prompt and presses **Enter** → When the next `usage.delta` arrives with `cumulative_cost_usd: 0.0091`, the status bar updates to `$0.0091` (cumulative session total).

### Variations

- **Very long run with many tool calls**: Each tool round-trip generates a `usage.delta` event; the cost counter ticks upward with every event.
- **Narrow terminal**: Lower-priority segments drop first before cost; cost reliably stays visible in most real-world terminal widths.

### Edge Cases

- **Zero-cost event**: If a `usage.delta` arrives with `cumulative_cost_usd: 0.0`, the cost segment remains hidden (the `costUSD > 0` guard).
- **Invalid JSON in usage.delta**: The TUI silently ignores the malformed event; the cost counter keeps its last valid value.

---

## STORY-084: Understanding Per-Run Cost vs. Session-Total Cost

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Engineer running multiple short tasks in one TUI session, trying to budget usage
**Goal**: Understand whether the cost shown is "this message" or "this whole session"
**Preconditions**: TUI is open. One prior exchange has already run and cost `$0.0050`. The status bar currently shows `$0.0050`.

### Steps

1. User reads the status bar: `gpt-4o ~ $0.0050` → This is the **session cumulative total**, not the cost of the most recent run.
2. User sends a second prompt → Run completes. The next `usage.delta` event carries `cumulative_cost_usd: 0.0098` → Status bar updates to `$0.0098`.
3. User wants to see what that second run cost in isolation → There is no per-run breakdown in the status bar itself. The user opens `/stats` to see total cost over a time window, or mentally subtracts the prior value.
4. User types `/clear` → Conversation history and transcript are cleared. The cumulative cost counter is **not** reset; the status bar still shows `$0.0098`.

### Variations

- **Multi-day usage**: The cost counter persists for the lifetime of the TUI process. Historical cost-per-day data lives in the stats panel.

### Edge Cases

- **TUI restart**: After closing and reopening the TUI, the cost counter resets to zero.
- **Multiple `usage.delta` events per run**: The event carries `cumulative_cost_usd`, which is the server's running total. The TUI takes the last value received.

---

## STORY-085: Opening the Context Grid to Check Token Fill

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer working on a long codebase exploration task, worried about hitting the context limit
**Goal**: See how much of the 200k-token context window has been consumed
**Preconditions**: TUI is open. Several tool-heavy exchanges have run. No overlay is currently open.

### Steps

1. User types `/context` → The autocomplete dropdown appears momentarily (filtered to `/context`), then the command auto-executes. The context grid overlay opens.
2. The overlay displays:
   ```
   Context Window Usage

     [████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]

     Used:  24576 tokens
     Total: 200000 tokens
     Usage: 12.3%
   ```
3. User reads: 24,576 tokens used, 175,424 remaining.
4. User presses **Escape** → The overlay closes. The main chat view is restored.

### Variations

- **Using slash command with autocomplete**: Typing `/con` + Tab auto-completes to `/context ` and executes.

### Edge Cases

- **Zero tokens used**: The progress bar is entirely empty. `Used: 0 tokens`, `Usage: 0.0%`.
- **Token count exceeds total**: The `contextgrid.Model` clamps `UsedTokens` to `TotalTokens`; `Usage: 100.0%` is the maximum displayed.
- **Default context window**: `TotalTokens` defaults to `200000` when not explicitly set.

---

## STORY-086: Opening the Stats Panel to Review Historical Activity

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer who has been using the harness daily for two weeks and wants a sense of usage trends
**Goal**: Get a visual overview of run frequency and spending over the past week
**Preconditions**: TUI has been used on multiple days. `usageDataPoints` contains historical DataPoints. No overlay is currently open.

### Steps

1. User types `/stats` + **Enter** → The stats panel overlay opens. The heatmap defaults to the **last 7 days** (`PeriodWeek`) period.
2. The overlay displays a day-of-week grid with intensity blocks (`░`, `▒`, `▓`, `█`) assigned by percentile rank, plus `Total runs: 23   Total cost: $0.41` at the bottom.
3. User reads the hint text `[r to toggle period]` and presses **r** → Period cycles to **last 30 days** (`PeriodMonth`). The heatmap re-renders. `Total runs: 87   Total cost: $1.73`.
4. User presses **r** again → Period cycles to **last 365 days** (`PeriodYear`). Total line shows full-year totals.
5. User presses **r** one more time → Period wraps back to **last 7 days**.
6. User presses **Escape** → The overlay closes.

### Variations

- **First run ever**: If `usageDataPoints` is empty, the grid renders all `░` cells. `Total runs: 0   Total cost: $0.00`.

### Edge Cases

- **Data points with zero-time dates**: Zero-value `time.Time` structs in `usageDataPoints` are silently skipped.
- **Data outside the current window**: If a DataPoint's date is older than the selected period, it is excluded from both the heatmap cells and the totals line.

---

## STORY-087: Using Stats to Decide Whether to Start a New Conversation

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer near the end of a multi-hour debugging session, conscious of LLM costs and context limits
**Goal**: Decide whether to continue the current conversation or clear it and start fresh
**Preconditions**: TUI has been running for ~2 hours. Status bar shows `$0.34`. The user has used a significant amount of context.

### Steps

1. User opens `/context` → Context grid shows `Used: 142,000 tokens`, `Total: 200,000 tokens`, `Usage: 71.0%`. The progress bar is about 70% filled.
2. User presses **Escape** to close the context overlay.
3. User opens `/stats` → Stats panel shows the last 7 days heatmap. Today's cell is the densest (`█`). `Total runs: 41   Total cost: $0.34` for the week.
4. User presses **r** → Switches to last 30 days. The user identifies that this is an unusually heavy day.
5. User presses **Escape** to close the stats overlay.
6. User evaluates: Context is 71% full; today has been unusually expensive. Decision: start a new conversation with `/clear`.
7. User types `/clear` → Conversation is cleared. The status bar cost counter is NOT reset.

### Variations

- **Context only 10% full**: The user decides to continue without clearing.
- **Cost much lower than expected**: The user glances at the status bar, sees `$0.02`, and continues without concern.

### Edge Cases

- **After /clear, context grid still shows old token count**: The `/clear` command does not reset `m.totalTokens`. Token count only resets when a new `usage.delta` event arrives.

---

## STORY-088: Watching the Context Bar Progress Toward Full

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer running a large agentic task that processes many files, expecting context pressure
**Goal**: Monitor context fill during the run, ready to intervene before the model degrades
**Preconditions**: TUI is open. A long-running run has been active for several minutes.

### Steps

1. Run starts. User opens `/context` early: `Used: 8,192 tokens`, `Usage: 4.1%`. User closes with **Escape**.
2. Several tool calls later. Each `usage.delta` event updates `m.totalTokens` and `m.contextGrid.UsedTokens`.
3. User opens `/context` again: `Used: 95,000 tokens`, `Usage: 47.5%`. The bar is half-filled.
4. User continues. More tool calls, more file content accumulated.
5. User opens `/context` again: `Used: 178,500 tokens`, `Usage: 89.3%`. The progress bar is nearly full.
6. User decides to finish the current subtask and then run `/clear` before the next one.
7. User presses **Escape** and monitors the status bar. The run completes. The user issues `/clear` and begins the next task from a clean context state.

### Variations

- **Checking context every few minutes as a habit**: The overlay takes one keystroke to open and one to close, making it low-friction.

### Edge Cases

- **Context hits 100%**: The `contextgrid.Model` clamps `UsedTokens` to `TotalTokens`; display reads `Usage: 100.0%`. The context grid itself does not warn or change color.
- **Model with different context window**: The `TotalTokens` field is updated accordingly; the percentage and bar reflect the correct ratio.

---

## STORY-089: Transient Status Bar Messages During Cost Operations

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer who relies on the status bar for quick feedback
**Goal**: Understand the relationship between persistent cost display and transient status messages
**Preconditions**: TUI is open. Status bar shows `gpt-4o ~ $0.0082`. No run is currently active.

### Steps

1. User types `/export` + **Enter** → The conversation is exported. The status bar briefly shows a transient message: `Exported: transcript-20260323-142501.md` for 3 seconds.
2. During those 3 seconds, the **transient message replaces** the normal status bar content (model + cost).
3. After 3 seconds, the auto-dismiss fires → The status bar returns to its normal state: `gpt-4o ~ $0.0082`.
4. User types an unknown command like `/foo` → Status bar shows a transient hint for 3 seconds. Then it reverts.
5. User presses **Escape** on an empty input → Status bar shows `Input cleared` for 3 seconds. Then reverts.

### Variations

- **Run interrupted**: A transient `Interrupted` message appears in the status bar for 3 seconds. After it clears, the cost counter reappears.
- **Export failed**: The transient message reads `Export failed` instead of a file path.

### Edge Cases

- **Multiple transient messages in quick succession**: Each new transient message replaces the previous one and resets the 3-second timer.
- **Cost update during transient message display**: The underlying `costUSD` field is updated immediately; the cost reflects the latest value once the transient message dismisses.

---

## STORY-090: Comparing Costs Across Time Periods in the Stats Panel

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Engineering team lead reviewing AI tool usage to report monthly spend
**Goal**: Compare weekly vs. monthly usage patterns and total costs without leaving the TUI
**Preconditions**: TUI has been in use for at least 30 days. `usageDataPoints` contains a rich history.

### Steps

1. User opens `/stats` → Stats panel opens in **last 7 days** mode. `Total runs: 67   Total cost: $1.22`.
2. User presses **r** → Switches to **last 30 days**. Sparse weeks are visible. `Total runs: 183   Total cost: $3.47`.
3. User mentally divides: `$3.47 / 30 days = ~$0.12/day` average. This week's `$1.22 / 7 = ~$0.17/day` is above average.
4. User presses **r** again → Switches to **last 365 days**. Most cells are `░` (the tool was only adopted 2 months ago). `Total runs: 214   Total cost: $4.01`.
5. User notes that the year total nearly matches the 30-day total, confirming the tool is relatively new.
6. User presses **Escape** → Overlay closes.

### Variations

- **Cost is higher than expected**: The heatmap identifies the high-activity days for correlation with memory or exported transcripts.

### Edge Cases

- **Heatmap shows no activity**: If no `usage.delta` events have been received yet in this session, all cells render as `░`.
- **Very high run count**: The `intensityBlock()` function uses percentile ranking, not absolute thresholds, so the heatmap remains readable at high usage scales.

---

## STORY-091: First Run — Cost Goes From Hidden to Visible

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: New user launching `harnesscli --tui` for the first time
**Goal**: Understand when and how the cost counter first appears
**Preconditions**: TUI has just been launched. No run has occurred yet. The status bar shows only the model name.

### Steps

1. User observes the status bar: `gpt-4o`. No cost is shown. This is the clean initial state.
2. User types their first prompt and presses **Enter** → A run starts. The status bar adds `...` (running indicator): `gpt-4o ~ ...`.
3. The agent calls the bash tool, which completes quickly. The server emits the first `usage.delta` event with `cumulative_cost_usd: 0.0003`.
4. The TUI receives the event, calls `m.statusBar.SetCost(0.0003)` → The status bar now shows `gpt-4o ~ ... ~ $0.0003`.
5. Run completes → `SSEDoneMsg` clears the running indicator. Status bar becomes: `gpt-4o ~ $0.0003`. The cost counter is now permanent for the session.

### Variations

- **User never sends a prompt**: The cost segment never appears.

### Edge Cases

- **Very first event carries a cost of exactly 0**: The segment remains hidden.
- **Terminal too narrow to show cost on first appearance**: The cost segment may be dropped in favor of the higher-priority model name and running indicator.

---

## STORY-092: Closing and Reopening Stats Overlay Mid-Session

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer who checks the stats panel between tasks during a multi-hour session
**Goal**: Verify that the stats panel accumulates today's data across the full session and is accurate when reopened
**Preconditions**: TUI has been running for 3 hours. Several runs have been completed.

### Steps

1. User opens `/stats` early in the session → Sees `Total runs: 4   Total cost: $0.07` for today.
2. User presses **Escape** → Overlay closes. Work continues.
3. User completes 12 more runs over the next 2 hours. Each run generates `usage.delta` events that call `upsertTodayDataPoint`, which **replaces** today's cost (cumulative) and **increments** today's run count.
4. User opens `/stats` again → Today's cell is now the densest in the heatmap. `Total runs: 16   Total cost: $0.31`.
5. User presses **r** to check the month view → Can see the current day stands out dramatically.
6. User presses **Escape** → Returns to the main chat view.

### Variations

- **Stats panel opened during an active run**: The overlay opens on top of the running conversation; the underlying `m.statsPanel` is updated immediately via `usage.delta` events.

### Edge Cases

- **Clock crosses midnight while TUI is open**: After midnight, a new DataPoint is inserted for the new day. Yesterday's data remains in the slice.
- **Stats panel opened with no runs today but prior history**: Today's cell is `░`. Prior days may show activity.

---

### Slash Commands & Autocomplete

## STORY-093: Typing "/" Opens Command Autocomplete Dropdown

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: New TUI user who wants to discover available commands
**Goal**: Open the autocomplete dropdown to see all available slash commands
**Preconditions**: TUI is open; input area is focused; no text has been typed

### Steps

1. User types `/` into the input area → `slashcomplete.Model.Open()` is called; `active = true`; `SetQuery("")` initializes filtered results with all commands via `FuzzyFilter`.
2. The dropdown appears above the input, showing all 10 available commands: `/clear`, `/context`, `/export`, `/help`, `/keys`, `/model`, `/profiles`, `/quit`, `/stats`, `/subagents` — each with its short description.
3. The first command (alphabetically) is highlighted by default (cursor position = 0) with reversed colors.
4. Input field shows just `/` with the cursor after it. Status bar shows no error.

### Variations

- **Already has text before typing `/`**: If the input area has regular text and the user appends `/`, behavior depends on the start-of-input check. The dropdown only opens when `/` is the first character.

### Edge Cases

- **Dropdown height**: Up to 8 rows are shown; if more than 8 commands exist, they are scrollable (not the current case with 10 commands).
- **Terminal too narrow**: The dropdown may be clipped; it renders with available width.

---

## STORY-094: Filtering Commands by Typing

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who knows the first letter of the command they want
**Goal**: Quickly narrow the dropdown to one or two matches by typing
**Preconditions**: Autocomplete dropdown is open; the first command `/clear` is highlighted

### Steps

1. User types `h` (input field now reads `/h`) → `SetQuery("h")` is called; `FuzzyFilter(suggestions, "h")` runs; only `/help` matches.
2. Dropdown now shows only `/help`; cursor resets to 0; `/help` is highlighted.
3. User continues typing `e` (input `/he`) → dropdown still shows `/help` (still the only match).

### Variations

- **Multiple matches**: Typing `/c` shows both `/clear` and `/context`; cursor resets to 0 on each keystroke.
- **No matches**: If no command contains the typed substring, the dropdown shows an empty state; no commands are listed.

### Edge Cases

- **`selected` resets on every query update**: Even if the user had arrow-keyed down, typing a character resets the selection to 0.
- **Case sensitivity**: `FuzzyFilter` is case-insensitive; typing `H` also matches `/help`.

---

## STORY-095: Navigating Autocomplete with Arrow Keys

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who wants to scroll through available commands with keyboard
**Goal**: Move the highlight up and down through the filtered list without closing the dropdown
**Preconditions**: Autocomplete dropdown is open; multiple commands are visible (e.g., after typing `/` with no filter)

### Steps

1. User presses **Down** arrow → `Model.Down()` increments `selected` with modulo wrapping: `(selected + 1) % len(filtered)`; highlight moves to the next command.
2. User presses **Down** repeatedly → highlight cycles through all filtered commands and wraps to the top.
3. User presses **Up** arrow → `Model.Up()` decrements `selected` with modulo wrapping; highlight moves to the previous command.
4. Navigation does NOT modify the input text — only the visual selection changes.

### Variations

- **Zero results**: With no matches, arrow keys are no-ops; no crash occurs.
- **Single result**: Down and Up both produce no visible movement but are safe no-ops.

### Edge Cases

- **Rapid keypress**: Each keypress is handled independently; rapid pressing cycles through commands quickly.

---

## STORY-096: Accepting a Command with Enter

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who has navigated to a command and wants to execute it
**Goal**: Execute the highlighted slash command by pressing Enter
**Preconditions**: Autocomplete dropdown is open; `/help` is highlighted

### Steps

1. User presses **Enter** → `Model.Accept()` is called, returning `(closed model, "/help ")` — the completed text with a trailing space.
2. The dropdown closes immediately.
3. The completed text is parsed via `ParseCommand()` into `Command{Name: "help", Args: []}`.
4. The command registry dispatches to `executeHelpCommand`; the help overlay opens.
5. Input field is cleared (reset to empty) after command execution.

### Variations

- **`/quit`**: TUI exits immediately after acceptance.
- **`/clear`**: Conversation history is cleared; status bar shows confirmation.
- **`/stats`, `/context`, `/help`, `/model`**: Each opens its respective overlay.

### Edge Cases

- **Unrecognized command**: If `ParseCommand` returns an unknown command name, `commandRegistry.Dispatch` returns `CmdUnknown`; the status bar shows a hint.
- **Enter with empty dropdown**: No-op; nothing is accepted or executed.

---

## STORY-097: Accepting a Command with Tab

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who wants to auto-complete without fully executing the command yet
**Goal**: Tab-complete a uniquely matching command to insert it with a trailing space, then press Enter to execute
**Preconditions**: Autocomplete dropdown is open; input is `/h` (uniquely matching `/help`)

### Steps

1. User presses **Tab** → `CompleteTab()` is called; single match is found; input becomes `/help ` (with trailing space).
2. Dropdown closes (command is fully typed).
3. User can now press **Enter** to execute, or type arguments.

### Alternative Case (multiple matches)

- User types `/c` (matches `/clear` and `/context`) → presses **Tab** → common prefix `c` is already present; no change to input; dropdown remains open showing both commands.
- User presses **Down** to highlight `/context` → presses **Tab** → input becomes `/context `.

### Edge Cases

- **Trailing space**: The space allows the user to type arguments without modifying the command name.
- **Zero matches + Tab**: No-op; input unchanged.

---

## STORY-098: Dismissing Autocomplete with Escape

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who opened the dropdown but decides not to run a command
**Goal**: Close the autocomplete dropdown without executing anything, preserving the typed text
**Preconditions**: Autocomplete dropdown is open; input reads `/mo` (partial)

### Steps

1. User presses **Escape** → `slashcomplete.Model.Close()` sets `active = false`.
2. Dropdown is hidden immediately; no command is executed.
3. Input field still reads `/mo`; cursor is unchanged.
4. User can continue typing (e.g., type `del` to get `/model`), or clear input and type something else.

### Variations

- **Escape with nothing typed**: Esc closes the dropdown; if no run is active and no other overlay is open, the Esc cascade continues to the next priority (cancel active run, then clear input).

### Edge Cases

- **Esc priority**: Autocomplete Esc-close is the highest-priority Esc action; it fires before run cancellation.
- **No status message**: Dismissing autocomplete with Escape is silent — no status bar message is shown.

---

## STORY-099: Typing Partial Command and Tab Completion

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who wants a fast path to a specific command
**Goal**: Use Tab to complete a partial slash command in one keystroke
**Preconditions**: Input area is empty; autocomplete is not yet open

### Steps

1. User types `/cle` → Autocomplete opens; only `/clear` matches; `/clear` is highlighted.
2. User presses **Tab** → Input changes from `/cle` to `/clear ` (with trailing space); dropdown closes.
3. User presses **Enter** → `/clear` is executed; conversation history is cleared.

### Alternative

- User types `/c` (ambiguous) → Tab does not advance past the common prefix (`/c`); dropdown stays open showing `/clear` and `/context`.
- User presses **Down** → `/context` is highlighted → presses **Tab** → input becomes `/context `.

### Edge Cases

- **Tab when dropdown is closed**: If the user types `/clear` fully without the dropdown open, Tab may reopen the dropdown or be treated as a regular Tab (depending on focus state).

---

## STORY-100: Using Input History with Up/Down While Autocomplete Is Open

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer familiar with shell-style history navigation who expects Up to recall past messages
**Goal**: Understand the interaction between input history and autocomplete dropdown navigation
**Preconditions**: User has previously typed messages; autocomplete dropdown is open

### Steps

1. User presses **Up** arrow while autocomplete is open → Behavior: Up/Down in the input area affect the **input component's history**, not the dropdown's selection. The previous message is loaded from history into the input area; the dropdown closes/updates.
2. Input shows the recalled message (e.g., `"Explain the runner loop"`); the autocomplete dropdown closes (since the input no longer starts with `/`).

### Variations

- **Dropdown selection vs. history**: The dropdown's own Up/Down selection is only active when the overlay itself has focus. In the current architecture, the input area remains focused; Up/Down from the input area trigger history navigation.

### Edge Cases

- **Empty history**: If no prior messages exist, Up is a no-op from within the input area.
- **History item starts with `/`**: If a historical message starts with `/`, loading it from history will reopen the autocomplete dropdown.

---

## STORY-101: Autocomplete Reopens After Editing

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer who accepted a command then changed their mind and backspaced
**Goal**: Verify the dropdown reopens when editing brings the input back to a `/`-prefix state
**Preconditions**: User had accepted `/help ` (with trailing space); autocomplete was closed

### Steps

1. User backspaces once → Input becomes `/help` (no trailing space).
2. Autocomplete dropdown reopens (input starts with `/` again and has unresolved completions); `/help` is the only match and is highlighted.
3. User can now navigate, accept, or dismiss.

### Alternative

- User types more text after `/help ` (e.g., `/help foo`) → Autocomplete does NOT reopen; input is treated as `command args`; the dropdown remains closed.

### Edge Cases

- **`strings.HasPrefix(input, "/")`**: This guard determines whether to open the autocomplete; the check runs on every input change.
- **Accepted command with no trailing space**: If somehow `/help` (without space) is in the input, the dropdown reopens to show `/help` as a match.

---

## STORY-102: Quitting the TUI with /quit

**Type**: short
**Topic**: Slash Commands & Autocomplete
**Persona**: Developer finished with their session who wants to exit cleanly
**Goal**: Exit the TUI using the slash command
**Preconditions**: TUI is open; no run is active; input area is focused

### Steps

1. User types `/quit` → Autocomplete shows `/quit` as the only match.
2. User presses **Enter** → `executeQuitCommand()` handler returns a command that emits `tea.Quit()`.
3. The BubbleTea model loop receives `tea.Quit` and exits gracefully.
4. Terminal returns to the shell prompt; no confirmation dialog is shown.

### Variations

- **Using `/q` + Tab**: User types `/q` (uniquely matches `/quit`) → presses Tab → input becomes `/quit ` → presses Enter → same result.
- **Ctrl+C with no active run**: Same effect — immediately quits the TUI.

### Edge Cases

- **No cleanup of external resources**: The `harnessd` server continues running; only the TUI exits.
- **Any unsaved conversation**: If the user has not run `/export`, conversation history in the viewport is lost on quit.

---

### Error Recovery & Interrupts

## STORY-103: Two-Stage Ctrl+C Interrupt During a Long-Running Agent Run

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who sent a long refactoring prompt and wants to stop it mid-run
**Goal**: Cancel an active run cleanly without accidentally quitting the TUI
**Preconditions**: TUI is open; a run is active (`runActive = true`); one or more tool call blocks are visible in the viewport; the thinking bar is showing

### Steps

1. User presses `Ctrl+C` once during the active run → The interrupt banner transitions from Hidden to Confirm state; a centered amber-bordered box appears between the viewport and the input area: `⚠  Press Ctrl+C again to stop, or Esc to continue`. Input area remains visible; the run continues. No tools are interrupted yet.
2. User presses `Ctrl+C` a second time → The banner transitions from Confirm to Waiting; the centered box is replaced by a dimmed faint line: `Stopping… (waiting for current tool to finish)`. The cancel function (`cancelRun`) is called, signaling server-side context cancellation.
3. The server acknowledges the cancellation and the SSE stream terminates → The banner transitions through Done (briefly) then to Hidden. `runActive` clears to false. The thinking bar is dismissed. Status bar flashes `Interrupted` for 3 seconds.
4. User sees the viewport with conversation history intact up to the point of interruption → The input area is ready with the `❯` cursor for the next message.

### Variations

- **Tool mid-execution at second Ctrl+C**: The "Stopping…" text is accurate — the cancel context propagates but the harness waits for the current tool call to return before closing the SSE stream. The tool block shows completed or error state once it finishes.
- **Ctrl+C with overlay open**: If `/help` or `/model` overlay is open, the first Ctrl+C closes the overlay rather than triggering the banner. The run continues. The user must close the overlay before using Ctrl+C to interrupt.

### Edge Cases

- **Second Ctrl+C from Waiting state is a no-op on the banner**: `Confirm()` called from Waiting returns unchanged. No double-cancel is issued.
- **Rapid double Ctrl+C**: Both presses may land in the same Update cycle; Hidden → Confirm → Waiting in the same frame. The cancel function fires once.
- **Run ends naturally while banner is in Confirm state**: `SSEDoneMsg` arrives and clears `runActive` before the second Ctrl+C. The banner is hidden on the next render.

---

## STORY-104: Esc Key as Single-Press Cancel During an Active Run

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who changed their mind immediately after submitting a prompt
**Goal**: Cancel the current run quickly with a single keypress rather than two Ctrl+C presses
**Preconditions**: TUI is open; a run has just started (`runActive = true`); no overlay is open; input is empty

### Steps

1. User presses `Esc` once with no overlay open and no input text → The multi-priority Esc handler checks: API keys overlay (no), model overlay (no), any other overlay (no), active run (yes). The cancel function is called immediately. `runActive` is set to false. `cancelRun` is cleared to nil.
2. Status bar shows `Interrupted` as a 3-second transient message → The thinking bar disappears. Run terminated without requiring confirmation.
3. The viewport retains whatever partial output was streamed → The input area is immediately ready.

### Variations

- **Esc with overlay open**: Overlay closes first; run is NOT cancelled on the first Esc. A second Esc (no overlay, no text) then cancels the run.
- **Esc with non-empty input and no run**: Esc clears the input text and shows `Input cleared`. No run is affected.
- **Esc with overlay open AND run active**: First Esc closes overlay; second Esc cancels run; third Esc is a no-op.

### Edge Cases

- **Esc with no run and empty input**: Handler is a no-op — no quit, no status message, no state change.
- **Esc races with run completing naturally**: If `SSEDoneMsg` arrives in the same tick as Esc, BubbleTea serializes them. Esc may call an already-nil `cancelRun`. The nil guard prevents a panic.

---

## STORY-105: Run Failure via SSE `run.failed` Event

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer running an agent against an API that returns a rate-limit error
**Goal**: Understand why the run failed and immediately retry or adjust
**Preconditions**: TUI is open; a run is active; the harness receives a non-retriable error from the provider (e.g., HTTP 429)

### Steps

1. The harness streams a `run.failed` event → SSE bridge converts this into `SSEDoneMsg{EventType: "run.failed", Error: "provider completion failed: openai request failed (429): {\"error\":{\"message\":\"Rate limit exceeded\",\"type\":\"rate_limit_error\"}}"}`.
2. TUI Update handler receives `SSEDoneMsg` with `EventType == "run.failed"` → `runActive` is false; `sseCh` is cleared; thinking bar is dismissed. `formatRunError()` splits the error at the first `{`, rendering: `✗ provider completion failed: openai request failed (429)` followed by indented JSON key-value lines.
3. The viewport scrolls to show the formatted error at the bottom → Status bar does not show a separate message (error is in the viewport).
4. User reads the error, identifies it as a rate-limit issue → User waits and retries, or uses `/model` to switch providers.

### Variations

- **`run.failed` with empty error string**: `formatRunError("")` returns `["✗ run failed"]`. A single generic line appears.
- **`run.failed` with non-JSON error**: The entire error string is prefixed with `✗ ` and rendered as one line.
- **`RunFailedMsg` arriving directly** (HTTP error at run creation): `✗ <error>` is appended directly without JSON parsing. Run state is cleared identically.

### Edge Cases

- **`cancelRun` is not nil at SSEDoneMsg**: The SSEDone handler calls `m.cancelRun()` then nils it — intentional to prevent goroutine leak in the bridge.
- **`lastAssistantText` is non-empty at failure**: The partial response is recorded into the transcript before appending the error.

---

## STORY-106: SSE Stream Error with Polling Continuation

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer on a flaky network who sees a stream hiccup mid-run
**Goal**: Understand that the TUI recovers automatically from transient stream read errors
**Preconditions**: TUI is open; a run is active; the SSE stream encounters a transient read error

### Steps

1. The SSE bridge encounters a read/parse error → Sends `SSEErrorMsg{Err: <error>}` to the model's message channel.
2. The TUI Update handler receives `SSEErrorMsg` → Appended to viewport: `⚠ stream error: connection reset by peer`. Run is NOT marked inactive. `sseCh` is still non-nil.
3. Because `sseCh` is non-nil, the handler returns `pollSSECmd(m.sseCh)` → Polling continues. If the server is still streaming, events pick up where they left off.
4. If the error is transient, subsequent `SSEEventMsg` messages arrive normally → The run completes successfully.

### Variations

- **Repeated stream errors**: Each error appends another `⚠ stream error:` line. After 256 dropped messages, the bridge sends `SSEErrorMsg` with text `"SSE bridge: too many dropped messages, stream may be corrupt"`.
- **Error when `sseCh` is nil**: `SSEErrorMsg` appends the warning but returns no `pollSSECmd`. No restart of polling occurs.

### Edge Cases

- **Stream error followed immediately by `SSEDoneMsg`**: Both arrive back-to-back; the error line appears, then normal run-completion teardown proceeds.
- **Network completely drops after stream error**: Each failed read produces another `SSEErrorMsg`. Multiple warning lines accumulate until the bridge's context is cancelled.

---

## STORY-107: Tool Call Error Rendering in the Viewport

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer watching an agent run that attempts a bash command which fails with a non-zero exit code
**Goal**: See exactly which tool failed and why, and understand the agent's next decision
**Preconditions**: TUI is open; a run is active; a `tool.call.started` event has been received for a bash tool; the tool returns an error

### Steps

1. The harness emits `tool.call.completed` with `Error: "bash exited with code 1: command not found: foobar"` and `DurationMS: 340` → TUI receives `SSEEventMsg{EventType: "tool.call.completed"}` with the error populated.
2. `handleToolError()` is called → The `tooluse.Model` for that call ID is updated to error state. The tool block is re-rendered via `ReplaceTailLines`: tool name in red, status `error`, error text in red, elapsed duration `340ms`.
3. The tool block is no longer "active" — the timer stops; the spinning indicator is replaced by a red error icon.
4. The agent continues: the next event is `assistant.message.delta` as the agent reasons about the failure → The assistant's response appears after the failed tool block.
5. User can press `Ctrl+O` to expand the failed tool block → Full args and any partial output are visible in the expanded view.

### Variations

- **Tool error with hint text**: If the tool definition provides a hint for this error, it appears below the error text in a muted color.
- **Multiple tools failing in sequence**: Each failed tool produces its own error-state block. They accumulate as the run progresses.

### Edge Cases

- **Tool error with empty error string**: `handleToolError()` receives `errText = "tool failed"` (fallback). Block renders `error: tool failed`.
- **Tool call error but call ID unknown** (ID mismatch): `toolViews[callID]` is nil/absent; `handleToolError` creates a minimal block for the orphaned ID rather than crashing.
- **`Ctrl+O` with no active tool call**: `activeToolCallID` is empty; the key press is a no-op.

---

## STORY-108: Export Failure Feedback via Status Bar

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer attempting to export a conversation transcript
**Goal**: Know immediately if the export failed so they can diagnose the cause
**Preconditions**: TUI is open; the user has had a multi-turn conversation; the export directory is not writable

### Steps

1. User types `/export` and presses Enter → `transcriptexport.ExportCmd(...)` fires as a background `tea.Cmd`.
2. The export goroutine attempts to write the timestamped markdown file → Write fails (e.g., `permission denied`). Returns `ExportTranscriptMsg{FilePath: ""}` — an empty `FilePath` signals failure.
3. TUI Update handler receives `ExportTranscriptMsg{FilePath: ""}` → `m.setStatusMsg("Export failed")` is called. Status bar shows `Export failed` for 3 seconds, then auto-dismisses.
4. No other UI state changes — viewport and conversation are intact → User can retry after fixing permissions.

### Variations

- **Successful export**: `ExportTranscriptMsg{FilePath: "/Users/alice/transcript-20260323-114704.md"}` → Status bar shows `Transcript saved to <path>` for 3 seconds.
- **Export with empty transcript**: If no messages have been sent, the export still runs and produces a header-only file. Status bar shows success with the file path.

### Edge Cases

- **Export triggered during an active run**: Only completed turns are in `transcript`. In-progress assistant delta is committed on `SSEDoneMsg`. Export captures conversation up to but not including the current streaming response.
- **Status bar race with a new status message**: If the user triggers another status-producing action before the 3-second timer fires, `setStatusMsg` sets a new expiry. The earlier `statusTickMsg` checks `time.Now().After(expiry)` — if the new expiry is in the future, the message is NOT cleared prematurely.

---

## STORY-109: Unknown Slash Command Feedback

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who mistypes a slash command
**Goal**: Get clear feedback that the command was not recognized, and recover quickly
**Preconditions**: TUI is open; no run is active; no overlay is open

### Steps

1. User types `/rune` (intending `/run` but mistyping) and presses Enter → `ParseCommand` extracts command name `rune`; `commandRegistry.Dispatch(Command{Name: "rune"})` returns `CmdResult{Status: CmdUnknown, Hint: "Unknown command: /rune. Type /help to see available commands."}`.
2. Update handler hits `case CmdUnknown` → `m.setStatusMsg(result.Hint)` is called. Status bar shows the hint for 3 seconds, then auto-dismisses.
3. The input area is cleared → User sees the hint and can type `/help` or retype the intended command.

### Variations

- **Partial command that matches nothing in autocomplete**: Dropdown shows no results. User presses Enter anyway with partial text. Same path: `CmdUnknown`, hint in status bar.
- **Command registered but with execution error**: Handler returns `CmdError` with an error message. Status bar shows the error text (not "Unknown command").

### Edge Cases

- **Slash command typed while run is active**: Run is not affected. Unknown-command hint appears in the status bar while the run continues.
- **`/` alone pressed Enter**: `ParseCommand("/")` extracts command name `""`; Dispatch returns `CmdUnknown`; status bar shows the hint.

---

## STORY-110: Server Unreachable at TUI Launch (Connection Refused)

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who forgets to start `harnessd` before launching the TUI
**Goal**: Understand the connection problem from the TUI, not from a cryptic crash
**Preconditions**: `harnesscli --tui` is run, but `harnessd` is not listening on port 8080

### Steps

1. TUI launches → BubbleTea starts; `Init()` replays any pending API keys via `setProviderKeyCmd`. The key-set requests fail silently. The full-screen TUI renders with the empty viewport, input area with `❯` prompt, and status bar.
2. The model switcher attempts `fetchModelsCmd` on first open of `/model` → HTTP GET to `/v1/models` fails: `dial tcp [::1]:8080: connect: connection refused`. Returns `ModelsFetchErrorMsg{Err: "connection refused"}`.
3. The model switcher overlay shows a load error state: `Error loading models: connection refused` rendered inside the model switcher → User can close the overlay with Esc.
4. User types a prompt and presses Enter → `startRunCmd` POSTs to `/v1/runs`; fails with `connect: connection refused`. Returns `RunFailedMsg{Error: "Post \"http://localhost:8080/v1/runs\": dial tcp...: connection refused"}`.
5. Update handler receives `RunFailedMsg` → `runActive` is false; viewport appends `✗ Post "http://localhost:8080/v1/runs": dial tcp ...: connection refused`. A blank line follows.
6. User starts `harnessd` in another terminal → The next prompt submission succeeds. No TUI restart needed.

### Variations

- **Server starts after TUI is launched**: Once `harnessd` is running, the next submission succeeds. No session state prevents reconnection.
- **Wrong port configured**: Same behavior as connection refused. The URL in the error message identifies the misconfiguration.
- **Server reachable but returns 5xx**: `startRunCmd` receives HTTP 500; returns `RunFailedMsg{Error: "start run: HTTP 500"}`; same viewport error path.

### Edge Cases

- **Init key-set requests fail**: `setProviderKeyCmd` failure falls through silently. The provider appears unconfigured in the model switcher.
- **Profiles load fails at `/profiles` open**: `loadProfilesCmd` returns `ProfilesLoadedMsg{Err: &err}`; profile picker shows empty list.

---

## STORY-111: Recovery and Continued Conversation After an Interrupt

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who interrupted a run mid-way and wants to resume with a refined prompt
**Goal**: Confirm that the TUI is fully ready for input after interruption and that conversation context is preserved
**Preconditions**: A run was just cancelled via Ctrl+C or Esc; status bar shows `Interrupted`

### Steps

1. User observes the TUI state immediately after interrupt → `runActive` is false; `cancelRun` is nil; `sseCh` is nil; thinking bar is gone; interrupt banner is Hidden; status bar shows `Interrupted` (fading after 3 seconds); viewport shows conversation up to the last complete event.
2. User types a new message in the input area → Input area is immediately responsive; no lock-out, no loading state.
3. User presses Enter → `startRunCmd` fires with the same `conversationID` as before (preserved across interrupts — set on the first run, not cleared on cancel). The harness links this new turn to the existing conversation.
4. The new run starts: `RunStartedMsg` arrives → `runActive` is true; the SSE bridge reconnects; streaming resumes normally.
5. User sees the new response appended below the interrupted content → Conversation history is intact.

### Variations

- **User clears conversation before retrying**: Types `/clear` after interrupt. Viewport and transcript are wiped. `conversationID` is NOT reset by `/clear`. Subsequent runs are still linked to the same conversation on the server.
- **User changes model before retrying**: Opens `/model`, selects a different model, closes overlay. The next `startRunCmd` uses the new `selectedModel`. `conversationID` is unchanged.

### Edge Cases

- **Interrupt during plan overlay**: Plan overlay remains open after interrupt (not auto-dismissed). User must press Esc or reject before sending next message.
- **Interrupt with in-progress assistant text**: `lastAssistantText` may contain partial content. It is NOT committed to the transcript on interrupt (only `SSEDoneMsg` commits it). Transcript export will not include the partial response.

---

## STORY-112: What Happens to In-Progress Tool Calls at Interrupt Time

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer curious about tool cleanup behavior during cancellation
**Goal**: Understand whether interrupted tool calls appear as running, completed, or error in the viewport
**Preconditions**: A run is active; the agent is mid-way through executing a tool (`tool.call.started` has arrived; `tool.call.completed` has not)

### Steps

1. User presses Ctrl+C twice (or Esc once) to cancel the run → `cancelRun()` is called; `runActive` is false immediately.
2. The server-side harness receives the context cancellation → Attempts to stop the current tool. Depending on the tool, this may happen quickly (bash: SIGINT) or wait for the tool's natural return. Either way, the server closes the SSE stream.
3. `SSEDoneMsg` arrives → Run state is fully cleared; thinking bar is dismissed; banner hides.
4. The tool call block in the viewport is in its last rendered state → If `tool.call.completed` arrived before the stream closed, the block shows completed or error state with a duration. If `tool.call.completed` did NOT arrive, the block remains in "running" state with the spinner stopped — no duration is shown, no completion icon.
5. User sees the tool block frozen in the running state → Expected behavior; the block does not retroactively update to "error" or "interrupted". User can expand with `Ctrl+O` to see any partial output.

### Variations

- **Server sends `tool.call.completed` with error before SSEDone**: The tool block transitions to error state with the error message and duration. Clean path: tool returned before the context cancellation fully propagated.
- **Multiple tools queued but not yet started**: Tools that the agent had decided to run but the harness had not dispatched simply never start. No `tool.call.started` event arrives; no block appears.

### Edge Cases

- **Permission prompt was pending at interrupt**: The permission prompt modal remains visible until the user presses a key to dismiss it. The user should press Esc or a permission key to clear it.

---

## STORY-113: Ctrl+C with No Active Run Quits the TUI

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer finished with a session who wants to exit
**Goal**: Exit the TUI cleanly via Ctrl+C when no work is in progress
**Preconditions**: TUI is open; no run is active (`runActive = false`); no overlay is open

### Steps

1. User presses `Ctrl+C` once → The key handler checks `m.runActive`. It is false. The handler falls through to the standard quit path: `return m, tea.Quit`. BubbleTea terminates the program cleanly.
2. The terminal is returned to the user → The TUI exits with no error message. Normal shell prompt returns.

### Variations

- **Ctrl+C with run active**: Does NOT quit — cancels the run instead. TUI remains open.
- **Ctrl+C with overlay open and no run**: Ctrl+C quits immediately even with an overlay open (unlike Esc, which closes overlays first).

### Edge Cases

- **`/quit` command**: Same effect as Ctrl+C with no run — triggers `tea.Quit`.
- **Ctrl+C pressed with autocomplete dropdown open**: The dropdown is not in the Ctrl+C path. Ctrl+C will quit even if the dropdown is open (since no run is active).

---

## STORY-114: Cascading Esc Priority Resolution Through All Layers

**Type**: long
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who has accumulated multiple active states simultaneously and wants to navigate back to idle with Esc
**Goal**: Understand and use the full Esc priority chain to cleanly unwind all active state layers
**Preconditions**: User has: (a) typed some text in the input area, (b) opened the `/model` overlay and navigated to level 1 with search active, AND (c) a run is active in the background

### Steps

**Phase 1 — Clear model overlay search:**
1. User presses `Esc` (first press) with the model overlay open, search active, config panel not open → Clears the search query. The overlay stays open at level 1, showing the unfiltered model list.

**Phase 2 — Exit model list to provider list:**
2. User presses `Esc` (second press) — overlay `"model"`, level 1, no search → `ExitToProviderList()` is called. The overlay returns to level 0 (provider list). Still open.

**Phase 3 — Close model overlay:**
3. User presses `Esc` (third press) — overlay `"model"`, level 0, no search → Close the model overlay entirely. `overlayActive = false`, `activeOverlay = ""`. An `EscapeMsg{}` is dispatched.

**Phase 4 — Cancel the active run:**
4. User presses `Esc` (fourth press) — no overlay, run is active → Cancel the run. `cancelRun()` is called; `runActive = false`. Status bar shows `Interrupted`.

**Phase 5 — Clear input text:**
5. User presses `Esc` (fifth press) — no overlay, no run, input has text → `m.input.Clear()` is called. Status bar shows `Input cleared`.

**Phase 6 — No-op:**
6. User presses `Esc` (sixth press) — no overlay, no run, empty input → No-op. No state change, no status message, no quit.

### Variations

- **API keys overlay with key input mode active**: First Esc exits key input mode (clears typed key text, returns to provider list mode within the overlay). Second Esc closes the overlay entirely.
- **Model config panel with key input active**: First Esc exits key input mode. Second Esc exits the config panel. Third Esc goes to provider list. Fourth Esc closes the overlay.

### Edge Cases

- **Esc when overlay is `"profiles"`**: Profile picker closes immediately (single Esc). No multi-step Esc path.
- **No `InterruptedMsg` is emitted**: The Esc-cancel path sets the status message and clears run state locally but does not emit an `InterruptedMsg` tea message. The live Update path relies on the `setStatusMsg("Interrupted")` side-effect and direct state mutation.

---

### Keyboard-Driven Navigation

## STORY-115: Paging Through a Long Conversation

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who has been in a multi-turn conversation for 30+ exchanges and needs to revisit something the assistant said earlier
**Goal**: Navigate backward through conversation history and return to the bottom without touching the mouse
**Preconditions**: TUI is open; the viewport contains many turns that exceed the visible height; no overlay is active; input area is focused

### Steps

1. User presses `pgup` → The viewport scrolls up by half the current viewport height. Earlier conversation turns come into view. Input area retains focus.
2. User presses `pgup` again → Viewport scrolls up another half-page.
3. User presses `pgdn` → Viewport scrolls back down by half-page toward the most recent content.
4. User presses `pgdn` again → Viewport returns to the bottom. The latest assistant message and input area are fully visible.

### Variations

- **At the top boundary**: If the viewport is already at the top, `pgup` is a no-op — no visual artifact or error.
- **At the bottom boundary**: If the viewport is already at the bottom, `pgdn` is a no-op.
- **Very short conversation**: If all content fits in the viewport, `pgup`/`pgdn` produce no visible scroll movement.

### Edge Cases

- **Active run while scrolling**: If a run is in progress and the agent is streaming content, `pgup` still scrolls but new lines are being appended at the tail. The viewport does not auto-scroll back to the bottom on new content while the user has manually scrolled up.
- **Resize during page navigation**: If the terminal is resized mid-scroll, the half-page calculation updates to the new viewport height on the next keypress.

---

## STORY-116: Line-by-Line Scroll for Precision Reading

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer carefully reading a long code block or diff in the conversation
**Goal**: Scroll one line at a time through a section of the viewport
**Preconditions**: TUI is open; a tool call block containing a long file diff is visible in the viewport; no overlay is active

### Steps

1. User presses `up` (or `ctrl+p`) → Viewport scrolls up exactly one line.
2. User presses `up` several more times → Viewport continues scrolling up one line per keypress.
3. User presses `down` (or `ctrl+n`) → Viewport scrolls down one line back toward the content they passed.
4. User continues with `down` presses → Returns incrementally to the current tail.

### Variations

- **Ctrl+P / Ctrl+N alternative**: `ctrl+p` and `ctrl+n` produce identical scroll behavior for users who prefer Emacs-style navigation.
- **Holding the key**: Holding `up` produces repeated one-line scrolls as the terminal repeats the keypress.

### Edge Cases

- **Slash command dropdown open**: When the autocomplete dropdown is visible, `up`/`down` navigate the dropdown entries rather than scrolling the viewport.
- **Overlay open**: When any overlay is active, `up`/`down` route to the overlay's navigation handler, not the viewport.
- **Input history navigation**: If the input area is focused and the user has not yet triggered scroll mode, behavior depends on cursor position in the input.

---

## STORY-117: Copying the Last Assistant Response

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who wants to paste the assistant's most recent response into another tool, document, or terminal window
**Goal**: Copy the last assistant response text to the system clipboard without leaving the TUI
**Preconditions**: At least one assistant response has been received; no overlay is active; the run is not currently active (or has completed)

### Steps

1. User presses `ctrl+s` → The TUI copies the full text accumulated in `lastAssistantText` to the system clipboard.
2. The status bar flashes a 3-second transient confirmation message: "Copied to clipboard".
3. User switches to another application and pastes → The assistant response text appears.

### Variations

- **During an active run**: `ctrl+s` can be pressed while the run is still streaming. It copies whatever text has accumulated in `lastAssistantText` up to that point (may be a partial response).
- **After `/clear`**: After clearing the conversation, `lastAssistantText` is reset. Pressing `ctrl+s` before any new response results in copying an empty string, with the status bar showing "Copy unavailable".

### Edge Cases

- **Clipboard unavailable**: If the system clipboard is inaccessible (e.g., running over SSH without clipboard forwarding), the status bar shows "Copy unavailable" as a 3-second transient message. No crash or hang occurs.
- **Large responses**: Very long assistant responses are copied in full. There is no truncation at the clipboard level within the TUI.
- **Multi-turn sessions**: `ctrl+s` always copies the *last* assistant response, not a concatenation of all responses.

---

## STORY-118: Navigating the Help Dialog with Tab and Vim Keys

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: New user who wants to learn available keyboard shortcuts without reading external documentation
**Goal**: Open the help dialog, read through all three tabs (Commands, Keybindings, About), and close it
**Preconditions**: TUI is open; no overlay is active; the user has not previously opened help

### Steps

1. User presses `ctrl+h` (or `?`) → The help dialog overlay opens. The first tab "Commands" is active. All slash commands with descriptions are displayed.
2. User presses `tab` (or `right` or `l`) → The next tab "Keybindings" becomes active. The full keybinding reference table is displayed.
3. User presses `tab` again (or `right` or `l`) → The "About" tab becomes active. Project information is displayed.
4. User presses `shift+tab` (or `left` or `h`) → The dialog moves back to the "Keybindings" tab.
5. User presses `shift+tab` again → "Commands" tab is active again.
6. User presses `esc` → The help dialog closes. Focus returns to the input area.

### Variations

- **Opening via slash command**: User can type `/help` and press `Enter` to open the same dialog. Keyboard navigation within the dialog is identical.
- **Vim-style horizontal navigation**: `h` and `l` serve as left/right tab navigation aliases; users who prefer vim-style movement can navigate without reaching for Tab.
- **Wrapping**: Tab on the last tab wraps to the first; `shift+tab` on the first tab wraps to the last.

### Edge Cases

- **Content taller than dialog**: If a tab's content exceeds the dialog height, vertical scroll may apply within the dialog. `up`/`down` scroll within the active tab rather than switching tabs.
- **Pressing `?` while a run is active**: The help dialog still opens. The run continues in the background. Closing the dialog returns the user to the live-streaming viewport.

---

## STORY-119: Navigating the Model Switcher with Vim Keys

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Power user who wants to switch from OpenAI GPT-4o to Anthropic Claude without using the mouse
**Goal**: Open the model browser, navigate to a different provider, select a model, and confirm entirely via keyboard
**Preconditions**: TUI is open; no overlay is active; the harness server is reachable and returns a model list

### Steps

1. User types `/model` and presses `Enter` → Model switcher overlay opens at level 0 (provider list). Providers listed: OpenAI, Anthropic, Google, DeepSeek, xAI, Groq, Qwen, Kimi.
2. User presses `j` (or `down`) → Cursor moves to the next provider.
3. User presses `j` until "Anthropic" is highlighted.
4. User presses `Enter` → Overlay drills into level 1, showing the list of Anthropic models.
5. User presses `j`/`k` to navigate the model list to the desired model (e.g., `claude-opus-4-6`).
6. User presses `Enter` → Overlay advances to level 2 (config panel): gateway selection, API key status, reasoning effort controls.
7. User reviews the config and presses `esc` → Config panel closes, returning to level 1.
8. User presses `esc` again → Returns to the provider list at level 0.
9. User presses `esc` again → Model switcher closes entirely. The selected model is reflected in the status bar.

### Variations

- **Typing to search**: At level 1, typing any printable character accumulates a search query. The model list filters in real time. `backspace`/`delete` removes the last character.
- **Starring a model**: Pressing `s` on a highlighted model toggles the star. Starred models persist to the config file.
- **`k` to go up**: `k` moves the cursor up in both the provider list and the model list, mirroring vim navigation.

### Edge Cases

- **Unconfigured provider**: Selecting a model from an unconfigured provider redirects the user to the `/keys` overlay with the cursor pre-positioned on the relevant provider.
- **Empty model list**: Level-1 view shows an empty state. User can press `esc` to return to the provider list.
- **Search with no results**: The list shows an empty state. Pressing `backspace` removes characters until results reappear.

---

## STORY-120: Starring a Frequently Used Model

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who regularly switches between two models and wants quick access to their preferred model
**Goal**: Star a model so it appears prominently in future sessions
**Preconditions**: Model switcher is open at level 1 (model list for a provider); target model is visible; model is not yet starred

### Steps

1. User navigates to the desired model using `j`/`k` (or `up`/`down`).
2. User presses `s` → The model is starred. A visual indicator (star glyph or highlight) appears next to the model name.
3. User presses `s` again on the same model → The star is removed (toggle behavior). The indicator disappears.
4. User presses `esc` to close the overlay → The star state is persisted to the config file and will be present in future TUI sessions.

### Variations

- **Starring from search**: If the user has typed a search query and the filtered list shows the target model, `s` still stars the highlighted model in the filtered view.
- **Multiple stars**: There is no limit on the number of starred models. Each `s` press toggles the individual model independently.

### Edge Cases

- **Star state not saved**: If the process exits abnormally before writing the config file, the star state from the current session may be lost. Normal exit (via `/quit` or `ctrl+c`) ensures the config write completes.
- **Model becomes unavailable**: If a starred model is no longer returned by the server, the star entry remains in the config file but the model does not appear in the switcher.

---

## STORY-121: Managing API Keys via Keyboard

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer setting up the harness for the first time who needs to enter API keys for multiple providers
**Goal**: Navigate to the API keys overlay, enter a key for one provider, and clear and re-enter an incorrect key for another
**Preconditions**: TUI is open; `/keys` command is available; at least two providers are listed; neither is currently configured

### Steps

1. User types `/keys` and presses `Enter` → API keys overlay opens. A list of providers is displayed with their configuration status.
2. User presses `j` (or `down`) to move to the "OpenAI" provider entry.
3. User presses `Enter` → Overlay enters key-input mode for OpenAI. A text input field appears for the API key value.
4. User types the API key string.
5. User presses `Enter` → The key is submitted to the server via PUT `/v1/providers/openai`. Provider is shown as "configured".
6. User presses `j` to move to the "Anthropic" entry.
7. User presses `Enter` → Enters key-input mode for Anthropic.
8. User types an incorrect key string.
9. User presses `ctrl+u` → The input field is cleared entirely.
10. User types the correct API key.
11. User presses `Enter` → The key is submitted. Anthropic is now shown as "configured".
12. User presses `esc` → Exits key-input mode. A second `esc` closes the API keys overlay entirely.

### Variations

- **Correcting a single character**: Instead of `ctrl+u`, the user can use `backspace` to delete the last character one at a time.
- **Re-entering for a configured provider**: If a provider is already configured and the user presses `Enter`, they can enter a new key to overwrite.

### Edge Cases

- **Server error on key submission**: If the server returns an error, the overlay may not update the "configured" status. The user sees a status bar error message.
- **Esc priority in key-input mode**: Pressing `esc` while in key-input mode exits input mode without submitting the typed key.
- **Empty key submission**: Pressing `Enter` with an empty input may submit an empty string; behavior depends on server-side validation.

---

## STORY-122: Using Esc to Unwind Nested Overlay State

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who opened the model switcher, drilled into a provider's config panel, and wants to return to the main chat view
**Goal**: Close all overlay layers with a predictable escape chain using only `esc`
**Preconditions**: Model switcher is open at level 2 (config panel); API key input mode is active within the config panel

### Steps

1. User presses `esc` → Exits API key input mode within the config panel. Config panel itself remains visible (level 2 is still active).
2. User presses `esc` → Closes the config panel, returning to level 1 (model list for the selected provider).
3. User presses `esc` → Clears the search query if one was active. The model list shows all models unfiltered.
4. User presses `esc` → Returns to level 0 (provider list).
5. User presses `esc` → Closes the model switcher overlay entirely. Focus returns to the input area.

### Variations

- **No search active**: If no search query was typed at level 1, step 3 is skipped — `esc` at level 1 goes directly to the provider list.
- **From stats or context overlay**: Pressing `esc` with the stats panel or context grid open closes the overlay in a single keypress.
- **From help dialog**: A single `esc` closes the help dialog regardless of which tab is active.

### Edge Cases

- **Active run during overlay navigation**: The escape priority chain applies to the overlay first. The run is not cancelled until all overlays are dismissed and the user presses `esc` again from the main view with no input text.
- **Non-empty input after closing overlay**: If the input area had text before the overlay was opened, the text is preserved and the cursor returns to the input area.

---

## STORY-123: Interrupting and Cancelling an Active Run

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who submitted a prompt and realizes the agent is heading in the wrong direction mid-run
**Goal**: Cancel the active run cleanly without quitting the TUI
**Preconditions**: A run is active and streaming; the interrupt banner is not yet shown; no overlay is open; input area is empty

### Steps

1. User presses `ctrl+c` → The interrupt banner appears in "Confirm" state: "Press Ctrl+C again to stop...". Prevents accidental cancellation on a single keypress.
2. User presses `ctrl+c` again → The interrupt banner transitions to "Waiting" state: "Stopping... (waiting for current tool to finish)". The harness cancels the SSE stream.
3. The run terminates. The interrupt banner transitions to "Done" briefly, then dismisses.
4. The status bar shows "Interrupted" as a 3-second transient message.
5. The input area is focused again. The user can type a new prompt.

### Variations

- **Using `esc` to cancel instead**: If the input area is empty and no overlay is open, pressing `esc` during an active run also cancels it (escape priority: cancel active run is priority 5). No two-stage confirmation required with `esc`.
- **Changing mind after first `ctrl+c`**: If the user presses `ctrl+c` once (Confirm state) and then does nothing, the interrupt banner may auto-dismiss. A single `ctrl+c` without a follow-up does not cancel the run.

### Edge Cases

- **Ctrl+C with no active run**: Quits the TUI entirely.
- **Tool call mid-execution**: The "Waiting" state conveys that cancellation waits for the current in-flight tool call to complete before halting.
- **Overlay open during active run**: If an overlay is open when `ctrl+c` is pressed, the `ctrl+c` quit/cancel binding still fires — it is not shadowed by overlays.

---

## STORY-124: Writing a Multi-Line Prompt Without Submitting

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer composing a structured, multi-paragraph prompt with code examples embedded
**Goal**: Enter multiple lines of text in the input area without accidentally submitting the prompt
**Preconditions**: TUI is open; no overlay is active; the input area is focused and empty

### Steps

1. User types the first line of the prompt.
2. User presses `shift+enter` → A newline is inserted in the input area. The cursor moves to the next line. The prompt is NOT submitted.
3. User types the second paragraph.
4. User presses `ctrl+j` → An alternative newline insertion shortcut. Another line break is added. Functionally identical to `shift+enter`.
5. User types the final line, including a code snippet.
6. User presses `enter` → The full multi-line prompt is submitted as a single message.

### Variations

- **Using ctrl+j exclusively**: Users on terminals where `shift+enter` is not reliably distinguished from `enter` (some SSH clients or terminal multiplexers) can use `ctrl+j` as the newline shortcut.
- **Pasting multi-line content**: If the user pastes text containing newlines, the input area accepts the full pasted content including embedded newlines without submitting.

### Edge Cases

- **`Enter` vs `shift+enter` confusion**: The most common error is pressing `enter` when `shift+enter` is intended, submitting an incomplete prompt. There is no undo for a submitted message — the user must issue a follow-up or use `/clear`.
- **Very long multi-line input**: The input area wraps text visually. Scrolling within the input area lets the user review the full content before submitting.

---

## STORY-125: Toggling Stats Panel Period with `r`

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer monitoring agent cost and usage who wants to compare activity across different time windows
**Goal**: Open the stats panel and cycle through week, month, and year views using a single key
**Preconditions**: TUI is open; at least one run has completed; no overlay is active

### Steps

1. User types `/stats` and presses `Enter` → The stats panel overlay opens. Default view shows the activity heatmap for the current week. Run count and cumulative USD cost are displayed.
2. User presses `r` → Period toggles from "week" to "month". Heatmap redraws to show the current month's activity.
3. User presses `r` again → Period toggles from "month" to "year". Heatmap expands to show the full year.
4. User presses `r` again → Period wraps back to "week".
5. User presses `esc` → The stats panel closes. Focus returns to the input area.

### Variations

- **No historical data**: If the harness has only just started, the heatmap may show a sparse or empty grid for month and year views, with zero run counts. The panel still renders correctly.

### Edge Cases

- **`r` key outside stats overlay**: The `r` key has no special binding in the main view or other overlays. It is typed as a regular character into the input area if the stats panel is not open.
- **Heatmap rendering at small terminal size**: If the terminal is narrower than the heatmap's natural width, the rendering may be truncated. The `r` toggle still functions correctly regardless of display quality.

---

## STORY-126: Opening the Editor for Long Prompt Composition

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who wants to compose a complex, structured prompt in their preferred text editor (e.g., vim or nano) rather than in the TUI's input area
**Goal**: Open an external editor for prompt composition and return the finished text to the TUI input area
**Preconditions**: TUI is open; the `EDITOR` or `VISUAL` environment variable is set; no overlay is active; input area is focused

### Steps

1. User presses `ctrl+e` → The TUI invokes the external editor (from `$EDITOR` or `$VISUAL`). The terminal is temporarily handed off to the editor process.
2. The user composes the prompt in the editor, using the editor's full feature set (syntax highlighting, macros, etc.).
3. The user saves and exits the editor → Control returns to the TUI. The editor's output is inserted into the input area.
4. The input area now contains the composed prompt text.
5. User presses `enter` to submit, or reviews and edits further.

### Variations

- **Pre-existing input text**: If the input area already contains text when `ctrl+e` is pressed, that text may be pre-populated in the editor as the starting content.

### Edge Cases

- **No `EDITOR` set**: If neither `$EDITOR` nor `$VISUAL` is set, the TUI may fall back to a default editor (e.g., `vi`) or show an error in the status bar indicating no editor is configured.
- **Editor exits without saving**: If the user quits the editor without saving (e.g., `:q!` in vim), the input area returns to its previous state unchanged.
- **Editor crashes**: If the editor process exits abnormally, the TUI should recover and return focus to the input area. Any content written before the crash may or may not be recovered depending on the editor's temp file handling.

---

### Session Resumption & Export

## STORY-127: Resuming a Past Session from the Session Picker

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Developer who ran a multi-turn debugging conversation yesterday and wants to continue where they left off
**Goal**: Pick a previous conversation from the session picker and continue it with the same conversation ID
**Preconditions**: At least one prior session exists on the server. The TUI has just launched. The main chat view is active with an empty viewport and `conversationID` is empty.

### Steps

1. User invokes the session picker (via a trigger mechanism such as a slash command or keybinding) → The `sessionpicker.Model` overlay opens, showing a rounded-border box titled **Sessions**.
2. The list renders entries from `GET /v1/conversations`; each row shows: short session ID (first 8 chars of UUID), start date (`Mar 14`), model name (`gpt-4.1-mini`), turn count (`5 turns`), and up to 60 characters of the last user message.
3. The first entry is highlighted (reverse-video purple background). User reads the row to identify the correct session.
4. User presses `j` or **Down** to move to the second entry → Highlight moves down. Metadata columns dim; last message text remains normal weight for unselected rows.
5. User continues navigating until the desired session is highlighted.
6. User presses **Enter** → `SessionSelectedMsg{Entry: entry}` is emitted. The picker closes. The TUI sets `conversationID = entry.ID`.
7. User types a follow-up message and presses **Enter** → `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id` set to `entry.ID`. The server links this new run to the prior conversation.
8. The assistant's response streams in as usual. The viewport now shows the new turn as a continuation of the prior conversation.

### Variations

- **Many sessions (>10)**: The picker shows 10 rows and a footer `... N more` in dimmed text. The user navigates past row 10 and the list scrolls to reveal additional entries.
- **Session with a long last message**: The `LastMsg` field is clipped at 60 runes in `SessionEntry`. The row truncates silently.
- **Wrapping navigation**: Pressing `k` or **Up** on the first row wraps to the last entry. Pressing `j` or **Down** on the last row wraps to the first.

### Edge Cases

- **Conversation server returns fewer fields than expected**: The picker renders with whatever data is available in `SessionEntry`; empty `LastMsg` results in no last-message column in that row.
- **User presses Enter with no entries loaded**: `Selected()` returns `(zero, false)`; no `SessionSelectedMsg` is emitted; picker stays open.
- **User changes their mind**: Pressing **Escape** closes the picker without changing `conversationID`.

---

## STORY-128: Empty State in the Session Picker (No Prior Sessions)

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: A developer running the harness for the first time on a fresh installation
**Goal**: Open the session picker, discover there are no past sessions, and understand the state clearly
**Preconditions**: No prior conversations exist on the server. The TUI is running with an empty viewport.

### Steps

1. User opens the session picker overlay → `sessionpicker.Model` opens.
2. The picker renders the **Sessions** title followed by a centered, dimmed message: `No sessions found`.
3. No navigation is possible (the entry list is empty). Pressing `j`, `k`, **Up**, or **Down** is a no-op.
4. Pressing **Enter** is a no-op (no `SessionSelectedMsg` is emitted because `Selected()` returns `ok=false`).
5. User presses **Escape** → Picker closes. The viewport is unchanged. User proceeds to start a new conversation.

### Variations

- **Sessions loaded asynchronously**: The picker may render an initial empty state while the server fetch is in flight, then receive a `SetEntries()` update once the server responds. Selection and scroll offset reset to zero on each `SetEntries()` call.

### Edge Cases

- **Server returns an error**: The picker renders `No sessions found` (same empty state); the error should surface as a status bar message from the calling layer.
- **Picker opened at narrow terminal width**: `View()` defaults to `width=80` when passed 0; at very small widths (`<24` inner chars), the centered message still renders without panicking.

---

## STORY-129: Continuing a Multi-Turn Conversation (conversationID Linkage)

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Developer in an ongoing code review session, sending multiple follow-up messages
**Goal**: Understand how the TUI maintains conversation continuity across multiple turns within a single TUI session
**Preconditions**: The TUI is running. The user has already sent at least one message. `conversationID` is set.

### Steps

1. User sends the first message (`"Explain the runner loop"`). Input area submits `CommandSubmittedMsg{Value: "Explain the runner loop"}`.
2. `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id: ""` (empty — first turn).
3. Server responds with `{"run_id": "run-abc123"}`. TUI receives `RunStartedMsg{RunID: "run-abc123"}`.
4. Because `conversationID` was empty, the model sets `conversationID = "run-abc123"`. This is the stable conversation identifier for all subsequent turns.
5. The assistant's response streams into the viewport. `RunCompletedMsg` arrives; `runActive` goes false.
6. User sends a follow-up message (`"Now explain the tool dispatch"`).
7. `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id: "run-abc123"`. The server groups this run under the same conversation.
8. The new response streams in as a continuation. `conversationID` remains `"run-abc123"` for the lifetime of this TUI session.

### Variations

- **User opens a session from the session picker**: `conversationID` is set to `entry.ID`. The very next run POST carries that ID, resuming the server-side conversation context.
- **User runs `/clear`**: The viewport and `transcript` slice are reset to nil. However, `conversationID` is NOT reset by `/clear`. A subsequent message still POSTs with the original `conversationID`.

### Edge Cases

- **Two rapid submits before the first RunStartedMsg arrives**: The second submit reads `conversationID == ""` and POSTs with no conversation ID. The two runs may start as separate conversations. This is an expected race condition; users should wait for the first response before sending a second message.
- **Server assigns a different run_id each time**: The TUI only uses the first run's ID as the `conversationID`; subsequent runs get their own `RunID` but the `conversationID` binding is unchanged.

---

## STORY-130: Exporting a Conversation Transcript with /export

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who wants a permanent record of a debugging session to share with a colleague or file a bug report
**Goal**: Save the current session transcript to a timestamped markdown file
**Preconditions**: The TUI is running. The user has had a multi-turn conversation. The `transcript` slice contains at least one `TranscriptEntry`.

### Steps

1. User types `/export` in the input area (or uses the autocomplete dropdown).
2. `executeExportCommand` takes a snapshot of `m.transcript` (`copy(snapshot, m.transcript)`) so the export does not race with future transcript appends.
3. `transcriptexport.NewExporter(defaultExportDir())` is constructed. `defaultExportDir()` resolves to the OS cache directory (`~/Library/Caches/harness/transcripts` on macOS, `~/.cache/harness/transcripts` on Linux), falling back to `~/.harness/transcripts`, then `$TMPDIR/harness/transcripts`.
4. The export runs as a background `tea.Cmd` (non-blocking — the UI remains interactive while the file is written).
5. The exporter generates a filename: `transcript-YYYYMMDD-HHMMSS.md` using the current local time (e.g., `transcript-20260323-114704.md`).
6. The output directory is created with `os.MkdirAll` (mode `0755`) if it does not already exist.
7. The markdown file is written with a header (`# Conversation Transcript`, `Exported: YYYY-MM-DD HH:MM:SS`) and sections for each entry (User, Assistant, Tool) with timestamps.
8. On success: `ExportTranscriptMsg{FilePath: "/Users/alice/.cache/harness/transcripts/transcript-20260323-114704.md"}`. Status bar flashes the path for 3 seconds.

### Variations

- **User runs `/export` multiple times**: Each invocation produces a distinct filename (different second timestamp). No existing files are overwritten.
- **User runs `/export` before sending any messages**: The `transcript` slice is nil. The export still runs, producing a markdown file with only the header and no conversation entries. Status bar reports success.

### Edge Cases

- **Output directory cannot be created**: `os.MkdirAll` returns an error. `exporter.Export()` returns `("", err)`. `ExportTranscriptMsg{FilePath: ""}`. Status bar shows `Export failed` for 3 seconds in red.
- **Disk full**: `os.WriteFile` returns an error. Same error path: `ExportTranscriptMsg{FilePath: ""}` → `Export failed`.
- **User navigates away during export**: Because the export runs as a `tea.Cmd` in a goroutine, the UI remains responsive. The status message appears when the goroutine completes.

---

## STORY-131: Locating the Exported Transcript File

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who ran `/export` and now wants to open or share the file
**Goal**: Find the transcript file on disk after a successful export
**Preconditions**: `/export` has been run successfully. The status bar showed `Transcript saved to <path>`.

### Steps

1. User reads the path from the status bar (visible for 3 seconds, e.g., `/Users/alice/Library/Caches/harness/transcripts/transcript-20260323-114704.md`).
2. User opens a separate terminal or file manager and navigates to the path.
3. The file is a plain markdown document:
   ```
   # Conversation Transcript
   Exported: 2026-03-23 11:47:04

   ---

   ## User [11:45 AM]
   Explain the runner loop

   ---

   ## Assistant [11:45 AM]
   The runner loop in internal/harness/runner.go...

   ---

   ## Tool: bash [11:45 PM]
   <tool output content>

   ---
   ```
4. User can open the file in any markdown viewer, share over Slack, or attach to a bug report.

### Variations

- **Status bar dismissed before user reads the path**: The user can find the file by listing the export directory: `ls ~/Library/Caches/harness/transcripts/`. Files are timestamp-sorted, so the most recent is easily identifiable.
- **Multiple exports in the same session**: Each export file has a unique timestamp; the user can distinguish by time.

### Edge Cases

- **Very long file path on small terminals**: The status bar message is constrained by `MaxWidth(width)` in the export status renderer. Overflow is clipped; the path may be cut off in narrow terminals. The user can still find the file via the directory listing.

---

## STORY-132: Navigating Input History Within the Current Session

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who wants to re-send or edit a message they sent a few turns ago
**Goal**: Retrieve a past message from in-session history to re-use or modify it
**Preconditions**: The user has sent at least 2 messages in the current TUI session. The `inputarea.History` slice is non-empty.

### Steps

1. User focuses the input area (it is always focused when no overlay is open).
2. User presses **Up** (`ctrl+p` or up arrow) once → History navigates backward to the most recently sent message. The current draft text is saved internally (`h.draft = currentText`). The input field shows the most recent message.
3. User presses **Up** again → Input field shows the second-most-recent message.
4. User presses **Up** repeatedly until the desired message appears. Navigation stops at the oldest entry.
5. User edits the retrieved message.
6. User presses **Enter** to submit the modified message → The history navigation position resets to the draft (`pos = -1`). The submitted text is pushed to the front of the history (unless it is a consecutive duplicate of the most recent entry).
7. User presses **Down** to navigate forward in history → Input shows the next newer entry, eventually returning to the saved draft when past the newest entry.

### Variations

- **Draft restoration**: If the user started typing something before pressing **Up**, that draft is saved and returned when the user presses **Down** past the most recent history entry.
- **Consecutive duplicate suppression**: Pressing **Up** to retrieve `"hello"` and then submitting `"hello"` again does not add a second copy to history.
- **History at capacity**: The history holds at most 100 entries. When full, the oldest entry is dropped on each new push.

### Edge Cases

- **Empty history**: Pressing **Up** with no history entries is a no-op. The input field text is unchanged.
- **Single entry in history**: **Up** shows that entry; **Up** again is a no-op (not a wrap); **Down** returns to the draft.
- **History is cleared with `/clear`**: `/clear` resets the viewport and `transcript` slice but does NOT clear `inputarea.History`. Previously sent messages remain accessible via **Up** after a `/clear`.

---

## STORY-133: Distinguishing Export (Transcript) from Resumption (Session)

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: New user who is confused about what "exporting" and "resuming" each mean
**Goal**: Understand the conceptual difference between the two operations and when to use each
**Preconditions**: User is running the TUI.

### Steps

1. **Export scenario**: User finishes a session and types `/export`. A markdown file is written to disk. The TUI continues running. The current conversation state is unchanged. The export is a point-in-time snapshot of the local `transcript` slice (roles, content, timestamps). It does NOT communicate with the server after the snapshot is taken.
2. **Resumption scenario**: User restarts the TUI later (`harnesscli --tui`). They open the session picker and select a past session. The TUI sets `conversationID` to the selected session's ID. The next message posted to `POST /v1/runs` carries that `conversationID`. The server retrieves the conversation context (stored server-side) and continues the conversation. The local viewport starts empty — the TUI does not reload prior messages into the viewport; it only links the next run to the prior conversation.

### Variations

- **Export + Resumption**: A user can export a session to have a local record, then quit and resume the session in a future TUI launch. Both operations are independent.
- **Resumption with a cleared viewport**: When resuming, the local viewport is empty. The conversation history exists on the server and is available to the LLM via the `conversationID`, but the user sees no prior messages in the TUI unless they scroll up (there are none to scroll — the viewport only contains messages from the current TUI session).

### Edge Cases

- **User expects to see prior messages after resuming**: This is a common misconception. Resumption links the new run to the prior conversation for LLM context continuity; it does not replay prior messages into the viewport. Users who need to see prior exchanges should consult an exported transcript file.

---

## STORY-134: Picking a Session from a Long List with Scroll

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Power user with dozens of past sessions who needs to find a specific conversation from two weeks ago
**Goal**: Locate and resume a specific past session from a list that exceeds the visible window
**Preconditions**: More than 10 sessions exist on the server. The session picker is open. The first 10 entries are visible.

### Steps

1. Session picker opens showing the 10 most recent sessions (the visible window; `maxVisibleRows = 10`). A dimmed footer reads `... N more` where N is the count of entries beyond row 10.
2. User presses `j` or **Down** → Selection moves from row 1 to row 2. No scroll occurs (selected item is still within the visible window).
3. User continues pressing `j` until they reach row 10 (the last visible row).
4. User presses `j` one more time → Selection moves to row 11. The `adjustScroll` function fires: `offset = selected - maxVisibleRows + 1 = 1`. The view now shows rows 2–11. The `... N-1 more` footer updates.
5. User continues scrolling down until the target session row is highlighted.
6. User presses **Enter** → `SessionSelectedMsg` is emitted with the selected entry. The picker closes. `conversationID` is set.

### Variations

- **Navigating upward into hidden rows**: If the user is at row 11 and presses `k` or **Up**, the scroll offset decreases so row 10 becomes visible again. `adjustScroll` clamps: `if selected < offset { offset = selected }`.
- **Wrap-around navigation**: Pressing **Up** at the first row wraps to the last entry. If the last entry is beyond `maxVisibleRows`, the scroll offset jumps to `total - maxVisibleRows`.
- **SetEntries called after scrolling**: If the session list is refreshed (e.g., a re-fetch), `SetEntries()` resets both `selected = 0` and `scrollOffset = 0`. The user starts at the top of the new list.

### Edge Cases

- **Exactly 10 entries**: No footer appears; all entries are visible; no scrolling occurs (`adjustScroll` returns 0 when `total <= maxVisible`).
- **List changes while picker is open**: If the server returns a new entry count, `SetEntries()` resets the scroll position. This could disorient a user who was deep in the list.

---

## STORY-135: Exporting an Empty Transcript

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who accidentally runs `/export` before sending any messages, or immediately after `/clear`
**Goal**: Understand what happens when `/export` is run with no conversation content
**Preconditions**: The TUI is running. Either no messages have been sent, or `/clear` was just used. `m.transcript` is `nil` or empty.

### Steps

1. User types `/export` and presses **Enter**.
2. `executeExportCommand` copies the `transcript` slice (nil or empty) into `snapshot`. `len(snapshot) == 0`.
3. The background export goroutine runs. `exporter.Export(entries)` generates the file header only (the `if len(entries) > 0` guard skips the closing separator).
4. The output file is created with content:
   ```
   # Conversation Transcript
   Exported: 2026-03-23 12:00:00
   ```
5. `ExportTranscriptMsg{FilePath: "<path>"}` is returned. Status bar shows `Transcript saved to <path>`.

### Variations

- **After `/clear`**: `/clear` sets `m.transcript = nil`. A subsequent `/export` exports the empty state. The previous conversation (before `/clear`) is not included.

### Edge Cases

- **User expects the pre-clear messages to be in the export**: They are not. `/export` snapshots the current in-memory `transcript` slice only. Users who want to preserve a conversation should run `/export` before running `/clear`.

---

## STORY-136: Aborting Session Picker with Escape

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who opened the session picker by mistake or changed their mind about resuming
**Goal**: Close the session picker without modifying the current session state
**Preconditions**: The session picker is open. The user has navigated partway through the list (e.g., selected row 4). `conversationID` may or may not already be set.

### Steps

1. Session picker is open with row 4 highlighted. The user decides not to resume any session.
2. User presses **Escape** → `sessionpicker.Model.Close()` is called. `open` is set to `false`. The view returns `""`.
3. The picker overlay is dismissed. The main chat view is restored.
4. `conversationID` is unchanged. No `SessionSelectedMsg` was emitted.
5. The input area regains focus. The user can type normally and start a fresh conversation, or continue the current one.

### Variations

- **Escape priority ordering**: In the TUI's global Escape handler, the session picker has a defined priority. If an overlay is open, Escape closes the overlay before any other action.
- **No overlay state leakage**: The scroll offset and selection index inside `sessionpicker.Model` are preserved within the same picker instance. If the picker is re-opened, the selection resets to 0 because `SetEntries()` is called again with a fresh server response.

### Edge Cases

- **Escape with input in the status bar**: If another overlay that has an input mode is open rather than the session picker, Escape first exits that input mode. The session picker uses no sub-input mode and always closes immediately on Escape.
- **Active run while trying to open the session picker**: The TUI should guard against opening the session picker mid-run, since changing `conversationID` mid-conversation could corrupt the multi-turn linkage.


## Gaps & Recommendations

### Covered
All 12 feature areas from the discovery document are fully covered with between 10 and 12 stories each. Every major UI component, keyboard shortcut, overlay, error state, and happy-path flow documented in `discovery.md` has at least one corresponding story.

### Recommended Follow-Up Stories
1. **@-mention file path autocomplete** — The `@` key triggers file mention mode in `inputarea.Model`. No stories cover this flow.
2. **Subagents overlay (/subagents)** — The `/subagents` command fetches `GET /v1/subagents` and injects into the viewport. Not covered.
3. **OpenRouter gateway model routing** — The full round-trip from OpenRouter model selection through run POST with `provider_name: "openrouter"` is mentioned in STORY-042/044 but no story verifies the actual SSE streaming path end-to-end under OpenRouter.
4. **Config persistence across restarts** — Starred models and gateway selection are persisted to `~/.config/harnesscli/config.json`. A story verifying the cold-start replay of these settings (beyond API keys) would be useful.
5. **Accessibility at minimum terminal size** — Several edge cases mention minimum terminal dimensions. A dedicated series of stories for the 80x24 minimum practical size across all overlays would complement STORY-082.
