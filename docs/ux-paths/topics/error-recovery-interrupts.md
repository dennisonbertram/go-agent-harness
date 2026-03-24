# UX Stories: Error Recovery & Interrupts

Generated: 2026-03-23
Topic: Error Recovery & Interrupts

---

## STORY-ER-001: Two-Stage Ctrl+C Interrupt During a Long-Running Agent Run

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who sent a long refactoring prompt and wants to stop it mid-run
**Goal**: Cancel an active run cleanly without accidentally quitting the TUI
**Preconditions**: TUI is open, a run is active (`runActive = true`), one or more tool call blocks are visible in the viewport, the thinking bar is showing

### Steps

1. User presses `Ctrl+C` once during the active run → The interrupt banner transitions from Hidden to Confirm state; a centered amber-bordered box appears between the viewport and the input area with the text: `⚠  Press Ctrl+C again to stop, or Esc to continue`. Input area remains visible but the run continues. No tools are interrupted yet.
2. User decides to follow through and presses `Ctrl+C` a second time → The banner transitions from Confirm to Waiting; the centered box is replaced by a dimmed, faint line: `Stopping… (waiting for current tool to finish)`. The cancel function (`cancelRun`) is called, signaling the server-side context cancellation. The run is still listed as active until the SSE stream closes.
3. The server acknowledges the cancellation and the SSE stream terminates → The banner transitions through Done (briefly invisible) and then to Hidden. The `runActive` flag clears to false. The thinking bar is dismissed. The status bar flashes `Interrupted` for 3 seconds.
4. User sees the viewport with the conversation history intact up to the point of interruption → The input area is ready with the `❯` cursor. User can immediately type a follow-up message.

### Variations

- **Tool mid-execution at second Ctrl+C**: The "Stopping… (waiting for current tool to finish)" text is accurate — the cancel context propagates but the harness waits for the current tool call to return before closing the SSE stream. The tool block in the viewport shows a completed (or error) state once it finishes.
- **Ctrl+C with overlay open**: If the `/help` or `/model` overlay is open, the first Ctrl+C closes the overlay (same behavior as Esc for overlays) rather than triggering the interrupt banner. The run continues. The user must close the overlay before using Ctrl+C to interrupt.

### Edge Cases

- **Second Ctrl+C from Waiting state is a no-op on the banner**: `Confirm()` called from Waiting state returns unchanged. The banner stays at Waiting. No double-cancel is issued.
- **Rapid double Ctrl+C (before render cycle)**: Because the model uses value semantics, each BubbleTea Update cycle produces a new copy. Both presses may land in the same Update; the first transitions Hidden → Confirm and the second transitions Confirm → Waiting in the same frame. The cancel function fires once.
- **Run ends naturally while banner is in Confirm state**: The `SSEDoneMsg` arrives and clears `runActive` before the second Ctrl+C. The banner is hidden on the next render (or the next Ctrl+C now becomes a quit).

---

## STORY-ER-002: Esc Key as Single-Press Cancel During an Active Run

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who changed their mind immediately after submitting a prompt
**Goal**: Cancel the current run quickly with a single keypress rather than two Ctrl+C presses
**Preconditions**: TUI is open, a run has just started (`runActive = true`), no overlay is open, input is empty

### Steps

1. User presses `Esc` once with no overlay open and no input text → The multi-priority Esc handler checks, in order: API keys overlay (no), model overlay (no), any other overlay (no), active run (yes). The cancel function is called immediately. `runActive` is set to false. `cancelRun` is cleared to nil.
2. The status bar shows `Interrupted` as a 3-second transient message → The thinking bar disappears. The run is terminated without requiring confirmation.
3. The viewport retains whatever partial output was streamed before cancellation → The input area is immediately ready for the next message.

### Variations

- **Esc with overlay open**: Esc priority means the overlay is closed first (run is NOT cancelled). A second Esc (with no overlay, no text) then cancels the run.
- **Esc with non-empty input and no run**: Esc clears the input text and shows `Input cleared` in the status bar. No run is affected.
- **Esc with overlay open AND run active**: First Esc closes overlay, second Esc cancels run, third Esc is a no-op (no input, no run).

### Edge Cases

- **Esc with no run and empty input**: The handler is a no-op — no quit, no status message, no state change.
- **Esc races with run completing naturally**: If `SSEDoneMsg` arrives in the same event loop tick as the Esc key, BubbleTea serializes them. The Esc may call an already-nil `cancelRun`. The nil guard (`m.cancelRun != nil`) prevents a panic.

