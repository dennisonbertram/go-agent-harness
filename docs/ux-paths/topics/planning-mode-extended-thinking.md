# UX Stories: Planning Mode (Extended Thinking)

Generated: 2026-03-23

---

## STORY-PM-001: Toggling Plan Mode Before a Run Starts

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who wants the agent to show its plan before executing
**Goal**: Enable plan mode so the next run pauses for approval before acting
**Preconditions**: TUI is open, no active run, plan overlay is in `PlanStateHidden`

### Steps
1. User presses `ctrl+o` with no active tool call and no run in progress → The key matches `m.keys.PlanMode` binding (same physical key `ctrl+o`); because `activeToolCallID` is empty, the ExpandTool branch is a no-op; plan mode toggle state is noted for the next submitted run.
2. User types a prompt and presses Enter → Run starts via POST `/v1/runs`; the server is aware of plan mode intent (passed as a run parameter); SSE stream begins.
3. Server produces a plan before calling any tools → A plan SSE event arrives; `planoverlay.Model.Show(planText)` is called; overlay transitions from `PlanStateHidden` to `PlanStatePending`.
4. Full-screen plan overlay appears → Header shows "📋 Plan Mode" with a yellow "[Awaiting Approval]" badge; plan markdown is rendered in the scroll area; footer hint reads "y approve  n reject  ↑/↓ scroll".
5. User reads the plan and presses `y` → `PlanApprovedMsg{}` is emitted; overlay transitions to `PlanStateApproved`; green "[Approved ✓]" badge replaces the yellow one; footer hint disappears; agent continues execution.

### Variations
- **Run submitted without plan mode active**: No plan overlay appears; agent proceeds directly to tool execution.
- **ctrl+o with no toggle support yet wired**: Falls through to the ExpandTool branch; if no active tool call exists, it is a no-op (no visible change).

### Edge Cases
- **`ctrl+o` pressed while overlay is open**: The overlay already owns input focus; the keypress is not routed to the ExpandTool or PlanMode binding — it is ignored or scrolls the overlay depending on overlay key routing.
- **Plan text is empty string**: `planoverlay.View()` renders the placeholder "(no plan text)" inside the box instead of blank content; the overlay is still shown and awaits approval.

---

## STORY-PM-002: Reviewing and Approving a Multi-Step Plan

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Careful developer reviewing an agent's proposed approach to a complex refactor
**Goal**: Read the full plan before the agent makes any file changes
**Preconditions**: Plan overlay is in `PlanStatePending` with a long multi-step plan (more than the visible height allows)

### Steps
1. Plan overlay appears in `PlanStatePending` state → Rounded-border box fills the terminal; header shows "📋 Plan Mode" and "[Awaiting Approval]" badge; visible lines of plan markdown are shown; footer shows "  ... N more line(s)" because content exceeds the visible area.
2. User presses `↓` (down arrow) → `planoverlay.Model.ScrollDown(maxLines)` is called; offset increments by one; next lines of the plan scroll into view; "more line(s)" counter decrements.
3. User continues pressing `↓` until all content is visible → When `end >= totalLines`, the "more line(s)" footer disappears; scroll offset is clamped at `maxLines - Height`.
4. User presses `↑` (up arrow) to re-read the first step → `planoverlay.Model.ScrollUp()` is called; offset decrements; first lines scroll back into view; offset is clamped at 0 and cannot go negative.
5. User presses `y` to approve → `planoverlay.Model.Approve()` transitions state to `PlanStateApproved`; badge turns green ("[Approved ✓]"); key-hint footer disappears; `PlanApprovedMsg{}` is emitted to the BubbleTea runtime; the run continues.

### Variations
- **Plan fits entirely on screen**: No "more line(s)" footer appears; scroll keys are still accepted but have no visible effect (offset clamped at 0 in both directions).
- **User reads plan then waits**: The overlay stays in `PlanStatePending` indefinitely; no timeout; the run is blocked until the user acts; the status bar shows the model and cost but does not flash any timeout warning.

