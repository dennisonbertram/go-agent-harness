# UX Stories: Conversation Management

**Topic**: Conversation Management
**Generated**: 2026-03-23

---

## STORY-CM-001: Clearing a Long Conversation to Start Fresh

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who has been iterating on a problem for 30 minutes and wants a clean slate without quitting the TUI
**Goal**: Reset the viewport, transcript accumulator, and assistant text so the next run starts from an empty state
**Preconditions**: TUI is open with multiple turns of conversation visible in the viewport; no run is currently active

### Steps

1. User types `/clear` in the input area → The slash-complete dropdown opens and shows `clear` as the only match
2. User presses Enter to accept → The command executes immediately
3. Viewport is replaced with a fresh empty `viewport.New(width, height)` → All previous message bubbles and tool call blocks disappear
4. `transcript []TranscriptEntry` is set to `nil` → The in-memory transcript accumulator is empty
5. `lastAssistantText` is reset to `""` and `responseStarted` is set to `false` → The assistant text accumulator for the next run starts clean
6. The thinking bar is cleared → No stale reasoning text appears above the input
7. Status bar shows "Conversation cleared" for 3 seconds, then auto-dismisses → User has visual confirmation

### Variations

- **Autocomplete shortcut**: User types `/cl` then presses Tab → dropdown auto-executes `/clear` because it is the only match
- **Full type**: User types `/clear` and presses Enter directly without using the dropdown → same outcome
- **Keyboard-only path**: User opens help with `ctrl+h`, sees `/clear` in the commands tab, then closes and types the command manually

### Edge Cases

- **Clear with active run**: `/clear` is submitted while a run is in progress — the command still clears the transcript and viewport immediately; the active SSE stream continues delivering events which will be appended to the newly empty viewport
- **Clear an already-empty conversation**: `/clear` with nothing in the viewport still executes cleanly; status bar shows "Conversation cleared" even though there was nothing to remove
- **conversationID is not reset**: After `/clear`, `conversationID` still holds the first run's ID; the next message sent will still pass the same conversation linkage to the server

---

## STORY-CM-002: Establishing a Multi-Turn Conversation Identity

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer building a multi-step coding task who needs the harness to link successive runs together as one conversation
**Goal**: Understand how the TUI establishes and propagates the conversationID across turns so the server groups them correctly
**Preconditions**: TUI is open, no prior messages sent in this session (`conversationID` is `""`)

### Steps

1. User types "Scaffold a Go HTTP handler for POST /api/users" and presses Enter → `inputarea.CommandSubmittedMsg` is emitted; a user-role `TranscriptEntry` is appended with the message text and current timestamp
2. TUI calls `startRunCmd(baseURL, prompt, conversationID="", ...)` → POST to `/v1/runs` with no `conversation_id` field in the JSON body
3. Server responds with `{"run_id": "run-7f3a9c"}` → `RunStartedMsg{RunID: "run-7f3a9c"}` is emitted
4. TUI sets `m.conversationID = "run-7f3a9c"` and `m.RunID = "run-7f3a9c"` → The first run ID becomes the stable conversation identifier for this session
5. SSE stream delivers assistant deltas; response appears in the viewport; run completes → An assistant-role `TranscriptEntry` is appended with the accumulated `lastAssistantText`
6. User types a follow-up: "Now add input validation middleware" and presses Enter → `startRunCmd` is called with `conversationID = "run-7f3a9c"`
7. POST to `/v1/runs` carries `"conversation_id": "run-7f3a9c"` → Server links this turn to the same conversation
8. Second run streams and completes → Transcript now has 4 entries: user/assistant/user/assistant

### Variations

- **No follow-up**: User sends only one message and never sends another; `conversationID` is set but never used in a subsequent POST — functionally harmless
- **Multiple conversations in one session**: After `/clear` the viewport resets but `conversationID` is preserved, so all messages in the session remain under the same conversation umbrella on the server

### Edge Cases

- **First POST fails (network error)**: `RunFailedMsg` is emitted with the error text; `conversationID` remains `""`; the user can retry and the retry will again POST without a conversation ID, establishing a new conversation on success
- **Server returns malformed run_id**: JSON decode fails; `RunFailedMsg` is emitted with "decode run response: …"; `conversationID` is never set
- **Run ID is empty string in response**: `RunStartedMsg{RunID: ""}` would set `conversationID` to `""`, effectively leaving multi-turn linkage disabled for the session until the TUI is restarted

---