---

## STORY-ER-003: Run Failure via SSE `run.failed` Event

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer running an agent against an API that returns a rate-limit error
**Goal**: Understand why the run failed and immediately retry or adjust
**Preconditions**: TUI is open, a run is active, the harness receives a non-retriable error from the provider (e.g., HTTP 429 with JSON error body)

### Steps

1. The harness streams a `run.failed` event through the SSE stream → The SSE bridge converts this into an `SSEDoneMsg{EventType: "run.failed", Error: "provider completion failed: openai request failed (429): {\"error\":{\"message\":\"Rate limit exceeded\",\"type\":\"rate_limit_error\"}}"}`.
2. The TUI Update handler receives `SSEDoneMsg` with `EventType == "run.failed"` → `runActive` is set to false, `sseCh` is cleared, the thinking bar is dismissed. `formatRunError()` splits the error string at the first `{`, rendering it as: `✗ provider completion failed: openai request failed (429)` followed by indented key-value lines from the JSON: `  error:`, `    message: Rate limit exceeded`, `    type: rate_limit_error`. A blank line is appended after the error block.
3. The viewport scrolls to show the formatted error at the bottom → The status bar does not show a separate message (the error is in the viewport, not the status bar).
4. User reads the error, identifies it as a rate-limit issue → User types a new message after waiting, or uses `/model` to switch to a different model or provider. The run state is fully reset; the next message starts a fresh run.

### Variations

- **`run.failed` with empty error string**: `formatRunError("")` returns `["✗ run failed"]`. A single generic line appears in the viewport.
- **`run.failed` with non-JSON error**: The `strings.Index(errStr, "{")` fails to find a `{`. The entire error string is prefixed with `✗ ` and rendered as one line.
- **`RunFailedMsg` arriving directly** (not via SSEDoneMsg, e.g., HTTP error at run creation): The handler at `case RunFailedMsg` appends `✗ <error>` directly, without the `formatRunError` JSON parsing path. Run state is cleared identically.

### Edge Cases

- **`cancelRun` is not nil at SSEDoneMsg**: The SSEDone handler calls `m.cancelRun()` and then nils it out. This is intentional — the server has already ended the run, but the local context must also be released to avoid goroutine leak in the bridge.
- **`lastAssistantText` is non-empty at failure**: The SSEDone handler records the partial assistant response into the transcript before appending the error. The user's export via `/export` will include the partial response.

---

## STORY-ER-004: SSE Stream Error with Polling Continuation

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer on a flaky network who sees a stream hiccup mid-run
**Goal**: Understand that the TUI recovers automatically from transient stream read errors without losing the run
**Preconditions**: TUI is open, a run is active, the SSE stream encounters a transient read error (e.g., TCP reset, partial chunk)

### Steps

1. The SSE bridge encounters a read/parse error on the event stream → The bridge sends `SSEErrorMsg{Err: <error>}` to the model's message channel.
2. The TUI Update handler receives `SSEErrorMsg` → The error is appended to the viewport as a single line: `⚠ stream error: connection reset by peer`. The run is NOT marked inactive. `sseCh` is still non-nil.
3. Because `sseCh` is non-nil, the handler returns `pollSSECmd(m.sseCh)` → Polling continues immediately. The bridge resumes reading. If the server is still streaming, events pick up where they left off.
4. If the error is transient, subsequent `SSEEventMsg` messages arrive normally → The `⚠ stream error:` line remains visible in the viewport history but the run completes successfully.

### Variations

- **Repeated stream errors**: Each error appends another `⚠ stream error:` line. After 256 dropped messages (the channel backpressure limit in the bridge), the bridge sends a specific `SSEErrorMsg` with the text `"SSE bridge: too many dropped messages, stream may be corrupt"`.
- **Error when `sseCh` is nil**: If `sseCh` is nil (run was already cancelled), `SSEErrorMsg` appends the warning line but returns no `pollSSECmd`. This is an orphaned error and does not restart polling.

### Edge Cases

- **Stream error followed immediately by `SSEDoneMsg`**: Both messages may arrive back-to-back in the channel. The `⚠ stream error:` line appears, then the normal run-completion teardown proceeds. The run ends cleanly.
- **Network completely drops after stream error**: The bridge's `http.Response.Body` read will return repeated errors. Each produces another `SSEErrorMsg`. The user sees multiple warning lines accumulating. Eventually the bridge's context is cancelled (by user interrupt or timeout) and the stream ends.

