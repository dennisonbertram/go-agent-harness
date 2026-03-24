# UX Stories: Permission & Safety Controls

**Topic**: Permission & Safety Controls
**Generated**: 2026-03-23

---

## STORY-PS-001: Approving a Single File Write

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer using the TUI for day-to-day coding assistance
**Goal**: Allow the agent to write a single file without granting blanket permissions
**Preconditions**: TUI running with an active run, server configured with `ApprovalPolicyDestructive`. The agent has decided to write `main.go`.

### Steps

1. Agent decides to write a file → Server runner emits a `tool.approval_required` SSE event with `call_id`, `tool: "WriteFile"`, and `arguments` including the file path. Run status transitions to `RunStatusWaitingForApproval`.
2. TUI receives the SSE event → The `permissionprompt.Model` is instantiated via `permissionprompt.New("WriteFile", "main.go", [OptionYes, OptionNo, OptionAllowAll])` and rendered as a rounded-border modal over the chat viewport. All keyboard input is now consumed by the prompt; the input area and slash commands are blocked.
3. Modal displays three options with cursor on `> Yes (allow once)` → User reads "Allow tool: WriteFile" and "Resource: main.go" in the modal header.
4. User presses `Enter` with the cursor on "Yes (allow once)" → `permissionprompt` resolves with `PromptResult{Option: OptionYes}`. The TUI POSTs to `POST /v1/runs/{id}/approve`.
5. Server receives approve request → `ApprovalBroker.Approve(runID)` unblocks the runner. A `tool.approval_granted` SSE event is emitted. Run status returns to `RunStatusRunning`.
6. Agent proceeds to execute `WriteFile` → A `tool.call.started` block appears in the viewport. The modal disappears and normal input is restored.

### Variations

- **Already approved tool in session**: If the user had previously selected "Allow all (this session)" for `WriteFile`, no prompt appears; the tool executes immediately.
- **Profile with `ApprovalPolicyNone`**: No `tool.approval_required` event is ever emitted; the tool runs without interruption.

### Edge Cases

- **Prompt appears at narrow terminal width (< 40 cols)**: The modal adapts by enforcing a minimum `innerWidth` of 10; the option labels and resource path are truncated with `…` rather than overflowing.
- **Enter pressed with no Options (empty slice)**: The prompt falls back to `OptionNo` and resolves immediately; a `deny` POST is issued.

---

## STORY-PS-002: Denying a Bash Command

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Security-conscious developer who wants to vet every shell command
**Goal**: Prevent the agent from running an unexpected `rm -rf` command
**Preconditions**: TUI running with `ApprovalPolicyAll` profile active, giving the agent approval prompts for every tool call including `bash`.

### Steps

1. Agent issues a `bash` tool call with arguments `{"command": "rm -rf /tmp/build"}` → `tool.approval_required` SSE event is emitted with `tool: "bash"` and the full command as the resource string.
2. Permission prompt modal appears with "Allow tool: bash" and "Resource: rm -rf /tmp/build" → User reads the command text.
3. User presses `Down` arrow once → Cursor moves from "Yes (allow once)" to `> No (deny)`.
4. User presses `Enter` → `permissionprompt` resolves with `PromptResult{Option: OptionNo}`. TUI POSTs to `POST /v1/runs/{id}/deny`.
5. Server `ApprovalBroker.Deny(runID)` returns `approved = false` to the runner → Runner emits `tool.approval_denied` SSE event. Runner constructs a denied tool result JSON: `{"error": {"code": "permission_denied", "message": "tool call denied by operator"}}`.
6. Runner emits `tool.call.completed` with the denial payload → The tool call block in the viewport renders in error state showing `permission_denied`. The agent receives the error as its tool result and responds accordingly.
7. TUI restores normal input → Status bar shows no interruption indicator; agent may choose to proceed differently given the denial.

### Variations

- **Pressing `Esc` instead of navigating to "No"**: `Esc` in selection mode also resolves with `OptionNo`, issuing the same deny POST.

### Edge Cases

- **Approval broker returns error (e.g., context cancelled)**: Runner treats this as a denial with `approval_timeout` code instead of `permission_denied`. The tool block still shows an error state but the reason differs.
- **Run cancelled (Ctrl+C twice) while prompt is open**: The interrupt confirmation banner is a separate overlay (`interruptui`). If the run is cancelled, the runner's context is cancelled, which causes the `ApprovalBroker.Ask` to return an error, resolving as a timeout denial.

---

## STORY-PS-003: Granting Session-Wide Permission

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
5. Agent writes the second file `src/bar.go` → No permission prompt appears; the tool block appears directly in the viewport as if in full-auto mode.
6. Agent writes a third file `src/baz.go` → Again no prompt. The refactor completes uninterrupted.

