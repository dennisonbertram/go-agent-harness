# Native Story Catalog — GoCode.app (macOS)

Generated: 2026-07-27
Target: `/Users/dennison/develop/go-code/macapp` (SwiftUI, SPM executable `GoCodeApp`)
Consumer: `/ux-walker-native` (Swift AX driver: `snapshot`, `click <ref|label>`, `type <ref> <text>`, `key`, `window`)

---

## Driver conventions (read before walking)

**AX blackout.** The app swaps whole panes on every icon-rail click (`ProjectView.content`, `AppShell.swift:143-166`). The tree is briefly empty. Rule: after any click that changes section or tab, `snapshot`, and if the tree is empty or missing the expected label, sleep ~1s and `snapshot` again — up to 3 attempts — before recording a failure.

**Labels are scarce.** The entire app contains exactly one explicit accessibility label (`ChatView.swift:150`, `"Copy message"`). Everything else relies on SwiftUI's derived labels. Icon-only controls — the five icon-rail buttons, the close-project button, the send button, the per-message copy buttons — carry only `.help(...)` tooltips. Where a story says "click the rail button for X", the walker must first `snapshot` and find a ref; **if no ref with a usable label exists, that is a reportable accessibility defect, not a walker failure.** Record it and fall back to clicking by position within the rail (top-to-bottom order: Chat, Activity, Sessions, Checkpoints, Settings, then a spacer, then Close).

**Ambiguous labels.** `"New"` appears on two different controls (composer footer, `ChatView.swift:727`; Sessions toolbar, `SessionsView.swift:15`). `"Cancel"`, `"Save"`, `"Remove"`, `"Add…"` recur. Always disambiguate by the containing pane in the snapshot, never by label alone.

**Sentinel tokens.** Assertions on model output are only checkable if the output is forced. Every story that inspects transcript text instructs the agent to emit a unique uppercase-hyphen token (e.g. `CALLBACK-ALPHA-7731`). Assert on the token, never on paraphrase. If a story is re-run, change the numeric suffix so a stale transcript cannot pass it.

**Launch.** Set `HARNESS_WORKSPACE=<a scratch git repo>` to skip the project picker, `HARNESS_BINARY=<path to harnessd>` so the binary resolves, and a working provider key so runs actually execute. Stories that must exercise the picker say so and require launching *without* `HARNESS_WORKSPACE`.

**Deferred tools.** Every capability in the four priority areas — callbacks, cron, workflows, subagents — is registered `TierDeferred` (`internal/harness/tools_default.go:347-400`) and is invisible to the model until `find_tool` activates it for that run. Prompts in those stories therefore say `Call find_tool with query "select:<tool>"` explicitly. Expect a `find_tool` tool row *before* the target tool row in every one of them. A story that shows no `find_tool` row and no target row means the model never even saw the tool — report that as a distinct outcome from "the tool ran and failed".

---

## Platform Surface

**Menu bar commands:** none. `GoCodeApp.swift` declares a bare `WindowGroup` with no `.commands { }`. Only AppKit's default File/Edit/Window/Help menus exist.

**Keyboard shortcuts:** none defined anywhere in the app (`grep keyboardShortcut` → zero hits). Only system defaults: ⌘W close window, ⌘Q quit, ⌘M minimise, ⌘N new window (from `WindowGroup`), ⌘C/⌘V in text fields. **⌘, does nothing** — there is no `Settings` scene; Settings is an in-window section reached from the icon rail.

**Permissions requested:** none directly. Two `NSOpenPanel` invocations (project picker, Settings → Access → Add…) and one `NSSavePanel` (Sessions → Export Transcript…) — these grant sandbox-free file access implicitly since the app is unsandboxed SPM.

**Window model:** single `WindowGroup("GoCode")`, min 960×600 (`AppShell.swift:64`). ⌘N opens a second independent window with its own `AppShell`, its own `ProjectSession`, and therefore its own `harnessd` child process. No document model, no tabs, no state restoration.

**Subprocess:** one `harnessd` per open project, supervised by `HarnessSupervisor`, port allocated by binding `:0`. Env set by the app: `HARNESS_ADDR`, `HARNESS_WORKSPACE`, `HARNESS_CONVERSATION_DB` (`<workspace>/.harness/conversations.db`), plus `HARNESS_PROMPTS_DIR` / `HARNESS_MODEL_CATALOG_PATH` / `HARNESS_PRICING_CATALOG_PATH` pinned to the installation root. **`HARNESS_RUN_DB` is never set** — so the run store is absent and `GET /v1/runs` answers 501 for the whole life of the app.

---

## A. Launch & Project Lifecycle

## STORY-001: Cold launch with no workspace lands on the project picker

**Type**: short
**Topic**: Launch & project lifecycle
**Persona**: Priya, opening the app for the first time from the Dock
**Goal**: Understand what the app wants from her before it will do anything
**Preconditions**: App launched with `HARNESS_WORKSPACE` unset. `HARNESS_BINARY` set so a later open can succeed.
**Entry**: Cold launch
**Window state**: Main window, no project open
**Ideal path**: 1 — the picker is the whole screen; one click should open a folder.
**Alternate paths**: `HARNESS_WORKSPACE` env var bypasses the picker entirely (`GoCodeApp.swift:11-13`).

### Steps
1. `window` → a window titled "GoCode" exists, at least 960×600.
2. `snapshot` → read all static text.

### Success condition
The tree contains the exact strings `Open a project`, `Each project runs its own harness server.`, and a clickable control labelled `Choose Folder…`. No transcript, no icon rail, and no Settings control is present.

### Edge cases
- Launched from Finder rather than a terminal, the process inherits a minimal `PATH`, so `HarnessBinary.locate()` finds no `harnessd` unless `HARNESS_BINARY` is set. The picker still appears; the failure only surfaces after choosing a folder (STORY-005).

---

## STORY-002: Opening a project starts its server and reaches the chat surface

**Type**: medium
**Topic**: Launch & project lifecycle
**Persona**: Priya, opening her repo for the first time
**Goal**: Get to a usable composer
**Preconditions**: `HARNESS_WORKSPACE` set to a scratch git repo; `HARNESS_BINARY` valid; a provider key configured.
**Entry**: Cold launch with `HARNESS_WORKSPACE` set
**Window state**: Main window
**Ideal path**: 0 — with a workspace supplied the app should land on chat with no clicks.
**Alternate paths**: Picker → Choose Folder… → NSOpenPanel (STORY-003).

### Steps
1. `snapshot` immediately → expect either the starting state or chat.
2. If the tree contains `Starting the harness for`, re-`snapshot` every 1s for up to 30s.
3. `snapshot` → read the toolbar and composer.

### Success condition
Within 30s the tree contains the composer placeholder `Ask the harness to do something…`, the status text `Ready`, a toggle labelled `Plan mode`, and a toolbar heading equal to the workspace folder's last path component. The string `Starting the harness for` is gone and `The harness server could not start` never appeared.

### Variations
- Slow first launch: the starting text names the folder — assert it interpolates the real folder name, not an empty string.

---

## STORY-003: Choosing a folder from the picker

**Type**: short
**Topic**: Launch & project lifecycle
**Persona**: Priya
**Goal**: Open a project by browsing
**Preconditions**: App on the project picker (STORY-001 state).
**Entry**: Click `Choose Folder…`
**Window state**: Main window plus a system open panel
**Ideal path**: 2 — click, pick, done.
**Alternate paths**: none in-app.

### Steps
1. `click Choose Folder…` → a system open panel appears.
2. `snapshot` → the panel is a separate AX element; locate its `Open` button.
3. Navigate to the scratch repo and click `Open`.
4. `snapshot`, retrying for the AX blackout.

### Success condition
The panel dismisses and within 30s the main window tree contains the composer placeholder `Ask the harness to do something…`. If the walker cannot drive the system panel, record `NSOpenPanel not drivable` as an environment limitation and mark this story skipped — do not mark it failed.

### Edge cases
- The panel is directory-only (`canChooseFiles = false`); selecting a file must be impossible.

---

## STORY-004: The icon rail reaches all five sections

**Type**: short
**Topic**: Navigation
**Persona**: Priya, orienting
**Goal**: See what the app can do
**Preconditions**: A project is open and ready.
**Entry**: The 50pt rail on the window's left edge
**Window state**: Main window
**Ideal path**: 5 — one click per section; there is no faster route.
**Alternate paths**: none. There is no menu item, no shortcut, and no other way to change section.

### Steps
1. `snapshot` → locate five rail controls with help text `Chat`, `Activity`, `Sessions`, `Checkpoints`, `Settings`, and a sixth with help `Close project and stop its server`.
2. For each of Activity, Sessions, Checkpoints, Settings in turn: click it, then `snapshot` (retrying through the blackout).
3. Click `Chat` last.

### Success condition
Each click produces a tree containing that section's signature text — Activity: `Background work`; Sessions: either `Search conversations` or `No saved conversations`; Checkpoints: either `No checkpoints yet` or at least one `Restore` button; Settings: a segmented control containing `Providers`, `Models`, `Project`, `Access`; Chat: the composer placeholder. All five must be reachable in one walk.

### Edge cases
- If the six rail buttons expose no distinguishable AX labels, record the defect and proceed positionally.

---

## STORY-005: Missing harnessd binary is explained, not silent

**Type**: short
**Topic**: Launch failure
**Persona**: Priya, who installed the app but not the server
**Goal**: Find out why nothing happened
**Preconditions**: Launch with `HARNESS_BINARY` unset and no `harnessd` on `PATH`; `HARNESS_WORKSPACE` set.
**Entry**: Cold launch
**Window state**: Main window
**Ideal path**: 0 — the failure should be on screen without any click.
**Alternate paths**: none.

### Steps
1. `snapshot` and retry for up to 10s.

### Success condition
The tree contains the heading `The harness server could not start`, a selectable body containing the literal substring `Could not find the harnessd binary`, and a button labelled `Try Again`. The window must not sit indefinitely on `Starting the harness for`.

---

## STORY-006: Closing a project returns to the picker and stops the server

**Type**: medium
**Topic**: Launch & project lifecycle
**Persona**: Priya, done for the day
**Goal**: Close the project without leaving a server running
**Preconditions**: A project is open and ready; note the harnessd PID.
**Entry**: The bottom rail button, help text `Close project and stop its server`
**Window state**: Main window
**Ideal path**: 1.
**Alternate paths**: ⌘W closes the window — but `AppShell` has no `onDisappear`, so window close does **not** call `project.shutdown()`. Compare the two paths; a divergence is a defect.

### Steps
1. `click` the bottom rail button.
2. `snapshot`, retrying through the blackout.
3. Out of band: check whether the harnessd child is still alive.

### Success condition
The tree returns to `Open a project` / `Choose Folder…`, and the harnessd process for that workspace has exited within 10s. Then repeat with ⌘W in a fresh launch and record whether the server survives — if it does, that is a leaked-process finding.

---

## B. Conversation Core

## STORY-007: Send a prompt and see a streamed reply

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus, a developer trying the app out
**Goal**: Get one answer
**Preconditions**: Project ready, a usable model configured.
**Entry**: Composer text field
**Window state**: Chat section
**Ideal path**: 2 — type, submit.
**Alternate paths**: none — no menu item and no shortcut sends a prompt.

### Steps
1. `snapshot` → find the field whose placeholder is `Ask the harness to do something…`.
2. `type <ref> Reply with exactly the token HELLO-SYNC-4412 and nothing else.`
3. `key return`.
4. `snapshot` repeatedly, at 1s intervals, for up to 120s.

### Success condition
A user row containing the typed prompt appears immediately. The status text moves off `Ready` to `Starting` or `Working` within 5s. Within 120s the transcript contains the exact token `HELLO-SYNC-4412` and the status reads `Done`. The composer field is empty after submit.

### Edge cases
- Submitting an empty or whitespace-only draft must do nothing: the send control is disabled on `run.draft.trimmed.isEmpty` (`ChatView.swift:717`).

---

## STORY-008: The composer is disabled correctly while a run is in flight

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus, impatient
**Goal**: Not accidentally start two runs
**Preconditions**: A run is in flight.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: n/a — a guardrail, not a goal.
**Alternate paths**: none.

### Steps
1. Send a long prompt (`Count slowly from 1 to 40, one number per line.`).
2. While status reads `Working`, `snapshot` the composer.
3. `type` a second prompt and `key return`.

### Success condition
While busy, the composer placeholder reads `Steer the running task…` (not `Ask the harness to do something…`), and the second submit produces a steering injection rather than a second user row followed by a second run. Exactly one run is active at any moment.

---

## STORY-009: Steer a running task mid-flight

**Type**: medium
**Topic**: Core conversation
**Persona**: Marcus, who realises he asked the wrong thing
**Goal**: Redirect without cancelling
**Preconditions**: Project ready.
**Entry**: Composer while a run is active
**Window state**: Chat
**Ideal path**: 2 — type the correction, submit.
**Alternate paths**: Stop then re-prompt (STORY-010) — a different outcome, both reachable from the same bar.

### Steps
1. Send `List every file in this repository one per line, slowly.`
2. Wait until status reads `Working` and at least one tool row or partial reply is visible.
3. `snapshot` → assert the placeholder is now `Steer the running task…` and the send control's help text is `Steer the running task`.
4. `type <composer ref> Stop listing files. Instead reply with exactly STEER-OK-9120 and finish.`
5. `key return`.
6. `snapshot` every 1s for 120s.

### Success condition
The transcript ends containing the exact token `STEER-OK-9120`, the run reaches `Done` (not `Cancelled`), and no second user-prompt bubble was created for the steering text. If the steering text appears as a normal user bubble and a second run starts, the steer path is broken — report it.

---

## STORY-010: Two-stage stop

**Type**: medium
**Topic**: Core conversation
**Persona**: Marcus
**Goal**: Interrupt a long run
**Preconditions**: A long run in flight.
**Entry**: `Stop` button in the status bar
**Window state**: Chat
**Ideal path**: 1 — a single stop should suffice; the second press exists because the first is cooperative.
**Alternate paths**: none.

### Steps
1. Send `Count slowly from 1 to 200, one number per line.`
2. When status reads `Working`, `snapshot` → locate the button labelled `Stop`.
3. `click Stop`.
4. `snapshot` within 2s.
5. If status still shows a stopping state after 10s, `click Stop` again.

### Success condition
After the first click the status text reads exactly `Stopping — press Stop again to force`. After the run ends the status reads `Cancelled` and the `Stop` button is gone. The transcript retains everything streamed before the stop.

### Edge cases
- Stopping before the server has issued a run id: `cancel()` falls through to cancelling the local stream (`RunSession.swift:94-97`) — the status should still settle, never hang on `Starting`.

---

## STORY-011: Usage and cost appear once tokens are spent

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus, cost-conscious
**Goal**: See what a turn cost
**Preconditions**: One completed run on a priced model.
**Entry**: Status bar, right side
**Window state**: Chat
**Ideal path**: 0 — passive display.
**Alternate paths**: Sessions list also shows a per-conversation cost (`SessionsView.swift:99`) — the same fact on a second surface.

### Steps
1. Complete STORY-007.
2. `snapshot` the status bar.

### Success condition
The status bar contains text matching `<digits> tok · $<digits>.<4 digits>` when the model is priced, or `<digits> tok · cost n/a` when it is not. It must never render `$0.0000` for a model with unknown pricing — that is the specific bug the `costIsKnown` check exists to prevent (`ChatView.swift:576-589`).

### Variations
- Run the same story on a model with no price set in Settings → Models and assert the `cost n/a` branch.

---

## STORY-012: @-mention completes a file path

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus, referencing a file
**Goal**: Insert a path without typing it
**Preconditions**: Project ready; workspace contains a file with a distinctive name.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 3 — type `@`, type a few characters, click the match.
**Alternate paths**: type the path by hand.

### Steps
1. `type <composer ref> Read @Packa`
2. Wait ~500ms (the lookup is debounced 120ms and scans off-thread).
3. `snapshot`.
4. `click` the row whose text ends `Package.swift`.
5. `snapshot` the composer.

### Success condition
Step 3 shows a popup list of at most 8 rows, each a repo-relative path. After step 4 the composer text is exactly `Read @Package.swift ` (note the trailing space) and the popup is gone.

### Edge cases
- Typing a space after `@` must dismiss the popup (`MentionQuery.current` returns nil on whitespace).
- A workspace with no match must show no popup rather than an empty box.

---

## STORY-013: New conversation clears the transcript

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus, switching topic
**Goal**: Start clean
**Preconditions**: A conversation with at least one exchange.
**Entry**: `New` in the composer footer
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: The `New` button in Sessions (`SessionsView.swift:15`) does the same thing and then jumps to Chat — a duplicated path worth flagging.

### Steps
1. Complete STORY-007 so a token is in the transcript.
2. `snapshot` → find the `New` control inside the composer footer (same row as `Plan mode`).
3. `click` it.
4. `snapshot`.

### Success condition
The transcript is empty — the token `HELLO-SYNC-4412` is gone — the status reads `Ready`, and Settings → Project → `Conversation` reads `None yet`.

---

## STORY-014: Prompt history recall is unreachable

**Type**: short
**Topic**: Core conversation / dead feature
**Persona**: Marcus, expecting a shell-like Up arrow
**Goal**: Recall his last prompt
**Preconditions**: At least one prompt has been sent.
**Entry**: Composer, Up arrow
**Window state**: Chat
**Ideal path**: 1 — press Up.
**Alternate paths**: none.

### Steps
1. Send one prompt and let it finish.
2. Click into the empty composer.
3. `key up`.
4. `snapshot` the composer.

### Success condition
This story is expected to FAIL as a feature and PASS as a finding. `RunSession.recallPreviousPrompt()` exists and is documented "Recalled with Up/Down in the composer" (`RunSession.swift:25-26, 163-166`) but nothing calls it — the `Composer` has no key handling. Record the outcome: if the composer text is still empty after `key up`, the documented behaviour is not implemented.

---

## STORY-015: Transcript auto-scroll respects a user who scrolled up