### Edge Cases
- **Scrolling past the bottom**: `ScrollDown` clamps at `maxLines - Height`; repeated presses do not advance the offset further; no panic or wrap-around.
- **Terminal is resized while overlay is open**: `tea.WindowSizeMsg` updates `m.width` and `m.height`; the overlay's `Width` and `Height` fields are updated; the view recalculates `innerWidth`, `visibleLines`, and `overhead` — content reflows to the new dimensions without resetting the scroll offset.

---

## STORY-PM-003: Rejecting a Plan and Observing the Outcome

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who disagrees with the agent's proposed approach
**Goal**: Stop the agent from executing a plan that would take the wrong approach
**Preconditions**: Plan overlay is in `PlanStatePending` with a plan the user wants to reject

### Steps
1. Plan overlay is visible with "[Awaiting Approval]" badge → User reads the plan and determines the approach is wrong.
2. User presses `n` → `planoverlay.Model.Reject()` transitions state to `PlanStateRejected`; badge turns red ("[Rejected ✗]"); key-hint footer disappears; `PlanRejectedMsg{}` is emitted.
3. TUI model receives `PlanRejectedMsg` → The overlay is hidden (`.Hide()` called); the run is signaled to stop or the server receives a denial via POST `/v1/runs/{id}/deny`; the viewport shows a message indicating the plan was rejected.
4. User is returned to the input area → The input area is refocused; the user can type a follow-up message explaining what approach they want instead.

### Variations
- **User rejects and immediately sends a correction**: After rejection, the user types "Instead of X, please do Y" and submits; a new run starts with the corrected guidance.
- **User presses Esc instead of `n`**: Escape's multi-priority logic does not close the plan overlay directly — the plan overlay is a modal interrupt with highest priority; behavior depends on whether Escape is routed to the plan overlay's key handler or falls through to the standard Esc chain.

### Edge Cases
- **Reject called when state is not Pending**: `planoverlay.Model.Reject()` is a no-op if state is already `PlanStateApproved`, `PlanStateRejected`, or `PlanStateHidden`; the state does not change.
- **Plan rejected but SSE stream continues sending events**: The TUI should cancel the run; stale SSE tool events arriving after rejection should be dropped or ignored since the run is no longer active.

---

## STORY-PM-004: Distinguishing the Thinking Bar from the Plan Overlay

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer new to the TUI who is confused by two different "thinking" indicators
**Goal**: Understand which part of the UI indicates reasoning in progress versus a plan awaiting approval
**Preconditions**: TUI is open; user has submitted a prompt; model supports extended thinking

### Steps
1. User submits a prompt → Run starts; SSE stream begins; the `thinkingbar.Model` activates.
2. `assistant.thinking.delta` SSE events arrive → Each delta is appended via `appendThinkingDelta(delta)`; `normalizeThinkingLabel` collapses whitespace and prepends "Thinking: "; `thinkingbar.Model{Active: true, Label: "Thinking: <text>"}` is set; the thinking bar renders above the input area as a single line like "Thinking: analyzing the codebase structure...".
3. More `assistant.thinking.delta` events stream in → The label accumulates all delta text; the bar updates in place above the input; it does NOT fill the screen and does NOT show approve/reject controls.
4. Reasoning completes; a plan SSE event arrives → `clearThinkingBar()` is called first; the thinking bar disappears from above the input; `planoverlay.Model.Show(planText)` is called; the full-screen plan overlay replaces the main view.
5. User now sees a visually distinct full-screen overlay with a rounded border, header "📋 Plan Mode", state badge "[Awaiting Approval]", and footer hint "y approve  n reject  ↑/↓ scroll" → This is clearly different from the transient single-line thinking bar.

