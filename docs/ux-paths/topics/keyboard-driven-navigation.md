# UX Stories: Keyboard-Driven Navigation

**Topic**: Keyboard-Driven Navigation
**Generated**: 2026-03-23

These stories cover all keyboard shortcut interactions in the TUI — from basic scrolling and copy
to overlay navigation, escape priority chains, and vim-style movement inside modal panels.

---

## STORY-KN-001: Paging Through a Long Conversation

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who has been in a multi-turn conversation for 30+ exchanges and needs to
revisit something the assistant said earlier
**Goal**: Navigate backward through conversation history and return to the bottom without touching
the mouse
**Preconditions**: TUI is open; the viewport contains many turns of assistant and user messages
that exceed the visible height; no overlay is active; input area is focused

### Steps

1. User presses `pgup` → The viewport scrolls up by half the current viewport height. Earlier
   conversation turns come into view. The input area retains focus (no mode change).
2. User presses `pgup` again → Viewport scrolls up another half-page. Older assistant responses
   are now visible.
3. User presses `pgdn` → Viewport scrolls back down by half-page toward the most recent content.
4. User presses `pgdn` again → Viewport returns to the bottom. The latest assistant message and
   input area are fully visible.

### Variations

- **At the top boundary**: If the viewport is already at the top, `pgup` is a no-op — no visual
  artifact or error.
- **At the bottom boundary**: If the viewport is already at the bottom, `pgdn` is a no-op.
- **Very short conversation**: If all content fits in the viewport, `pgup`/`pgdn` produce no
  visible scroll movement.

### Edge Cases

- **Active run while scrolling**: If a run is in progress and the agent is streaming content,
  `pgup` still scrolls but new lines are being appended at the tail. The viewport does not
  auto-scroll back to the bottom on new content while the user has manually scrolled up.
- **Resize during page navigation**: If the terminal is resized mid-scroll, the half-page
  calculation updates to the new viewport height on the next keypress.

---

## STORY-KN-002: Line-by-Line Scroll for Precision Reading

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer carefully reading a long code block or diff in the conversation and wanting
fine-grained scroll control
**Goal**: Scroll one line at a time through a section of the viewport
**Preconditions**: TUI is open; a tool call block containing a long file diff is visible in the
viewport; no overlay is active

### Steps

1. User presses `up` (or `ctrl+p`) → Viewport scrolls up exactly one line. The code block shifts
   down by one rendered line.
2. User presses `up` several more times → Viewport continues scrolling up one line per keypress,
   allowing the user to read at their own pace.
3. User presses `down` (or `ctrl+n`) → Viewport scrolls down one line back toward the content
   they passed.
4. User continues with `down` presses → Returns incrementally to the current tail.

### Variations

- **Ctrl+P / Ctrl+N alternative**: `ctrl+p` and `ctrl+n` produce identical scroll behavior to
  `up` and `down` when the input area is not in a mode that captures them. Users familiar with
  Emacs-style navigation can use these instead.
- **Holding the key**: Holding `up` produces repeated one-line scrolls as the terminal repeats
  the keypress.

### Edge Cases

- **Slash command dropdown open**: When the autocomplete dropdown is visible (`/` was typed),
  `up`/`down` navigate the dropdown entries rather than scrolling the viewport. The viewport
  does not scroll until the dropdown is dismissed.
- **Overlay open**: When any overlay is active, `up`/`down` route to the overlay's navigation
  handler, not the viewport. The viewport is not scrolled.
- **Input history navigation**: If the input area is focused and the user has not yet triggered
  scroll mode, behavior depends on cursor position in the input — the distinction between input
  history and viewport scroll is managed at the model level.

---

## STORY-KN-003: Copying the Last Assistant Response

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who wants to paste the assistant's most recent response into another tool,
document, or terminal window
**Goal**: Copy the last assistant response text to the system clipboard without leaving the TUI
**Preconditions**: At least one assistant response has been received in the current session; no
overlay is active; the run is not currently active (or it has completed)

### Steps

1. User presses `ctrl+s` → The TUI copies the full text accumulated in `lastAssistantText` to
   the system clipboard.