**Type**: medium
**Topic**: Core conversation
**Persona**: Marcus, reading an earlier tool result while the reply streams
**Goal**: Not be yanked to the bottom
**Preconditions**: A conversation long enough to scroll.
**Entry**: Transcript scroll view
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Send `Print the numbers 1 to 150, one per line.`
2. While streaming, scroll the transcript to the top.
3. `snapshot` twice, 3s apart, without touching anything.

### Success condition
The visible first line does not change between the two snapshots — i.e. new tokens did not force a scroll. Note: `pinnedToBottom` is initialised `true` and **never assigned anywhere else** (`ChatView.swift:41`), so the guard is permanently on; this story is expected to expose that the scroll-position tracking is not wired. Record whichever behaviour occurs.

---

## STORY-016: A failed run surfaces the server's reason

**Type**: short
**Topic**: Error handling
**Persona**: Marcus with a bad key
**Goal**: Understand a failure
**Preconditions**: Select a model whose provider has no working credential (see STORY-034), or clear the key first.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0 — the error should be in the transcript.
**Alternate paths**: none.

### Steps
1. Send `Say anything.`
2. `snapshot` every 1s for 60s.

### Success condition
The status reads `Failed` and the transcript contains a red error row whose text is non-empty and includes the server's own message (a provider or auth phrase), plus a copy control. A bare `Failed` with no error row is a defect — the reason is discarded.

---

## STORY-017: Provider substitution is shown as a notice, not an error

**Type**: short
**Topic**: Core conversation
**Persona**: Marcus
**Goal**: Know the run did not use the model he picked
**Preconditions**: A configuration where the server emits `prompt.warning`. Reachable by leaving the model on `Server default` with a fallback provider configured.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Leave the model chip on `Server default`.
2. Send `Reply with NOTICE-CHK-2201.`
3. `snapshot` after completion.

### Success condition
If the server warned, an orange notice row appears containing the warning text, distinct from the red error styling, and the run still reaches `Done`. If no warning was emitted this story is not applicable — record "not exercised", not "passed".

---

## C. Transcript Rendering & Markdown

The block parser is `MarkdownBlock.parse` (`ChatView.swift:245-310`). It handles headings, unordered and ordered lists, block quotes, thematic breaks, and fenced code. It explicitly does **not** handle tables or nested lists. These stories exist to prove each branch renders as a structured block rather than literal punctuation.

## STORY-018: Headings render at their level, not as literal hashes

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus reading a structured answer
**Goal**: Read a document-shaped reply
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send exactly: `Reply with only this markdown, nothing else: "# HEAD-ONE-3301" then a newline then "## HEAD-TWO-3302" then a newline then "### HEAD-THREE-3303".`
2. `snapshot` after `Done`.

### Success condition
The transcript contains three separate text elements reading exactly `HEAD-ONE-3301`, `HEAD-TWO-3302`, `HEAD-THREE-3303`. **No element contains a `#` character.** If the tree exposes font size, the three must differ in size, largest first.

### Edge cases
- `#NoSpace` is not a heading (the parser requires a following space) and must render as literal paragraph text including the `#`.
- Seven or more hashes is not a heading either.

---

## STORY-019: Bulleted and numbered lists render as separate rows with markers

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus
**Goal**: Read a list as a list
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send: `Reply with only this markdown: a bulleted list with the three items LIST-A-4401, LIST-B-4402, LIST-C-4403, then a blank line, then a numbered list with the three items NUM-A-4411, NUM-B-4412, NUM-C-4413.`
2. `snapshot` after `Done`.

### Success condition
Six distinct rows. The three bullet rows each pair a marker element `•` with its item text. The three numbered rows pair markers `1.`, `2.`, `3.` with their item text. No single element contains two items concatenated, and no element contains a literal leading `-` or `1.` fused to the text.

### Edge cases
- Nested list items are flattened by design — a two-level list must still produce one row per item, never a run-together paragraph.

---

## STORY-020: A fenced code block gets its own panel with a language tag and a Copy button

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus, about to paste code
**Goal**: Get the code out cleanly
**Preconditions**: Project ready; clipboard readable out of band.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 1 — one click to copy.
**Alternate paths**: The whole-message copy button (STORY-027) also captures the code, mixed with prose.

### Steps
1. Send: `Reply with only a fenced code block tagged swift containing exactly the single line: let token = "CODE-BLK-5501"`
2. `snapshot` after `Done`.
3. `click Copy` inside the code panel.
4. `snapshot`; read the clipboard out of band.

### Success condition
A panel exists whose header text is `swift` and whose body is the monospaced line containing `CODE-BLK-5501`, with no surrounding backticks. After the click the button label changes to `Copied` and the clipboard contains the code line only — no fence markers, no language tag, no surrounding prose.

### Edge cases
- An untagged fence must show the header `code`, not an empty header.
- A block never closed by the model (still streaming) must still render as code, not as raw text (`ChatView.swift:307`).

---

## STORY-021: Block quotes and horizontal rules render

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus
**Goal**: Read a quoted aside
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send: `Reply with only: a block quote line containing QUOTE-TXT-6601, then a line containing three hyphens, then a plain paragraph containing AFTER-RULE-6602.`
2. `snapshot` after `Done`.

### Success condition
An element reads exactly `QUOTE-TXT-6601` with no leading `>`. A separator/divider element sits between it and an element reading exactly `AFTER-RULE-6602`. The three hyphens must not appear as literal text anywhere.

---

## STORY-022: Inline markdown inside blocks still renders

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus
**Goal**: Read emphasis and inline code
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send: `Reply with only one bulleted list item whose text is: bold INLINE-BOLD-7701 in double asterisks, then inline code INLINE-CODE-7702 in single backticks.`
2. `snapshot` after `Done`.

### Success condition
A single list row exists whose text contains both `INLINE-BOLD-7701` and `INLINE-CODE-7702` and contains **no** `*` or `` ` `` characters. This proves block splitting did not disable inline rendering.

---

## STORY-023: Multi-line paragraphs keep their line breaks

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus
**Goal**: Read an address-style block without it collapsing to one line
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send: `Reply with only three consecutive plain lines with no blank line between them: PARA-L1-8801 then PARA-L2-8802 then PARA-L3-8803.`
2. `snapshot` after `Done`.

### Success condition
The three tokens appear inside a single paragraph element whose text contains newline separation between them — not `PARA-L1-8801 PARA-L2-8802 PARA-L3-8803` run together on one line. (The parser joins with a CommonMark hard break, `ChatView.swift:260`.)

---

## STORY-024: Tables are a known gap

**Type**: short
**Topic**: Transcript rendering / known limitation
**Persona**: Marcus asking for a comparison
**Goal**: Read a table
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Send: `Reply with only a two-column markdown table with a header row Name and Value and one data row TAB-KEY-9901 and TAB-VAL-9902.`
2. `snapshot` after `Done`.

### Success condition
Record the actual rendering. Tables are explicitly out of scope (`ChatView.swift:221`), so the expected outcome is pipe-delimited literal text. The check that matters: the tokens `TAB-KEY-9901` and `TAB-VAL-9902` must both be present and legible, and the pipe rows must not be swallowed or reordered. Report readability as a UX finding, not a crash.

---

## STORY-025: Thinking blocks are collapsed by default and expand

**Type**: short
**Topic**: Transcript rendering
**Persona**: Marcus, curious about reasoning
**Goal**: Read the model's thinking without it dominating the transcript
**Preconditions**: A model that emits `assistant.thinking.delta`.
**Entry**: Transcript
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Send a prompt likely to produce reasoning on a reasoning-capable model.
2. `snapshot` after `Done` → locate a disclosure control labelled `Thinking`.
3. `click Thinking`.
4. `snapshot`.

### Success condition
Before the click the thinking body text is absent from the tree; after the click a monospaced body appears beneath the `Thinking` label. If the model emits no thinking deltas, record "not exercised".

---

## STORY-026: A tool call renders as a row and opens in the inspector

**Type**: medium
**Topic**: Transcript rendering
**Persona**: Marcus, checking what the agent actually did
**Goal**: See a tool's arguments and output
**Preconditions**: Project ready; workspace has a readable file.
**Entry**: Transcript tool row
**Window state**: Chat, both split panes
**Ideal path**: 1 — click the row.
**Alternate paths**: none.

### Steps
1. `snapshot` → assert the right pane reads `Select a tool call to inspect it`.
2. Send `Read the file Package.swift and then reply with exactly TOOLROW-1102.`
3. `snapshot` after `Done` → locate a row whose leading text is `read`.
4. `click` that row.
5. `snapshot` the right pane.

### Success condition
The tool row shows the tool name `read`, a summary equal to the file path (`ToolSummary` prefers `path` / `file_path`, `ChatView.swift:471`), a green checkmark, and a duration in `ms`. After the click the right pane heading is `read`, and it contains sections labelled `Arguments` and `Output`, with `Output` non-empty. The row's background changes to indicate selection.

### Variations
- A failed tool shows a red x-mark; a policy-blocked tool shows an orange raised-hand and status `blocked` — assert blocked is styled differently from failed.

---

## STORY-027: An edit tool renders as a diff, not raw JSON

**Type**: medium
**Topic**: Transcript rendering
**Persona**: Marcus reviewing a change
**Goal**: See exactly what changed
**Preconditions**: Project ready, workspace writable.
**Entry**: Transcript tool row → inspector
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none — there is no other diff surface in the app.

### Steps
1. Send `Create a file named ux-diff-probe.txt containing the single line DIFF-PROBE-1201, then edit it to say DIFF-PROBE-1202 instead. Then reply DONE-1203.`
2. `snapshot` after `Done` → find the tool row named `edit`.
3. `click` it.
4. `snapshot` the inspector.
5. `click` the checkbox labelled `Show unchanged lines` to toggle it off.
6. `snapshot`.

### Success condition
The inspector shows the file path, an addition count prefixed `+` and a deletion count prefixed `−`, and per-line rows with old/new line-number gutters. A row exists whose marker is `+` and whose text contains `DIFF-PROBE-1202`, and a row whose marker is `−` and whose text contains `DIFF-PROBE-1201`. After step 5, rows with a blank marker are gone while the `+`/`−` rows remain. The section labelled `Arguments` must **not** be shown for this tool (the diff replaces it).

---

## D. Copy Controls

## STORY-028: Copy a single assistant message

**Type**: short
**Topic**: Copy controls
**Persona**: Marcus, pasting an answer into a ticket
**Goal**: Get one reply onto the clipboard whole
**Preconditions**: A completed reply spanning at least a heading, a paragraph, and a code block.
**Entry**: The copy control at the bottom-right of the assistant bubble
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: Drag-select — which cannot span separate `Text` views, the documented reason this button exists (`ChatView.swift:126-128`).

### Steps
1. Produce a multi-block reply (reuse STORY-020's prompt plus a heading and paragraph, all containing token `COPYMSG-1301`).
2. `snapshot` → find the control whose accessibility label is `Copy message` within that bubble.
3. `click` it.
4. `snapshot` within 1s; read the clipboard out of band.

### Success condition
The clipboard contains the full raw markdown of that message including `COPYMSG-1301` and the fenced code, and nothing from any other message. The button's icon switches to a checkmark and its help text reads `Copied`, reverting after ~1.6s.

### Edge cases
- A still-streaming message must expose **no** copy button (`ChatView.swift:202`) — assert its absence mid-stream and its presence after `Done`.

---

## STORY-029: Copy a user prompt

**Type**: short
**Topic**: Copy controls
**Persona**: Marcus, re-using a prompt elsewhere
**Goal**: Recover what he asked
**Preconditions**: At least one user bubble.
**Entry**: Copy control beneath the user bubble
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Send `Remember the token USERCOPY-1401.`
2. `snapshot` → find the `Copy message` control attached to the user bubble (right-aligned).
3. `click` it; read the clipboard.

### Success condition
The clipboard equals the prompt text exactly, with no `You: ` prefix (that prefix belongs only to the whole-transcript flattener).

---

## STORY-030: Copy the whole conversation

**Type**: medium
**Topic**: Copy controls
**Persona**: Marcus, filing a bug report
**Goal**: Paste the entire session into an issue
**Preconditions**: A conversation containing a user prompt, an assistant reply, and at least one tool call.
**Entry**: Copy control in the status bar, help text `Copy the whole conversation`
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: Sessions → context menu → `Export Transcript…` writes JSONL to a file — a different format for the same goal, worth flagging as a duplicate path with divergent output.

### Steps
1. Build a conversation containing token `WHOLECOPY-1501` in the prompt and a `read` tool call.
2. `snapshot` the status bar → find the copy control (present only when the transcript is non-empty).
3. `click` it; read the clipboard.

### Success condition
The clipboard contains a line beginning `You: ` followed by the prompt including `WHOLECOPY-1501`, the assistant reply text, and a line of the form `[read] <path>`. Entries are separated by blank lines. Nothing is truncated to the last message only.

### Edge cases
- On an empty transcript the control must be absent entirely, not present-and-disabled.

---

## STORY-031: Copy an error message

**Type**: short
**Topic**: Copy controls
**Persona**: Marcus, reporting a failure
**Goal**: Paste the exact error
**Preconditions**: A failed run (reuse STORY-016).
**Entry**: Copy control inside the red error row
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: whole-conversation copy (STORY-030) also carries it, prefixed `Error: `.

### Steps
1. Reproduce a failed run.
2. `snapshot` → find the `Copy message` control inside the error row.
3. `click` it; read the clipboard.

### Success condition
The clipboard equals the error text verbatim, with no `Error: ` prefix.

---

## E. Model Selection

## STORY-032: Switch model from the composer chip

**Type**: short
**Topic**: Model selection
**Persona**: Dana, switching to a cheaper model
**Goal**: Change which model the next run uses
**Preconditions**: At least two providers configured with exposed models.
**Entry**: The chip in the composer footer, labelled with the current model or `Server default`
**Window state**: Chat
**Ideal path**: 3 — open menu, open provider submenu, pick model.
**Alternate paths**: Settings → Models list rows are tappable and set `project.selectedModel` too (`SettingsView.swift:136`) — but that code path (`ModelsTab`) is **not wired into the tab switch**; `.models` renders `ModelSettingsTab` instead. Verify whether the alternate path exists at runtime; if it does not, `ModelsTab` is dead code.

### Steps
1. `snapshot` → find the chip labelled `Server default`.
2. `click` it.
3. `snapshot` → assert a `Server default` item, a divider, and one submenu per usable provider.
4. `click` a provider submenu, then `click` a model entry; note its exact id.
5. `snapshot` the composer footer.

### Success condition
The chip label becomes exactly the chosen model id. Settings → Project → `Model` reads the same id. Sending a prompt afterwards produces a run that does not fall back (`allowFallback` is set false whenever a model is chosen by hand, `RunSession.swift:71`), so a mismatched provider must fail loudly rather than silently substituting.

---

## STORY-033: Model menu entries carry a price

**Type**: short
**Topic**: Model selection
**Persona**: Dana, choosing on cost
**Goal**: Compare price without leaving the menu
**Preconditions**: At least one model with pricing.
**Entry**: Model chip
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: Settings → Models shows the same price per row — the same fact on two surfaces.

### Steps
1. `click` the model chip, open a provider submenu.
2. `snapshot`.

### Success condition
At least one entry reads `<model id> — <price summary>` rather than the bare id. A model with no price shows the bare id (never `— $0`).

---

## STORY-034: Models from unusable providers are hidden, and the hiding is explained

**Type**: medium
**Topic**: Model selection
**Persona**: Dana, whose key expired
**Goal**: Understand why her model vanished
**Preconditions**: One provider configured with a broken/absent credential, one healthy.
**Entry**: Model chip
**Window state**: Chat
**Ideal path**: 1 — the explanation should be visible when the menu opens.
**Alternate paths**: Settings → Providers shows per-provider configured state; Settings → Models shows `no working credential`. Three surfaces state the same fact.

### Steps
1. `click` the model chip.
2. `snapshot` and read every item, including disabled ones.

### Success condition
No submenu exists for the unusable provider. A disabled caption line at the bottom matches either `<n> models hidden — <provider> needs re-authentication` (when `health == "failed"`) or `<n> models hidden — no credentials for their providers`. `<n>` must equal the number of models belonging to unusable providers. A menu that silently omits them with no caption is the specific failure this guard exists to prevent.

### Edge cases
- If the provider list itself failed to load, the menu must **fail open** and list every provider rather than showing only `Server default` (`ChatView.swift:808-811`). Force this by killing network access to the catalog endpoints and assert the menu is not empty.

---

## STORY-035: Revert to the server default

**Type**: short
**Topic**: Model selection
**Persona**: Dana
**Goal**: Undo a model choice
**Preconditions**: A model is currently selected.
**Entry**: Model chip → `Server default`
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. `click` the chip, `click Server default`.
2. `snapshot` the footer and Settings → Project.

### Success condition
The chip reads `Server default` and Settings → Project → `Model` reads `Server default`. Subsequent runs re-enable provider fallback.

---

## STORY-036: The model choice survives switching sections

**Type**: short
**Topic**: Model selection
**Persona**: Dana
**Goal**: Not lose her selection by looking at Settings
**Preconditions**: A model selected.
**Entry**: Icon rail
**Window state**: Chat → Settings → Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Select a model (STORY-032).
2. Click the rail buttons Settings, then Activity, then Chat, snapshotting through each blackout.
3. `snapshot` the composer footer.

### Success condition
The chip still reads the chosen model id. `selectedModel` lives on `ProjectSession`, which outlives section changes — a reset here would be a state-ownership bug.

---

## F. Settings — Providers

## STORY-037: Read provider status at a glance

**Type**: short
**Topic**: Settings / providers
**Persona**: Dana, auditing what is set up
**Goal**: See which providers are usable
**Preconditions**: Project ready.
**Entry**: Rail → Settings → `Providers` tab (the default tab)
**Window state**: Settings section
**Ideal path**: 1.
**Alternate paths**: Settings → Models shows per-provider credential state too.

### Steps
1. Click the Settings rail button; `snapshot` through the blackout.
2. `snapshot` the list.

### Success condition
The segmented control shows `Providers` selected. Each row shows the provider name, a green seal icon when configured or a grey exclamation when not, and a `<n> models` caption where the count is known. Each unconfigured row with a known env var shows `Reads <ENV_NAME>, or set a key here.` Each row offers either `Set Key…` or `Import Login`, never both.

---

## STORY-038: Set an API key for a provider

**Type**: medium
**Topic**: Settings / providers
**Persona**: Dana, adding a key
**Goal**: Make a provider usable without restarting anything
**Preconditions**: An unconfigured provider with `Set Key…`; a valid key available.
**Entry**: Settings → Providers → `Set Key…`
**Window state**: Settings
**Ideal path**: 4 — open Settings, click Set Key, type, save.
**Alternate paths**: Settings → Models → Add Provider sheet also takes a key, but only when creating a provider (STORY-045).

### Steps
1. `snapshot` → click the `Set Key…` on the target row.
2. `snapshot` → assert an inline secure field labelled `API key` appeared **on that row**, and that the button now reads `Cancel`.
3. `type <field ref> <key>`.
4. `click Save`.
5. `snapshot` after ~2s.

### Success condition
A toast appears reading exactly `Saved key for <provider>`. The row's icon flips to the green seal and the model count becomes non-zero or the row becomes `configured`. The typed characters must never appear in plain text in the AX tree — a `SecureField` should expose no value. If the key is readable from the tree, that is a security finding.

### Edge cases
- `Save` must be disabled while the field is empty.
- Clicking `Cancel` must discard the draft — reopening shows an empty field, never the previous text.

---

## STORY-039: Import a subscription login

**Type**: short
**Topic**: Settings / providers
**Persona**: Dana with a Codex or Kimi CLI login on this machine
**Goal**: Use her subscription without pasting a key
**Preconditions**: A provider whose row shows `Import Login`.
**Entry**: Settings → Providers → `Import Login`
**Window state**: Settings
**Ideal path**: 2.
**Alternate paths**: none in-app.

### Steps
1. `snapshot` → find the row offering `Import Login`; read its help text.
2. `click Import Login`.
3. `snapshot` after ~3s.

### Success condition
On success a toast reads exactly `Imported <provider> credential` and the row becomes configured. On failure the toast contains the server's message **followed by** ` — log in with the vendor CLI on the machine running harnessd.` The help text on the button must read `Reads the vendor CLI credential from the machine running the harness server`. A bare failure with no host explanation is the defect this wording exists to prevent.

---

## STORY-040: Provider changes propagate to the model picker without relaunch

**Type**: medium
**Topic**: Settings / providers
**Persona**: Dana
**Goal**: Use the model she just enabled
**Preconditions**: STORY-038 completed for a provider that was previously hidden.
**Entry**: Settings → back to Chat
**Window state**: Settings → Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Note the `<n> models hidden` caption in the model chip menu before setting a key.
2. Set the key (STORY-038).
3. Click the Chat rail button; `snapshot` through the blackout.
4. `click` the model chip; `snapshot`.

### Success condition
A submenu for the newly-configured provider now exists, and the hidden-count caption has decreased by that provider's model count or vanished entirely. No relaunch was required.

---

## STORY-041: A toast does not become permanent

**Type**: short
**Topic**: Settings / providers
**Persona**: Dana
**Goal**: Not be left with a stale banner
**Preconditions**: A toast has just appeared.
**Entry**: Settings → Providers
**Window state**: Settings
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Trigger a toast (STORY-038).
2. `snapshot`, then wait 30s, then `snapshot` again.

### Success condition
Record whether the toast is still present. `project.statusMessage` is only ever overwritten, never cleared (`ProjectSession.swift` — no assignment to `nil` outside `shutdown`), and it is **also rendered in the chat status bar** (`ChatView.swift:522`). Expected finding: the last status message persists indefinitely in both places. Confirm by returning to Chat and asserting whether the message text follows.

---

## G. Settings — Models

## STORY-042: The Models tab lists providers with exposure counts

**Type**: short
**Topic**: Settings / models
**Persona**: Dana, curating the picker
**Goal**: See how many models each provider offers versus exposes
**Preconditions**: Project ready.
**Entry**: Settings → `Models`
**Window state**: Settings, two-pane split
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Click the Settings rail button, then `click Models`.
2. `snapshot` through the blackout (this tab loads asynchronously — retry up to 3 times).

### Success condition
The left pane is headed `Providers` and each row shows the provider name, its base URL, and a monospaced `<exposed>/<fetched>` count. Providers not built in carry a `custom` badge. The right pane shows either the selected provider's header or `No provider selected`.

---

## STORY-043: Fetch a provider's real model list

**Type**: medium
**Topic**: Settings / models
**Persona**: Dana, adding a provider that has hundreds of models
**Goal**: Pull the live list
**Preconditions**: A provider with a working credential and zero fetched models.
**Entry**: Settings → Models → `Fetch Models`
**Window state**: Settings
**Ideal path**: 3.
**Alternate paths**: The same `Fetch Models` button appears twice — in the empty-state `No models yet` panel and in the provider header. Two controls, one action.

### Steps
1. Select the provider in the left pane.
2. `snapshot` → assert either `No models yet` with its explanatory text, or the header's `Fetch Models`.
3. `click Fetch Models`.
4. `snapshot` every 2s for up to 60s.

### Success condition
A status bar at the bottom reads exactly `Fetched <n> models from <provider>.` with `<n>` matching the new denominator in the left pane's `<exposed>/<fetched>` count. The header's `Never fetched` becomes `Fetched <timestamp>`. While in flight, a progress indicator is visible and both `Fetch Models` controls are disabled.

### Edge cases
- On failure the status must read `Fetch failed: <the provider's own message>` verbatim, and the left-pane row must show an orange warning whose text ends `(last tried <timestamp>)` — an unlabelled stale error is the specific bug that wording prevents (`ModelSettingsView.swift:192-195`).