### Variations

- **Starting a new TUI session (harnesscli --tui restarted)**: Session-wide grants are in-memory only and do not persist; the first `WriteFile` in the new session will prompt again.
- **Different tool in same session**: "Allow all (this session)" for `WriteFile` does not suppress prompts for `bash` or `ReadFile` if those are also subject to approval; each tool's session grant is tracked independently.

### Edge Cases

- **Run fails partway through**: If the run fails after "Allow all" was granted, subsequent runs in the same TUI session retain the session-wide grant (it is session-scoped, not run-scoped).

---

## STORY-PS-004: Amending a Resource Path Before Approving

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer who wants to redirect a file write to a safer path
**Goal**: Approve the action but change the target file path before confirming
**Preconditions**: Permission prompt is open showing "Allow tool: WriteFile", "Resource: /etc/hosts".

### Steps

1. Permission prompt appears with `> Yes (allow once)` as the default cursor position → User sees the resource path `/etc/hosts` and is concerned.
2. User presses `Tab` → The prompt enters amend mode. The footer changes from "[Tab to amend path]" to "Amend path: _" (a text cursor). The `amended` buffer starts empty.
3. User types `~/scratch/hosts-copy.txt` character by character → Each keystroke updates `m.amended`; the "Resource:" line in the modal header shows the typed text live as the user types (or `m.amended` in the view if amending and `m.amended != ""`).
4. User presses `Enter` to confirm the amendment → `IsAmending()` returns to `false`. The modal returns to selection mode displaying the amended path. The `amended` field now holds `~/scratch/hosts-copy.txt`.
5. User presses `Enter` again to confirm the option → `permissionprompt` resolves with `PromptResult{Option: OptionYes, Amended: "~/scratch/hosts-copy.txt"}`. TUI POSTs `POST /v1/runs/{id}/approve` with the amended resource.
6. Server approves and the tool runs with the amended path → The tool block shows the corrected path in the viewport.

### Variations

- **User changes their mind mid-amendment and presses `Esc`**: In amend mode, `Esc` cancels the amendment, clears `m.amended`, and returns to selection mode. The original resource path is displayed again.
- **User uses Backspace to correct a typo in amend mode**: Each `Backspace`/`Delete` removes the last rune from `m.amended` (UTF-8 safe rune slicing).

### Edge Cases

- **Terminal too narrow to show the full amended path**: The `truncate()` helper clips the line to `innerWidth - 1` runes and appends `…`.
- **User presses `Tab` again while already in amend mode**: The `Tab` key press is handled by `updateAmending`, which has no case for `KeyTab`, so it falls through without action (no nested amend mode).

---

## STORY-PS-005: Clearing Amend Input with Ctrl+U

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

- **Ctrl+U pressed in selection mode (not amend mode)**: `Ctrl+U` is only handled in `updateAmending`. In selection mode, the key falls through unhandled; no change occurs to the prompt.

---

## STORY-PS-006: Distinguishing Permission Prompt from Interrupt Banner

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer new to the harness TUI, unsure of the different modal surfaces
**Goal**: Understand which overlay is which and how to interact with each correctly
**Preconditions**: TUI is active with a run in progress.

### Steps

1. Developer presses `Ctrl+C` once during an active run → The **interrupt banner** appears: a yellow-bordered box with "⚠  Press Ctrl+C again to stop, or Esc to continue". This is `interruptui.StateConfirm`. The banner appears above the input area, not as a full viewport takeover.
2. Developer presses `Esc` → `interruptui` returns to `StateHidden`. The run continues uninterrupted. The banner disappears.
3. Shortly after, the agent calls a mutating tool → The **permission prompt** appears as a rounded-border modal: "Allow tool: WriteFile", "Resource: output/report.md", with three selectable options and a "[Tab to amend path]" footer. This is a completely different component (`permissionprompt.Model`) with no yellow warning color.
4. Developer notices the visual difference → Interrupt banner: yellow border, warning icon, one-line, dismissible with `Esc`. Permission prompt: neutral rounded border, tool/resource header, numbered options, resolved only by selecting an option or pressing `Esc` (which resolves as deny).
5. Developer selects "Yes (allow once)" and presses `Enter` → Prompt resolves. Normal input restored.

### Variations

- **Ctrl+C pressed twice rapidly when a permission prompt is already open**: The interrupt banner transition (`StateHidden → StateConfirm → StateWaiting`) is independent of the permission prompt; both can potentially be visible simultaneously depending on rendering order. In practice, the run would be cancelled, causing the approval broker's context to be cancelled, which resolves the prompt as a timeout denial.

### Edge Cases

