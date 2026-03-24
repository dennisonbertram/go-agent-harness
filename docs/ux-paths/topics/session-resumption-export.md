# UX Stories: Session Resumption & Export

**Topic**: Session Resumption & Export
**Application**: go-agent-harness TUI (`harnesscli --tui`)
**Status**: Generated 2026-03-23

---

## STORY-SR-001: Resuming a Past Session from the Session Picker

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Developer who ran a multi-turn debugging conversation yesterday and wants to continue where they left off.
**Goal**: Pick a previous conversation from the session picker and continue it with the same conversation ID.
**Preconditions**: At least one prior session exists on the server. The TUI has just launched (`harnesscli --tui`). The main chat view is active with an empty viewport and the `conversationID` field is empty.

### Steps
1. User invokes the session picker (via a trigger mechanism such as a slash command or keybinding) → The `sessionpicker.Model` overlay opens, showing a rounded-border box titled **Sessions**.
2. The list renders entries from `GET /v1/conversations` (or equivalent); each row shows: short session ID (first 8 chars of UUID), start date (`Mar 14`), model name (`gpt-4.1-mini`), turn count (`5 turns`), and up to 60 characters of the last user message.
3. The first entry is highlighted (reverse-video purple background). User reads the row to identify the correct session.
4. User presses `j` or the **Down** arrow to move to the second entry → Highlight moves down. The metadata columns dim and the last message text remains normal weight for unselected rows.
5. User continues navigating until the desired session is highlighted.
6. User presses **Enter** → `SessionSelectedMsg{Entry: entry}` is emitted. The picker closes. The TUI sets `conversationID = entry.ID`.
7. User types a follow-up message in the input area and presses **Enter** to submit → `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id` set to `entry.ID`. The server links this new run to the prior conversation.
8. The assistant's response streams in as usual. The viewport now shows the new turn as a continuation of the prior conversation.

### Variations
- **Many sessions (>10)**: The picker shows 10 rows and a footer `... N more` in dimmed text. The user navigates past row 10 and the list scrolls to reveal additional entries. The scroll window adjusts to keep the selected row visible.
- **Session with a long last message**: The `LastMsg` field is clipped at 60 runes in `SessionEntry`. The row truncates silently (no ellipsis character added by the data layer; the view truncates via `innerWidth` clipping).
- **Wrapping navigation**: Pressing `k` or **Up** on the first row wraps to the last entry. Pressing `j` or **Down** on the last row wraps to the first.

### Edge Cases
- **Conversation server returns fewer fields than expected**: The picker still renders with whatever data is available in `SessionEntry`; empty `LastMsg` results in no last-message column in that row.
- **User presses Enter with no entries loaded** (empty list): `Selected()` returns `(zero, false)`; no `SessionSelectedMsg` is emitted; picker stays open.
- **User changes their mind**: Pressing **Escape** closes the picker without changing `conversationID`. The current session state is unchanged.

---

## STORY-SR-002: Empty State in the Session Picker (No Prior Sessions)

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: A developer running the harness for the first time on a fresh installation.
**Goal**: Open the session picker, discover there are no past sessions, and understand the state clearly.
**Preconditions**: No prior conversations exist on the server. The TUI is running with an empty viewport.

### Steps
1. User opens the session picker overlay → `sessionpicker.Model` opens.
2. The picker renders the **Sessions** title followed by a centered, dimmed message: `No sessions found`.
3. No navigation is possible (the entry list is empty). Pressing `j`, `k`, **Up**, or **Down** is a no-op.
4. Pressing **Enter** is a no-op (no `SessionSelectedMsg` is emitted because `Selected()` returns `ok=false`).
5. User presses **Escape** → picker closes. The viewport is unchanged. User proceeds to start a new conversation by typing in the input area.

### Variations
- **Sessions loaded asynchronously**: The picker may render an initial empty state while the server fetch is in flight, then receive a `SetEntries()` update once the server responds. The selection and scroll offset reset to zero on each `SetEntries()` call.

### Edge Cases
- **Server returns an error**: The picker renders `No sessions found` (same empty state); the error should surface as a status bar message from the calling layer (not from the picker itself).
- **Picker opened at narrow terminal width**: `View()` defaults to `width=80` when passed 0; at very small widths (`<24` inner chars), the centered message still renders without panicking.

---

## STORY-SR-003: Continuing a Multi-Turn Conversation (conversationID Linkage)

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Developer in an ongoing code review session, sending multiple follow-up messages.
**Goal**: Understand how the TUI maintains conversation continuity across multiple turns within a single TUI session.
**Preconditions**: The TUI is running. The user has already sent at least one message. `conversationID` is set.