---

## STORY-044: Expose one model and see it reach the picker

**Type**: medium
**Topic**: Settings / models
**Persona**: Dana
**Goal**: Add exactly one model to the composer menu
**Preconditions**: A provider with fetched models, at least one unexposed.
**Entry**: Settings → Models → per-row toggle
**Window state**: Settings → Chat
**Ideal path**: 4.
**Alternate paths**: `Expose All` (STORY-045).

### Steps
1. Note the provider's `<exposed>/<fetched>` count and the model ids currently in the composer chip menu.
2. `type <filter field ref>` a substring of one unexposed model's id.
3. `snapshot` → assert the list narrowed to matching rows only.
4. `click` that row's toggle (help text `Show this model in the picker`).
5. `snapshot` → assert the count incremented by one.
6. Click the Chat rail button, `click` the model chip, `snapshot`.

### Success condition
The exposed model id now appears in the chip menu under its provider, and the exposed count is exactly one higher. A toggle that flips visually but leaves the count unchanged means the save round-trip failed silently — report it.

---

## STORY-045: Bulk expose and un-expose respect the current filter

**Type**: medium
**Topic**: Settings / models
**Persona**: Dana, curating a 300-model provider
**Goal**: Expose only the models matching a family name
**Preconditions**: A provider with many fetched models.
**Entry**: Settings → Models → `Expose All` / `Expose None`
**Window state**: Settings
**Ideal path**: 3.
**Alternate paths**: per-row toggles, one at a time.

### Steps
1. `click Expose None`; `snapshot` → the exposed count reads `0/<n>`.
2. `type <filter ref>` a family substring matching a known subset; note the visible row count `k`.
3. `click Expose All`.
4. `snapshot`.
5. Clear the filter; `snapshot`.

### Success condition
After step 3 the count reads exactly `k/<n>` — **not** `<n>/<n>`. `setAllVisible` operates on `visibleModels`, so the filter is load-bearing; if it exposes everything the filter is being ignored, which is a destructive-at-scale bug.

---

## STORY-046: Set a price for a model with unknown cost

**Type**: medium
**Topic**: Settings / models
**Persona**: Dana, whose gateway reports no pricing
**Goal**: Get real cost estimates in the status bar
**Preconditions**: A fetched model whose price summary renders in orange (unknown).
**Entry**: Settings → Models → click the price cell
**Window state**: Settings
**Ideal path**: 4.
**Alternate paths**: none — this is the only price-entry surface.

### Steps
1. `snapshot` → find a row whose price cell is styled as unknown; `click` it.
2. `snapshot` → assert two fields with placeholders `in` and `out`, plus `Save` and `Cancel`.
3. `type` `3.0` into `in` and `15.0` into `out`.
4. `click Save`; `snapshot`.

### Success condition
The cell leaves edit mode, shows a non-orange price summary derived from the entered figures, and gains a pencil indicator whose help text reads `Price you entered`. Then run a prompt on that model and assert the chat status bar shows `<n> tok · $<amount>` rather than `cost n/a`.

### Edge cases
- Entering non-numeric text must set the status to exactly `Costs must be numbers, in dollars per million tokens.` and must not leave edit mode.

---

## STORY-047: Add a custom OpenAI-compatible provider

**Type**: long
**Topic**: Settings / models
**Persona**: Ravi, pointing the app at a local llama server
**Goal**: Use a model the built-in catalog has never heard of
**Preconditions**: A reachable OpenAI-compatible endpoint (a local server on `http://127.0.0.1:<port>/v1` is ideal because it needs no key).
**Entry**: Settings → Models → the `+` control, help text `Add any OpenAI-compatible endpoint`
**Window state**: Settings, then a sheet
**Ideal path**: 6.
**Alternate paths**: none.

### Steps
1. `snapshot` → find the add control in the left pane header; `click` it.
2. `snapshot` → a sheet headed `Add Provider` with fields `Name`, `Endpoint`, pickers `Protocol` and `Auth`, and buttons `Cancel` / `Add`.
3. `type` `local-llama` into `Name` and the base URL into `Endpoint`.
4. `click` the `Auth` picker and choose `None (local server)`.
5. `snapshot` → assert the `API key` field and its Keychain caption have disappeared.
6. `click Add`.
7. `snapshot` after ~3s.
8. Select `local-llama` in the left pane and `click Fetch Models`.
9. Expose one model (STORY-044) and switch to it in the composer chip.
10. Send `Reply with exactly LOCALPROV-1601.`

### Success condition
Step 7: the sheet dismisses, a status reads exactly `Saved local-llama.`, and the left pane contains a row `local-llama` with a `custom` badge and header caption `no credential needed`. Step 8: a fetch count appears. Step 10: the transcript contains `LOCALPROV-1601`. This is the full add-a-provider loop and any broken link in it invalidates the feature.

### Edge cases
- `Add` must stay disabled while `Name` or `Endpoint` is blank or whitespace.
- Choosing `API key` auth must show the caption `Stored in your macOS Keychain. It is never written to the settings file.`

---

## STORY-048: Remove a custom provider

**Type**: short
**Topic**: Settings / models — destructive
**Persona**: Ravi, cleaning up
**Goal**: Delete a provider he no longer uses
**Preconditions**: STORY-047's `local-llama` exists with at least one exposed model.
**Entry**: Settings → Models → provider header → `Remove`
**Window state**: Settings
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Select `local-llama`; `snapshot` → assert a `Remove` button exists (built-in providers must not offer one).
2. `click Remove`.
3. `snapshot` after ~2s.
4. Click the Chat rail button and open the model chip.

### Success condition
The provider row is gone from the left pane, the right pane falls back to another provider or to `No provider selected`, and the chip menu no longer contains any `local-llama` model. A picker still offering a deleted provider's models would fail every run — that is why `delete` triggers a catalog refresh.

### Edge cases
- Removal is immediate with **no confirmation dialog**. Flag this: every other destructive action in the app (rewind, delete conversation) confirms or is marked `role: .destructive` with a dialog. Inconsistent destructive-action treatment is a UX finding.

---

## H. Settings — Project

## STORY-049: Read the project's current configuration

**Type**: short
**Topic**: Settings / project
**Persona**: Dana, verifying what a run will use
**Goal**: Confirm workspace, model, profile, plan mode, and conversation id in one place
**Preconditions**: Project ready, at least one run completed.
**Entry**: Settings → `Project`
**Window state**: Settings
**Ideal path**: 2.
**Alternate paths**: Model is also on the composer chip; plan mode is also a composer toggle; conversation id appears nowhere else. Two of the five facts are duplicated.

### Steps
1. Click the Settings rail button, `click Project`, `snapshot` through the blackout.

### Success condition
A grouped form contains labelled rows `Workspace` (the absolute path, selectable and monospaced), `Model`, `Profile`, `Plan mode` (`On` or `Off`), `Conversation` (a monospaced id or `None yet`), and `Conversation actions` with buttons `Fork` and `Undo Last Prompt`. The `Workspace` path must equal the folder actually opened.

---

## STORY-050: Select a profile and confirm it reaches the run

**Type**: medium
**Topic**: Settings / project
**Persona**: Dana, restricting the agent's tools
**Goal**: Run under a named profile
**Preconditions**: The daemon exposes at least one profile via `/v1/profiles`.
**Entry**: Settings → Project → `Profile` picker
**Window state**: Settings → Chat
**Ideal path**: 3.
**Alternate paths**: none.

### Steps
1. `click` the `Profile` picker; `snapshot` → assert a `None` option plus one entry per profile.
2. Choose a profile.
3. Return to Chat and send `List the tools you have available, then reply PROFILE-CHK-1701.`
4. `snapshot` after `Done`.

### Success condition
The picker retains the chosen name across a section switch, and the run completes with token `PROFILE-CHK-1701`. If the daemon exposes no profiles the picker must still show `None` and not be empty or crash — record "not exercised" for the selection half.

### Edge cases
- The UI gives no description of what a profile does. Flag as a discoverability finding: the user chooses a name with no stated effect.

---

## STORY-051: Fork the current conversation

**Type**: medium
**Topic**: Settings / project
**Persona**: Dana, branching an exploration
**Goal**: Keep the current history but continue separately
**Preconditions**: A conversation with at least two exchanges.
**Entry**: Settings → Project → `Fork`
**Window state**: Settings
**Ideal path**: 3.
**Alternate paths**: none.

### Steps
1. Note the current `Conversation` id.
2. `click Fork`.
3. `snapshot` after ~2s.
4. Click the Sessions rail button; `snapshot`.

### Success condition
A status message reads exactly `Forked into a new conversation`, the `Conversation` field now shows a **different** id, and the Sessions list contains both the original and the fork. Critically: after forking, the on-screen transcript is **not** reloaded (`fork()` calls `rebind` only, `ProjectSession.swift:223-233`) — assert whether the visible transcript still shows the pre-fork content and report any mismatch between what is displayed and which conversation the next prompt will join.

---

## STORY-052: Undo the last prompt

**Type**: medium
**Topic**: Settings / project — destructive
**Persona**: Dana, who asked something wrong
**Goal**: Remove her last turn from history
**Preconditions**: A conversation with at least two user prompts, the last containing token `UNDO-ME-1801`.
**Entry**: Settings → Project → `Undo Last Prompt`
**Window state**: Settings → Chat
**Ideal path**: 3.
**Alternate paths**: Checkpoints → Restore also truncates history, plus restores files — overlapping capability worth flagging.

### Steps
1. Send two prompts, the second containing `UNDO-ME-1801`; wait for `Done`.
2. Settings → Project → `click Undo Last Prompt`.
3. Return to Chat; `snapshot`.

### Success condition
The transcript no longer contains `UNDO-ME-1801` but still contains the first prompt's content — `undo()` reloads the conversation from the server (`ProjectSession.swift:239`), so the visible transcript must actually change, not just the server state.

### Edge cases
- With no conversation yet, the button must be a no-op, not an error toast.
- **No confirmation dialog** for a history-destroying action — flag the inconsistency with Checkpoints, which does confirm.

---

## STORY-053: Copy the workspace path

**Type**: short
**Topic**: Settings / project
**Persona**: Dana, opening the folder in a terminal
**Goal**: Get the path onto the clipboard
**Preconditions**: Project ready.
**Entry**: Settings → Project → `Workspace` value
**Window state**: Settings
**Ideal path**: 2 — select and ⌘C.
**Alternate paths**: none — there is no copy button and no "Reveal in Finder".

### Steps
1. Click into the `Workspace` value text, select all, `key cmd+c`.
2. Read the clipboard.

### Success condition
The clipboard equals the absolute workspace path. If text selection cannot be driven, record that the path is display-only with no affordance to extract it — a small but real friction finding.

---

## I. Settings — Access

## STORY-054: Empty access state explains the confinement

**Type**: short
**Topic**: Settings / access
**Persona**: Ravi, wondering why the agent cannot read a sibling repo
**Goal**: Understand the boundary
**Preconditions**: Project ready, no extra directories added.
**Entry**: Settings → `Access`
**Window state**: Settings
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Settings → `click Access`; `snapshot` through the blackout.

### Success condition
The pane contains the heading `Extra directories`, an `Add…` button, the explanatory line `Runs can read and write inside the workspace. Add a directory to grant access beyond it for this session.`, and an empty state titled `No extra directories` whose detail reads `The agent is limited to <workspace folder name>.` with the real folder name interpolated.

---

## STORY-055: Grant access to a second directory and use it

**Type**: long
**Topic**: Settings / access
**Persona**: Ravi, needing the agent to read a shared library repo
**Goal**: Let one run reach outside the workspace
**Preconditions**: A second directory exists outside the workspace containing a file with the line `EXTRADIR-1901`.
**Entry**: Settings → Access → `Add…`
**Window state**: Settings, then a system open panel, then Chat
**Ideal path**: 4.
**Alternate paths**: none.

### Steps
1. Settings → Access → `click Add…`.
2. Drive the open panel (prompt button reads `Grant Access`) to the second directory and confirm.
3. `snapshot` → assert a list row showing the directory's absolute path with a `Remove` button.
4. Go to Chat and send `Read the file <abs path to the probe file> and reply with the token you find.`
5. `snapshot` after `Done`.