## STORY-CM-003: Exporting a Conversation Transcript to Markdown

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who wants to preserve a conversation for a code review or share findings with a colleague
**Goal**: Write the current session's transcript entries to a timestamped markdown file and see the file path confirmed in the status bar
**Preconditions**: At least one full turn (user message + assistant response) has completed; `transcript` slice has at least two entries

### Steps

1. User types `/export` and presses Enter → The slash-complete dropdown opens and auto-executes because "export" is an unambiguous prefix
2. `executeExportCommand` takes a snapshot copy of `m.transcript` → A new slice is allocated via `copy()` so the live accumulator is not modified
3. `transcriptexport.NewExporter(defaultExportDir())` is constructed → Output directory resolves to `~/Library/Caches/harness/transcripts` on macOS (or `~/.harness/transcripts` as fallback)
4. `exporter.Export(snapshot)` runs in a background `tea.Cmd` → The output directory is created if it does not exist; the file is named `transcript-20260323-114704.md` using `time.Now().Format("20060102-150405")`
5. Markdown file is written with a `# Conversation Transcript` header, export timestamp, and sections for each entry formatted as `## User [3:47 PM]` or `## Assistant [3:47 PM]` with the message content below
6. `ExportTranscriptMsg{FilePath: "/Users/alice/Library/Caches/harness/transcripts/transcript-20260323-114704.md"}` is returned → Status bar shows "Transcript saved to /Users/alice/Library/Caches/harness/transcripts/transcript-20260323-114704.md" for 3 seconds

### Variations

- **Tab completion**: User types `/ex` then Tab → single match auto-executes `/export`
- **Export after /clear**: If the user runs `/clear` before `/export`, the `transcript` slice is `nil`; the export still runs and produces a valid (empty) markdown file with only the header and timestamp

### Edge Cases

- **Directory not writable**: `os.MkdirAll` or `os.WriteFile` returns an error → `ExportTranscriptMsg{FilePath: ""}` is returned; status bar shows "Export failed" for 3 seconds
- **Export during active run**: The snapshot is taken from the transcript at the moment `/export` is issued; any assistant deltas that arrive after the snapshot is taken are not included in this export
- **Long file path in status bar**: If the resolved absolute path is very long, the status bar renders it truncated by `lipgloss.MaxWidth`

---

## STORY-CM-004: Recovering a Previous Message with History Navigation

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who typed a long prompt, sent it, and now wants to send a slightly modified version without retyping
**Goal**: Use the up arrow in the input area to recall the last submitted message and edit it before re-sending
**Preconditions**: At least one message has been submitted in the current session; no run is currently active

### Steps

1. Input area is empty and focused → cursor shows after `❯`
2. User presses the up arrow key → `inputarea` calls `h.Up(currentText)` — since `currentText` is `""` (empty draft), the empty draft is saved internally and the most recent history entry is loaded
3. Input area now shows the previously submitted message text → User can see the full prior prompt
4. User edits the text (adds, removes, or changes words) → Cursor movement and editing work normally within the recalled text
5. User presses Enter to submit the edited prompt → The edited text is sent; the edited text is pushed to history as a new entry; the draft position resets

### Variations

- **Navigate multiple steps back**: User presses up again after step 3 → the second-most-recent message loads; each subsequent up press moves one step older
- **Return to draft**: After navigating into history, user presses the down arrow → history position moves forward; pressing down past the most recent entry restores the saved empty draft
- **ctrl+p / ctrl+n**: Same behavior as up/down arrow for users who prefer Emacs-style navigation

### Edge Cases

- **History is empty**: User presses up on the first message of a session → `h.Up` returns the current text unchanged; input area content does not change
- **History cap at 100 entries**: After 100 submitted messages, the 101st push evicts the oldest entry; the 100-entry ring always reflects the most recent 100 messages
- **History does not persist across TUI restarts**: `History` is an in-memory value type on `inputarea.Model`; closing and reopening the TUI starts with an empty history

---

## STORY-CM-005: Copying the Last Assistant Response to the Clipboard

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who wants to paste the assistant's code snippet or explanation into an editor without selecting text in the terminal
**Goal**: Copy the full accumulated assistant response from the current (or most recently completed) run to the system clipboard
**Preconditions**: At least one run has completed; `lastAssistantText` is non-empty; terminal supports OSC52 (not headless/dumb)

### Steps

1. User presses `ctrl+s` → TUI matches `m.keys.Copy` in the key handler
2. `CopyToClipboard(m.lastAssistantText)` is called → OSC52 escape sequence `\033]52;c;<base64-encoded-text>\a` is written to stdout
3. Terminal forwards the clipboard write to the OS clipboard → System clipboard now contains the full assistant response text
4. `CopyToClipboard` returns `true` → Status bar shows "Copied!" for 3 seconds, then auto-dismisses