---

## STORY-ER-005: Tool Call Error Rendering in the Viewport

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer watching an agent run that attempts a bash command which fails with a non-zero exit code
**Goal**: See exactly which tool failed and why, and understand the agent's next decision
**Preconditions**: TUI is open, a run is active, a `tool.call.started` event has been received for a bash tool, and the tool returns an error

### Steps

1. The harness emits `tool.call.completed` with `Error: "bash exited with code 1: command not found: foobar"` and `DurationMS: 340` → The TUI receives `SSEEventMsg{EventType: "tool.call.completed"}` with the error populated.
2. `handleToolError()` is called with the call ID and error text → The `tooluse.Model` for that call ID is updated to error state. The tool block in the viewport is re-rendered via `ReplaceTailLines`: the block shows the tool name in red, the status as `error`, the error text (`bash exited with code 1: command not found: foobar`) in red, and the elapsed duration (`340ms`).
3. The tool block is no longer "active" — the timer stops and the spinning indicator is replaced with a red error icon → An optional hint may appear if the tool definition includes one (not all tools do).
4. The agent continues: the next event is typically `assistant.message.delta` as the agent reasons about the failure → The assistant's response appears in the viewport after the failed tool block.
5. User can press `Ctrl+O` to expand the failed tool block → Full args and any partial output before the error are visible in the expanded view.

### Variations

- **Tool error with hint text**: If the tool definition provides a hint for this error condition (e.g., "Check that the binary is installed"), the hint appears below the error text in the block, styled in muted color.
- **Multiple tools failing in sequence**: Each failed tool produces its own error-state block in the viewport. They accumulate as the run progresses.

### Edge Cases

- **Tool error with empty error string**: `handleToolError()` receives `errText = "tool failed"` (the fallback in the Update handler). The block renders `error: tool failed`.
- **Tool call error but call ID is unknown** (ID mismatch): `toolViews[callID]` is nil/absent. The `handleToolError` path creates a new minimal block for the orphaned ID rather than crashing. The block appears at the tail of the viewport.
- **`Ctrl+O` with no active tool call**: `activeToolCallID` is empty; the key press is a no-op. No toggle occurs.

---

## STORY-ER-006: Export Failure Feedback via Status Bar

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer attempting to export a conversation transcript
**Goal**: Know immediately if the export failed so they can diagnose the cause
**Preconditions**: TUI is open, the user has had a multi-turn conversation, the export directory is not writable (permissions error or disk full)

### Steps

1. User types `/export` and presses Enter (or accepts via autocomplete) → The export command fires `transcriptexport.ExportCmd(...)` as a background `tea.Cmd`.
2. The export goroutine attempts to write the timestamped markdown file to the default export directory → The write fails (e.g., `permission denied`). The goroutine returns `ExportTranscriptMsg{FilePath: ""}` — an empty `FilePath` signals failure.
3. The TUI Update handler receives `ExportTranscriptMsg{FilePath: ""}` → `m.setStatusMsg("Export failed")` is called. The status bar shows `Export failed` in the transient message slot for 3 seconds, then auto-dismisses.
4. No other UI state changes — the viewport is unaffected, the conversation is intact → The user can retry after fixing the permissions or choosing a different path.

### Variations

- **Successful export**: `ExportTranscriptMsg{FilePath: "/Users/alice/transcript-20260323-114704.md"}` → Status bar shows `Transcript saved to /Users/alice/transcript-20260323-114704.md` for 3 seconds.
- **Export with empty transcript**: If no messages have been sent (`transcript` slice is nil or empty), the export command still runs but produces an empty markdown file. The file path is returned; status bar shows the success message. No error.

### Edge Cases

- **Export triggered during an active run**: The transcript slice only contains completed turns. Any in-progress assistant delta is not yet committed to the transcript (it is committed on `SSEDoneMsg`). The export captures the conversation up to but not including the current streaming response.
- **Status bar auto-dismiss races with a new status message**: If the user triggers another status-producing action (e.g., `Ctrl+S` to copy) before the 3-second timer fires, the `setStatusMsg` call sets a new expiry. The earlier `statusTickMsg` arrives and checks `time.Now().After(expiry)` — if the new expiry is in the future, the message is NOT cleared prematurely.

