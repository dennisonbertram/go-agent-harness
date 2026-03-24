# UX Stories: Slash Commands & Autocomplete

**Topic**: Typing `/` to discover and execute slash commands, autocomplete filtering and navigation, keyboard shortcuts for efficiency.

**Application**: go-agent-harness TUI (`harnesscli --tui`)

**Status**: Generated 2026-03-23

---

## STORY-001: Typing "/" Opens Command Autocomplete Dropdown

**Context**: User is in the TUI, input area is focused, and has typed nothing.

**Action**:
1. User types `/` into the input area
2. The slash command autocomplete dropdown appears above the input, showing all 10 available commands

**Result**:
- Dropdown is visible with commands: `/clear`, `/context`, `/export`, `/help`, `/keys`, `/model`, `/profiles`, `/quit`, `/stats`, `/subagents`
- Each command shows its short description: e.g. "Clear conversation history", "View context window usage", etc.
- The first command (alphabetically) is highlighted by default (cursor position = 0)
- Input field shows just `/` with the cursor after it
- Status bar does not show an error

**Technical Notes**:
- `slashcomplete.Model.Open()` is called, setting `active = true`
- `SetQuery("")` initializes filtered results with all commands (via `FuzzyFilter`)
- Dropdown height is up to 8 rows; if more than 8 commands exist, they're scrollable (not the case currently with 10 commands)

---

## STORY-002: Filtering Commands by Typing

**Context**: User has typed `/` and the autocomplete dropdown is open. The first command `/clear` is highlighted.

**Action**:
1. User types `h` (so the input field now reads `/h`)
2. The autocomplete dropdown filters in real-time to show only commands matching `h`

**Result**:
- Dropdown now shows only `/help` (and no others, since only `/help` contains the substring `h` at an early position)
- `/help` is highlighted by default (cursor reset to 0)
- Input field shows `/h` with the cursor after it
- Status bar shows no error

**Technical Notes**:
- `SetQuery("h")` is called, which calls `FuzzyFilter(suggestions, "h")`
- `FuzzyFilter` does fuzzy/prefix matching: `h` matches the `h` in `help`
- `selected` is reset to 0 on every query update
- This matches the regex-based filtering behavior in the slashcomplete component

---

## STORY-003: Navigating Autocomplete with Arrow Keys

**Context**: Autocomplete dropdown is open, showing multiple matching commands (e.g., after typing `/` or `/c`). One command is highlighted.

**Action**:
1. User presses the **Down** arrow key
2. The highlight moves to the next command in the list (wrapping to the first if at the bottom)

**Alternative**:
1. User presses the **Up** arrow key
2. The highlight moves to the previous command (wrapping to the last if at the top)

**Result**:
- Highlighted command changes smoothly without closing the dropdown
- The selected suggestion is visually distinct (reversed colors in the renderer)
- Pressing Down repeatedly cycles through all filtered commands and wraps to the top
- Pressing Up repeatedly cycles in reverse

**Technical Notes**:
- `Model.Down()` increments `selected` with modulo wrapping: `(selected + 1) % len(filtered)`
- `Model.Up()` decrements `selected` with modulo wrapping: `(selected - 1 + len(filtered)) % len(filtered)`
- Zero results → arrow keys are no-ops
- Navigation does not modify the input text

---

## STORY-004: Accepting a Command with Enter

**Context**: Autocomplete dropdown is open, a command is highlighted (e.g., `/help`).

**Action**:
1. User presses **Enter** to accept the highlighted command

**Result**:
- Dropdown closes immediately
- The command is automatically executed (e.g., the help overlay opens for `/help`)
- Input field is cleared (reset to empty)
- Status bar may show a transient message (e.g., if the command is unrecognized, it shows a hint)
- For `/quit`, the TUI exits; for `/help`, `/model`, `/stats`, etc., the corresponding overlay opens

**Technical Notes**:
- `Model.Accept()` is called, returning `(closed model, "/help ")` (the completed text with trailing space)
- The completed text is then parsed via `ParseCommand()` into a `Command{Name: "help", Args: []}`
- The command registry dispatches to the handler (e.g., `executeHelpCommand`)
- The main model updates based on the command's result (overlay state, run cancellation, etc.)