### Variations

- **After multiple turns**: `lastAssistantText` holds only the most recent run's assistant response (it is reset to `""` at the start of each new run); pressing `ctrl+s` copies only the last turn's response, not the entire session history
- **Pressing ctrl+s mid-stream**: If a run is active and the assistant is still streaming, `lastAssistantText` holds what has accumulated so far; the partial response is copied

### Edge Cases

- **Headless/CI terminal**: `IsHeadless()` returns true when `TERM` is unset or `"dumb"`; `CopyToClipboard` returns `false` without writing OSC52 → status bar shows "Copy unavailable" for 3 seconds
- **Empty lastAssistantText**: If no assistant response has been received yet (e.g. immediately after `/clear`), `ctrl+s` copies an empty string; `CopyToClipboard("")` still returns true (OSC52 succeeds); status bar shows "Copied!" but clipboard is empty
- **Terminal does not support OSC52**: The write to stdout succeeds at the OS level, `CopyToClipboard` returns `true`, but the terminal silently ignores the escape sequence; the user sees "Copied!" but clipboard is unchanged — no further feedback is possible from the TUI

---

## STORY-CM-006: Exporting After a Multi-Turn Session

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer who has had a 10-turn conversation and wants a complete markdown record of all user and assistant messages
**Goal**: Export all turns accumulated in the session transcript to a single markdown file, with each turn's role and timestamp preserved
**Preconditions**: 10 turns completed; `transcript` has 20 entries (alternating user/assistant); no run is active

### Steps

1. User types `/export` and presses Enter
2. `executeExportCommand` takes a snapshot: `copy(snapshot, m.transcript)` — 20 entries are copied
3. Background `tea.Cmd` runs `exporter.Export(snapshot)`
4. Markdown file is written with all 20 sections in submission order:
   - `## User [2:30 PM]` → message text
   - `## Assistant [2:30 PM]` → full assistant response
   - `## User [2:31 PM]` → next user message
   - … and so on for all 10 turns
5. Each section is separated by `---` horizontal rules
6. `ExportTranscriptMsg{FilePath: "..."}` is received → Status bar shows "Transcript saved to …" for 3 seconds
7. User opens the file in a text editor → Sees the full conversation readable as a structured markdown document

### Variations

- **Export mid-conversation**: User exports after 5 turns, continues chatting for 5 more, then exports again → Two files are created; the second file contains all 10 turns (the transcript accumulator is never trimmed mid-session)
- **Export-then-clear**: User exports, then runs `/clear` → The exported file is preserved on disk; the in-memory `transcript` is set to `nil`; a subsequent `/export` creates a new (empty) file

### Edge Cases

- **Very long assistant responses**: Each entry's `Content` field is written verbatim; there is no size limit on individual entries — the markdown file can be arbitrarily large
- **Concurrent export calls**: If the user runs `/export` twice in quick succession, two separate files are written with slightly different timestamps; both are valid exports of the snapshot at that instant
- **Tool entries in transcript**: `TranscriptEntry` supports `Role: "tool"` with a `ToolName` field; if any tool-role entries are present they render as `## Tool: bash [2:31 PM]` in the export

---

## STORY-CM-007: Using /clear to Reset Between Independent Tasks

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer who uses a single TUI session for multiple unrelated tasks throughout the day
**Goal**: Clear one task's conversation history before starting a new unrelated task, keeping the session cost counter running without confusion from stale viewport content
**Preconditions**: Task A conversation is complete; at least 5 turns are visible; no run is active

### Steps

1. User has finished task A; viewport shows all its turns
2. User types `/clear` → Dropdown shows `clear`; user presses Enter
3. `executeClearCommand` runs:
   - `m.vp = viewport.New(m.width, m.layout.ViewportHeight)` — viewport is fresh
   - `m.transcript = nil` — transcript accumulator emptied
   - `m.lastAssistantText = ""` — assistant text accumulator reset
   - `m.responseStarted = false` — streaming state reset
   - `m.activeAssistantLineCount = 0` — tail-splice counter reset
   - `m.clearThinkingBar()` — thinking bar cleared
4. Status bar shows "Conversation cleared" for 3 seconds
5. Status bar still shows the cumulative cost from task A (cost counter is not reset by `/clear`)
6. User types the first message for task B and sends it
7. Run POST carries `conversationID` from task A (conversation ID is not reset by `/clear`) → Server groups task B's runs with task A's on the server side, even though the TUI viewport is fresh