### Success condition
Step 3 shows the path; step 5's transcript contains `EXTRADIR-1901`. Then remove the directory (STORY-056), start a **new** run with the same prompt, and assert it now fails or is blocked — proving the grant is actually applied per-run (`ProjectSession.submit` sets `run.extraDirs`, `ProjectSession.swift:186`) rather than being decorative.

### Edge cases
- Adding the same directory twice must produce one row, not two (`addDirectory` de-duplicates).
- The grant is session-scoped: after closing and reopening the project, the list must be empty again.

---

## STORY-056: Revoke a directory

**Type**: short
**Topic**: Settings / access — destructive
**Persona**: Ravi
**Goal**: Take the grant back
**Preconditions**: STORY-055 added a directory.
**Entry**: Settings → Access → `Remove`
**Window state**: Settings
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. `snapshot` → find the row's `Remove` button; `click` it.
2. `snapshot`.

### Success condition
The row disappears and, if it was the only one, the `No extra directories` empty state returns. No confirmation is shown — acceptable here since the action is non-destructive and reversible, unlike STORY-048.

---

## STORY-057: Access grants do not leak between projects

**Type**: medium
**Topic**: Settings / access
**Persona**: Ravi, working in two repos
**Goal**: Keep grants scoped
**Preconditions**: Two windows, two projects (see STORY-119).
**Entry**: Settings → Access in each window
**Window state**: Two main windows
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Open project A in window 1 and project B in window 2 (⌘N, then Choose Folder…).
2. Add a directory in window 1's Access tab.
3. `window` to window 2 and open its Access tab; `snapshot`.

### Success condition
Window 2's Access list is empty. `extraDirs` lives on `ProjectSession`, one per window — a shared list would be a cross-project confinement leak and a security finding.

---

## J. Plan Mode

## STORY-058: Turn plan mode on and see the status change

**Type**: short
**Topic**: Plan mode
**Persona**: Sam, who wants a plan before any edits
**Goal**: Restrict the agent to planning
**Preconditions**: Project ready, idle.
**Entry**: The `Plan mode` checkbox in the composer footer
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none — no menu item and no shortcut. (The TUI has Ctrl+O; the Mac app has no equivalent.)

### Steps
1. `snapshot` → assert the status reads `Ready` and a checkbox labelled `Plan mode` is unchecked.
2. `click Plan mode`.
3. `snapshot`.
4. Click the Settings rail button, `click Project`, `snapshot`.

### Success condition
The status text becomes exactly `Plan mode — ready`, the checkbox reads checked, its help text is `Restrict the agent to writing a plan file until you approve it`, and Settings → Project → `Plan mode` reads `On`.

---

## STORY-059: A plan-mode run is prevented from mutating anything but the plan file

**Type**: long
**Topic**: Plan mode
**Persona**: Sam
**Goal**: Prove the restriction is real
**Preconditions**: Plan mode on; workspace writable.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. With plan mode on, send `Create a file named plan-probe.txt containing PLANMODE-2001. Then write your plan to the plan file.`
2. `snapshot` every 2s for up to 180s.
3. Inspect any tool rows and their inspector output.

### Success condition
At least one tool row exists with the orange raised-hand `blocked` status for the attempted write to `plan-probe.txt`, **or** a write tool row targeting `.harness/plan.md` succeeds while the `plan-probe.txt` write does not. `plan-probe.txt` must not exist on disk afterwards. A run that creates the file means plan mode is not enforced — a correctness failure, not a UX one.

---

## STORY-060: Approve a plan and choose an approach

**Type**: long
**Topic**: Plan mode
**Persona**: Sam, ready to let the agent build
**Goal**: Leave plan mode with a chosen approach
**Preconditions**: Plan mode on; the agent has produced a plan and requested exit.
**Entry**: The plan approval panel above the composer
**Window state**: Chat
**Ideal path**: 3 — read plan, pick approach, approve.
**Alternate paths**: none.

### Steps
1. With plan mode on, send `Plan how to add a README to this repository. Offer two approaches. Then ask to exit plan mode.`
2. `snapshot` every 2s until a panel appears.
3. Read the panel; `snapshot`.
4. `click` an approach radio row.
5. `click Approve`.
6. `snapshot` every 2s for 120s.

### Success condition
Step 3 shows the label `Ready to leave plan mode`, a scrollable plan body containing the plan text, a caption `Approach`, one selectable row per approach with label and description, and buttons `Keep Planning` and `Approve`. Before step 4, `Approve` is **disabled** (approaches were offered, so one must be chosen). After step 5 the panel disappears, the status leaves `Waiting for you`, and the run continues to `Done`. The composer must be replaced by this panel while it is up — a composer that stays usable during approval means the modal state is not enforced.

---

## STORY-061: Keep planning instead of approving

**Type**: medium
**Topic**: Plan mode
**Persona**: Sam, unconvinced by the plan
**Goal**: Send the agent back to planning
**Preconditions**: A plan approval panel is up.
**Entry**: `Keep Planning`
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Reach the panel (STORY-060 steps 1-2).
2. `click Keep Planning`.
3. `snapshot` every 2s for 60s.

### Success condition
The panel disappears, the run does not terminate as `Failed`, and plan mode remains on — the `Plan mode` checkbox is still checked and the status returns to a planning-capable state. If the run dies on denial, the deny path is mishandled.

---

## STORY-062: Plan mode persists across a new conversation

**Type**: short
**Topic**: Plan mode
**Persona**: Sam
**Goal**: Not silently lose the restriction
**Preconditions**: Plan mode on.
**Entry**: Composer footer `New`
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Turn plan mode on.
2. `click New` in the composer footer.
3. `snapshot`.

### Success condition
`planMode` lives on `ProjectSession` and `newConversation()` does not reset it — so the status should still read `Plan mode — ready`. Assert this. If it silently reverts to `Ready`, the user's safety setting was dropped by an unrelated action, which is a significant finding.

---

## K. Approvals & Agent Questions

## STORY-063: Approve a tool call

**Type**: medium
**Topic**: Approvals
**Persona**: Sam, supervising a risky command
**Goal**: Let a specific command through
**Preconditions**: A permission configuration where a `bash` call requires approval.
**Entry**: The approval bar above the composer
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Send `Run the shell command: echo APPROVE-ME-2101`
2. `snapshot` every 1s until the bar appears.
3. `snapshot` → read the bar.
4. `click Details`; `snapshot`.
5. `click Allow`.
6. `snapshot` every 1s for 60s.

### Success condition
Step 3 shows text of the form `Allow <tool> to run?` with the tool name emphasised, plus `Details`, `Deny`, and a prominent `Allow`. Step 4 reveals a scrollable monospaced argument body containing `APPROVE-ME-2101`, and the button flips to `Hide`. After step 5 the bar disappears, the status leaves `Waiting for you`, the corresponding tool row completes, and the transcript contains `APPROVE-ME-2101`.

### Edge cases
- **Nothing may be bound to Return** while the bar is up (`ChatView.swift:591-592`) — assert `key return` does not approve. An accidental Enter approving a shell command is the exact hazard this design avoids.
- While an approval is pending, `canSteer` is false, so the composer must read `Ask the harness to do something…`, not the steer placeholder.

---

## STORY-064: Deny a tool call

**Type**: medium
**Topic**: Approvals
**Persona**: Sam, seeing a command he does not want
**Goal**: Block it without killing the run
**Preconditions**: An approval bar is up.
**Entry**: `Deny`
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: `Stop` also prevents the command, by killing the run — a blunter path to the same protection.

### Steps
1. Reach an approval bar for a command creating file `deny-probe.txt`.
2. `click Deny`.
3. `snapshot` every 1s for 60s.
4. Check the filesystem.

### Success condition
The bar disappears, `deny-probe.txt` does not exist, and the run continues rather than terminating — the agent should report the denial. A denial that silently ends the run with no explanation in the transcript is a finding.

---

## STORY-065: Answer a structured question from the agent

**Type**: long
**Topic**: Agent questions
**Persona**: Sam
**Goal**: Unblock a run that needs a decision
**Preconditions**: `ask_user_question` is a core tool and always available.
**Entry**: The question panel above the composer
**Window state**: Chat
**Ideal path**: 2 — choose, send.
**Alternate paths**: none.

### Steps
1. Send `Use the ask_user_question tool to ask me one multiple-choice question with the options ALPHA and BETA. After I answer, reply with exactly ANSWERED- followed by my choice.`
2. `snapshot` every 1s until a panel appears.
3. `snapshot` → read the panel.
4. `click` the option row labelled `BETA`.
5. `snapshot` → check the `Send` button state.
6. `click Send`.
7. `snapshot` every 1s for 60s.

### Success condition
Step 3 shows the label `The agent needs your input`, the question text, and two selectable option rows. Step 5: `Send` was **disabled** before any option was chosen and is enabled after. Step 7: the panel disappears and the transcript contains exactly `ANSWERED-BETA`. The status must have read `Waiting for you` while the panel was up.

### Variations
- A freeform question renders a text field with placeholder `Your answer` instead of radio rows; `Send` stays disabled until it is non-empty by count. Exercise this with a second prompt asking for a freeform question.
- If the question carries a deadline, a caption `Answer by <time>` appears. Record whether the deadline is enforced or purely decorative.

---

## STORY-066: Multiple questions must all be answered before Send

**Type**: medium
**Topic**: Agent questions
**Persona**: Sam
**Goal**: Not submit a half-answered form
**Preconditions**: As STORY-065.
**Entry**: Question panel
**Window state**: Chat
**Ideal path**: 3.
**Alternate paths**: none.

### Steps
1. Send `Use ask_user_question to ask me two separate multiple-choice questions in one call. Then reply with exactly BOTH-ANSWERED-2201.`
2. Wait for the panel; answer only the first question.
3. `snapshot` → check `Send`.
4. Answer the second; `click Send`.

### Success condition
`Send` is disabled after step 2 and enabled after step 4 (`answers.count < prompt.questions.count`, `ChatView.swift:677`). The transcript ends with `BOTH-ANSWERED-2201`.

---

## STORY-067: An approval that arrives while the user is in another section

**Type**: medium
**Topic**: Approvals / notification gap
**Persona**: Sam, who wandered into Settings while a run was going
**Goal**: Notice the run needs him
**Preconditions**: A run that will request approval.
**Entry**: Icon rail
**Window state**: Settings, then Chat
**Ideal path**: 1 — ideally the rail would badge the Chat icon.
**Alternate paths**: none.

### Steps
1. Start a run that will hit an approval.
2. Immediately click the Settings rail button.
3. Wait 30s. `snapshot` the whole window including the rail and toolbar.
4. Return to Chat; `snapshot`.

### Success condition
Record whether **anything** outside the Chat section indicates the run is blocked — a rail badge, a toolbar indicator, a notification. Expected finding: nothing does. The Chat section is unmounted while another section is showing, so the approval is invisible until the user happens to return. Report as a discoverability gap; step 4 must still show the pending bar (the run state survives the unmount because it lives on `RunSession`).

---

## L. Sessions & Conversation History

## STORY-068: Empty sessions state explains optional persistence

**Type**: short
**Topic**: Sessions
**Persona**: Priya, first run
**Goal**: Understand why there is no history
**Preconditions**: A brand-new workspace with no `.harness/conversations.db` content.
**Entry**: Rail → Sessions
**Window state**: Sessions
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Click the Sessions rail button; `snapshot` through the blackout.

### Success condition
The pane shows the title `No saved conversations` and the detail `Conversations appear here once the server has a conversation store configured.` Note: the app *always* configures a store (`HarnessSupervisor.swift:62-68`), so this state should only occur before the first conversation — if it appears after a completed run, persistence is broken.

---

## STORY-069: A completed conversation appears in the list with metadata

**Type**: medium
**Topic**: Sessions
**Persona**: Priya
**Goal**: Find yesterday's work
**Preconditions**: At least one completed conversation.
**Entry**: Rail → Sessions
**Window state**: Sessions
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Complete a run containing token `SESSION-2301`.
2. Click the Sessions rail button; `snapshot` (the list refreshes on `.task`; retry up to 3 times).

### Success condition
A row exists showing a title (or `Untitled conversation`), a formatted date, a `<n> messages` count, and — if cost is known and non-zero — a dollar amount. The message count must be greater than zero. A row appearing with `0 messages` after a real exchange means the store is not recording.

---

## STORY-070: Reopen a past conversation and continue it

**Type**: long
**Topic**: Sessions
**Persona**: Priya, resuming
**Goal**: Pick up where she left off
**Preconditions**: A saved conversation containing `SESSION-2301`.
**Entry**: Sessions → click a row
**Window state**: Sessions → Chat
**Ideal path**: 2.
**Alternate paths**: Right-click → `Open` does the same thing — a duplicated path on the same row.

### Steps
1. Start a fresh conversation so the live transcript is empty.
2. Click the Sessions rail button; `click` the saved row.
3. `snapshot` through the blackout.
4. Send `What token did I mention earlier? Reply with just the token.`
5. `snapshot` after `Done`.

### Success condition
Step 3: the app switches to Chat automatically and the transcript is rebuilt — it contains `SESSION-2301`, the user prompts as user bubbles, and past tool calls as completed tool rows with their output attached (not as stray rows). Step 5: the reply contains `SESSION-2301`, proving the run actually joined the existing conversation rather than starting a fresh one with a cosmetic transcript.

### Edge cases
- Past tool output is re-attached heuristically to the most recent unfilled call (`Transcript.swift:337-346`). In a conversation with several consecutive tool calls, assert output is attached to the *right* call; mis-attachment is a real risk in this code path.

---

## STORY-071: Filter the conversation list

**Type**: short
**Topic**: Sessions
**Persona**: Priya with dozens of conversations
**Goal**: Find one by name
**Preconditions**: Several saved conversations with distinct titles.
**Entry**: Sessions → `Search conversations`
**Window state**: Sessions
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. `type <search ref>` a substring of one title.
2. `snapshot`.
3. Clear the field; `snapshot`.

### Success condition
The list narrows to rows whose titles contain the substring, case-insensitively, and restores fully when cleared. Note the filter is client-side over the first 100 fetched conversations (`ProjectSession.swift:152`) — with more than 100 saved, older matches are unreachable. Test this if the corpus allows and report it as a scale limitation.

---

## STORY-072: Export a transcript to a file

**Type**: medium
**Topic**: Sessions
**Persona**: Priya, archiving
**Goal**: Get a conversation out of the app
**Preconditions**: A saved conversation.
**Entry**: Sessions → right-click a row → `Export Transcript…`
**Window state**: Sessions, then a system save panel
**Ideal path**: 3.
**Alternate paths**: Status-bar `Copy the whole conversation` produces plain text instead of JSONL — same goal, different fidelity.

### Steps
1. Right-click a row; `snapshot` the context menu.
2. `click Export Transcript…`.
3. Drive the save panel; note the default filename.
4. Confirm; `snapshot` after ~2s.
5. Inspect the written file.

### Success condition
The context menu contains `Open`, `Export Transcript…`, a divider, and a destructive `Delete`. The default filename is `<conversation title>.jsonl`. The saved file is non-empty newline-delimited JSON. A status message reads `Exported to <filename>`.

### Edge cases
- Exporting **switches the app's active conversation** as a side effect (`SessionsView.swift:72-73` opens it first). Assert whether the on-screen transcript changed after an export — an export that silently reassigns which conversation the next prompt joins is a real bug worth reporting.

---

## STORY-073: Delete a conversation

**Type**: medium
**Topic**: Sessions — destructive
**Persona**: Priya, clearing an experiment
**Goal**: Remove a conversation permanently
**Preconditions**: Two saved conversations; the currently-open one is the target.
**Entry**: Sessions → right-click → `Delete`
**Window state**: Sessions
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Note which conversation is currently open (Settings → Project → `Conversation`).
2. Right-click that row; `click Delete`.
3. `snapshot` after ~2s.
4. Check Settings → Project → `Conversation`.

### Success condition
The row is gone from the list. Because the deleted conversation was the active one, `newConversation()` fires — the `Conversation` field must read `None yet` and the Chat transcript must be empty. Deleting a *non-active* conversation must leave the active one untouched: verify both branches.

### Edge cases
- **No confirmation dialog** on a permanently destructive action. Flag against the Checkpoints alert, which does confirm.

---

## STORY-074: Start a new conversation from Sessions

**Type**: short
**Topic**: Sessions
**Persona**: Priya
**Goal**: Begin fresh without going back to Chat first
**Preconditions**: A conversation is open.
**Entry**: Sessions → `New`
**Window state**: Sessions → Chat
**Ideal path**: 2.
**Alternate paths**: The composer footer's `New` (STORY-013) — identical effect minus the section jump.

### Steps
1. Click the Sessions rail button; `click New` in its toolbar.
2. `snapshot` through the blackout.

### Success condition
The app switches to Chat with an empty transcript and status `Ready`. Two identically-labelled `New` buttons producing near-identical behaviour is a redundancy candidate.

---

## M. Checkpoints & Rewind

## STORY-075: Empty checkpoints state

**Type**: short
**Topic**: Checkpoints
**Persona**: Priya, before any file changes
**Goal**: Know what checkpoints are for
**Preconditions**: A conversation with no file-mutating tool calls.
**Entry**: Rail → Checkpoints
**Window state**: Checkpoints
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Click the Checkpoints rail button; `snapshot` through the blackout.

### Success condition
The pane shows `No checkpoints yet` with the detail `A checkpoint is recorded whenever the agent changes files, so you can restore them.`

---

## STORY-076: A file-changing run produces a checkpoint card naming the files

**Type**: medium
**Topic**: Checkpoints
**Persona**: Priya
**Goal**: See what she could roll back to
**Preconditions**: Project ready; workspace writable and clean.
**Entry**: Rail → Checkpoints
**Window state**: Chat → Checkpoints
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Send `Create a file named checkpoint-probe.txt containing CKPT-2401. Then reply DONE.`
2. After `Done`, click the Checkpoints rail button; `snapshot` (retry up to 3 times — the list loads on `.task`).