### Steps
1. User sends the first message (`"Explain the runner loop"`). The input area submits `CommandSubmittedMsg{Value: "Explain the runner loop"}`.
2. `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id: ""` (empty — first turn).
3. Server responds with `{"run_id": "run-abc123"}`. The TUI receives `RunStartedMsg{RunID: "run-abc123"}`.
4. Because `conversationID` was empty, the model sets `conversationID = "run-abc123"`. This is the stable conversation identifier for all subsequent turns.
5. The assistant's response streams into the viewport. `RunCompletedMsg` arrives; `runActive` goes false.
6. User sends a follow-up message (`"Now explain the tool dispatch"`).
7. `startRunCmd` POSTs to `POST /v1/runs` with `conversation_id: "run-abc123"`. The server groups this run under the same conversation.
8. The new response streams in, appearing as a continuation in the same viewport. The `conversationID` field remains `"run-abc123"` for the lifetime of this TUI session.

### Variations
- **User opens a session from the session picker**: `conversationID` is set to `entry.ID` (the ID of the selected session). The very next run POST carries that ID, resuming the server-side conversation context.
- **User runs `/clear`**: The viewport and `transcript` slice are reset to nil. However, `conversationID` is NOT reset by `/clear`. A subsequent message still POSTs with the original `conversationID`, so the server retains the conversation link even after the local view is cleared.

### Edge Cases
- **Two rapid submits before the first RunStartedMsg arrives**: The second submit reads `conversationID == ""` and POSTs with no conversation ID. The two runs may start as separate conversations. This is an expected race condition; users should wait for the first response before sending a second message.
- **Server assigns a different run_id each time**: The TUI only uses the first run's ID as the `conversationID`; subsequent runs get their own `RunID` but the `conversationID` binding is unchanged in the TUI model.

---

## STORY-SR-004: Exporting a Conversation Transcript with /export

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who wants a permanent record of a debugging session to share with a colleague or file a bug report.
**Goal**: Save the current session transcript to a timestamped markdown file.
**Preconditions**: The TUI is running. The user has had a multi-turn conversation. The `transcript` slice contains at least one `TranscriptEntry`.

### Steps
1. User types `/export` in the input area (or uses the autocomplete dropdown to select it).
2. The input is parsed by `executeExportCommand`. A snapshot of `m.transcript` is taken (`copy(snapshot, m.transcript)`) so the export does not race with future transcript appends.
3. `transcriptexport.NewExporter(defaultExportDir())` is constructed. `defaultExportDir()` resolves to the OS cache directory (`~/Library/Caches/harness/transcripts` on macOS, `~/.cache/harness/transcripts` on Linux), falling back to `~/.harness/transcripts`, then `$TMPDIR/harness/transcripts`.
4. The export runs as a background `tea.Cmd` (non-blocking — the UI remains interactive while the file is written).
5. The exporter generates a filename: `transcript-YYYYMMDD-HHMMSS.md` using the current local time (e.g., `transcript-20260323-114704.md`).
6. The output directory is created with `os.MkdirAll` (mode `0755`) if it does not already exist.
7. The markdown file is written with a header (`# Conversation Transcript`, `Exported: YYYY-MM-DD HH:MM:SS`) and sections for each entry (User, Assistant, Tool) with timestamps.
8. On success: `ExportTranscriptMsg{FilePath: "/Users/alice/.cache/harness/transcripts/transcript-20260323-114704.md"}` is returned.
9. The main model receives `ExportTranscriptMsg` and calls `m.setStatusMsg("Transcript saved to " + msg.FilePath)`. The status bar flashes the path for 3 seconds.

### Variations
- **User runs `/export` multiple times**: Each invocation produces a distinct filename (different second timestamp). No existing files are overwritten.
- **User runs `/export` before sending any messages**: The `transcript` slice is nil. The export still runs, producing a markdown file with only the header and no conversation entries. The status bar reports success with the file path.

### Edge Cases
- **Output directory cannot be created** (permissions issue): `os.MkdirAll` returns an error. `exporter.Export()` returns `("", err)`. `ExportTranscriptMsg{FilePath: ""}` is returned. The main model receives this and calls `m.setStatusMsg("Export failed")`. The status bar shows **Export failed** for 3 seconds in red.
- **Disk full**: `os.WriteFile` returns an error. Same error path: `ExportTranscriptMsg{FilePath: ""}` → status bar shows **Export failed**.
- **User navigates away during export**: Because the export runs as a `tea.Cmd` in a goroutine, the UI remains responsive. The status message appears when the goroutine completes, regardless of what the user is doing.

---