---

## STORY-005: Accepting a Command with Tab

**Context**: Autocomplete dropdown is open, a unique command is highlighted (e.g., when the input is `/h` and only `/help` matches).

**Action**:
1. User presses **Tab** to accept and auto-complete the command
2. The command is accepted and the dropdown closes
3. The input field is updated to the full command with a trailing space

**Result**:
- Input field now reads `/help ` (or whichever command was completed)
- Dropdown closes
- User can now type additional arguments or press Enter to execute
- If there are multiple matches (e.g., `/h` matches only `/help` but `/` matches all), Tab still auto-completes to the full name + space

**Alternative Case** (multiple matches):
- User types `/c` (matches `/clear` and `/context`)
- User presses **Tab**
- Common prefix is computed: only `c` is shared by both, so input becomes `/c` (no change)
- Dropdown remains open, showing both commands, so the user can press Down/Up to select one or type more

**Technical Notes**:
- `CompleteTab()` in inputarea calls the autocomplete provider (slash command completer)
- Single match → input becomes `"/command "` (with trailing space)
- Multiple matches → input becomes the common prefix (which may be unchanged)
- Zero matches → no-op
- Trailing space allows the user to type arguments without modifying the command name

---

## STORY-006: Dismissing Autocomplete with Escape

**Context**: Autocomplete dropdown is open, possibly with a partial command typed (e.g., `/mo`).

**Action**:
1. User presses **Escape**
2. The dropdown closes without accepting any command

**Result**:
- Dropdown is hidden (overlay is closed)
- Input field text remains unchanged (e.g., `/mo` is still there)
- Cursor remains in the input field, ready for the user to:
  - Continue typing (e.g., type `del` to change `/mo` to `/model`)
  - Clear and start over
  - Submit a different message
- No command is executed
- No status message is shown (Escape is silent for autocomplete dismiss)

**Technical Notes**:
- `slashcomplete.Model.Close()` sets `active = false`
- The input text and cursor position are untouched
- Escape has priority: it closes overlay input mode first, then closes the overlay, then cancels active runs, then clears input

---

## STORY-007: Typing Partial Command and Tab Completion

**Context**: User types a partial slash command and relies on Tab to complete it quickly.

**Action**:
1. User types `/cle` (aiming for `/clear`)
2. User presses **Tab**

**Result**:
- Input field changes from `/cle` to `/clear ` (with trailing space)
- Autocomplete dropdown closes (because the command is now complete)
- User can now press **Enter** to execute, or type arguments

**Alternative**: User types `/c` (ambiguous between `/clear` and `/context`)
- User presses **Tab**
- No change to input (common prefix is `/c`)
- Dropdown remains open
- User presses **Down** to highlight `/context`
- User presses **Tab** again
- Input becomes `/context ` (command is accepted)

**Technical Notes**:
- Tab completion is wired via `SetAutocompleteProvider()` in the main model
- The provider queries the command registry for matching entries
- Single match triggers auto-space; multiple matches show the common prefix
- The dropdown remains open as long as the input starts with `/` and has unresolved completions

---

## STORY-008: Using Input History with Up/Down (While Autocomplete Is Open)

**Context**: User has previously typed messages (e.g., past prompts or past `/help` commands). Autocomplete dropdown is open.

**Action**:
1. User presses **Up** arrow while autocomplete is open and focused

**Result**:
- Behavior depends on implementation:
  - If Up applies to the autocomplete dropdown: the highlighted command moves up in the list (wrapping to the bottom)
  - If Up applies to input history: the input field replaces with the previous message from history, and the dropdown closes/updates

**Current Implementation**: Up/Down in the input area affect the **input component's history**, not the dropdown's selection. The dropdown's Up/Down are only reachable when the overlay is focused.