### Success condition
At least one card exists titled `Before <tool name>` (or `Checkpoint`), with a formatted timestamp, a `Restore` button, and a file list containing `checkpoint-probe.txt` in monospaced text. A card with an empty file list after a real file write means the checkpoint captured nothing.

### Edge cases
- Files the server will skip are shown with a minus icon and a `· <reason>` caption. Force this by making a file read-only and assert the reason renders.

---

## STORY-077: Restoring a checkpoint is confirmed before it destroys anything

**Type**: long
**Topic**: Checkpoints — destructive
**Persona**: Priya, undoing a bad edit
**Goal**: Get her files and history back
**Preconditions**: STORY-076's checkpoint exists; a second run has since modified the file to `CKPT-2402`.
**Entry**: Checkpoints → `Restore`
**Window state**: Checkpoints, then an alert
**Ideal path**: 3 — click Restore, confirm, done.
**Alternate paths**: Settings → Project → `Undo Last Prompt` truncates history without touching files — overlapping but not equivalent.

### Steps
1. `snapshot` → `click Restore` on the earlier card.
2. `snapshot` → read the alert.
3. `click Cancel`; verify nothing changed on disk.
4. `click Restore` again, then `click Restore` in the alert.
5. `snapshot` after ~3s; check the file and the Chat transcript.

### Success condition
The alert title is `Restore this checkpoint?` and its message is `This overwrites the files in this checkpoint and removes every message after it. It cannot be undone.` with `Cancel` and a destructive `Restore`. Cancel must be a true no-op. After confirming, a status message of the form `Restored <n> file(s), removed <n> message(s)` appears, the file on disk contains `CKPT-2401` again, and the Chat transcript no longer contains the later turn.

---

## STORY-078: Rewind refusal on an externally-modified file is surfaced, not auto-forced

**Type**: long
**Topic**: Checkpoints — safety
**Persona**: Priya, who edited a file in her editor after the agent touched it
**Goal**: Not lose her own edit
**Preconditions**: A checkpoint exists; modify one of its files outside the app afterwards.
**Entry**: Checkpoints → `Restore` → confirm
**Window state**: Checkpoints
**Ideal path**: n/a — a guardrail.
**Alternate paths**: none. The UI has **no** `force` control: `forceNext` is initialised `false` and never set true anywhere (`SessionsView.swift:114, 148`).

### Steps
1. Create a checkpoint (STORY-076).
2. Out of band, edit the checkpointed file.
3. Restore and confirm.
4. `snapshot` after ~3s.

### Success condition
A status message appears carrying the server's refusal (the daemon answers `409 rewind_refused`), the externally-modified file is **unchanged**, and no `Restore Anyway` / force affordance is offered. Report the resulting dead end as a finding: the user is refused with no in-app way to proceed, and the only documented route is the TUI's `/rewind <point-id> confirm`.

---

## STORY-079: Checkpoints follow the active conversation

**Type**: medium
**Topic**: Checkpoints
**Persona**: Priya, switching between two conversations
**Goal**: See the right checkpoints
**Preconditions**: Two conversations, only one of which changed files.
**Entry**: Sessions → open each → Checkpoints
**Window state**: Sessions → Checkpoints
**Ideal path**: 2 per conversation.
**Alternate paths**: none.

### Steps
1. Open the file-changing conversation; go to Checkpoints; note the card count.
2. Go to Sessions, open the other conversation; go to Checkpoints; `snapshot`.
3. Reopen the first; `snapshot`.

### Success condition
Step 2 shows `No checkpoints yet`; step 3 shows the original card count again. Checkpoints keyed to the wrong conversation would let a restore overwrite unrelated files — a high-severity failure if it occurs.

### Edge cases
- After `New` in the composer, `rewindPoints` is cleared explicitly (`ProjectSession.swift:220`) — assert the pane empties.

---

## N. Activity View

## STORY-080: Activity shows the current run's plan

**Type**: medium
**Topic**: Activity
**Persona**: Marcus, watching a long task
**Goal**: See the agent's todo list
**Preconditions**: A run that uses the `todos` tool.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1.
**Alternate paths**: none — the transcript shows a `todos` tool row but not the rendered list.

### Steps
1. Send `Make a todo list with three steps for adding a README, then work through them, then reply PLANLIST-2501.`
2. While the run is active, click the Activity rail button; `snapshot` through the blackout.
3. `snapshot` again after 5s.

### Success condition
A section headed `Plan` lists the todo items, each with a circle or filled green checkmark, completed items struck through. Between the two snapshots at least one item's state should advance if the agent progressed. The section must be absent entirely when there are no todos, not present-and-empty.

### Edge cases
- Todos are fetched only when `run?.currentRunID` is non-nil (`ProjectSession.swift:168`) — so after the run completes, the `Plan` section disappears. Assert this and report it: the finished plan is unrecoverable.

---

## STORY-081: Background work is empty when nothing is running

**Type**: short
**Topic**: Activity
**Persona**: Marcus
**Goal**: Confirm nothing is running in the background
**Preconditions**: No cron jobs, callbacks, subagents, or bash jobs.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Click the Activity rail button; `snapshot` through the blackout.

### Success condition
A section headed `Background work` contains exactly the text `Nothing running.` This is the baseline every priority-capability story asserts against.

---

## STORY-082: The Runs section is permanently unavailable in the Mac app

**Type**: short
**Topic**: Activity / known gap
**Persona**: Marcus, looking for a past run
**Goal**: See runs he did not start from this transcript
**Preconditions**: Project ready.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Complete several runs.
2. Click the Activity rail button; `snapshot`.

### Success condition
The section headed `Runs` shows exactly `This server has no run store configured, so past runs are not listed.` This is expected and permanent: `HarnessSupervisor` never sets `HARNESS_RUN_DB`, so `GET /v1/runs` answers 501 for every app-supervised daemon. Assert the message rather than the alternative branches — and record it as the finding that blocks STORY-092 and STORY-097.

### Variations
- Launch with `HARNESS_BASE_URL` pointing at an externally-run harnessd started **with** `HARNESS_RUN_DB` set. Then this section must list runs with status dot, prompt, model, and time. Run both variants; the divergence is the point.

---

## STORY-083: Activity polls while visible and stops when it is not

**Type**: medium
**Topic**: Activity
**Persona**: Marcus
**Goal**: See background state update without refreshing
**Preconditions**: A long-running background bash job.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: n/a.
**Alternate paths**: none — there is no manual refresh control.

### Steps
1. Send `Run this in the background: sleep 120`
2. Click the Activity rail button; `snapshot`.
3. Wait 8s; `snapshot` again and compare the age value.

### Success condition
A `Background work` row exists whose type reads `bash_job` and whose age in seconds has increased between the two snapshots by roughly the elapsed time — proving the 3s poll (`ActivityView.swift:66-72`) is running. A frozen age means the poll is dead.

---

## STORY-084: Background work rows offer no actions

**Type**: short
**Topic**: Activity / known gap
**Persona**: Marcus, wanting to kill a runaway job
**Goal**: Stop a background task
**Preconditions**: A background bash job is running (STORY-083).
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1 — a Cancel button on the row.
**Alternate paths**: none in-app.

### Steps
1. With a job listed, `snapshot` the row and look for any button, menu, or context menu.
2. Right-click the row; `snapshot`.

### Success condition
Expected finding: no actionable control exists. `TaskInfo` decodes an `actions` array and exposes `isCancellable`, but `TaskRow` renders only an icon, label, type, status and age — the property is never read. The daemon offers `POST /v1/jobs/{id}/kill`, `POST /v1/callbacks/{id}/cancel`, `POST /v1/subagents/{id}/cancel`, and cron pause/resume/delete, and **none of them is reachable from this app.** Record the full list.

---

## O. Delayed Callbacks — PRIORITY

Mechanics established from the daemon source, and required for every assertion below:

- All three tools are **deferred**; the model must call `find_tool` first.
- `set_delayed_callback` takes `delay` (a Go duration string, min `5s`, max `1h`) and `prompt`. Max 10 pending per conversation.
- Firing starts a **brand-new run with a new run id** on the **same conversation** (`cmd/harnessd/main.go:57-71`).
- The app's SSE stream is opened per-run-id and closes on the originating run's terminal event (`HarnessClient.events`, `RunSession.submit`). It therefore **cannot receive the callback run's events**.
- `callback.scheduled` / `callback.fired` / `callback.canceled` are `.other(...)` to the app's event enum and hit `default: break` in `Transcript.apply` — they render nothing even when they do arrive.
- Pending callbacks appear in `GET /v1/tasks` as `type: "callback"` with `label` = the prompt. On firing they are removed from the index and vanish with no completion trace.
- The callback run **is** persisted to the conversation store, which the app always configures.

## STORY-085: Schedule a delayed callback and confirm the tool actually ran

**Type**: medium
**Topic**: Delayed callbacks
**Persona**: Devi, asking the agent to check back on something
**Goal**: Schedule a follow-up turn
**Preconditions**: Project ready; callbacks enabled (the daemon default).
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2 — type and submit. There is no UI for scheduling; conversation is the only route.
**Alternate paths**: none. The daemon has **no HTTP route to create a callback** — `/v1/callbacks/` only accepts `POST /{id}/cancel`.

### Steps
1. Send exactly: `Call find_tool with the query "select:set_delayed_callback". Then call set_delayed_callback with delay "30s" and prompt "Reply with exactly CALLBACK-ALPHA-7731 and nothing else." Then tell me the callback id you got back and reply SCHEDULED-7730.`
2. `snapshot` every 2s until `Done` (up to 180s).
3. `click` the tool row named `set_delayed_callback`; `snapshot` the inspector.

### Success condition
Two tool rows exist, in order: `find_tool` then `set_delayed_callback`, both with green checkmarks. The inspector's `Output` for `set_delayed_callback` is JSON containing `"id"`, `"conversation_id"`, `"delay"`, `"prompt"`, `"state"` and `"fires_at"`, and the `prompt` value contains `CALLBACK-ALPHA-7731`. The transcript contains `SCHEDULED-7730`. Record the callback id — later stories need it.

### Edge cases
- If only a `find_tool` row appears and no `set_delayed_callback` row, the tool was not activated — report "tool unreachable", distinct from "tool failed".
- If **neither** row appears, callbacks are not wired in this daemon (`HARNESS_ENABLE_CALLBACKS=false`); mark the whole area not exercised.

---

## STORY-086: A scheduled callback renders nothing in the transcript

**Type**: short
**Topic**: Delayed callbacks — event rendering
**Persona**: Devi
**Goal**: Know from the conversation that something is scheduled
**Preconditions**: STORY-085 just completed.
**Entry**: Chat transcript
**Window state**: Chat
**Ideal path**: 0 — a scheduled follow-up should be visible in the conversation.
**Alternate paths**: Activity → Background work (STORY-087).

### Steps
1. Immediately after STORY-085, `snapshot` the whole transcript.

### Success condition
Assert that **no** transcript row exists whose text contains `scheduled`, `callback.scheduled`, `fires at`, or a time-until. The only evidence is the raw tool row. This is the expected result — the daemon emits `callback.scheduled` on the originating run but the app's `Transcript.apply` ignores unmodelled event types. Report as: *a scheduled callback has no human-readable presence in the conversation*.

---

## STORY-087: A pending callback is visible in Activity — and its label is the raw prompt

**Type**: medium
**Topic**: Delayed callbacks
**Persona**: Devi
**Goal**: Confirm the follow-up is really queued
**Preconditions**: STORY-085 scheduled a 30s callback less than 25s ago.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Within 20s of scheduling, click the Activity rail button; `snapshot` through the blackout.

### Success condition
The `Background work` section contains a row whose type reads `callback` and whose label is the callback's **prompt text**, containing `CALLBACK-ALPHA-7731`. The row shows a status and an age in seconds. The section must no longer read `Nothing running.`

### Edge cases
- The row exposes no `fires_at`, no remaining time, and no cancel control — the user cannot tell *when* it will fire or stop it. Record both as findings.
- Note the label leaks the full prompt text into a list view with `lineLimit(1)`; a long prompt is truncated with no way to see the rest.

---

## STORY-088: **THE CRUX** — does a fired callback's output ever reach the transcript?

**Type**: long
**Topic**: Delayed callbacks — the primary open question
**Persona**: Devi, expecting the follow-up to appear like a normal reply
**Goal**: See the callback's answer
**Preconditions**: Project ready, idle, empty transcript. Provider working.
**Entry**: Composer, then passive observation
**Window state**: Chat
**Ideal path**: 0 — the follow-up belongs in the conversation it was scheduled from.
**Alternate paths**: Sessions → reopen the conversation (step 8) is the **only** route; Activity → Runs is blocked by STORY-082.

### Steps
1. Click `New` in the composer footer so the transcript is empty; `snapshot` to confirm.
2. Send exactly: `Call find_tool with the query "select:set_delayed_callback". Then call set_delayed_callback with delay "20s" and prompt "Reply with exactly CALLBACK-FIRED-8801 and nothing else." Then reply SCHEDULED-8800.`
3. Wait for status `Done`; `snapshot`. Record the transcript contents.
4. Do nothing for 60s. `snapshot` the transcript at t+25s, t+40s, and t+60s **without clicking anything**.
5. `snapshot` the status bar at t+60s.
6. Click the Activity rail button at t+60s; `snapshot`.
7. Return to Chat; `snapshot`.
8. Click the Sessions rail button; `click` the row for this conversation; `snapshot` through the blackout.

### Success condition
This story succeeds by **producing a definitive answer for each of five observation points**. Record each explicitly:

- **(a) Live transcript, t+25/40/60s (step 4):** does any row contain `CALLBACK-FIRED-8801`? *Expected: no.* The SSE stream for the originating run closed at its terminal event and the callback run has a different run id the app never subscribes to.
- **(b) Status bar (step 5):** does it still read `Done`, or does it change to indicate new activity? *Expected: unchanged `Done`.*
- **(c) Activity → Background work (step 6):** is the `callback` row still listed, or gone? *Expected: gone* — `fire` removes it from the conversation index — with **no completion or history row replacing it**. So the callback vanishes without trace.
- **(d) Activity → Runs (step 6):** *Expected:* `This server has no run store configured…`, so the fired run is invisible here too.
- **(e) Sessions reopen (step 8):** does the rebuilt transcript contain `CALLBACK-FIRED-8801`? *Expected: yes* — the callback run is persisted to the conversation store the app configures.

**The story FAILS as a product behaviour if (a) is "no" and (e) is also "no"** — that would mean the callback's output is unreachable from the app entirely. It fails as a *design* if (a) is "no" and (e) is "yes": the output exists but only appears if the user happens to reopen the conversation from a different section, with nothing anywhere telling them to. Either outcome is a reportable defect; report which one occurred, with the snapshot evidence.

### Variations
- Repeat with the app sitting in the **Activity** section for the whole wait, to confirm nothing surfaces there either.
- Repeat with a `5s` delay (the minimum) while the originating run is still working — send a long follow-up prompt immediately after scheduling so a run is live when the callback fires. Because `emitCallbackEvent` targets the newest live run on the conversation (`callback_bridge.go:80-118`), `callback.fired` *may* reach an open stream here. Even so, `Transcript.apply` ignores it, so assert whether anything renders. This is the one configuration where the event can arrive — if nothing shows, the gap is in the app, not the daemon.

---

## STORY-089: List pending callbacks from the conversation

**Type**: medium
**Topic**: Delayed callbacks
**Persona**: Devi, who scheduled several
**Goal**: See what is queued
**Preconditions**: Two callbacks scheduled with long delays (e.g. `10m`) in the current conversation.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2 — but ideally this would be a list in the UI, not a prompt.
**Alternate paths**: Activity → Background work lists them too (STORY-087) — the same fact reachable two ways, with different fidelity.

### Steps
1. Schedule two callbacks with delays `10m` and `12m` and distinct prompts containing `LISTCB-A-8901` and `LISTCB-B-8902`.
2. Send: `Call find_tool with query "select:list_delayed_callbacks", then call list_delayed_callbacks and tell me how many are pending and their ids.`
3. `snapshot` after `Done`; `click` the `list_delayed_callbacks` tool row.

### Success condition
The inspector `Output` is a JSON array of exactly 2 objects, each with an `id`, and their `prompt` fields contain `LISTCB-A-8901` and `LISTCB-B-8902`. Cross-check against Activity: `Background work` must show exactly 2 `callback` rows with matching labels. A mismatch between the tool's view and the Activity view is a consistency defect.

---

## STORY-090: Cancel a scheduled callback

**Type**: medium
**Topic**: Delayed callbacks
**Persona**: Devi, who changed her mind
**Goal**: Stop a scheduled follow-up
**Preconditions**: STORY-089's two callbacks are pending; their ids are known.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 1 — a Cancel button on the Activity row would be one click. Via conversation it is a full turn.
**Alternate paths**: The daemon exposes `POST /v1/callbacks/{id}/cancel`, but `HarnessKit` has no client method for it and no UI calls it. Record as unreachable.

### Steps
1. Send: `Call find_tool with query "select:cancel_delayed_callback", then call cancel_delayed_callback with callback_id "<the id for LISTCB-A-8901>". Then reply CANCELLED-9001.`
2. `snapshot` after `Done`; `click` the `cancel_delayed_callback` tool row.
3. Click the Activity rail button; `snapshot`.

### Success condition
The inspector `Output` contains `"state"` with value `canceled` and the matching id. Activity's `Background work` now shows exactly **one** `callback` row, labelled with `LISTCB-B-8902`. Wait past the cancelled callback's fire time and assert it never appears in the conversation on reopen — a cancelled callback that still fires is a correctness bug.

---

## STORY-091: Callback bounds are enforced with a legible message

**Type**: medium
**Topic**: Delayed callbacks — edge cases
**Persona**: Devi, asking for an unreasonable delay
**Goal**: Understand the limits
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Send: `Call find_tool with query "select:set_delayed_callback", then call set_delayed_callback with delay "1s" and prompt "too soon". Report exactly what error you get, then reply BOUNDS-1-9101.`
2. Repeat with delay `"2h"` and token `BOUNDS-2-9102`.
3. `click` each `set_delayed_callback` tool row and read its inspector.