## STORY-SR-005: Locating the Exported Transcript File

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who ran `/export` and now wants to open or share the file.
**Goal**: Find the transcript file on disk after a successful export.
**Preconditions**: `/export` has been run successfully. The status bar showed `Transcript saved to <path>`.

### Steps
1. User reads the path from the status bar (visible for 3 seconds, e.g., `/Users/alice/Library/Caches/harness/transcripts/transcript-20260323-114704.md`).
2. User opens a separate terminal or file manager and navigates to the path.
3. The file is a plain markdown document structured as:
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
4. User can open the file in any markdown viewer, share it over Slack, or attach it to a bug report.

### Variations
- **Status bar dismissed before user reads the path**: The path is no longer visible in the TUI after 3 seconds. The user can find the file by listing the export directory: `ls ~/Library/Caches/harness/transcripts/` (or the platform-appropriate path). Files are sorted by timestamp in their names, so the most recent transcript is easily identifiable.
- **Multiple exports in the same session**: Each export file has a unique timestamp. The user can distinguish them by time.

### Edge Cases
- **Very long file path on small terminals**: The status bar message is constrained by `MaxWidth(width)` in the export status renderer. Overflow is clipped, not truncated with ellipsis — the path may be cut off in narrow terminals. The user can still find the file via the directory listing.

---

## STORY-SR-006: Navigating Input History Within the Current Session

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who wants to re-send or edit a message they sent a few turns ago.
**Goal**: Retrieve a past message from in-session history to re-use or modify it.
**Preconditions**: The user has sent at least 2 messages in the current TUI session. The `inputarea.History` slice is non-empty.

### Steps
1. User focuses the input area (it is always focused when no overlay is open).
2. User presses **Up** (`ctrl+p` or up arrow) once → The history navigates backward to the most recently sent message. The current draft text is saved internally (`h.draft = currentText`). The input field now shows the most recent message.
3. User presses **Up** again → The input field shows the second-most-recent message (if it exists and is not a consecutive duplicate).
4. User presses **Up** repeatedly until the desired message appears. Navigation stops at the oldest entry (pressing **Up** further is a no-op at the oldest item, not a wrap).
5. User edits the retrieved message (e.g., changes a parameter or corrects a typo).
6. User presses **Enter** to submit the modified message → The new message is submitted. The history navigation position resets to the draft (`pos = -1`). The submitted text is pushed to the front of the history (unless it is a consecutive duplicate of the most recent entry).
7. User presses **Down** to navigate forward in history → The input shows the next newer entry, eventually returning to the saved draft (`h.draft`) when past the newest entry.

### Variations
- **Draft restoration**: If the user started typing something before pressing **Up**, that draft is saved and returned when the user presses **Down** past the most recent history entry.
- **Consecutive duplicate suppression**: Pressing **Up** to retrieve `"hello"` and then submitting `"hello"` again does not add a second copy to history (`Push()` skips consecutive duplicates).
- **History at capacity**: The history holds at most 100 entries. When full, the oldest entry is dropped on each new push.

### Edge Cases
- **Empty history**: Pressing **Up** with no history entries is a no-op. The input field text is unchanged.
- **Single entry in history**: **Up** shows that entry; **Up** again is a no-op (not a wrap); **Down** returns to the draft.
- **History is cleared with `/clear`**: `/clear` resets the viewport and `transcript` slice but does NOT clear `inputarea.History`. Previously sent messages remain accessible via **Up** after a `/clear`.

---

## STORY-SR-007: Distinguishing Export (Transcript) from Resumption (Session)

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: New user who is confused about what "exporting" and "resuming" each mean.
**Goal**: Understand the conceptual difference between the two operations and when to use each.
**Preconditions**: User is running the TUI.

### Steps
1. **Export scenario**: User finishes a session and types `/export`. A markdown file is written to disk. The TUI continues running. The current conversation state is unchanged. The export is a point-in-time snapshot of the local `transcript` slice (roles, content, timestamps). It does NOT communicate with the server after the snapshot is taken.
2. **Resumption scenario**: User restarts the TUI later (`harnesscli --tui`). They open the session picker and select a past session. The TUI sets `conversationID` to the selected session's ID. The next message posted to `POST /v1/runs` carries that `conversationID`. The server retrieves the conversation context (stored server-side) and continues the conversation. The local viewport starts empty — the TUI does not reload prior messages into the viewport; it only links the next run to the prior conversation.

### Variations
- **Export + Resumption**: A user can export a session to have a local record, then quit and resume the session in a future TUI launch. Both operations are independent.
- **Resumption with a cleared viewport**: When resuming, the local viewport is empty (it is not pre-populated with prior messages). The conversation history exists on the server and is available to the LLM via the `conversationID`, but the user sees no prior messages in the TUI unless they scroll up (there are none to scroll — the viewport only contains messages from the current TUI session).

