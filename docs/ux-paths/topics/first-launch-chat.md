# First Launch & Chat — User Journey Stories

Generated: 2026-03-23
Topic: First Launch & Chat

---

## STORY-LC-001: Minimal First Launch — Empty Chat State

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

## STORY-LC-002: Sending the First Message

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

## STORY-LC-003: Watching the Streamed Assistant Response

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

## STORY-LC-004: Run Completes — Status Bar Cost Update

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

## STORY-LC-005: Multi-Turn Conversation — ConversationID Linkage

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

## STORY-LC-006: Typing `/help` on First Launch

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

## STORY-LC-007: Long First Session — Multi-Turn with Cost Accumulation

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

## STORY-LC-008: First Message When Server Is Unreachable

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

## STORY-LC-009: Using the `❯` Prompt for Multi-Line Input

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

## STORY-LC-010: Input History Navigation

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

## STORY-LC-011: Status Bar Transient Message on Completion

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

## STORY-LC-012: Cold Start with Persisted API Keys

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