### Success condition
Both attempts produce a **failed** tool row (red x-mark) rather than a silent success, and the inspector `Output` or the error surfaces a message naming the bound (min 5s / max 1h). The delay is rejected, not clamped — so no callback appears in Activity for either attempt. Assert `Background work` still reads `Nothing running.`

### Variations
- Schedule 11 callbacks in one conversation and assert the 11th fails with a per-conversation-limit message (max is 10). Then check Activity lists exactly 10.

---

## STORY-092: Callbacks do not survive a project restart

**Type**: medium
**Topic**: Delayed callbacks — durability
**Persona**: Devi, who closed the app
**Goal**: Find out whether her follow-up still happens
**Preconditions**: A callback scheduled with a `10m` delay.
**Entry**: Rail close button, then relaunch
**Window state**: Main window
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Schedule a callback with delay `10m` and token `RESTART-CB-9201`.
2. Confirm it appears in Activity → Background work.
3. Click the close-project rail button; confirm the picker returns and harnessd exits.
4. Reopen the same workspace.
5. Click the Activity rail button; `snapshot`.
6. Wait past the 10m mark, then open the conversation from Sessions; `snapshot`.

### Success condition
Step 5: `Background work` reads `Nothing running.` — the callback is gone. Step 6: the conversation does **not** contain `RESTART-CB-9201`. This is expected (callbacks are in-process `time.AfterFunc` timers) but the app gives no warning at scheduling time that a scheduled follow-up will be silently dropped by closing the project. Report the missing warning as the finding.

---

## STORY-093: A callback that fires while the app is on a different conversation

**Type**: long
**Topic**: Delayed callbacks — cross-conversation
**Persona**: Devi, who moved on to other work
**Goal**: Not miss the follow-up
**Preconditions**: Project ready.
**Entry**: Composer, then Sessions
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. In conversation A, schedule a `30s` callback with token `CROSSCONV-9301`.
2. Immediately click `New` to start conversation B and send `Reply with CONVB-9302.`
3. Wait 90s, staying in conversation B. `snapshot` the transcript and the Activity section.
4. Go to Sessions; `snapshot` the list.
5. Open conversation A; `snapshot`.

### Success condition
Step 3: conversation B's transcript contains `CONVB-9302` and **not** `CROSSCONV-9301` — a callback bleeding into the wrong conversation would be a serious defect; assert its absence. Step 4: record whether conversation A's row shows an updated timestamp or a higher message count reflecting the callback turn — that is the only ambient signal the app could give. Step 5: conversation A's rebuilt transcript should contain `CROSSCONV-9301`. Report whether step 4 gave any signal at all; expected: the row's `updated_at` changes but nothing draws attention to it.

---

## STORY-094: Two callbacks firing close together

**Type**: medium
**Topic**: Delayed callbacks — concurrency
**Persona**: Devi
**Goal**: Get both follow-ups
**Preconditions**: Project ready, empty conversation.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0.
**Alternate paths**: none.

### Steps
1. Schedule two callbacks in one turn, delays `10s` and `15s`, prompts producing `MULTI-CB-A-9401` and `MULTI-CB-B-9402`.
2. Wait 90s without interacting.
3. Reopen the conversation from Sessions; `snapshot`.

### Success condition
The rebuilt transcript contains **both** tokens, each as a separate assistant turn with its own preceding user prompt. A missing second token means the two runs collided on the same conversation. Also check Activity mid-wait: both `callback` rows should be listed, then both should disappear.

---

## STORY-095: Reopening after a callback replaces the live transcript wholesale

**Type**: medium
**Topic**: Delayed callbacks — recovery UX
**Persona**: Devi, who had unsent work in the composer
**Goal**: See the callback result without losing context
**Preconditions**: A fired callback exists in the current conversation (STORY-088 state).
**Entry**: Sessions → the conversation row
**Window state**: Chat → Sessions → Chat
**Ideal path**: 1 — a refresh in place.
**Alternate paths**: none.

### Steps
1. With a fired callback outstanding, type (do not send) `DRAFT-KEEP-9501` into the composer.
2. Click the Sessions rail button and open the same conversation.
3. `snapshot` the composer and the transcript.

### Success condition
The transcript now contains the callback token. Record whether the composer draft `DRAFT-KEEP-9501` survived: `load(messages:)` replaces the transcript but does not touch `draft` (`RunSession.swift:142-148`), so it should survive — assert it. Also assert the transcript was fully rebuilt (past tool rows present with output attached) rather than appended to, so the user does not see duplicated turns.

---

## P. Cron — PRIORITY

Mechanics: all six tools are **deferred**. `cron_create` always builds a **shell** job (`sh -c <command>`), never a harness run — there is no harness executor wired. Schedules are standard 5-field cron in **UTC**, so the finest granularity is one minute. Output is truncated to 4096 bytes and stored on the execution row, readable only via `cron_get`. Jobs appear in `GET /v1/tasks` as `type: "cron"` with `label` = the job name and `status` = `active`/`paused`.

## STORY-096: Create a cron job from the conversation

**Type**: medium
**Topic**: Cron
**Persona**: Ola, automating a recurring check
**Goal**: Schedule a repeating command
**Preconditions**: Project ready; cron enabled (the daemon default when `HARNESS_CRON_URL` is unset — an embedded scheduler).
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2. There is **no cron UI at all**; conversation is the only route from this app.
**Alternate paths**: The daemon exposes `GET/POST /v1/cron/jobs` and per-job PATCH/DELETE/pause/resume, but `HarnessKit` has no client methods for any of them. Record as unreachable.

### Steps
1. Send exactly: `Call find_tool with query "select:cron_create". Then call cron_create with name "ux-probe-heartbeat", schedule "* * * * *", command "echo CRON-BEAT-1001 >> /tmp/ux-cron-probe.log", and timeout_seconds 10. Then tell me the job id and reply CRONMADE-1000.`
2. `snapshot` after `Done`; `click` the `cron_create` tool row.

### Success condition
Tool rows `find_tool` then `cron_create`, both green. The inspector `Output` is JSON containing `"id"`, `"name": "ux-probe-heartbeat"`, `"schedule": "* * * * *"`, `"execution_type": "shell"`, `"status": "active"`, and a `"next_run_at"` timestamp. The transcript contains `CRONMADE-1000`. Record the job id.

### Edge cases
- If only `find_tool` appears, cron is not wired (`buildCronBootstrap` failed) — mark the area not exercised rather than failed.

---

## STORY-097: A cron job appears in Activity as background work

**Type**: short
**Topic**: Cron
**Persona**: Ola
**Goal**: Confirm the schedule is live without asking the agent
**Preconditions**: STORY-096 created `ux-probe-heartbeat`.
**Entry**: Rail → Activity
**Window state**: Activity
**Ideal path**: 1.
**Alternate paths**: `cron_list` via the composer (STORY-098) — the same fact, one full model turn instead of one click.

### Steps
1. Click the Activity rail button; `snapshot` through the blackout.

### Success condition
`Background work` contains a row whose type reads `cron`, whose label is exactly `ux-probe-heartbeat`, and whose status reads `active`. The row must **not** show the schedule, the next run time, or any pause/delete control — assert their absence and record it: `TaskInfo.actions` arrives from the server carrying `["pause","delete"]` and is discarded by the UI.

---

## STORY-098: List and inspect cron jobs including execution history

**Type**: medium
**Topic**: Cron
**Persona**: Ola, checking whether it actually ran
**Goal**: See past executions
**Preconditions**: `ux-probe-heartbeat` created at least 2 minutes ago.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2 — but ideally zero; execution history has no UI whatsoever.
**Alternate paths**: none. harnessd exposes **no** execution-history route (only the standalone `cronsd` does), so `cron_get` is the sole path.

### Steps
1. Send: `Call find_tool with query "select:cron_list", then call cron_list and tell me every job name and status. Then reply CRONLIST-1100.`
2. `click` the `cron_list` tool row; read the inspector.
3. Send: `Call find_tool with query "select:cron_get", then call cron_get with id "<job id>". Tell me how many recent executions there were and what their output was. Then reply CRONGET-1101.`
4. `click` the `cron_get` tool row; read the inspector.

### Success condition
Step 2: the output array contains an object with `"name": "ux-probe-heartbeat"` and `"status": "active"`. Step 4: the output is an object with a `"job"` key and a `"recent_executions"` array with **at least one** entry, whose output summary contains `CRON-BEAT-1001`. If `recent_executions` is empty after 2+ minutes on a `* * * * *` schedule, the scheduler is not firing — a correctness failure, not a UX one. Cross-check `/tmp/ux-cron-probe.log` out of band.

---

## STORY-099: Pause a cron job and see the status change in Activity

**Type**: medium
**Topic**: Cron
**Persona**: Ola, silencing a noisy job
**Goal**: Stop it firing without deleting it
**Preconditions**: `ux-probe-heartbeat` is `active`.
**Entry**: Composer, then Activity
**Window state**: Chat → Activity
**Ideal path**: 1 — a Pause button on the Activity row. Actual: a full model turn.
**Alternate paths**: `POST /v1/cron/jobs/{id}/pause` exists on the daemon and is unreachable from the app.

### Steps
1. Note the current line count of `/tmp/ux-cron-probe.log`.
2. Send: `Call find_tool with query "select:cron_pause", then call cron_pause with id "<job id>". Then reply CRONPAUSE-1201.`
3. Click the Activity rail button; `snapshot`.
4. Wait 150s, then check the log line count again.

### Success condition
Step 3: the `ux-probe-heartbeat` row's status reads `paused`, not `active` — this is the one cron state change the app can actually display, so it must be observed. Step 4: the log has gained **no** new lines. A paused job that keeps firing is a correctness failure.

---

## STORY-100: Resume a paused cron job

**Type**: medium
**Topic**: Cron
**Persona**: Ola
**Goal**: Turn it back on
**Preconditions**: STORY-099 left the job `paused`.
**Entry**: Composer, then Activity
**Window state**: Chat → Activity
**Ideal path**: 1.
**Alternate paths**: `POST /v1/cron/jobs/{id}/resume`, unreachable from the app.

### Steps
1. Send: `Call find_tool with query "select:cron_resume", then call cron_resume with id "<job id>". Then reply CRONRESUME-1301.`
2. Click the Activity rail button; `snapshot`.
3. Wait 150s; check the log.

### Success condition
The Activity row's status returns to `active` and the log gains at least one new line containing `CRON-BEAT-1001` within 150s.

---

## STORY-101: Delete a cron job

**Type**: short
**Topic**: Cron — destructive
**Persona**: Ola, cleaning up
**Goal**: Remove the job entirely
**Preconditions**: `ux-probe-heartbeat` exists.
**Entry**: Composer, then Activity
**Window state**: Chat → Activity
**Ideal path**: 2.
**Alternate paths**: `DELETE /v1/cron/jobs/{id}`, unreachable from the app.

### Steps
1. Send: `Call find_tool with query "select:cron_delete", then call cron_delete with id "<job id>". Then reply CRONDEL-1401.`
2. Click the Activity rail button; `snapshot`.
3. Wait 150s; check the log.

### Success condition
The `cron` row is gone from `Background work` (if it was the only task, the section reads `Nothing running.`) and the log gains no new lines. Note there is **no confirmation** at any layer — the agent deletes on request. Flag the asymmetry: deleting a recurring job is more consequential than restoring a checkpoint, which does confirm.

---

## STORY-102: A cron job's shell command runs on the daemon host, not in a run

**Type**: medium
**Topic**: Cron — mental model
**Persona**: Ola, who expected cron to prompt the agent
**Goal**: Understand what a cron job can actually do
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: n/a — a comprehension check.
**Alternate paths**: none.

### Steps
1. Create a job with command `pwd >> /tmp/ux-cron-cwd.log` on `* * * * *`.
2. Wait 90s; read the file out of band.
3. Send: `Call cron_get with id "<id>" and tell me the recent execution output verbatim. Then reply CWDCHK-1501.`

### Success condition
The command executed as a plain shell command on the harnessd host. Assert that no run appeared anywhere in the app as a result — the transcript gains nothing, and Activity shows no new `subagent` or `bash_job` row. Report the resulting model mismatch as a finding: the tool description invites "schedule a task", but the only schedulable thing is a shell command whose output the user can never see in the UI.

### Edge cases
- Ask the agent to create a job with `execution_type: "harness"`. The daemon validates that type as legal but has **no executor for it**, so the job will fail at fire time with a misleading "missing 'command' field". Assert the failure text and report the misleading message.

---

## STORY-103: An invalid cron expression is rejected legibly

**Type**: short
**Topic**: Cron — edge cases
**Persona**: Ola, mistyping
**Goal**: Get a usable error
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Send: `Call find_tool with query "select:cron_create", then call cron_create with name "ux-bad-cron", schedule "every minute", command "true". Report the exact error, then reply BADCRON-1601.`
2. `click` the `cron_create` tool row.

### Success condition
The tool row is red (failed), the inspector shows an error naming the schedule field, and Activity shows no `ux-bad-cron` row. A job created with an unparseable schedule that silently never fires would be worse than an error.

---

## STORY-104: Cron jobs outlive the conversation but not necessarily the project

**Type**: medium
**Topic**: Cron — durability
**Persona**: Ola
**Goal**: Know whether a job survives closing the app
**Preconditions**: One active cron job.
**Entry**: Rail close button, relaunch
**Window state**: Main window
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Create `ux-persist-probe` on `* * * * *` writing to a log.
2. Click `New` for a fresh conversation; click the Activity rail button; `snapshot`.
3. Close the project, relaunch, reopen the same workspace.
4. Click the Activity rail button; `snapshot`.
5. Check the log for lines written while the app was closed.

### Success condition
Step 2: the job is still listed despite the conversation change — cron is daemon-scoped, not conversation-scoped. Step 4: record whether the job reappears after relaunch (the embedded scheduler is backed by a store, so it should). Step 5: no lines are written while harnessd is down, because the scheduler dies with the app's supervised child. Report the durability model plainly, because nothing in the UI states it.

---

## Q. Workflows — PRIORITY

Mechanics: `create_workflow` and `run_workflow` are **deferred** and registered only when a workflow service is wired. `create_workflow` writes `workflow.json` plus `main.go` under `<workspace>/.go-harness/workflows/<name>` (or `~/.harness/workflows` for global scope) and then **compiles it with the Go toolchain**. `run_workflow` blocks by default (`wait: true`) and returns a batch result plus filtered feedback events — it does **not** stream onto the calling run's SSE stream. The app has no workflow UI and `HarnessKit` has no workflow client methods.

## STORY-105: Create a workflow from the conversation

**Type**: long
**Topic**: Workflows
**Persona**: Ravi, building a reusable pipeline
**Goal**: Author a workflow the daemon can run
**Preconditions**: Project ready; a Go toolchain reachable from harnessd's `PATH`. **Critical:** if the app was launched from Finder rather than a terminal, `PATH` is minimal and `go` will not be found — launch from a terminal for this story.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2. There is no workflow UI of any kind.
**Alternate paths**: none — `GET /v1/script-workflows` is list-only; there is no create route on the daemon either, so the tool is the sole authoring path.

### Steps
1. Send: `Call find_tool with query "select:create_workflow". Then call create_workflow with name "ux-probe-flow", description "A probe workflow for UX testing", and a minimal Go source using the workflowsdk that logs the string WORKFLOW-BUILT-1701 and returns it. Then tell me the path and hash it returned and reply CREATED-1700.`
2. `snapshot` every 3s until `Done` (allow up to 300s — this compiles Go).
3. `click` the `create_workflow` tool row; read the inspector.

### Success condition
The inspector `Output` is JSON containing `"status": "created"`, `"name": "ux-probe-flow"`, a `"path"` under the workspace, a `"hash"`, and `"scope": "workspace"`. Verify out of band that `<workspace>/.go-harness/workflows/ux-probe-flow/workflow.json` and `main.go` exist. The transcript contains `CREATED-1700`.

### Edge cases
- **The compile failure case matters most.** If `go` is absent the tool row goes red. Assert that the inspector `Output` names the real cause (a missing toolchain or a compiler error) rather than a generic failure. Deliberately submit invalid Go in a second run and assert the compiler's message reaches the inspector — a workflow author with no error text cannot iterate.
- Creating the same name twice without `overwrite: true` must fail with a name-collision message, not silently replace.

---

## STORY-106: Run a workflow and read its result

**Type**: long
**Topic**: Workflows
**Persona**: Ravi
**Goal**: Execute the workflow he just wrote
**Preconditions**: STORY-105 created `ux-probe-flow`.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: `POST /v1/script-workflows/{name}/runs` exists on the daemon; unreachable from the app.

### Steps
1. Send: `Call find_tool with query "select:run_workflow", then call run_workflow with name "ux-probe-flow". Tell me the run_id, status, and result, then reply RAN-1801.`
2. `snapshot` every 3s until `Done`.
3. `click` the `run_workflow` tool row; read the inspector.

### Success condition
The inspector `Output` is JSON containing `"run_id"`, `"workflow_name": "ux-probe-flow"`, `"status"`, and `"result_json"` containing `WORKFLOW-BUILT-1701`. The transcript contains `RAN-1801`.

---

## STORY-107: Workflow progress is invisible while it runs

**Type**: medium
**Topic**: Workflows — the observability gap
**Persona**: Ravi, running a multi-minute workflow
**Goal**: See what stage it is at
**Preconditions**: A workflow with at least two phases and a deliberate delay per phase.
**Entry**: Composer, then passive observation
**Window state**: Chat, then Activity
**Ideal path**: 0 — phase progress should stream into the transcript.
**Alternate paths**: `GET /v1/script-workflow-runs/{id}/events` streams phases; unreachable from the app.

### Steps
1. Create and run a workflow with two phases separated by ~30s each.
2. While it runs, `snapshot` the transcript every 5s.
3. Click the Activity rail button mid-run; `snapshot`.
4. Return to Chat; after completion, `click` the `run_workflow` tool row.