### Variations

- **User wants a true fresh conversation identity**: There is no in-TUI way to reset `conversationID` without restarting the process; the user must quit (`/quit` or `ctrl+c`) and relaunch `harnesscli --tui`

### Edge Cases

- **Viewport scroll offset**: After `/clear`, the new viewport starts at the bottom with zero scroll offset; `ViewportScrollOffset()` returns 0
- **Tool state maps not cleared**: `toolViews`, `toolTimers`, `toolNames`, `toolArgs`, `toolLineCounts` are not explicitly cleared by `/clear`; these maps hold references to past tool call data but are keyed by call ID and will be overwritten by new runs without causing visible artifacts

---

## STORY-CM-008: Navigating Long History Within a Session

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who is refining a prompt through many iterations and wants to reuse or modify a message from several turns ago
**Goal**: Navigate backward through up to 100 history entries using the up arrow, find a specific prior message, and send it again
**Preconditions**: 15 or more messages have been submitted in the current session; input area is focused and empty

### Steps

1. User presses up arrow three times → History position moves from draft (-1) to entries at indices [0], [1], [2] (newest-first order); input area shows the text of each prior message in sequence
2. On the third press, input area shows the message from 3 turns ago
3. User presses Enter to submit it unchanged → The recalled text is submitted; it is pushed to history as a new entry (duplicate entries accumulate independently); history position resets to draft
4. Input area returns to empty, ready for the next message

### Variations

- **Modify before sending**: User presses up twice, edits the recalled text, then presses Enter → The modified text is sent; the original entry in history is unchanged; the modified text becomes the newest history entry
- **Abandon recall**: User navigates up into history, then presses the down arrow until returning to the draft (empty input) → No message is submitted; nothing is pushed to history; the draft is restored exactly as saved

### Edge Cases

- **Up past oldest entry**: User keeps pressing up after reaching the oldest entry (entry at index len(history)-1) → `h.Up` is a no-op when already at the oldest entry; the oldest entry remains displayed
- **History after /clear**: `/clear` does not call `h.Clear()` on the input area's history; history entries from before `/clear` remain navigable after the viewport is cleared
- **Submitting empty via history**: If the user navigates to an entry, deletes all its content, and presses Enter → Empty string is submitted; `startRunCmd` sends an empty prompt; this is permitted by the TUI but may return a server error

---

## STORY-CM-009: Observing Transcript Accumulation Across Turns

**Type**: medium
**Topic**: Conversation Management
**Persona**: Developer or QA engineer who wants to verify that transcript entries are being recorded correctly for eventual export
**Goal**: Confirm that each user message and assistant response is recorded as a separate `TranscriptEntry` with role, content, and timestamp
**Preconditions**: TUI has just been opened; no messages sent yet; `transcript` is `nil`

### Steps

1. User sends message "What is the capital of France?" → At submit time, a `TranscriptEntry{Role: "user", Content: "What is the capital of France?", Timestamp: <now>}` is appended to `m.transcript`; transcript length is now 1
2. `RunStartedMsg{RunID: "run-a1b2"}` arrives → `conversationID` is set; `lastAssistantText` is reset to `""`
3. SSE stream delivers `assistant.message.delta` events: "Paris", ".", " The", " capital", " is", " Paris." → Each delta is concatenated into `m.lastAssistantText`; the viewport shows the streamed text in real time
4. `SSEDoneMsg{EventType: "run.completed"}` arrives → `lastAssistantText` ("Paris. The capital is Paris.") is appended as `TranscriptEntry{Role: "assistant", Content: "Paris. The capital is Paris.", Timestamp: <now>}`; transcript length is now 2; `lastAssistantText` is reset to `""`
5. User sends message "And Germany?" → Another user entry appended; transcript length is 3
6. Run completes with response "Berlin." → Another assistant entry appended; transcript length is 4
7. User types `/export` → All 4 entries are exported in order; timestamps reflect when each user submit or run completion occurred

### Variations

- **No assistant deltas received**: Run completes with zero `assistant.message.delta` events → `lastAssistantText` is `""`; no assistant entry is appended (empty assistant entries are suppressed); transcript has only the user entry for that turn

### Edge Cases

- **Assistant delta arrives but run never completes**: SSE stream is interrupted; `SSEDoneMsg` never arrives → `lastAssistantText` accumulates but is never flushed to the transcript; if the user then sends a new message, the old `lastAssistantText` is reset at the start of the new run; the partial response is lost from the transcript
- **Timestamps are local time**: `entry.Timestamp` is `time.Now()` at the moment of submission or completion; the export renders them as `3:47 PM` (12-hour with no date) — two entries at the same clock minute will show the same time string even if seconds differ