- **Pressing `Ctrl+C` in Confirm state**: `interruptui.Confirm()` transitions to `StateWaiting` and the text changes to "Stopping… (waiting for current tool to finish)".
- **Run completes before second `Ctrl+C`**: The `interruptui` banner returns to `StateHidden` once the `run.completed` SSE event arrives and the run state clears.

---

## STORY-PS-007: Approval Prompt Blocking All Input

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer who tries to type a message while the agent is waiting for approval
**Goal**: Understand that the permission prompt is a hard gate — no other interaction proceeds until it is resolved
**Preconditions**: A `tool.approval_required` event has arrived and the permission prompt modal is active.

### Steps

1. Permission prompt modal is displayed → `permissionprompt.IsActive()` returns `true`. The modal holds the BubbleTea update loop's exclusive attention.
2. User tries to type a new message in the input area → All key events are routed to the `permissionprompt.Update()` handler first. Printable characters in selection mode are not handled by `updateSelecting`, so they fall through silently. The input area receives no characters.
3. User tries `/help` or any slash command → Same routing: the slash character and subsequent characters are consumed by the prompt as unhandled keys (no effect in selection mode). The slash-complete dropdown does not open.
4. User presses `Up` or `Down` → These are handled by `updateSelecting`, moving the cursor between the three options.
5. User presses `Enter` to resolve → The prompt resolves; normal input routing resumes.

### Variations

- **User presses `Esc` while in selection mode**: Resolves with `OptionNo` (deny). Normal input returns immediately.

### Edge Cases

- **Long-running approval with user idle**: The harness runner uses `AskUserTimeout` as a deadline. If the user does not respond before the timeout, `ApprovalBroker.Ask` returns an error, and the runner emits `tool.approval_denied` with `code: "approval_timeout"`. The TUI would need to receive this via SSE and clear the pending prompt state.

---

## STORY-PS-008: Reviewing Session Permissions via /permissions Panel

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer mid-session who wants to audit which tools have been granted or denied
**Goal**: View and manage the accumulated permission rules for the current session
**Preconditions**: Several permission decisions have been made: `WriteFile` allowed permanently (Allow all), `bash` denied once, `ReadFile` allowed once.

### Steps

1. User types `/permissions` and accepts from the slash-complete dropdown → The `permissionspanel.Model` opens, displaying a scrollable list of `PermissionRule` entries.
2. Panel shows three rows → Format: `  [✓/✗] [toolname]  once/permanent`. For example:
   - `  ✓ WriteFile  permanent`
   - `  ✗ bash  once`
   - `  ✓ ReadFile  once`
3. User presses `j` or `Down` → Selection cursor moves down through rows. The selected row renders with reverse video and a `>` prefix.
4. User selects the `bash` deny row and presses `t` (toggle) → `ToggleSelected()` flips `Allowed` from `false` to `true`. The row now shows `✓ bash  once`.
5. User selects the `ReadFile` allow row and presses `d` (delete) → `RemoveSelected()` removes the rule. The `ReadFile` row disappears; the selection index clamps to the new length.
6. User presses `Esc` → Panel closes (`IsOpen = false`). Remaining rules are preserved in memory for display if the panel is reopened.

### Variations

- **Panel opened when no rules exist**: The panel renders "No permission rules active" in dimmed style.
- **Rules list has wrap-around navigation**: `SelectUp()` at the top wraps to the last entry; `SelectDown()` at the bottom wraps to the first.

### Edge Cases

- **Toggling the only rule**: Works normally; the list shows one entry with the flipped allowed state.
- **Deleting the last rule**: `RemoveSelected()` leaves an empty slice; the panel shows "No permission rules active".

---

## STORY-PS-009: Approval Timeout — User Steps Away

**Type**: medium
**Topic**: Permission & Safety Controls
**Persona**: Developer who left the terminal while a permission prompt was waiting
**Goal**: Understand what happens when the approval deadline passes without a response
**Preconditions**: Permission prompt is active for a `WriteFile` call. The harness runner's `AskUserTimeout` is configured (e.g., 2 minutes).

### Steps

1. Permission prompt modal is displayed → TUI shows the modal. User steps away from terminal without responding.
2. Two minutes pass → On the server side, `ApprovalBroker.Ask` times out. It returns an error to the runner.
3. Runner detects `approvalErr != nil` and `ctx.Err() == nil` → Runner sets status back to `RunStatusRunning`. Runner emits `tool.approval_denied` SSE event with `reason: "approval timeout"`.
4. Runner constructs denied tool result with `code: "approval_timeout"` → Emits `tool.call.completed` with the error payload. The agent receives this as its tool result.
5. TUI receives both SSE events → The tool block in the viewport updates to show an error state: "Error: approval_timeout". The permission prompt should be dismissed as the run continues.
6. Agent receives the timeout error as tool output → Agent responds, possibly retrying the operation or abandoning it.

### Variations