---

## STORY-ER-007: Unknown Slash Command Feedback

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who mistypes a slash command
**Goal**: Get clear feedback that the command was not recognized, and recover quickly
**Preconditions**: TUI is open, no run is active, no overlay is open

### Steps

1. User types `/rune` (intending `/run` but mistyping) and presses Enter → The autocomplete dropdown closes. The `ParseCommand` function extracts command name `rune`. `commandRegistry.Dispatch(Command{Name: "rune"})` returns `CmdResult{Status: CmdUnknown, Hint: "Unknown command: /rune. Type /help to see available commands."}`.
2. The Update handler hits `case CmdUnknown` → `m.setStatusMsg(result.Hint)` is called. The status bar shows the hint message for 3 seconds, then auto-dismisses.
3. The input area is cleared (command was consumed) → The user sees the status bar hint and can type `/help` to browse commands or retype the intended command.

### Variations

- **Partial command that matches nothing in autocomplete**: The dropdown shows no results. User presses Enter anyway with the partial text `/rune`. Same path as above — `CmdUnknown` status, hint in status bar.
- **Command registered but with execution error**: The handler returns `CmdError` with an error message. The status bar shows the error text (not "Unknown command"), giving more specific feedback.

### Edge Cases

- **Slash command typed while run is active**: The run is not affected. The unknown-command hint appears in the status bar while the run continues.
- **`/` alone pressed Enter**: The input is just `/`. `ParseCommand("/")` extracts command name `""`. Dispatch returns `CmdUnknown`. Status bar shows the hint.

---

## STORY-ER-008: Server Unreachable at TUI Launch (Connection Refused)

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who forgets to start `harnessd` before launching the TUI
**Goal**: Understand the connection problem from the TUI, not from a cryptic crash
**Preconditions**: `harnesscli --tui` is run, but `harnessd` is not listening on port 8080 (or the configured base URL)

### Steps

1. TUI launches: `harnesscli --tui` → BubbleTea starts, `Init()` replays any pending API keys (via `setProviderKeyCmd`). The key-set requests fail silently (the server is not running). The full-screen TUI renders with the empty viewport, input area with `❯` prompt, and status bar.
2. The model switcher may attempt `fetchModelsCmd` on first open of `/model` → This fires an HTTP GET to `/v1/models`. The `http.Get` returns an error: `dial tcp [::1]:8080: connect: connection refused`. The cmd returns `ModelsFetchErrorMsg{Err: "dial tcp ... connection refused"}`.
3. The model switcher overlay shows a load error state: `Error loading models: connection refused` is rendered inside the model switcher overlay → The user can close the overlay with Esc and continue.
4. User types a prompt and presses Enter → `startRunCmd` POSTs to `/v1/runs`. The `http.Post` returns an error: `connect: connection refused`. The cmd returns `RunFailedMsg{Error: "Post \"http://localhost:8080/v1/runs\": dial tcp...: connection refused"}`.
5. The Update handler receives `RunFailedMsg` → `runActive` is set to false. The viewport appends `✗ Post "http://localhost:8080/v1/runs": dial tcp ...: connection refused`. A blank line follows.
6. The user sees the error in the viewport → They can start `harnessd` in another terminal and retry immediately. No restart of the TUI is needed.

### Variations

- **Server starts after TUI is launched**: Once `harnessd` is running, the next prompt submission succeeds. The TUI reconnects automatically — there is no session state that prevents reconnection.
- **Wrong port configured**: Same behavior as connection refused. The URL in the error message helps identify the misconfiguration (e.g., `http://localhost:9090`).
- **Server reachable but returns 5xx**: `startRunCmd` receives HTTP 500. Returns `RunFailedMsg{Error: "start run: HTTP 500"}`. Same viewport error path.

### Edge Cases