### Edge Cases
- **User expects to see prior messages after resuming**: This is a common misconception. Resumption links the new run to the prior conversation for LLM context continuity; it does not replay prior messages into the viewport. Users who need to see prior exchanges should consult an exported transcript file.

---

## STORY-SR-008: Picking a Session from a Long List with Scroll

**Type**: medium
**Topic**: Session Resumption & Export
**Persona**: Power user with dozens of past sessions who needs to find a specific conversation from two weeks ago.
**Goal**: Locate and resume a specific past session from a list that exceeds the visible window.
**Preconditions**: More than 10 sessions exist on the server. The session picker is open. The first 10 entries are visible.

### Steps
1. Session picker opens showing the 10 most recent sessions (the visible window; `maxVisibleRows = 10`). A dimmed footer reads `... N more` where N is the count of entries beyond row 10.
2. User presses `j` or **Down** → Selection moves from row 1 to row 2. No scroll occurs (selected item is still within the visible window).
3. User continues pressing `j` until they reach row 10 (the last visible row). The footer `... N more` is still visible.
4. User presses `j` one more time → Selection moves to row 11. The `adjustScroll` function fires: `offset = selected - maxVisibleRows + 1 = 1`. The view now shows rows 2–11. The `... N-1 more` footer updates.
5. User continues scrolling down until the target session row is highlighted.
6. User presses **Enter** → `SessionSelectedMsg` is emitted with the selected entry. The picker closes. `conversationID` is set.

### Variations
- **Navigating upward into hidden rows**: If the user is at row 11 and presses `k` or **Up**, the scroll offset decreases so row 10 becomes visible again. The `adjustScroll` function clamps the offset: `if selected < offset { offset = selected }`.
- **Wrap-around navigation in a long list**: Pressing **Up** at the first row wraps to the last entry. If the last entry is beyond `maxVisibleRows`, the scroll offset jumps to `total - maxVisibleRows`.
- **SetEntries called after scrolling**: If the session list is refreshed (e.g., a re-fetch from the server), `SetEntries()` resets both `selected = 0` and `scrollOffset = 0`. The user starts at the top of the new list.

### Edge Cases
- **Exactly 10 entries**: No footer appears; all entries are visible; no scrolling occurs (`adjustScroll` returns 0 when `total <= maxVisible`).
- **List changes while picker is open**: If the server returns a new entry count, `SetEntries()` resets the scroll position. This could disorient a user who was deep in the list. The picker does not preserve scroll position across `SetEntries()` calls.

---

## STORY-SR-009: Exporting an Empty Transcript

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who accidentally runs `/export` before sending any messages, or immediately after `/clear`.
**Goal**: Understand what happens when `/export` is run with no conversation content.
**Preconditions**: The TUI is running. Either no messages have been sent, or `/clear` was just used. `m.transcript` is `nil` or empty.

### Steps
1. User types `/export` and presses **Enter**.
2. `executeExportCommand` copies the `transcript` slice (which is nil or empty) into `snapshot`. `len(snapshot) == 0`.
3. The background export goroutine runs. `exporter.Export(entries)` generates the file header only (the `if len(entries) > 0` guard skips the closing separator).
4. The output file is created with content:
   ```
   # Conversation Transcript
   Exported: 2026-03-23 12:00:00
   ```
5. `ExportTranscriptMsg{FilePath: "<path>"}` is returned. Status bar shows `Transcript saved to <path>`.

### Variations
- **After `/clear`**: `/clear` sets `m.transcript = nil`. A subsequent `/export` exports the empty state. The previous conversation (before `/clear`) is not included — only the post-clear session (which is empty).

### Edge Cases
- **User expects the pre-clear messages to be in the export**: They are not. `/export` snapshots the current in-memory `transcript` slice only. Users who want to preserve a conversation should run `/export` before running `/clear`.

---

## STORY-SR-010: Aborting Session Picker with Escape

**Type**: short
**Topic**: Session Resumption & Export
**Persona**: Developer who opened the session picker by mistake or changed their mind about resuming.
**Goal**: Close the session picker without modifying the current session state.
**Preconditions**: The session picker is open. The user has navigated partway through the list (e.g., selected row 4). `conversationID` may or may not already be set.

### Steps
1. Session picker is open with row 4 highlighted. The user decides not to resume any session.
2. User presses **Escape** → `sessionpicker.Model.Close()` is called. `open` is set to `false`. The view returns `""`.
3. The picker overlay is dismissed. The main chat view is restored.
4. `conversationID` is unchanged. No `SessionSelectedMsg` was emitted.
5. The input area regains focus. The user can type normally and start a fresh conversation, or continue the current one.