2. The status bar flashes a 3-second transient confirmation message (e.g., "Copied to
   clipboard").
3. User switches to another application and pastes with the system paste shortcut → The assistant
   response text appears.

### Variations

- **During an active run**: `ctrl+s` can be pressed while the run is still streaming. It copies
  whatever text has accumulated in `lastAssistantText` up to that point, which may be
  a partial response.
- **After `/clear`**: After clearing the conversation, `lastAssistantText` is reset. Pressing
  `ctrl+s` before any new response has arrived results in copying an empty string or a no-op,
  with the status bar showing "Copy unavailable".

### Edge Cases

- **Clipboard unavailable**: If the system clipboard is inaccessible (e.g., running over SSH
  without clipboard forwarding, or in a headless environment), the status bar shows "Copy
  unavailable" as a 3-second transient message. No crash or hang occurs.
- **Large responses**: Very long assistant responses are copied in full. There is no truncation
  at the clipboard level within the TUI.
- **Multi-turn sessions**: `ctrl+s` always copies the *last* assistant response, not a
  concatenation of all responses. Each completed run replaces `lastAssistantText`.

---

## STORY-KN-004: Navigating the Help Dialog with Tab and Vim Keys

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: New user who wants to learn what keyboard shortcuts are available without reading
external documentation
**Goal**: Open the help dialog, read through all three tabs (Commands, Keybindings, About), and
close it
**Preconditions**: TUI is open; no overlay is active; the user has not previously opened help

### Steps

1. User presses `ctrl+h` (or `?`) → The help dialog overlay opens. The first tab "Commands" is
   active. A list of all slash commands with descriptions is displayed.
2. User presses `tab` (or `right` or `l`) → The next tab "Keybindings" becomes active. The panel
   displays the full keybinding reference table.
3. User presses `tab` again (or `right` or `l`) → The "About" tab becomes active. Project
   information is displayed.
4. User presses `shift+tab` (or `left` or `h`) → The dialog moves back to the "Keybindings" tab.
5. User presses `shift+tab` again → The "Commands" tab is active again.
6. User presses `esc` → The help dialog closes. Focus returns to the input area.

### Variations

- **Opening via slash command**: The user can also type `/help` and press `Enter` to open the
  same dialog. The keyboard navigation within the dialog is identical.
- **Vim-style horizontal navigation**: `h` and `l` serve as left/right tab navigation aliases
  inside the help dialog, allowing users who prefer vim-style movement to navigate without
  reaching for the arrow keys or Tab.
- **Wrapping**: Tab on the last tab wraps to the first; `shift+tab` on the first tab wraps to
  the last.

### Edge Cases

- **Content taller than dialog**: If a tab's content exceeds the dialog height, vertical scroll
  may apply within the dialog. The `up`/`down` keys scroll within the active tab rather than
  switching tabs.
- **Pressing `?` while a run is active**: The help dialog still opens. The run continues in the
  background. Closing the dialog returns the user to the live-streaming viewport.

---

## STORY-KN-005: Navigating the Model Switcher with Vim Keys

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Power user who wants to switch from OpenAI GPT-4o to Anthropic Claude without using
the mouse
**Goal**: Open the model browser, navigate to a different provider, select a model, and confirm
the selection entirely via keyboard
**Preconditions**: TUI is open; no overlay is active; the harness server is reachable and returns
a model list

### Steps

1. User types `/model` and presses `Enter` (or uses tab-completion) → The model switcher overlay
   opens at level 0 (provider list). Providers are listed: OpenAI, Anthropic, Google, DeepSeek,
   xAI, Groq, Qwen, Kimi.
2. User presses `j` (or `down`) → The cursor moves down to the next provider in the list.
3. User presses `j` again until "Anthropic" is highlighted.
4. User presses `Enter` → The overlay drills into level 1, showing the list of Anthropic models.
5. User presses `j`/`k` to navigate the model list to the desired model (e.g., claude-opus-4-6).
6. User presses `Enter` → The overlay advances to level 2 (config panel for that model), showing
   gateway selection, API key status, and reasoning effort controls.
7. User reviews the config and presses `esc` → The config panel closes, returning to level 1
   (model list).
8. User presses `esc` again → Returns to the provider list at level 0.
9. User presses `esc` again → The model switcher overlay closes entirely. Focus returns to the
   input area. The selected model is reflected in the status bar.

### Variations

- **Typing to search**: At level 1 (model list), typing any printable character (no `/`, not
  in config mode) accumulates a search query. The model list filters in real time. Pressing
  `backspace`/`delete` removes the last character from the search.
- **Starring a model**: At level 1 or in search results, pressing `s` toggles the star on the
  highlighted model. Starred models persist to the config file and appear first or marked in
  future sessions.
- **k to go up**: `k` moves the cursor up in both the provider list and the model list, mirroring
  vim navigation.

### Edge Cases

- **Unconfigured provider**: If the selected provider has no API key configured, selecting a
  model from that provider redirects the user to the `/keys` overlay with the cursor
  pre-positioned on the relevant provider. The user must configure the key before the model
  becomes usable.
- **Empty model list**: If the server returns no models for a provider, the level-1 view shows an
  empty state. The user can press `esc` to return to the provider list.
- **Search with no results**: If the typed search string matches no models, the list shows an
  empty state. Pressing `backspace` removes characters from the query until results reappear.

---

## STORY-KN-006: Starring a Frequently Used Model

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who regularly switches between two models and wants quick access to their
preferred model without scrolling through the full provider/model list each time
**Goal**: Star a model so it appears prominently in future sessions
**Preconditions**: Model switcher is open at level 1 (model list for a provider); target model is
visible in the list; model is not yet starred

### Steps

1. User navigates to the desired model using `j`/`k` (or `up`/`down`).
2. User presses `s` → The model is starred. A visual indicator (star glyph or highlight) appears
   next to the model name in the list.
3. User presses `s` again on the same model → The star is removed (toggle behavior). The
   indicator disappears.
4. User presses `esc` to close the overlay → The star state is persisted to the config file and
   will be present in future TUI sessions.

### Variations

- **Starring from search**: If the user has typed a search query and the filtered list shows the
  target model, `s` still stars the highlighted model in the filtered view.
- **Multiple stars**: There is no limit on the number of starred models. Each `s` press toggles
  the individual model independently.

### Edge Cases

- **Star state not saved**: If the process exits abnormally before writing the config file, the
  star state from the current session may be lost. Normal exit (via `/quit` or `ctrl+c`) ensures
  the config write completes.
- **Model becomes unavailable**: If a starred model is no longer returned by the server in a
  future session, the star entry remains in the config file but the model does not appear in the
  switcher.

---

## STORY-KN-007: Managing API Keys via Keyboard

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer setting up the harness for the first time who needs to enter API keys for
multiple providers without leaving the TUI
**Goal**: Navigate to the API keys overlay, enter a key for one provider, and clear and re-enter
an incorrect key for another
**Preconditions**: TUI is open; `/keys` command is available; at least two providers are listed
(e.g., OpenAI and Anthropic); neither provider is currently configured

### Steps

1. User types `/keys` and presses `Enter` → The API keys overlay opens. A list of providers is
   displayed with their configuration status (configured / not configured).
2. User presses `j` (or `down`) to move to the "OpenAI" provider entry.
3. User presses `Enter` → The overlay enters key-input mode for OpenAI. A text input field
   appears for the API key value.
4. User types the API key string.
5. User presses `Enter` → The key is submitted to the server via PUT /v1/providers/openai/key. The
   overlay exits key-input mode and shows the provider as "configured".
6. User presses `j` to move to the "Anthropic" entry.
7. User presses `Enter` → Enters key-input mode for Anthropic.
8. User types an incorrect key string.
9. User realizes the mistake and presses `ctrl+u` → The input field is cleared entirely.
10. User types the correct API key.
11. User presses `Enter` → The key is submitted. Anthropic is now shown as "configured".
12. User presses `esc` → Exits key-input mode (if still active) first. A second `esc` closes the
    API keys overlay entirely.

### Variations

- **Correcting a single character**: Instead of `ctrl+u`, the user can use `backspace` to delete
  the last character one at a time.
- **No change needed**: If a provider is already configured and the user presses `Enter`, they
  can re-enter a new key to overwrite. `ctrl+u` clears the existing placeholder before typing.

### Edge Cases

- **Server error on key submission**: If the server returns an error when the key is submitted,
  the overlay may not update the "configured" status. The user sees a status bar error message.
- **Esc priority in key-input mode**: Pressing `esc` while in key-input mode exits input mode
  without submitting the typed key. The provider's previous state is preserved.
- **Empty key submission**: Pressing `Enter` with an empty input field may submit an empty string
  to the server; behavior depends on the server-side validation.

---

## STORY-KN-008: Using Esc to Unwind Nested Overlay State

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who opened the model switcher, drilled into a provider's config panel, and
wants to return to the main chat view through a multi-level escape sequence
**Goal**: Close all overlay layers with a predictable escape chain using only `esc`
**Preconditions**: Model switcher is open at level 2 (config panel); API key input mode is active
within the config panel

### Steps

1. User presses `esc` → Exits API key input mode within the config panel. The config panel itself
   remains visible (level 2 is still active).
2. User presses `esc` → Closes the config panel, returning to level 1 (model list for the
   selected provider).
3. User presses `esc` → Clears the search query if one was active. The model list shows all
   models unfiltered.
4. User presses `esc` → Returns to level 0 (provider list).
5. User presses `esc` → Closes the model switcher overlay entirely. Focus returns to the input
   area.

### Variations

- **No search active**: If no search query was typed at level 1, step 3 is skipped — `esc` at
  level 1 goes directly to the provider list.
- **From stats or context overlay**: Pressing `esc` with the stats panel or context grid open
  closes the overlay in a single keypress with no intermediate states.
- **From help dialog**: A single `esc` closes the help dialog regardless of which tab is active.

### Edge Cases

- **Active run during overlay navigation**: If a run is in progress and the user has navigated
  deep into the model switcher, the escape priority chain applies to the overlay first. The run
  is not cancelled until all overlays are dismissed and the user presses `esc` or `ctrl+c` again
  from the main view with no input text.
- **Non-empty input after closing overlay**: If the input area had text before the overlay was
  opened, the text is preserved and the cursor returns to the input area. Pressing `esc` again
  clears the input text, with the status bar showing "Input cleared".

---

## STORY-KN-009: Interrupting and Cancelling an Active Run

**Type**: medium
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who submitted a prompt and realizes the agent is heading in the wrong
direction mid-run
**Goal**: Cancel the active run cleanly without quitting the TUI
**Preconditions**: A run is active and streaming; the interrupt banner is not yet shown; no
overlay is open; input area is empty

### Steps

1. User presses `ctrl+c` → The interrupt banner appears in the "Confirm" state with the message
   "Press Ctrl+C again to stop...". This prevents accidental cancellation on a single keypress.
2. User presses `ctrl+c` again → The interrupt banner transitions to the "Waiting" state
   ("Stopping... (waiting for current tool to finish)"). The harness cancels the SSE stream.
3. The run terminates. The interrupt banner transitions to the "Done" state briefly, then
   dismisses.
4. The status bar shows "Interrupted" as a 3-second transient message.
5. The input area is focused again. The user can type a new prompt.

### Variations

- **Using `esc` to cancel instead**: If the input area is empty and no overlay is open, pressing
  `esc` during an active run also cancels it (escape priority: cancel active run is priority 5).
  No two-stage confirmation is required with `esc` — it cancels immediately if the input is
  empty.
- **Changing mind after first `ctrl+c`**: If the user presses `ctrl+c` once (Confirm state) and
  then does nothing for a moment, the interrupt banner may auto-dismiss (depending on
  implementation). A single `ctrl+c` without a follow-up does not cancel the run.

### Edge Cases

- **Ctrl+C with no active run**: If no run is active, `ctrl+c` quits the TUI entirely (the
  "Quit" binding behavior). The user should be aware of this distinction.
- **Tool call mid-execution**: The "Waiting" state of the interrupt banner conveys that the
  cancellation waits for the current in-flight tool call to complete before halting. The user
  may see one more tool result appear in the viewport before the run fully stops.
- **Overlay open during active run**: If an overlay is open when `ctrl+c` is pressed, the
  `ctrl+c` quit/cancel binding still fires (it is not shadowed by overlays). The run is
  cancelled and/or the TUI quits.

---

## STORY-KN-010: Writing a Multi-Line Prompt Without Submitting

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer composing a structured, multi-paragraph prompt with code examples embedded
**Goal**: Enter multiple lines of text in the input area without accidentally submitting the prompt
**Preconditions**: TUI is open; no overlay is active; the input area is focused and empty

### Steps

1. User types the first line of the prompt.
2. User presses `shift+enter` → A newline is inserted in the input area. The cursor moves to the
   next line. The prompt is NOT submitted.
3. User types the second paragraph.
4. User presses `ctrl+j` → An alternative newline insertion shortcut. Another line break is
   added. This is functionally identical to `shift+enter`.
5. User types the final line, including a code snippet.
6. User presses `enter` → The full multi-line prompt is submitted as a single message to the
   harness.

### Variations

- **Using ctrl+j exclusively**: Users on terminals where `shift+enter` is not reliably
  distinguished from `enter` (some SSH clients or terminal multiplexers) can use `ctrl+j` as
  the newline shortcut instead.
- **Pasting multi-line content**: If the user pastes text containing newlines, the input area
  accepts the full pasted content including embedded newlines without submitting.

### Edge Cases

- **`Enter` vs `shift+enter` confusion**: The most common error is pressing `enter` when
  `shift+enter` is intended, which submits an incomplete prompt. There is no undo for a
  submitted message — the user must issue a follow-up message or use `/clear` to reset the
  conversation.
- **Very long multi-line input**: The input area wraps text visually. Scrolling within the input
  area (if supported) lets the user review the full content before submitting.

---

## STORY-KN-011: Toggling Stats Panel Period with `r`

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer monitoring agent cost and usage who wants to compare activity across
different time windows
**Goal**: Open the stats panel and cycle through week, month, and year views using a single key
**Preconditions**: TUI is open; at least one run has completed in the current harness deployment;
no overlay is active

### Steps

1. User types `/stats` and presses `Enter` → The stats panel overlay opens. The default view
   shows the activity heatmap for the current week. Run count and cumulative USD cost are
   displayed.
2. User presses `r` → The period toggles from "week" to "month". The heatmap redraws to show
   the current month's activity.
3. User presses `r` again → Period toggles from "month" to "year". The heatmap expands to show
   the full year.
4. User presses `r` again → Period wraps back to "week".
5. User presses `esc` → The stats panel closes. Focus returns to the input area.

### Variations

- **No historical data**: If the harness has only just started, the heatmap may show a sparse or
  empty grid for month and year views, with zero run counts. The panel still renders correctly.

### Edge Cases

- **`r` key outside stats overlay**: The `r` key has no special binding in the main view or
  other overlays. It is typed as a regular character into the input area if the stats panel is
  not open.
- **Heatmap rendering at small terminal size**: If the terminal is narrower than the heatmap's
  natural width, the rendering may be truncated or reflowed. The `r` toggle still functions
  correctly regardless of display quality.

---

## STORY-KN-012: Opening the Editor for Long Prompt Composition

**Type**: short
**Topic**: Keyboard-Driven Navigation
**Persona**: Developer who wants to compose a complex, structured prompt in their preferred text
editor (e.g., vim or nano) rather than in the TUI's input area
**Goal**: Open an external editor for prompt composition and return the finished text to the TUI
input area
**Preconditions**: TUI is open; the `EDITOR` or `VISUAL` environment variable is set; no overlay
is active; the input area is focused

### Steps

1. User presses `ctrl+e` → The TUI invokes the external editor (from `$EDITOR` or `$VISUAL`).
   The terminal is temporarily handed off to the editor process.
2. The user composes the prompt in the editor, using the editor's full feature set (syntax
   highlighting, macros, etc.).
3. The user saves and exits the editor → Control returns to the TUI. The editor's output is
   inserted into the input area.
4. The input area now contains the composed prompt text.
5. User presses `enter` to submit, or reviews and edits further before submitting.

### Variations

- **Pre-existing input text**: If the input area already contains text when `ctrl+e` is pressed,
  that text may be pre-populated in the editor as the starting content, allowing the user to
  continue editing an in-progress prompt.

### Edge Cases

- **No `EDITOR` set**: If neither `$EDITOR` nor `$VISUAL` is set, the TUI may fall back to a
  default editor (e.g., `vi`) or show an error in the status bar indicating that no editor is
  configured.
- **Editor exits without saving**: If the user opens the editor and quits without saving (e.g.,
  `:q!` in vim), the input area returns to its previous state unchanged.
- **Editor crashes**: If the editor process exits abnormally, the TUI should recover and return
  focus to the input area. Any content written before the crash may or may not be recovered
  depending on the editor's temp file handling.