### Variations
- **Model does not emit thinking deltas**: The thinking bar never activates (`Active` stays false; `View()` returns ""); only the plan overlay appears if plan mode is active.
- **Thinking bar shows custom label**: If the server emits reasoning text with a specific label, `normalizeThinkingLabel` formats it as "Thinking: <normalized text>"; the default fallback is "Thinking..." when the accumulated text is empty.

### Edge Cases
- **Thinking delta arrives with empty content string**: The `content != ""` guard in the SSE handler prevents `appendThinkingDelta` from being called; the thinking bar label does not update; no blank "Thinking: " label appears.
- **clearThinkingBar is called before any delta arrives**: `thinkingBar` is reset to `thinkingbar.New()` (inactive); `thinkingText` is set to ""; this is safe and idempotent — no visible change since the bar was already inactive.

---

## STORY-PM-005: Plan Overlay Blocks Input While Pending

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who tries to type a new message while a plan is awaiting approval
**Goal**: Verify that the plan overlay correctly locks out other input until resolved
**Preconditions**: Plan overlay is in `PlanStatePending`; user did not notice the overlay and tries to continue typing

### Steps
1. Plan overlay is in `PlanStatePending` → Full-screen overlay is rendered; input area is obscured or rendered behind the overlay.
2. User types characters → Characters are NOT entered into the input area; the plan overlay is a modal interrupt at the highest priority in the navigation hierarchy (higher than regular overlays); keystrokes are routed to the plan overlay's own key handler.
3. User presses a letter key that is not `y` or `n` → The plan overlay ignores unrecognized keys; no state change occurs; the "[Awaiting Approval]" badge remains.
4. User presses `y` or `n` → Approval or rejection is processed as in STORY-PM-002 or STORY-PM-003; control returns to the main chat view; the input area is refocused.

### Variations
- **User tries to open `/help` while plan is pending**: Slash-command autocomplete is not active since the input area does not receive keystrokes while the plan overlay is blocking; the `/help` command does not execute.
- **User presses `ctrl+c` while plan is pending**: `ctrl+c` in the KeyMap is matched by the Quit binding; if `runActive` is true, the run is cancelled; `cancelRun()` is called; the plan overlay's `PlanStatePending` state becomes moot; the overlay is hidden; a status message "Interrupted" appears (see STORY-PM-007).

### Edge Cases
- **Two overlays stacking**: The plan overlay and another overlay (e.g., help) cannot both be open simultaneously; the overlay system uses `activeOverlay` as a single string; the plan overlay is shown separately as a modal interrupt that supersedes the overlay system.
- **Scroll keys while pending**: `↑` and `↓` are handled by the plan overlay's own scroll logic (`ScrollUp`, `ScrollDown`), not by the viewport; the conversation history does not scroll while the plan overlay is pending.

---

## STORY-PM-006: Expanding an Active Tool Call with ctrl+o

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who wants to see the full output of a running bash tool call
**Goal**: Expand the truncated tool call output to see all lines
**Preconditions**: A tool call is active (`activeToolCallID` is set); tool output exceeds `MaxLines` and is showing the "+N more lines (ctrl+o to expand)" truncation hint

### Steps
1. A tool call block appears in the viewport with truncated output → The block shows a spinner, elapsed time, the command, and then a truncation hint like "+12 more lines (ctrl+o to expand)".
2. User presses `ctrl+o` → Because `activeToolCallID != ""`, the `ExpandTool` branch in the `Update` function fires; `m.toolExpanded[activeToolCallID]` is toggled from false to true; `rerenderActiveToolView()` is called.
3. `rerenderActiveToolView` calls `appendToolUseView` with `Expanded = true` → The tool block is re-rendered at the tail of the viewport via `ReplaceTailLines`; the full output replaces the truncated view; no "+N more lines" hint is shown.
4. User presses `ctrl+o` again → `m.toolExpanded[activeToolCallID]` toggles back to false; `rerenderActiveToolView()` re-renders the collapsed view with the truncation hint restored.