- **Init key-set requests fail**: If `setProviderKeyCmd` fails due to a connection error at startup, there is currently no dedicated TUI error state for key-set failures — the failure falls through silently. The provider will appear unconfigured in the model switcher until the next successful key submission.
- **Profiles load fails at `/profiles` open**: `loadProfilesCmd` returns `ProfilesLoadedMsg{Err: &err}`. The profile picker receives this and shows an empty list. Status bar shows `Loading profiles...` then no further update (the picker's empty state is the visual indicator).

---

## STORY-ER-009: Recovery and Continued Conversation After an Interrupt

**Type**: medium
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who interrupted a run mid-way and wants to resume with a refined prompt
**Goal**: Confirm that the TUI is fully ready for input after interruption and that conversation context is preserved
**Preconditions**: A run was just cancelled via Ctrl+C or Esc; status bar shows `Interrupted`

### Steps

1. User observes the TUI state immediately after interrupt → `runActive` is false. `cancelRun` is nil. `sseCh` is nil. The thinking bar is gone. The interrupt banner is Hidden. The status bar shows `Interrupted` (fading after 3 seconds). The viewport shows the conversation up to the last complete event before cancellation.
2. User types a new message in the input area → Input area is immediately responsive. There is no lock-out, no loading state, no waiting required.
3. User presses Enter → `startRunCmd` fires with the same `conversationID` as before (the conversation ID is preserved across interrupts — it was set on the first run and is not cleared on cancel). The harness links this new turn to the existing conversation.
4. The new run starts: `RunStartedMsg` arrives → `runActive` is set to true. The SSE bridge reconnects. Streaming resumes normally.
5. The user sees the new response appended to the viewport below the interrupted content → The conversation history is intact and continuous.

### Variations

- **User clears conversation before retrying**: Types `/clear` after interrupt. The viewport and transcript are wiped. `conversationID` is NOT reset (no mechanism exists to reset it via `/clear`). Subsequent runs are still linked to the same conversation on the server side.
- **User changes model before retrying**: Opens `/model`, selects a different model, closes the overlay. The next `startRunCmd` uses the new `selectedModel`. The conversation ID is unchanged.

### Edge Cases

- **Interrupt during plan overlay**: If the plan overlay was open when the interrupt occurred, the plan overlay remains open after the interrupt (it is not auto-dismissed). The user must press Esc or reject the plan before sending the next message.
- **Interrupt with in-progress assistant text**: `lastAssistantText` may contain partial content. It is NOT committed to the transcript on interrupt (only `SSEDoneMsg` commits it). The transcript export will not include the partial response. The viewport shows the partial streamed text, but the transcript is clean.

---

## STORY-ER-010: What Happens to In-Progress Tool Calls at Interrupt Time

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer curious about tool cleanup behavior during cancellation
**Goal**: Understand whether interrupted tool calls appear as running, completed, or error in the viewport
**Preconditions**: A run is active; the agent is mid-way through executing a tool (a `tool.call.started` event has arrived, but `tool.call.completed` has not yet)

### Steps

1. User presses Ctrl+C twice (or Esc once) to cancel the run → `cancelRun()` is called, which cancels the server-side context. `runActive` is false immediately in the TUI.
2. The server-side harness receives the context cancellation → It attempts to stop the current tool. Depending on the tool, this may happen quickly (bash: SIGINT propagated) or may wait for the tool's natural return. Either way, the server closes the SSE stream.
3. `SSEDoneMsg` arrives at the TUI (bridge closes) → Run state is fully cleared. The thinking bar is dismissed. The banner hides.
4. The tool call block in the viewport is still in its last rendered state → If `tool.call.completed` arrived before the stream closed, the block shows completed or error state with a duration. If `tool.call.completed` did NOT arrive (tool was hard-killed), the block remains in "running" state with the spinner stopped — no duration is shown, no completion icon.
5. User sees the tool block frozen in the running state → This is expected behavior. The block does not retroactively update to "error" or "interrupted". The user can expand it with `Ctrl+O` to see any partial output that was streamed before cancellation.

### Variations

- **Server sends `tool.call.completed` with error before SSEDone**: The tool block transitions to error state with the error message and duration. This is the clean path: the tool returned before the context cancellation fully propagated.
- **Multiple tools queued but not yet started**: Tools that the agent had decided to run but that the harness had not yet dispatched simply never start. No `tool.call.started` event arrives for them, so no block appears in the viewport.

### Edge Cases

- **Permission prompt was pending at interrupt**: The permission prompt modal is rendered above the viewport. On interrupt, `runActive` is cleared but the permission prompt component is not explicitly dismissed. The prompt will remain visible until the user presses a key that dismisses it (or until the next `WindowSizeMsg` resets layout). The user should press Esc or a permission key to clear it.

---

## STORY-ER-011: Ctrl+C with No Active Run Quits the TUI

**Type**: short
**Topic**: Error Recovery & Interrupts
**Persona**: Developer finished with a session who wants to exit
**Goal**: Exit the TUI cleanly via Ctrl+C when no work is in progress
**Preconditions**: TUI is open, no run is active (`runActive = false`), no overlay is open

### Steps

1. User presses `Ctrl+C` once → The key handler checks `m.runActive`. It is false. The handler falls through to the standard quit path: `return m, tea.Quit`. BubbleTea terminates the program cleanly.
2. The terminal is returned to the user → The TUI exits with no error message. Normal shell prompt returns.

### Variations

- **Ctrl+C with run active**: Does NOT quit — cancels the run instead (see STORY-ER-001). The TUI remains open.
- **Ctrl+C with overlay open and no run**: The overlay is NOT checked in the Ctrl+C path (only Esc has overlay-priority logic). Ctrl+C quits immediately even with an overlay open. The user should use Esc to close overlays and Ctrl+C to quit when truly done.

### Edge Cases

- **`/quit` command**: Same effect as Ctrl+C with no run — triggers `tea.Quit`. The `Execute` handler for the `quit` command returns `quit = true`, causing the Update handler to return `tea.Quit`.
- **Ctrl+C pressed inside input text that happens to end the slash autocomplete dropdown**: The dropdown is not in the Ctrl+C path. The dropdown closes when Esc is pressed, not Ctrl+C. Ctrl+C will quit even if the dropdown is open (since no run is active).

---

## STORY-ER-012: Cascading Esc Priority Resolution Through All Layers

**Type**: long
**Topic**: Error Recovery & Interrupts
**Persona**: Developer who has accumulated multiple active states simultaneously and wants to navigate back to idle with Esc
**Goal**: Understand and use the full Esc priority chain to cleanly unwind all active state layers
**Preconditions**: User has: (a) typed some text in the input area, (b) opened the `/model` overlay and navigated to level 1 with search active, AND (c) a run is active in the background

### Steps

1. User presses `Esc` (first press) with the model overlay open, search active, and config panel not open → The Esc handler checks the active overlay. It is `"model"`. Config panel is not open (`modelConfigMode = false`). Search query is non-empty. Action: clear the search query. The overlay stays open at level 1, showing the unfiltered model list. No other state changes.
2. User presses `Esc` (second press) — overlay still `"model"`, level 1, no search → The Esc handler: config panel not open, search empty, browse level is 1. Action: `ExitToProviderList()`. The overlay returns to level 0 (provider list). Still open.
3. User presses `Esc` (third press) — overlay `"model"`, level 0, no search → Action: close the model overlay entirely. `overlayActive = false`, `activeOverlay = ""`. An `EscapeMsg{}` is dispatched.
4. User presses `Esc` (fourth press) — no overlay, run is active → The Esc handler: no overlay active, `runActive = true`, `cancelRun != nil`. Action: cancel the run. `cancelRun()` is called. `runActive = false`. Status bar shows `Interrupted`. The run is now stopped.
5. User presses `Esc` (fifth press) — no overlay, no run, input has text → Action: `m.input.Clear()` is called. Status bar shows `Input cleared`.
6. User presses `Esc` (sixth press) — no overlay, no run, empty input → The handler is a no-op. No state change, no status message, no quit.

### Variations

- **API keys overlay with key input mode active**: First Esc exits key input mode (clears typed key text, returns to provider list mode within the overlay). Second Esc closes the overlay entirely.
- **Model config panel with key input active**: First Esc exits key input mode within the config panel. Second Esc exits the config panel (back to level 1 model list). Third Esc goes back to provider list. Fourth Esc closes the overlay.

### Edge Cases

- **Esc when overlay is `"profiles"`**: The profile picker closes immediately (single Esc). The picker does not have a multi-step Esc path.
- **Esc priority skips "provider" overlay check order**: The `"provider"` overlay (gateway routing) is checked before the generic `m.overlayActive` check. A single Esc closes it, same as other simple overlays.
- **No `InterruptedMsg` is emitted**: The Esc-cancel path sets the status message and clears run state locally but does not emit an `InterruptedMsg` tea message. `InterruptedMsg` is a message type used in tests (`tui.InterruptedMsg{At: time.Now()}`) but the live Update path does not dispatch it — it relies on the `setStatusMsg("Interrupted")` side-effect and direct state mutation.