### Success condition
Record each observation point:
- **(a)** During the run, the only transcript evidence is a single `run_workflow` tool row with a spinner. Assert that no phase names, no log lines, and no progress text appear. *Expected: nothing.*
- **(b)** Activity → `Background work` shows **no** row for the workflow run (workflow runs are not in `/v1/tasks`). Assert this.
- **(c)** After completion the inspector's `Output` contains a `"feedback"` array with phase/log entries — i.e. the progress information existed all along and was delivered only as a lump at the end.

Report as: *workflow progress is available but batched; the app has no live surface for it.*

### Edge cases
- Ask the agent to call `run_workflow` with `wait: false` and assert what comes back — a run id with no way for the user to follow it, since the app cannot open a workflow event stream.

---

## STORY-108: A failing workflow reports a usable reason

**Type**: medium
**Topic**: Workflows — errors
**Persona**: Ravi, debugging
**Goal**: Find out why it failed
**Preconditions**: A workflow that returns an error.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Create a workflow whose body returns an error containing `WORKFLOW-BOOM-1901`.
2. Run it.
3. `click` the `run_workflow` tool row.

### Success condition
The inspector `Output` contains `"status"` indicating failure and an `"error"` field containing `WORKFLOW-BOOM-1901`. The tool row's status must reflect the failure. A workflow that fails while the tool row shows a green checkmark is a status-mapping bug.

---

## STORY-109: Workflow timeout

**Type**: medium
**Topic**: Workflows — edge cases
**Persona**: Ravi
**Goal**: Not have the app hang on a runaway workflow
**Preconditions**: A workflow that sleeps longer than its timeout.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: `Stop` in the status bar cancels the calling run — check whether it also stops the workflow.

### Steps
1. Run a long workflow with `timeout_seconds: 15`.
2. `snapshot` every 5s for 90s.
3. In a second attempt, run it with a long timeout and click `Stop` after 10s; `snapshot` and check whether the workflow keeps running out of band.

### Success condition
Step 2: the tool row resolves (failed or completed) within roughly the timeout, the run reaches a terminal status, and the composer becomes usable again. Step 3: record whether cancelling the harness run also cancels the workflow run — if the workflow continues after the user pressed Stop, that is an orphaned-work finding.

---

## STORY-110: Workflows are not discoverable from the app

**Type**: short
**Topic**: Workflows — known gap
**Persona**: Ravi, returning a week later
**Goal**: Find the workflows he wrote
**Preconditions**: At least one workflow exists.
**Entry**: Every section of the app
**Window state**: All
**Ideal path**: 1 — a list.
**Alternate paths**: Asking the agent, which requires knowing that workflows exist at all.

### Steps
1. Visit Chat, Activity, Sessions, Checkpoints, and all four Settings tabs. `snapshot` each.

### Success condition
Confirm that the string `workflow` appears nowhere in any pane. `GET /v1/script-workflows` lists them and `HarnessKit` has no method for it. Record as a hard gap: an authored workflow is invisible in the UI and recoverable only by remembering its name.

---

## R. Subagents — PRIORITY

Mechanics: all seven tools are **deferred**. `spawn_agent` runs a forked child synchronously and the parent sees only a normal `tool.call.started`/`completed` pair for it. `start_subagent` returns a `subagent_id` and a `run_id` and auto-activates `get_subagent`, `wait_subagent`, `cancel_subagent`, and `message_subagent` for that run. Subagents appear in `GET /v1/tasks` as `type: "subagent"`. **`spawn_agent.started`, `spawn_agent.completed`, and `task.completed` are declared in the app's event enum but are never emitted anywhere in the daemon** — the app models three events that cannot occur.

## STORY-111: Spawn a synchronous subagent

**Type**: medium
**Topic**: Subagents
**Persona**: Nia, delegating a bounded research task
**Goal**: Get a sub-task done without polluting the main context
**Preconditions**: Project ready; a subagent manager wired. Subagents may use git worktrees, so use a **git repository** as the workspace.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none in the UI.

### Steps
1. Send: `Call find_tool with query "select:spawn_agent". Then call spawn_agent with task "Reply with exactly SUBAGENT-SYNC-2001 and nothing else." Then tell me exactly what it returned and reply SPAWNED-2000.`
2. `snapshot` every 3s until `Done` (allow 300s).
3. `click` the `spawn_agent` tool row; read the inspector.

### Success condition
Tool rows `find_tool` then `spawn_agent`. The inspector `Output` contains a status and a summary containing `SUBAGENT-SYNC-2001`. The transcript contains `SPAWNED-2000`. Assert that the child's own intermediate steps do **not** appear as transcript rows — only the single `spawn_agent` row — so the delegation boundary is respected.

### Edge cases
- Run the same story in a **non-git** workspace and record whether spawn fails, and whether the failure text explains the git requirement.

---

## STORY-112: A running subagent is visible in Activity

**Type**: medium
**Topic**: Subagents
**Persona**: Nia, watching a long delegation
**Goal**: Confirm work is happening
**Preconditions**: A long-running subagent.
**Entry**: Rail → Activity
**Window state**: Chat → Activity
**Ideal path**: 1.
**Alternate paths**: `get_subagent` via the composer (STORY-114).

### Steps
1. Send a prompt that spawns a subagent with a long task (e.g. `Count to 200 slowly, then reply SUBLONG-2101.`).
2. While it runs, click the Activity rail button; `snapshot` through the blackout.
3. `snapshot` again 6s later.

### Success condition
`Background work` contains a row whose type reads `subagent`, whose label is a branch name or run id, and whose age increases between snapshots. The status reads a running/queued state. No cancel control exists on the row — assert its absence and record it (the daemon offers `POST /v1/subagents/{id}/cancel`).

---

## STORY-113: Start an asynchronous subagent and get its id

**Type**: medium
**Topic**: Subagents
**Persona**: Nia, fanning out work
**Goal**: Launch without blocking the conversation
**Preconditions**: Project ready, git workspace.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: `POST /v1/subagents` exists on the daemon; `HarnessKit` has no method and no UI calls it.

### Steps
1. Send: `Call find_tool with query "select:start_subagent". Then call start_subagent with task "Wait 60 seconds, then reply with exactly ASYNC-SUB-2201." Tell me the subagent_id and run_id, then reply STARTED-2200.`
2. `snapshot` after `Done`; `click` the `start_subagent` tool row.
3. Click the Activity rail button; `snapshot`.

### Success condition
The inspector `Output` contains `"subagent_id"`, `"run_id"`, and `"status"`. The parent run reaches `Done` **quickly** — it did not block on the child. Activity shows a `subagent` row still running after the parent finished. Record the subagent id.

---

## STORY-114: Poll and then wait on a subagent

**Type**: long
**Topic**: Subagents
**Persona**: Nia
**Goal**: Collect the delegated result
**Preconditions**: STORY-113's subagent is running.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2 per check — but ideally the result would arrive on its own.
**Alternate paths**: `POST /v1/subagents/{id}/wait` on the daemon; unreachable from the app.

### Steps
1. Send: `Call get_subagent with id "<subagent id>" and tell me its status verbatim. Then reply POLLED-2301.` (No `find_tool` needed — `start_subagent` auto-activates its siblings for the run. **If the tool is not found, that auto-activation is scoped to the original run only** — record that as the finding and retry with an explicit `find_tool`.)
2. `click` the `get_subagent` tool row.
3. Send: `Call wait_subagent with id "<subagent id>" and tell me its output verbatim. Then reply WAITED-2302.`
4. `snapshot` every 5s until `Done`.
5. `click` the `wait_subagent` tool row.

### Success condition
Step 2: output contains `"id"`, `"run_id"`, `"status"` and, while running, an empty `output`. Step 5: output's `status` is a completed state and `output` contains `ASYNC-SUB-2201`. After completion, Activity's `subagent` row should show a terminal status or disappear — assert which. The key check on step 1 is whether the sibling tools were genuinely auto-activated across turns; a `find_tool` row appearing in step 1's transcript when none was requested means the model had to rediscover them.

---

## STORY-115: Message a running subagent

**Type**: long
**Topic**: Subagents
**Persona**: Nia, refining a delegation in flight
**Goal**: Redirect a child without restarting it
**Preconditions**: A long-running subagent; a run-steerer wired (`message_subagent` requires it).
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: `POST /v1/runs/{child run id}/steer` — reachable only if the child's run id is known, and the app has no UI to enter one.

### Steps
1. Start a subagent tasked `Keep counting slowly to 500 and report each number.`
2. Send: `Call find_tool with query "select:message_subagent", then call message_subagent with id "<subagent id>" and message "Stop counting. Reply with exactly MSGSUB-2401 and finish." Then reply MESSAGED-2400.`
3. `click` the `message_subagent` tool row.
4. Send `Call wait_subagent with id "<subagent id>" and tell me its output. Then reply MSGDONE-2402.`

### Success condition
Step 3's output contains `"status": "sent"`. Step 4's output contains `MSGSUB-2401`, proving the message actually reached and redirected the child. If `message_subagent` is not registered at all (no run-steerer), record "capability not wired" rather than a failure.

---

## STORY-116: Cancel a subagent

**Type**: medium
**Topic**: Subagents — destructive
**Persona**: Nia, stopping runaway work
**Goal**: Kill a child task
**Preconditions**: A long-running subagent.
**Entry**: Composer
**Window state**: Chat → Activity
**Ideal path**: 1 — a Cancel button on the Activity row. Actual: a full model turn.
**Alternate paths**: `POST /v1/subagents/{id}/cancel`, unreachable from the app.

### Steps
1. Start a long subagent; confirm it in Activity.
2. Send: `Call cancel_subagent with id "<subagent id>". Then reply CANCELSUB-2501.`
3. `click` the `cancel_subagent` tool row.
4. Click the Activity rail button; `snapshot` immediately and again after 15s.

### Success condition
The inspector output contains `"status": "cancelling"`. Within 15s the Activity `subagent` row shows a terminal status or is gone. A row stuck on `cancelling` indefinitely, or a child that keeps producing work, is a leak.

---

## STORY-117: Fan out with agent_swarm

**Type**: long
**Topic**: Subagents
**Persona**: Nia, processing a batch
**Goal**: Run the same task over several inputs in parallel
**Preconditions**: Project ready, git workspace, swarm runner wired.
**Entry**: Composer
**Window state**: Chat → Activity
**Ideal path**: 2.
**Alternate paths**: none — there is **no HTTP fan-out route** on the daemon either; the tool is the only path.

### Steps
1. Send: `Call find_tool with query "select:agent_swarm". Then call agent_swarm with prompt_template "Reply with exactly SWARM-{{item}} and nothing else." and items ["A2601","B2602","C2603"]. Report the full swarm report, then reply SWARMED-2600.`
2. Within 20s, click the Activity rail button; `snapshot`.
3. Return to Chat; `snapshot` every 5s until `Done` (allow 600s).
4. `click` the `agent_swarm` tool row; read the inspector.

### Success condition
Step 2: `Background work` contains **three** rows of type `subagent` simultaneously. Step 4: the output contains `"total": 3`, `"completed": 3`, `"failed": 0`, and a `members` array whose three entries' outputs contain `SWARM-A2601`, `SWARM-B2602`, and `SWARM-C2603` respectively. A swarm reporting fewer members than items, or `completed` not matching, is a correctness failure.

### Edge cases
- A `prompt_template` without `{{item}}` must be rejected with a message naming the requirement.
- Run with 20 items and assert Activity does not visibly degrade and all 20 are accounted for in the report.

---

## STORY-118: Swarm partial failure is reported per member

**Type**: medium
**Topic**: Subagents — errors
**Persona**: Nia
**Goal**: Know which items failed
**Preconditions**: A swarm where one item will fail.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 2.
**Alternate paths**: none.

### Steps
1. Run a swarm over three items where one instructs the child to fail.
2. `click` the `agent_swarm` tool row.

### Success condition
The report's `failed` count is non-zero, `completed + failed + cancelled` equals `total`, and the failing member carries an `error` string and an identifiable `item`. A swarm that reports overall success while a member failed hides the failure.

---

## STORY-119: The app models three subagent events the daemon never emits

**Type**: short
**Topic**: Subagents — dead code finding
**Persona**: n/a (static finding, verified by walk)
**Goal**: Confirm the app cannot be relying on them
**Preconditions**: STORY-111 and STORY-117 completed.
**Entry**: Chat transcript
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Re-read the transcripts from STORY-111 and STORY-117.

### Success condition
Confirm that subagent activity produced **only** ordinary tool rows — no distinct "agent started"/"agent completed" presentation exists. `HarnessEventType` declares `spawnAgentStarted`, `spawnAgentCompleted`, and `taskCompleted` (`HarnessEvent.swift:36`) and `Transcript.apply` ignores all three anyway (`default: break`), while the daemon's matching constants have zero emit sites. Record as: three declared events that are dead on both ends; any future subagent-progress UI must not be built on them.

---

## STORY-120: Subagent work leaves no trace after the run ends

**Type**: medium
**Topic**: Subagents — recovery
**Persona**: Nia, returning after lunch
**Goal**: Find what her swarm produced
**Preconditions**: STORY-117's swarm completed at least 10 minutes ago; a new conversation has since been started.
**Entry**: All sections
**Window state**: All
**Ideal path**: 1.
**Alternate paths**: Sessions → reopen the conversation.

### Steps
1. Click `New` for a fresh conversation.
2. Visit Activity; `snapshot`.
3. Visit Sessions, open the swarm's conversation; `snapshot`.

### Success condition
Step 2: `Background work` reads `Nothing running.` and `Runs` reads the no-run-store message — so the completed swarm has no history surface at all. Step 3: the reopened transcript contains the `agent_swarm` tool row with its report in the inspector. Report that reopening the originating conversation is the sole recovery route, exactly as for delayed callbacks (STORY-088).

---

## S. Native Platform Surface

## STORY-121: The app has no menu bar commands

**Type**: short
**Topic**: Menu bar / keyboard
**Persona**: Ada, a Mac power user
**Goal**: Drive the app from the menu bar
**Preconditions**: A project is open.
**Entry**: Menu bar
**Window state**: Main window frontmost
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. `snapshot` the menu bar. Enumerate every menu and every item.

### Success condition
Only AppKit's defaults appear (app menu, File, Edit, Window, Help — whatever SwiftUI synthesises). **No** item exists for New Conversation, Send, Stop, Plan Mode, Settings, Open Project, Export, or any section. `GoCodeApp.swift` declares no `.commands { }`. Record the full list of missing commands; each is a keyboard-reachability gap.

---

## STORY-122: ⌘, does not open Settings

**Type**: short
**Topic**: Keyboard / platform convention
**Persona**: Ada
**Goal**: Open preferences the way every Mac app does
**Preconditions**: A project is open, focus in the window.
**Entry**: ⌘,
**Window state**: Main window
**Ideal path**: 1.
**Alternate paths**: The Settings rail button — mouse only.

### Steps
1. `snapshot` and note the current section.
2. `key cmd+,`.
3. `snapshot`.

### Success condition
Expected finding: nothing happens. There is no `Settings` scene, so the standard shortcut is inert while a Settings *section* exists behind an unlabelled icon. Confirm the section did not change and no window appeared. Report the convention violation.

---

## STORY-123: Complete a full conversation using only the keyboard

**Type**: long
**Topic**: Keyboard-only operation
**Persona**: Ada, who does not use a mouse
**Goal**: Send a prompt, read the reply, copy it, and start a new conversation — no clicks
**Preconditions**: A project open on the Chat section.
**Entry**: Keyboard only
**Window state**: Main window
**Ideal path**: 6 keystrokes for a well-designed app.
**Alternate paths**: All of it by mouse.

### Steps
1. `key tab` repeatedly (max 25 presses), `snapshot` after each, recording which element holds focus.
2. When the composer has focus, `type` `Reply with exactly KBD-ONLY-2701.` and `key return`.
3. After `Done`, continue tabbing and record whether focus ever reaches: the copy-message button, the model chip, the `Plan mode` toggle, the `New` button, the `Stop` button, or any icon-rail button.
4. Attempt to change section using only the keyboard.

### Success condition
Step 2 must succeed — the composer takes focus on appear (`ChatView.swift:732`), so a prompt is keyboard-sendable. Then record precisely which controls are and are not reachable by Tab. **The rail is the decisive check:** if no amount of tabbing reaches the five section buttons, the entire app outside Chat is keyboard-unreachable, which is both an accessibility failure and a hard blocker for keyboard-only users. Report the reachable set and the unreachable set as two explicit lists.

---

## STORY-124: A second window opens a second project and a second server

**Type**: long
**Topic**: Window management
**Persona**: Ada, comparing two repos
**Goal**: Work on two projects at once
**Preconditions**: One project already open; a second scratch repo available.
**Entry**: ⌘N (File → New Window)
**Window state**: Two main windows
**Ideal path**: 3.
**Alternate paths**: none.

### Steps
1. `key cmd+n`.
2. `window` → assert two windows exist.
3. `snapshot` the new window → it should show `Open a project` (a fresh `AppShell` with no `initialWorkspace`... unless `HARNESS_WORKSPACE` is set, in which case both windows open the same project — record which).
4. Open a different repo in window 2.
5. Send a distinct prompt in each window (`WIN-A-2801`, `WIN-B-2802`).
6. `snapshot` each window's transcript.

### Success condition
Two harnessd processes are running, one per workspace, on different ports. Each window's transcript contains only its own token. Each window's toolbar shows its own project name. Cross-contamination of transcripts, models, or extra directories between windows is a state-scoping failure.

### Edge cases
- With `HARNESS_WORKSPACE` set, ⌘N produces a second window on the **same** workspace and therefore a **second harnessd on the same directory**, both writing `.harness/conversations.db`. Test this explicitly and report any corruption or conflict — two servers sharing one SQLite file is a real hazard.

---

## STORY-125: Closing a window does not stop its server

**Type**: medium
**Topic**: Window management — resource leak
**Persona**: Ada, closing a window she is done with
**Goal**: Not leave processes behind
**Preconditions**: Two windows, two projects, both servers running; PIDs noted.
**Entry**: ⌘W
**Window state**: Two windows → one
**Ideal path**: 1.
**Alternate paths**: The rail's close-project button, which **does** call `shutdown()` (STORY-006).