### Variations
- **Tool call has already completed when ctrl+o is pressed**: `activeToolCallID` still points to the last completed call; the expand/collapse toggle still works; `rerenderActiveToolView` re-renders the completed (not running) block in expanded state.
- **No active tool call when ctrl+o is pressed**: `activeToolCallID == ""`; the ExpandTool case is a no-op; nothing changes in the viewport.

### Edge Cases
- **ctrl+o pressed during plan pending state**: The plan overlay holds modal focus; the ExpandTool branch is not reachable while the plan overlay is the active modal; `ctrl+o` is routed to the plan overlay's handler (which does not map that key to anything) and is ignored.
- **ctrl+o and PlanMode share the same key binding**: Both `m.keys.PlanMode` and `m.keys.ExpandTool` are bound to `ctrl+o`; the `switch` in `Update` matches `ExpandTool` first when `activeToolCallID != ""`; the dual binding serves two distinct contexts based on whether a tool call is active.

---

## STORY-PM-007: Cancelling a Run While Plan Is Pending

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer who realizes they want to abort entirely after seeing a plan they cannot fix with just a rejection
**Goal**: Cancel the run completely, not just reject the plan
**Preconditions**: Plan overlay is in `PlanStatePending`; user has decided to abandon the entire run

### Steps
1. Plan overlay is showing "[Awaiting Approval]" → User decides the entire task framing is wrong, not just the plan.
2. User presses `ctrl+c` → The `Quit` key binding matches; because `m.runActive == true` and `m.cancelRun != nil`, the cancellation path fires: `m.cancelRun()` is called, `m.runActive` is set to false, `m.cancelRun` is cleared, and `setStatusMsg("Interrupted")` schedules a status bar flash.
3. Plan overlay state becomes stale → The overlay is still in `PlanStatePending` on the model struct; the TUI must clear it — `planoverlay.Model.Hide()` is called (transitions to `PlanStateHidden`); `IsVisible()` returns false; the overlay stops rendering.
4. SSE stream is closed by `cancelRun` → No further SSE events arrive; the thinking bar (if active) is cleared via `clearThinkingBar()`; the status bar flashes "Interrupted" for 3 seconds.
5. User is returned to the input area → The full-screen overlay is gone; the conversation viewport is visible; the user can start a new run with a corrected prompt.

### Variations
- **User presses Esc instead of ctrl+c**: Escape's multi-priority chain checks `runActive` after overlay checks; if the plan overlay is treated as an overlay in `m.overlayActive`, Escape might close the overlay first before cancelling the run; the exact behavior depends on whether the plan overlay sets `m.overlayActive = true`; in the current implementation, the plan overlay is separate from the `activeOverlay` string and may not be dismissed by Escape.
- **cancelRun is nil when ctrl+c is pressed**: The nil guard `m.cancelRun != nil` prevents a panic; the run is marked inactive but no actual cancellation is sent; the SSE stream may continue delivering events briefly before the connection drops.

### Edge Cases
- **ctrl+c pressed twice (two-stage interrupt)**: The first Ctrl+C cancels the run and clears `m.cancelRun`; a second Ctrl+C finds `m.runActive == false`, falls through to `tea.Quit`, and exits the TUI entirely.
- **Plan overlay Hide is not called after cancellation**: If the TUI only sets `m.runActive = false` but forgets to hide the plan overlay, the overlay would remain rendered on screen in `PlanStatePending` state even though the run is dead; the key handler would still route `y`/`n` to it, emitting `PlanApprovedMsg`/`PlanRejectedMsg` for a run that no longer exists.

---

## STORY-PM-008: Plan Overlay State Transitions — Full Lifecycle

**Type**: long
**Topic**: Planning Mode (Extended Thinking)
**Persona**: QA engineer validating the complete state machine of the plan overlay
**Goal**: Exercise all four `PlanState` values and all legal transitions in a single session
**Preconditions**: TUI is open; plan overlay starts in `PlanStateHidden`

