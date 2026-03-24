# UX Stories: Tool Execution Flow

**Topic**: Tool Execution Flow
**Generated**: 2026-03-23

---

## STORY-TF-001: Watching a Tool Call Appear and Resolve

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

## STORY-TF-002: Streaming Output Replaces the Tail Line

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

## STORY-TF-003: Expanding a Running Tool Call with ctrl+o

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

## STORY-TF-004: Tool Call Error State Rendered in Red

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

## STORY-TF-005: Observing the Elapsed Timer During a Slow Tool Call

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

## STORY-TF-006: A Diff-Producing Tool Renders with Syntax Highlighting

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

- **Diff detected by `\ndiff --git` mid-string**: `looksLikeUnifiedDiff()` also matches when the diff prefix appears after a leading newline, e.g. multi-tool output that starts with a header line before the diff.
- **Diff rendering returns empty string**: Falls through to `ExpandedView` which renders the raw result text without diff highlighting.

### Edge Cases

- **Params present with diff**: Key-value param lines are rendered between the header and the diff block via `renderTreeLine`.
- **Very large diff**: `diffview` has its own `MaxLines` cap (defaulting to `defaultMaxLines`) to prevent the viewport from becoming unusable.

---

## STORY-TF-007: Sequential Tool Calls Accumulate in the Viewport

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

## STORY-TF-008: Nested Tool Calls from a Subagent

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
   The child is indented using the `depth1Prefix` (`"  ⎿  "`); the inner width available to the child is reduced by the prefix rune count.
3. A second nested `tool.call.started` (call ID: `call-grandchild`, parent ID: `call-child`) arrives → Grandchild attaches under the child; depth 2 prefix is `"  ⎿    "` (extra 2 spaces added per depth level):
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

## STORY-TF-009: File Operation Tool Renders a Summary Line

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

## STORY-TF-010: Interrupt During an Active Tool Call

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

## STORY-TF-011: Reviewing Completed Tool Calls by Scrolling

**Type**: long
**Topic**: Tool Execution Flow
**Persona**: Developer reviewing what actions the agent took during a completed run
**Goal**: Scroll back through the conversation viewport to inspect past tool call blocks and expand specific ones for full output
**Preconditions**: A run has completed; the viewport contains 8 tool call blocks (a mix of completed and error states); the user is at the bottom of the viewport

### Steps

1. Run ends; assistant final text appears below the tool blocks → User is at the bottom of the viewport looking at the assistant's summary.
2. User presses `pgup` → Viewport scrolls up by half its height; older tool call blocks come into view. Each completed block shows its dim dot, args, and duration; errored blocks show the red `✗` suffix.
3. User reads: `⏺ BashExec(go test ./internal/...) (12.4s)` — notes it took 12 seconds.
4. User presses `ctrl+o` → The most recently active tool call's `expanded` flag toggles; if the block under consideration is no longer "active" in the model's `toolViews` map, the key binding may not toggle it (behavior depends on implementation of which call is "active").
5. User presses `pgdn` → Scrolls back down; the `⏺ WriteFile(api.go) (0.1s)` block is visible with `⎿  Wrote 120 lines to api.go`.
6. User presses `ctrl+s` → The last assistant response is copied to the clipboard. Status bar shows `"Copied"` for 3 seconds.
7. User types `/export` → Transcript export runs in background; status bar shows `"Exported: transcript-20260323-145832.md"` for 3 seconds.

### Variations

- **Error block with hint**: `⏺ GrepSearch(badpattern) ✗` followed by `⎿  Error: invalid regex: ...` and `⎿  Hint: Check your regex syntax` — full error detail visible without expanding.
- **Diff block in history**: User scrolls to an `edit_file` block; presses `ctrl+o`; sees the syntax-highlighted unified diff with duration and timestamp in the footer.

### Edge Cases

- **Viewport at top**: `pgup` is a no-op; `up`/`ctrl+p` scroll one line at a time and also no-op at the top boundary.
- **Only one tool call**: Viewport height is sufficient to show everything without scrolling; `pgup`/`pgdn` do nothing useful.

---

## STORY-TF-012: Permission Prompt Blocking Tool Execution

**Type**: long
**Topic**: Tool Execution Flow
**Persona**: Developer who configured the agent to require explicit approval before writing files
**Goal**: Approve a file-write tool call, observe the block transition from pending to completed
**Preconditions**: The agent is mid-run; a `write_file` call has been proposed; the server is waiting for approval via `POST /v1/runs/{id}/approve` or `/deny`

### Steps

1. SSE stream delivers a permission-required event for `write_file` targeting `src/main.go` → The `permissionprompt.Model` modal appears over the viewport; the tool block `⏺ WriteFile(src/main.go)…` is visible behind the modal but frozen (no chunks arriving).
2. Modal displays three options:
   - `[ Yes (allow once) ]`
   - `[ No (deny) ]`
   - `[ Allow all (this session) ]`
3. User tabs to `[ Yes (allow once) ]` and presses `Enter` → TUI sends `POST /v1/runs/{id}/approve`; the permission modal closes.
4. The server proceeds with the `write_file` call; `tool.call.output.chunk` events resume → The block updates with live output.
5. `tool.call.done` arrives → Block dims: `⏺ WriteFile(src/main.go) (0.3s)` with `⎿  Wrote 47 lines to main.go`.
6. Run continues; next tool call fires.

### Variations

- **Tab-amend flow**: Before pressing `Enter`, user presses `Tab` to enter amend mode; they edit the resource path from `src/main.go` to `src/main_backup.go`; then confirm — the amended path is sent in the approve request.
- **Deny**: User selects `[ No (deny) ]`; `POST /v1/runs/{id}/deny` is sent; the tool block transitions to error state with an appropriate message.
- **Allow all**: `[ Allow all (this session) ]` suppresses future permission prompts for this session; subsequent tool calls of the same type proceed without modal interruption.

### Edge Cases

- **Run cancelled while modal is open**: `ctrl+c` cancels the run; the modal closes; the tool block stays frozen in the running state (no done event arrives); the interrupt banner sequence plays out.
- **Simultaneous tool calls requiring approval**: The current implementation shows one permission prompt at a time; subsequent pending approvals queue until the first is resolved.