### Steps
1. Note both harnessd PIDs.
2. Focus window 2; `key cmd+w`.
3. `window` → confirm one window remains.
4. Wait 15s; check whether window 2's harnessd is still alive.

### Success condition
Record the outcome. `AppShell` has no `onDisappear` and `ProjectView` has no window-close hook, so the expected result is an **orphaned harnessd** surviving its window. Confirm and report, comparing against the rail button path which does terminate cleanly. Also quit the app entirely (⌘Q) and check whether orphans survive that too.

---

## STORY-126: The app works in a small window

**Type**: medium
**Topic**: Window sizing
**Persona**: Ada on a laptop with a split screen
**Goal**: Use the app at its minimum size
**Preconditions**: A project open with a transcript containing a code block, a tool row, and a selected inspector item.
**Entry**: Window resize
**Window state**: Main window at minimum
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Resize the window to exactly 960×600 (its declared minimum).
2. `snapshot` the Chat section.
3. Attempt to drag the split further so the left pane is at its 400pt minimum.
4. Visit each of the five sections at this size and `snapshot`.

### Success condition
At 960×600 the transcript, the status bar, and the composer are all visible and non-overlapping; the inspector keeps at least its 380pt minimum; no control is clipped out of the tree. The status bar's row — status text, usage label, copy button, `Stop` — must not truncate the `Stop` button out of reach while a run is active. In Settings → Models the two-pane split (240 + 380 minimum) must still fit. Attempt resizing below the minimum and confirm the window refuses.

---

## STORY-127: Dark mode

**Type**: short
**Topic**: Appearance
**Persona**: Ada, working at night
**Goal**: Read the app in dark mode
**Preconditions**: A transcript containing a user bubble, an assistant reply with a code block, a green-checked tool row, a red error row, and an orange notice row.
**Entry**: System appearance switch
**Window state**: Main window
**Ideal path**: n/a.
**Alternate paths**: none — the app has no appearance setting of its own.

### Steps
1. In light mode, `snapshot` and capture the Chat section.
2. Switch macOS to dark mode.
3. `snapshot` the same section, plus Settings → Models and Activity.

### Success condition
Every text element remains present in the tree and legible. Specifically check the hand-rolled colour opacities that do not adapt automatically: the user bubble (`Color.accentColor.opacity(0.15)`), the code block and inspector panels (`.quaternary.opacity(0.4)`), the error row (`Color.red.opacity(0.1)`), the notice row (`Color.orange.opacity(0.1)`), the plan and question panels (`Color.accentColor.opacity(0.08)`), and the icon rail (`.quaternary.opacity(0.25)`). Report any surface where foreground and background collapse to near-identical values.

---

## STORY-128: Increase Contrast and larger text

**Type**: medium
**Topic**: Accessibility
**Persona**: Ada, using accessibility settings
**Goal**: Read the app with system accessibility options on
**Preconditions**: A populated Chat section.
**Entry**: System Settings → Accessibility
**Window state**: Main window
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Enable Increase Contrast; `snapshot` Chat, Activity, and Settings → Providers.
2. Enable Reduce Motion; send a prompt and observe whether the auto-scroll animation still plays.
3. Raise the system text size; `snapshot` Chat and Settings → Models.

### Success condition
No text is clipped or overlapped. In particular: the status bar row (which packs five elements horizontally with no wrap) and the Settings → Models row (toggle + name + context + price cell) are the two most likely to break at larger sizes — check them explicitly. With Reduce Motion on, the 0.12s scroll animation (`ChatView.swift:71`) should ideally be suppressed; record whether it is.

---

## T. Errors, Empty States & Edge Cases

## STORY-129: Server dies mid-run

**Type**: medium
**Topic**: Errors
**Persona**: Marcus, whose daemon crashed
**Goal**: Not be left with a spinner forever
**Preconditions**: A run in flight.
**Entry**: Kill the harnessd process out of band
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Send `Count slowly from 1 to 300.`
2. Once streaming, `kill -9` the harnessd child.
3. `snapshot` every 2s for 60s.

### Success condition
Within 60s the status leaves `Working` and reads `Failed`, the spinner stops, an error row appears carrying the transport error, and the composer becomes usable again. A permanent spinner is the exact failure `markFailed()` exists to prevent (`RunSession.swift:184-187`). Then attempt another prompt and record whether the app offers any route back — expected finding: it does not; only closing and reopening the project restarts the server.

---

## STORY-130: Server restart is not offered after a mid-session death

**Type**: short
**Topic**: Errors / recovery gap
**Persona**: Marcus
**Goal**: Get working again
**Preconditions**: STORY-129 state — harnessd is dead, the app still shows Chat.
**Entry**: Every section
**Window state**: Main window
**Ideal path**: 1 — a Reconnect button.
**Alternate paths**: Close project → reopen.

### Steps
1. With the server dead, visit each of the five sections and `snapshot`.
2. Look for any retry, reconnect, or restart affordance.

### Success condition
Confirm that no retry control exists on the Chat, Activity, Sessions, or Checkpoints sections — `Try Again` only exists on the startup-failure screen, which is only reachable when the *initial* start fails. Every data pane will silently show empty results (all the `try?` calls swallow errors). Report as: after a mid-session server death the app degrades to silently-empty panes with no error and no recovery path.

---

## STORY-131: Sending with no provider configured

**Type**: short
**Topic**: Errors / first-run
**Persona**: Priya on a machine with no keys
**Goal**: Find out what she needs to do
**Preconditions**: No provider keys set; no `OPENAI_API_KEY` in the environment.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: 0 — the app should say so before she types.
**Alternate paths**: Settings → Providers shows unconfigured rows, but only if she goes looking.

### Steps
1. On a fresh project with no credentials, `snapshot` the Chat section before typing anything.
2. `click` the model chip; `snapshot`.
3. Send `Say hello.`; `snapshot` after failure.

### Success condition
Record whether step 1 gives any warning — expected: none; the composer looks fully functional. Step 2's menu should contain `Server default` plus the hidden-models caption. Step 3 must produce a `Failed` status and an error row naming the missing credential. Report the gap between "looks ready" and "cannot run" as a first-run finding.

---

## STORY-132: A very long single reply

**Type**: medium
**Topic**: Performance / rendering
**Persona**: Marcus asking for a big dump
**Goal**: Read a long output without the app stalling
**Preconditions**: Project ready.
**Entry**: Composer
**Window state**: Chat
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Send `Print the numbers 1 to 800, one per line, then reply LONGOUT-3001.`
2. `snapshot` every 5s during streaming, timing each snapshot.
3. After `Done`, click the whole-conversation copy button and check the clipboard length.

### Success condition
The app stays responsive — snapshots return within a few seconds throughout — the transcript ends with `LONGOUT-3001`, and the copy captures the full text. Note the markdown parser reruns over the entire accumulated message on **every streamed token** (`MarkdownBlock.parse` is called in `AssistantBubble.body`); if snapshot latency grows visibly as the message lengthens, that is the cause and it is worth reporting with the measured timings.

---

## STORY-133: A tool producing very large output

**Type**: medium
**Topic**: Performance / rendering
**Persona**: Marcus
**Goal**: Inspect a big tool result
**Preconditions**: Project ready; a large file in the workspace.
**Entry**: Transcript tool row → inspector
**Window state**: Chat
**Ideal path**: 1.
**Alternate paths**: none.

### Steps
1. Send `Read the largest file in this repository, then reply BIGREAD-3101.`
2. After `Done`, `click` the `read` tool row; time the `snapshot`.
3. `snapshot` the inspector.

### Success condition
The inspector renders the output within a few seconds and the tool row's one-line summary is truncated in the middle (never expanded to full height). The `Output` panel is a single `Text` in a horizontal scroll view with no virtualisation — if the click stalls, report the measured delay and the size threshold at which it appears. Contrast with the diff view, which *is* virtualised (`DiffView.swift:32`).

---

## STORY-134: Rapid section switching during an active run

**Type**: medium
**Topic**: Stability
**Persona**: Marcus, restless
**Goal**: Not break the app by clicking around
**Preconditions**: A run in flight.
**Entry**: Icon rail
**Window state**: All sections
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Start a long run.
2. Click through all five rail buttons twice in quick succession, snapshotting after each with blackout retries.
3. Return to Chat; `snapshot` every 2s until the run finishes.

### Success condition
The app does not crash or leave a blank pane. The run continues to completion and the Chat transcript is intact and complete — no lost tokens, no duplicated rows. The SSE stream lives on `RunSession`, which outlives the view, so unmounting Chat must not interrupt it; a truncated transcript here would be a serious streaming bug.

---

## STORY-135: Two projects, two conversations, no cross-talk

**Type**: long
**Topic**: State isolation
**Persona**: Ada, running two repos
**Goal**: Keep everything separate
**Preconditions**: Two windows, two distinct git repos.
**Entry**: Two windows
**Window state**: Two main windows
**Ideal path**: n/a.
**Alternate paths**: none.

### Steps
1. Open repo A in window 1, repo B in window 2.
2. In window 1: select a distinct model, turn on plan mode, add an extra directory, and send `ISO-A-3201`.
3. In window 2: leave defaults and send `ISO-B-3202`.
4. `snapshot` both windows' Chat, Settings → Project, and Settings → Access panes.
5. In each window, click the Sessions rail button; `snapshot`.

### Success condition
Window 1: model chip shows its chosen model, plan mode `On`, one extra directory, transcript contains only `ISO-A-3201`. Window 2: `Server default`, plan mode `Off`, no extra directories, transcript contains only `ISO-B-3202`. Each Sessions list contains only its own workspace's conversations. Any bleed is a state-ownership failure.

---

## Coverage Matrix

| Area | Stories | Known gaps left uncovered |
|---|---|---|
| Launch & project lifecycle | 001-006 | State restoration across app quit (not implemented) |
| Conversation core | 007-017 | Cost limits, max steps, reasoning effort — no UI |
| Transcript rendering | 018-027 | Nested lists, tables (documented out of scope) |
| Copy controls | 028-031 | — |
| Model selection | 032-036 | — |
| Settings — Providers | 037-041 | Key deletion (no UI) |
| Settings — Models | 042-048 | Per-model context/modality editing |
| Settings — Project | 049-053 | Profile contents are opaque |
| Settings — Access | 054-057 | Grants are not persisted by design |
| Plan mode | 058-062 | Custom plan file path (`plan_file` unsettable) |
| Approvals & questions | 063-067 | Approval rules editing (no UI) |
| Sessions | 068-074 | Pinning (decoded, never set); server-side search |
| Checkpoints & rewind | 075-079 | Forced rewind (no UI at all) |
| Activity | 080-084 | All task actions (no UI at all) |
| **Delayed callbacks** | **085-095** | Scheduling UI, cancel UI, fire notification |
| **Cron** | **096-104** | Entire cron UI; execution history |
| **Workflows** | **105-110** | Entire workflow UI; live progress |
| **Subagents** | **111-120** | Entire subagent UI; per-child transcripts |
| Native platform | 121-128 | Menu bar, shortcuts, notifications, drag/drop, URL schemes, dock menu — none exist |
| Errors & edge cases | 129-135 | Offline/sleep behaviour |

---

## Redundancy Candidates

### Duplicate paths (same goal, multiple routes)
- **Start a new conversation**: composer footer `New` (STORY-013) and Sessions toolbar `New` (STORY-074). Identical label, near-identical effect, the second also jumps section.
- **Open a saved conversation**: row tap and context-menu `Open` (STORY-070) — the same handler, two affordances on one row.
- **Get a conversation out of the app**: status-bar `Copy the whole conversation` (plain text) vs Sessions `Export Transcript…` (JSONL). Same goal, different fidelity, no cross-reference between them.
- **Fetch a provider's models**: the empty-state button and the header button (STORY-043) — two controls, one action, both visible when the list is empty.
- **Roll back**: Settings → Project `Undo Last Prompt` (history only) vs Checkpoints `Restore` (history + files). Overlapping, differently confirmed — one has an alert, one has nothing.
- **Change model**: composer chip (live) vs Settings → `ModelsTab` row tap — but `ModelsTab` is never rendered (`.models` maps to `ModelSettingsTab`). Verify at runtime; if unreachable it is dead code, not a duplicate path.

### Duplicate information (same fact, multiple surfaces)
- **Selected model**: composer chip and Settings → Project → `Model`.
- **Plan mode state**: composer checkbox, chat status text (`Plan mode — ready`), and Settings → Project → `Plan mode`.
- **Provider credential health**: Settings → Providers (seal icon), Settings → Models (`no working credential`), and the model chip's hidden-models caption. Three renderings of one fact, with three different wordings.
- **Model price**: model chip menu entries and Settings → Models price cells.
- **Conversation cost**: Sessions row and the chat status bar's usage label.
- **`project.statusMessage`**: rendered simultaneously in the chat status bar and as a Providers-tab toast, and never cleared (STORY-041).
- **Pending callbacks**: Activity → Background work and `list_delayed_callbacks`, with different fields exposed by each.
- **Cron jobs**: Activity → Background work and `cron_list`, likewise.

### Overlapping features
- `spawn_agent` (synchronous fork) and `start_subagent` + `wait_subagent` (async pair) reach the same outcome by different routes; nothing in the UI distinguishes them.
- Delayed callbacks and cron both schedule future work, with completely different execution models (a harness run vs a shell command) and no shared surface explaining the difference.
- The two workflow systems (`internal/workflow` script workflows behind `create_workflow`/`run_workflow`, and `internal/workflows` declarative workflows behind `/v1/workflows`) — the app reaches neither, and the tools reach only the first.

---

## Daemon capabilities the app cannot reach at all

Each of these is exposed by harnessd and has **no path** from the Mac app's UI, and in most cases no `HarnessKit` client method either.

**Blocked by configuration, permanently:**
1. **Run listing / history.** `HarnessSupervisor` never sets `HARNESS_RUN_DB`, so `GET /v1/runs` answers 501 for every app-supervised daemon. Activity → Runs is dead for the app's entire supervised lifetime (STORY-082). This also removes the only surface where a delayed-callback run or a subagent run could be found after the fact.

**Routes that exist but no client method calls:**
2. **Cancel a delayed callback** — `POST /v1/callbacks/{id}/cancel`.
3. **All cron management** — `GET/POST /v1/cron/jobs`, `GET/PATCH/DELETE /v1/cron/jobs/{id}`, `POST .../pause`, `POST .../resume`.
4. **Subagent control** — `POST /v1/subagents`, `POST /v1/subagents/{id}/wait`, `POST /v1/subagents/{id}/cancel`.
5. **Bash job control** — `POST /v1/jobs/{id}/kill`, `GET /v1/jobs/{id}/output`.
6. **Both workflow systems** — `/v1/workflows`, `/v1/workflow-runs/{id}/resume|events`, `/v1/script-workflows`, `/v1/script-workflow-runs/{id}/resume|events`.
7. **Run context and compaction** — `GET /v1/runs/{id}/context`, `POST /v1/runs/{id}/compact`, `GET /v1/runs/{id}/summary`, `POST /v1/runs/{id}/continue`. The app *renders* compaction rows but cannot trigger one.
8. **Conversation compaction** — `POST /v1/conversations/{id}/compact`.
9. **Skills** — `/v1/skills`, `/v1/skills/{name}/verify`.
10. **Recipes** — `/v1/recipes`.
11. **Hooks** — `/v1/hooks`.
12. **MCP servers** — `/v1/mcp/servers`.
13. **Agent networks** — `/v1/networks`.
14. **Checkpoints route family** — `/v1/checkpoints/` (distinct from the conversation rewind points the app does use).
15. **Tool catalog** — `/v1/tools`. The user cannot see what tools exist, which is what makes every deferred-tool story require the user to already know the tool's name.
16. **Code search** — `/v1/search/code`.
17. **Config reload** — `/v1/config/reload`.
18. **Visualisation** — `/viz`.
19. **Relay / worker placement** — the whole `/v1/relay/*` family.
20. **Agents** — `POST /v1/agents`.
21. **Summarise** — `POST /v1/summarize`.

**Reachable only by asking the model, with no UI:**
22. Scheduling a delayed callback (there is no HTTP route either — the tool is the only path daemon-wide).
23. Creating, running, or listing workflows.
24. Fanning out a swarm (`agent_swarm` has no HTTP equivalent).
25. Messaging a running subagent (`message_subagent` has no route).
26. Cron execution history (harnessd has no history route; only the standalone `cronsd` does).

**Run-request fields the client models but no UI sets:**
27. `systemPrompt`, `maxSteps`, `maxCostUSD`, `reasoningEffort`, `allowedTools`, `deniedTools`, `workspaceType`, `planFile`, `providerName`. All nine are declared on `StartRunRequest` (`HarnessClient.swift:48-88`) and never assigned outside `submit()`, which sets only model, conversation, plan mode, extra dirs, profile, and fallback.

**Client methods with no caller:**
28. `HarnessClient.events(runID:lastEventID:)`'s resume parameter — `Transcript.lastEventID` is tracked on every event and never used to reconnect. A dropped stream is not resumed; the run is marked failed instead.
29. `undo(count:toStep:)`'s `toStep` — only `count: 1` is ever sent.
30. `conversations(limit:search:)`'s `search` — the UI filters client-side over the first 100 instead.

**Declared-but-dead:**
31. `HarnessEventType.spawnAgentStarted`, `.spawnAgentCompleted`, `.taskCompleted` — modelled by the app, never emitted by the daemon (STORY-119).
32. `TaskInfo.isCancellable` and `TaskInfo.actions` — decoded from `/v1/tasks`, never read by any view (STORY-084).
33. `RunSession.recallPreviousPrompt()` and `promptHistory` — documented as Up/Down recall, never called (STORY-014).
34. `TranscriptView.pinnedToBottom` — initialised `true`, never assigned, so the scroll guard is permanently open (STORY-015).
35. `SettingsView.ModelsTab` — defined and never instantiated; `.models` renders `ModelSettingsTab`.