### Steps

**Phase 1 — Hidden to Pending:**
1. Run starts; server emits a plan event → `planoverlay.Model.Show(planText)` is called; state transitions `PlanStateHidden → PlanStatePending`; scroll offset is reset to 0; `IsVisible()` returns true; overlay is rendered with "[Awaiting Approval]" badge.

**Phase 2 — Pending to Approved:**
2. User presses `y` → `planoverlay.Model.Approve()` transitions `PlanStatePending → PlanStateApproved`; badge changes from yellow "[Awaiting Approval]" to green "[Approved ✓]"; key-hint footer disappears; `PlanApprovedMsg{}` is emitted; agent execution continues; overlay is eventually hidden via `.Hide()` → `PlanStateHidden`.

**Phase 3 — Pending to Rejected:**
3. On the next run, a new plan arrives → `planoverlay.Model.Show(newPlanText)` is called again; scroll offset is reset; state goes to `PlanStatePending` regardless of previous state.
4. User presses `n` → `planoverlay.Model.Reject()` transitions `PlanStatePending → PlanStateRejected`; badge changes to red "[Rejected ✗]"; key-hint footer disappears; `PlanRejectedMsg{}` is emitted; overlay is hidden via `.Hide()`.

**Phase 4 — Idempotent no-op calls:**
5. `Approve()` called on a `PlanStateApproved` model → No-op; state remains `PlanStateApproved`; value semantics guarantee the original model is not mutated; a new copy is returned with the same state.
6. `Reject()` called on a `PlanStateRejected` model → No-op; same guarantee.
7. `Hide()` called on any state → Always transitions to `PlanStateHidden`; `IsVisible()` returns false; `View()` returns "".

### Variations
- **Show called on an already-Pending model**: The scroll offset is reset and plan text is replaced; the state stays `PlanStatePending`; the new plan replaces the old one atomically via value semantics.
- **Show called on an Approved model**: Transitions back to `PlanStatePending`; the user must re-approve the new plan; useful if the server wants to present a revised plan.

### Edge Cases
- **Value semantics guarantee**: All method calls (`Show`, `Approve`, `Reject`, `Hide`, `ScrollUp`, `ScrollDown`) return a new `planoverlay.Model` copy; the original is unchanged; this makes it safe to pass the model across goroutines without a mutex, consistent with the project-wide BubbleTea value semantics rule.
- **Concurrent copies in test goroutines**: `TestTUI055_ConcurrentModels` demonstrates 10 goroutines each holding their own `Model` and running through the full lifecycle without data races; this confirms the immutable-copy design is correctly implemented.

---

## STORY-PM-009: Thinking Bar During Extended Reasoning Without Plan Mode

**Type**: medium
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer using a reasoning-capable model (e.g., o3 or Claude with extended thinking) who did NOT opt into plan mode
**Goal**: Observe streaming reasoning text without the approval gate
**Preconditions**: TUI is open; a reasoning-capable model is selected; plan mode is not activated; run is in progress

### Steps
1. User submits a complex coding question → Run starts; the model enters its reasoning phase before generating a response.
2. `assistant.thinking.delta` SSE events begin arriving → Each event has `content` field with a reasoning text fragment; `appendThinkingDelta(delta)` is called for each; `m.thinkingText` accumulates all fragments.
3. `normalizeThinkingLabel` processes the accumulated text → Collapses all whitespace (replaces runs of spaces/tabs/newlines with a single space); prepends "Thinking: "; produces a single-line label like "Thinking: I need to analyze the function signatures and determine the call graph before making changes".
4. `thinkingbar.Model{Active: true, Label: label}` is set → The thinking bar renders above the input area as one line: "Thinking: I need to analyze..."; it is NOT full-screen and does NOT show approve/reject controls.
5. Reasoning completes; `assistant.content.delta` arrives → `clearThinkingBar()` is called; `m.thinkingText` is reset to ""; `m.thinkingBar = thinkingbar.New()` (inactive); the "Thinking: ..." line disappears; the assistant's response begins streaming into the viewport.