**Technical Notes**:
- When autocomplete is open, the input area is still focused
- Up/Down key handling is in `inputarea.Model.Update()`
- `m.history.Up(m.value)` fetches the previous message; `m.history.Down()` fetches the next
- History is stored in `inputarea.History` (LIFO stack, max 100 entries)
- Restarting a new message from history resets the history position

---

## STORY-009: Autocomplete Reopens After Editing

**Context**: User has typed `/help ` (a complete command with trailing space), autocomplete dropdown is closed.

**Action**:
1. User backspaces once to remove the space, so input is now `/help`
2. The autocomplete dropdown reopens (since input starts with `/` again and has unresolved completions)

**Result**:
- Dropdown shows all commands again (or filters based on the new query)
- User can continue typing, navigate with arrow keys, or dismiss with Escape

**Alternative**: User types more text after `/help `, e.g., `/help foo`
- Autocomplete does NOT reopen (input no longer starts with only `/` and a command; it's treated as `command args`)
- Dropdown remains closed until user clears the input and types `/` again

**Technical Notes**:
- The main model checks `strings.HasPrefix(input, "/")` to decide whether to open the autocomplete
- Once a command is accepted, input is typically cleared or a new message is submitted
- If the user manually edits a completed command (e.g., remove the space), the autocomplete re-opens

---

## STORY-010: Quitting the TUI with /quit

**Context**: User is in the TUI, input area is focused.

**Action**:
1. User types `/quit` into the input
2. Autocomplete dropdown shows `/quit` as the only match (or as one of few matches if they typed `/q`)
3. User presses **Enter** (or highlights and presses Enter)

**Result**:
- TUI exits immediately
- Terminal returns to the shell prompt
- No confirmation dialog is shown (quit is immediate)
- Any unsaved conversation state is lost (unless the user previously ran `/export`)

**Alternative**: User presses **Ctrl+C** instead
- If a run is active: the interrupt banner appears with two-stage confirmation
- If no run is active: Ctrl+C also exits the TUI (with no additional prompt)

**Technical Notes**:
- `executeQuitCommand()` handler returns a command that emits `tea.Quit()`
- The main BubbleTea model loop receives this and exits gracefully
- No cleanup of external resources happens (harnessd server continues running)

---

## Summary

| **Story** | **Interaction** | **Key Insight** |
|-----------|-----------------|-----------------|
| STORY-001 | `/` opens dropdown | Dropdown shows all 10 commands, first is highlighted |
| STORY-002 | Type to filter | Real-time fuzzy matching narrows command list |
| STORY-003 | Arrow keys navigate | Up/Down wrap around, highlight changes, input text unchanged |
| STORY-004 | Enter accepts | Dropdown closes, command executes, input clears |
| STORY-005 | Tab completes | Single match → adds command + space; multiple → common prefix |
| STORY-006 | Escape dismisses | Dropdown closes, input text remains, no command executed |
| STORY-007 | Partial + Tab | Fast path: `/cle` + Tab = `/clear ` |
| STORY-008 | History Up/Down | Up/Down apply to input history when autocomplete is open (not dropdown selection) |
| STORY-009 | Autocomplete reopens | Removing trailing space reopens dropdown; typing args closes it |
| STORY-010 | /quit exits | Immediate exit; Ctrl+C is also a shortcut to quit (or interrupt if run active) |

---

## Accessibility & Keyboard-First Design

- **All commands are discoverable via `/`**: users never have to memorize slash command names; they can always type `/` to see what's available
- **Visual feedback**: highlighted command is reversed colors; description is visible
- **Fast paths**: Tab for unambiguous completion, arrow keys for slow navigation, Enter to execute
- **No mouse required**: all interactions are keyboard-based
- **Escape always works**: users can cancel at any stage
- **History integration**: previous messages are one Up-arrow away, seamless with slash command input

---

## Related Topics

- **Input History & Multi-line**: UX stories covering Up/Down navigation of previous messages
- **Overlays & Modals**: Help dialog, model switcher, profile picker (all opened via slash commands)
- **Error Recovery & Interrupts**: Ctrl+C handling during active runs
- **First Launch & Chat**: Initial TUI experience

---

**Story Count**: 10
**Last Updated**: 2026-03-23