---

## STORY-CM-010: Handling an Export Failure Gracefully

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer working on a read-only filesystem or in a restricted environment where the cache directory is not writable
**Goal**: Attempt to export and receive a clear failure message rather than a silent error
**Preconditions**: `defaultExportDir()` resolves to a directory the current user cannot write to; transcript has content

### Steps

1. User types `/export` and presses Enter
2. Background `tea.Cmd` runs `exporter.Export(snapshot)`
3. `os.MkdirAll(outputDir, 0o755)` fails with a permission error → `Export` returns `("", error)`
4. `ExportTranscriptMsg{FilePath: ""}` is returned to the TUI update loop
5. The `case ExportTranscriptMsg:` handler checks `msg.FilePath == ""` → calls `m.setStatusMsg("Export failed")`
6. Status bar shows "Export failed" for 3 seconds, then auto-dismisses → User knows the export did not succeed
7. Transcript accumulator is unchanged — no data was lost; the user could retry after fixing the directory permissions

### Variations

- **WriteFile fails but MkdirAll succeeded**: Directory exists and is writable, but a quota is exceeded during `os.WriteFile` → Same flow; `Export` returns an error; status bar shows "Export failed"

### Edge Cases

- **Partial file written**: If `os.WriteFile` fails mid-write, the incomplete file may exist on disk; the TUI does not attempt cleanup of partial files
- **Retry after failure**: User fixes permissions and runs `/export` again → A new snapshot is taken; a new timestamped file is written; status bar shows the success path

---

## STORY-CM-011: Understanding What /clear Does Not Reset

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who has used `/clear` and is unsure whether the conversation is truly isolated from the previous one
**Goal**: Understand the exact scope of what `/clear` resets versus what it preserves
**Preconditions**: User has had a long session; ran `/clear`; is about to send a new message

### Steps

1. User runs `/clear` → viewport and transcript are wiped; status bar confirms "Conversation cleared"
2. User notices status bar still shows the cumulative cost (e.g. "$0.12") → cost counter is NOT reset by `/clear`; it accumulates for the entire TUI session lifetime
3. User opens `/context` → Token count displayed is the total since TUI launch, not since the last clear
4. User opens `/stats` → Usage heatmap shows all runs since TUI launch, including those before the clear
5. User sends a new message → POST carries the same `conversationID` as before the clear → Server still groups this run with all prior runs in the session
6. User realizes: `/clear` is a viewport and transcript reset only; it does not create a new conversation identity or reset cost/token counters

### Variations

- **True session isolation**: To get a genuinely fresh conversation with a new ID, the user must quit (`/quit`) and relaunch `harnesscli --tui`

### Edge Cases

- **Input history preserved**: The input area's history buffer (up to 100 entries) is not cleared by `/clear`; prior messages remain navigable with the up arrow after clearing

---

## STORY-CM-012: Drafting a Message, Abandoning It, and Recalling It Later

**Type**: short
**Topic**: Conversation Management
**Persona**: Developer who starts drafting a message, gets interrupted, navigates to a past message to check wording, then returns to the in-progress draft
**Goal**: Use history navigation without losing the partially typed message in progress
**Preconditions**: Input area contains a partially typed message "Refactor the auth module to use"; user has at least 2 prior messages in history

### Steps

1. Input area shows "Refactor the auth module to use" (partial draft; not yet submitted)
2. User presses up arrow → `h.Up("Refactor the auth module to use")` is called; the current text is saved as the draft ("Refactor the auth module to use"); the most recent history entry is loaded into the input
3. Input area shows the most recent prior message
4. User presses up again → second most-recent message loads
5. User presses down → first most-recent message loads again
6. User presses down again → `h.Down()` returns the saved draft text "Refactor the auth module to use" and resets `pos` to -1 (AtDraft)
7. Input area shows "Refactor the auth module to use" exactly as left → User can continue editing without retyping

### Variations

- **Draft abandoned**: User navigates into history and presses Enter on a historical entry instead of returning to draft → The historical entry is submitted; the draft "Refactor the auth module to use" is permanently lost (it was saved internally but replaced when a new entry is pushed)

### Edge Cases

- **Empty draft navigation**: If the input is empty when up is first pressed, an empty string is saved as the draft; returning to draft via down produces an empty input — same user experience as starting fresh
- **Draft is never submitted to history**: The saved draft text is held only in `h.draft`; it is not pushed to the history entries slice unless the user eventually submits it