### Variations
- **Escape priority ordering**: In the TUI's global Escape handler, the session picker (like other overlays) has a defined priority. If an overlay is open, Escape closes the overlay before any other action (it does not cancel an active run or clear input while an overlay is blocking).
- **No overlay state leakage**: The scroll offset and selection index inside `sessionpicker.Model` are preserved within the same picker instance. If the picker is re-opened (in a future interaction), the selection resets to 0 because `SetEntries()` is called again with a fresh server response.

### Edge Cases
- **Escape with input in the status bar**: If the model switcher (or another overlay that has an input mode) is open rather than the session picker, Escape first exits that input mode. The session picker uses no sub-input mode and always closes immediately on Escape.
- **Active run while trying to open the session picker**: The TUI should guard against opening the session picker mid-run, since changing `conversationID` mid-conversation could corrupt the multi-turn linkage. The current implementation does not explicitly block this — it is a design consideration.

---

## Summary

| Story | Type | Interaction | Key Insight |
|-------|------|-------------|-------------|
| STORY-SR-001 | medium | Resume from session picker | `SessionSelectedMsg` sets `conversationID`; next run POSTs with it |
| STORY-SR-002 | short | Empty session picker | `No sessions found` centered in dim text; navigation no-ops |
| STORY-SR-003 | medium | Multi-turn `conversationID` linkage | First `RunStartedMsg.RunID` becomes `conversationID`; persists across turns |
| STORY-SR-004 | short | `/export` command | Background goroutine writes `transcript-YYYYMMDD-HHMMSS.md`; status bar shows path |
| STORY-SR-005 | short | Finding the exported file | Default dir: OS cache dir / `harness/transcripts`; filename is timestamp-sorted |
| STORY-SR-006 | short | Input history Up/Down | Up navigates toward older entries; Down returns toward draft; max 100 entries |
| STORY-SR-007 | short | Export vs. Resumption concepts | Export = local markdown snapshot; Resumption = server-side context via `conversationID` |
| STORY-SR-008 | medium | Long session list with scroll | Visible window is 10 rows; `adjustScroll` keeps selected row in view; `... N more` footer |
| STORY-SR-009 | short | Export with empty transcript | Produces header-only markdown file; success status shown; pre-clear messages not included |
| STORY-SR-010 | short | Escape closes picker | No `conversationID` change; scroll state preserved within instance; input regains focus |

---

## Architecture Notes

### conversationID Flow
```
First run:   POST /v1/runs { conversation_id: "" }
             ← { run_id: "run-abc" }
             m.conversationID = "run-abc"

Second run:  POST /v1/runs { conversation_id: "run-abc" }
             ← { run_id: "run-def" }
             m.conversationID unchanged ("run-abc")

After resumption via session picker:
             m.conversationID = entry.ID (the selected session's ID)
Next run:    POST /v1/runs { conversation_id: entry.ID }
```

### Transcript Entry Roles
The `transcript` slice accumulates three entry roles:
- `"user"` — when the user submits a message (`inputarea.CommandSubmittedMsg`)
- `"assistant"` — when a `run.completed` SSE event fires and `lastAssistantText` is non-empty
- `"tool"` — (future; currently assistant text accumulates without per-tool entries)

### Export Output Directory Resolution
```
1. os.UserCacheDir() + "/harness/transcripts"  (preferred)
2. os.UserHomeDir() + "/.harness/transcripts"   (fallback)
3. os.TempDir() + "/harness/transcripts"        (last resort)
```
The directory is created with `os.MkdirAll(dir, 0755)` on each export. Files are named `transcript-YYYYMMDD-HHMMSS.md` using Go's `time.Format("20060102-150405")`.

### Session Picker Display Row Layout
```
  <shortID>  <date>  <model>  <N turns>  <last msg up to 60 chars>
```
- `shortID`: first 8 characters of the full UUID
- `date`: `time.Format("Jan 2")`
- `model`: the model string verbatim (e.g., `gpt-4.1-mini`)
- turns: `fmt.Sprintf("%d turns", e.TurnCount)`
- last message: clipped to `lastMsgMaxLen = 60` runes
- Unselected rows: metadata dimmed (`#5C5C5C` dark / `#9B9B9B` light), message text at normal color
- Selected row: full row highlighted with purple background (`#7D56F4` dark / `#874BFD` light), white foreground, padded to `innerWidth`

---

**Story Count**: 10
**Last Updated**: 2026-03-23