- **`AskUserTimeout` is zero / very short**: Prompts fail almost immediately; effectively disables interactive approval even when the policy is set.

### Edge Cases

- **User returns and presses `Enter` on the prompt after the timeout has already resolved**: The `PromptResult` is sent via POST to `/v1/runs/{id}/approve`, but the run has already moved on. The server returns a 404 or error (no pending approval for that run ID). The TUI should handle the error gracefully.

---

## STORY-PS-010: Full-Auto Profile Bypasses All Prompts

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Operator setting up an automated pipeline run
**Goal**: Confirm that a profile with `ApprovalPolicyNone` produces zero permission interruptions
**Preconditions**: A profile named `ci-auto` has been created with `approval: "none"`. The developer selects it via `/profiles` before starting a run.

### Steps

1. Developer types `/profiles` → Profile picker opens showing available profiles. Developer navigates to `ci-auto` and presses `Enter`.
2. Status bar (or next-run indicator) reflects the selected profile → Profile applies to the next run only.
3. Developer submits a prompt that causes the agent to call several mutating tools → Multiple `tool.call.started` blocks appear in the viewport.
4. No `tool.approval_required` SSE events are emitted → The runner checks `needsApproval` and with `ApprovalPolicyNone`, the condition is never true. No approval pause occurs.
5. All tool calls execute and complete → Viewport shows tool blocks transitioning to completed state. No permission modal ever appears.

### Variations

- **Switching back to a `permissions` profile on the next run**: The next prompt submission uses the newly selected profile and resumes prompting for destructive tools.

### Edge Cases

- **Profile `approval` field missing**: Defaults to `ApprovalPolicyNone` (the zero value for the config). Behavior is identical to the full-auto case.

---

## STORY-PS-011: Deny Produces Tool Error Visible in Viewport

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer debugging why an agent task stalled
**Goal**: Trace the visible effect of a denial from the permission prompt through to the tool call block
**Preconditions**: User denied a tool call via the permission prompt earlier in the session. The tool call block is visible in the viewport.

### Steps

1. User denied `bash` via the permission prompt → `POST /v1/runs/{id}/deny` was sent. Server emitted `tool.approval_denied` then `tool.call.completed` with `{"error": {"code": "permission_denied", "message": "tool call denied by operator"}}`.
2. TUI processed the `tool.call.completed` SSE event → The `tooluse.Model` for that call ID transitioned to error state.
3. User scrolls up in the viewport → The `bash` tool block is visible. It shows the tool name, elapsed duration as `0 ms`, and renders the error text in red: "Error: permission_denied — tool call denied by operator".
4. User can see this in context → Other tool calls before and after show green completed state; only the denied one is red.
5. Agent's next message is also visible → The assistant, having received the error as its tool result, responded with an explanation or alternative approach.

### Variations

- **Timeout denial (approval_timeout)**: The error code differs (`approval_timeout`) but the visual treatment in the tool block is identical — red error state with the JSON error content.

### Edge Cases

- **User expands the tool block with `Ctrl+O`**: The full error JSON is shown in the expanded view, confirming the exact `code` and `message` fields.

---

## STORY-PS-012: Permission Prompt with Unknown/Generic Tool

**Type**: short
**Topic**: Permission & Safety Controls
**Persona**: Developer using a custom or third-party tool registered with the harness
**Goal**: Confirm that the permission prompt renders correctly even for tool names the TUI has no specific knowledge of
**Preconditions**: A custom tool named `PushToRegistry` is registered. The server is configured with `ApprovalPolicyAll`.

### Steps

1. Agent calls `PushToRegistry` with arguments referencing a container image → `tool.approval_required` emitted with `tool: "PushToRegistry"`.
2. Permission prompt modal appears → Header shows "Allow tool: PushToRegistry" and "Resource: registry.internal/myapp:latest". All three options are present.
3. User verifies the display is correct → `TestTUI033_UnknownToolFallsBackToAsk` covers this: the tool name renders verbatim; the fallback is not triggered because the `Options` slice is non-nil; the full three-option list is shown.
4. User selects "No (deny)" and presses `Enter` → Prompt resolves; deny POST sent; tool fails with `permission_denied`.

### Variations

- **Tool name is very long (> innerWidth runes)**: The header line "Allow tool: VeryLongToolNameThatExceedsWidth" is truncated with `…` by the `truncate()` helper.
- **Resource path is a URL**: The resource field renders as-is; no URL-specific formatting is applied.

### Edge Cases

- **Tool name is empty string**: The header reads "Allow tool: " with no name. The prompt still functions correctly; the user can deny or allow.
- **Options slice is nil (not populated by caller)**: Prompt shows "(no options available — press Esc to dismiss)". The fallback Enter resolves as `OptionNo`.