### Variations
- **No thinking deltas are emitted by the model**: The thinking bar remains inactive for the entire run; `View()` returns ""; the input area layout is unaffected.
- **Reasoning text is very long**: `normalizeThinkingLabel` still produces a single line by collapsing all whitespace; the line may be truncated by the terminal width; no overflow or wrapping occurs in the bar itself.
- **Tool call starts during reasoning**: `handleToolStart` calls `clearThinkingBar()` before setting up the tool call block; the thinking bar disappears when the first tool fires.

### Edge Cases
- **`assistant.thinking.delta` with content = ""**: The `p.Content != ""` guard prevents `appendThinkingDelta` from being called; the thinking bar label does not update; no spurious "Thinking: " label (empty prefix) appears.
- **Thinking bar and plan overlay coexist**: In practice these are mutually exclusive in timing — the thinking bar clears when a plan event arrives (clearing happens in `clearThinkingBar` before `planoverlay.Show`); if both were somehow active simultaneously, the plan overlay's full-screen render would cover the thinking bar.

---

## STORY-PM-010: Narrow Terminal — Plan Overlay Layout at 80x24

**Type**: short
**Topic**: Planning Mode (Extended Thinking)
**Persona**: Developer running the TUI in a constrained terminal window
**Goal**: Verify the plan overlay renders correctly at minimum practical terminal width
**Preconditions**: Terminal is 80 columns wide and 24 rows tall; plan overlay is in `PlanStatePending`

### Steps
1. Plan overlay is shown at 80x24 → `innerWidth = 80 - 6 = 74` (border 2 + padding 4); `contentHeight = 24 - 2 - 2 = 20` (border top/bottom and two footer rows for pending state); `visibleLines = 20 - 2 = 18` (header row + separator row).
2. The header line renders "📋 Plan Mode" left-aligned, "[Awaiting Approval]" right-aligned within the 74-character inner width → The badge text may wrap if the gap calculation produces a negative gap; a minimum gap of 1 character is enforced.
3. The separator renders as 74 dash characters → Visible as a full-width horizontal rule.
4. Plan content lines fill up to 18 visible rows → Lines longer than 74 characters are not soft-wrapped by the overlay (they are displayed as-is); they may be truncated by the terminal's own line clipping.
5. The footer hint "  y approve  n reject  ↑/↓ scroll" appears at the bottom → User can see all available actions without scrolling.
6. The snapshot `TUI-055-plan-80x24.txt` captures this exact layout → The snapshot shows the rounded border box filling the terminal width, header with "[Awaiting Approval]" badge, separator, 8-step plan content, empty padding rows, and the approve/reject hint at the bottom.

### Variations
- **120x40 terminal**: `innerWidth = 114`; more plan lines are visible at once; the "more line(s)" footer appears only for very long plans; the approved/rejected badge fits on one line without wrapping.
- **Very small terminal (5x5)**: `innerWidth` is clamped to minimum 10; `visibleLines` is clamped to minimum 1; `contentHeight` is clamped to minimum 1; `TestTUI055_ViewBoundaryDimensions` verifies no panic occurs even at degenerate sizes.

### Edge Cases
- **Plan text contains ANSI escape sequences**: The overlay renders raw plan text; if the server sends pre-colored markdown, ANSI codes pass through; lipgloss's `Width()` function may miscalculate rendered widths; for clean layout, plan text should be plain markdown.
- **Width or Height set to 0 or negative**: The `View()` function applies floor defaults (`width = 80` if `Width <= 0`, `height = 20` if `Height <= 0`) before calculating layout; the overlay renders at sensible defaults rather than crashing.
