# TUI User Path Test Results — 2026-03-18

**Tester**: Claude Code (automated testing via tmux)
**Binary tested**: `harnesscli` rebuilt at 21:49 from latest source
**Server**: harnessd at localhost:8080
**Terminal**: tmux session `harness-test:1`, 220x50 columns
**Date**: 2026-03-18

---

## Executive Summary

All 20 user paths were tested against the latest rebuilt binary. The TUI is functionally solid with several bugs found, mostly UX-level issues. The single critical finding from the initial test (Level-1 config panel not appearing) was caused by the **deployed binary being stale** — the binary pre-dated the Level-1 config panel implementation. After rebuilding, Level-1 config works correctly. Six bugs were found, plus several UX observations.

---

## Test Environment Notes

- Initial binary at `harnesscli` was built 2026-03-18 17:24, **before** `feat: unified /model command with config panel (Level-0 + Level-1)` was committed at 18:41.
- The binary was rebuilt to 21:49 (latest code) before completing the tests.
- The tests below reflect the **new binary** unless otherwise noted.

---

## Path Results

### PATH 1: Initial State Check
**Status**: PASS with observations

The initial screen shows:
```
────────────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────────────
```
- Status bar at bottom shows nothing (no model configured initially)
- No hint text or guidance for new users
- After model selection from config file, status bar shows model name (e.g., `GPT-4.1 ↗OR`)
- No top status bar — model is in the bottom status bar
- Input area is active and accepting keystrokes

**UX Note**: New users see a completely blank screen with just an input prompt and no guidance on what to type or how to select a model.

---

### PATH 2: Type /help
**Status**: PASS (with autocomplete-double-Enter workaround needed)

After the double-Enter workaround (see BUG-1), the help dialog opens correctly:

```
╭──────────────────────────────────────────────────────────────────╮
│             Commands    Keybindings    About                      │
│──────────────────────────────────────────────────────────────────│
│  /clear      Clear conversation history                           │
│  /context    Show context usage grid                              │
│  /export     Export conversation transcript to a markdown file    │
│  /help       Show help dialog                                     │
│  /keys       Manage provider API keys                             │
│  /model      Switch model, gateway, and API keys                  │
│  /provider   Switch routing gateway (use /model for per-model     │
│config)                                                            │
│  /quit       Quit the TUI                                         │
│  /stats      Show usage statistics                                │
│  /subagents  List managed subagents and their isolation state     │
╰──────────────────────────────────────────────────────────────────╯
```

- 10 commands listed (test spec expected 9 — /subagents is extra, recently added)
- **BUG-2**: `/provider` description text wraps mid-word outside the dialog box
- **BUG-3**: "Keybindings" and "About" tabs visible but inaccessible (no key binding wired)
- **BUG-4**: When help overlay is open, keyboard input goes to the input area, not the overlay

---

### PATH 3: Close help with Escape, open /stats
**Status**: PASS

- Escape closes the help dialog correctly
- `/stats` opens the activity chart:
```
Activity (last 7 days)  [r to toggle period]

Mon  ░
Tue  ░
Wed  █
...
Total runs: 1   Total cost: $0.00
```
- Stats display is inline (replaces viewport), not a floating overlay
- **BUG-5**: The `[r to toggle period]` hint doesn't work — the 'r' key always goes to the input area, period toggle is inaccessible

---

### PATH 4: Test /model command
**Status**: PASS

The model switcher overlay opens with full model list:
- Models organized by provider (Anthropic, DeepSeek, Google, Groq, Kimi, OpenAI, OpenRouter, Qwen, Together, xAI)
- Availability indicators: `●` = provider key configured, `○` = unconfigured, `(unavailable)` text shown for unconfigured providers
- Navigation hint: `↑/↓ navigate  enter select  s star  esc cancel`

---

### PATH 5: Navigate model list, select a model
**Status**: PASS

- Up/Down navigation works correctly
- Provider headers are skipped during navigation (non-selectable)
- `← current` marker shows currently selected model
- **BUG-6**: Long model names wrap incorrectly — `"GPT-4.1 ●"` displayed as `"GPT-"` on one line and `"4.1 ●"` on the next line in certain terminal states

---

### PATH 6: Search in model list
**Status**: PASS

Typing characters in the model switcher filters the list:
```
Filter: gpt

>  GPT-4.1  ← current ●
   GPT-4.1 Mini ●
   gpt-5.1-codex ●
   ... (all GPT models)
```
- Real-time filtering works
- Backspace removes characters from filter
- Escape with active filter clears filter (then second Escape closes overlay)

---

### PATH 7: Select first filtered result
**Status**: PASS

Pressing Enter on a filtered result enters the Level-1 config panel (with new binary).

---

### PATH 8: Level-1 Config Panel — navigate to API key section
**Status**: PARTIAL PASS (key UX bug found)

The Level-1 config panel shows correctly:
```
╭──────────────────────────────────────────────────╮
│                                                  │
│  GPT-4.1                                         │
│  OpenAI                                          │
│                                                  │
│  Gateway                                         │
│  ▶ Direct       Use each model's native provider │
│    OpenRouter   Route all models via openrouter  │
│                                                  │
│  API Key    ● configured  (enter to update)      │
│                                                  │
│  ↑/↓ sections  ←/→ gateway  enter confirm  back  │
╰──────────────────────────────────────────────────╯
```

Navigation:
- Down moves focus from Gateway to API Key section (shown by `(enter to update)` hint appearing)
- Left/Right changes gateway selection

**BUG-7**: The hint `(enter to update)` in the API Key section is misleading — pressing Enter in the API Key section **confirms and closes** the config panel (because the Submit key handler runs before the config panel key handler). Only the `K` key triggers key input mode.

**Visual Bug**: Left border `╭` is often cut off in captures, suggesting the panel centering may have an off-by-one pixel alignment issue.

When `K` is pressed in the API Key section:
```
│  API Key    ● configured          │
│  > ▌                              │
│  enter confirm  ctrl+u clear  esc │
│                                   │
│  ↑/↓ sections  ←/→ gateway  enter │
```
**BUG-8**: When in key input mode, BOTH the key input hint AND the config panel navigation hint are shown simultaneously, which is visually confusing.

**Security Note**: API keys are displayed in plaintext while typing (not masked). For terminal security this is a consideration worth noting.

---

### PATH 9: Confirm model selection, verify status bar
**Status**: PASS

- Confirming via Enter in Level-1 config closes the overlay
- Transient status message: `"Model: GPT-4.1"` (3 seconds)
- Persistent status bar: `"GPT-4.1"` or `"GPT-4.1 ↗OR"` (when OpenRouter gateway active)

---

### PATH 10: Run a simple message
**Status**: PASS (with correct provider configured)

- Anthropic API key expired (credit balance zero) — run fails cleanly
- OpenAI API key invalid — run fails with error displayed
- OpenRouter gateway with valid key: **SUCCESS**

Successful run output:
```
❯ Say the word four

four
```

- User message prefixed with `❯`
- Response streams correctly
- Error messages are well-formatted with `✗` prefix and structured error details

---

### PATH 11: After response, check status bar for cost/tokens
**Status**: FAIL

- Status bar shows `"GPT-4.1 ↗OR"` only — no cost or token count
- **BUG-9**: `statusBar.SetCost()` is never called from `model.go`, so cost never appears in the persistent status bar even after successful runs. Cost data IS tracked in `m.cumulativeCostUSD` but is never forwarded to the status bar component.
- The `/stats` panel shows `Total cost: $0.00` even after successful runs — the usage.delta event may not include cost data for OpenRouter via the configured model.

---

### PATH 12: Test /context
**Status**: PASS

```
Context Window Usage

  [█░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]

  Used:  6412 tokens
  Total: 200000 tokens
  Usage: 3.2%
```

- Shows token usage bar correctly
- Escape closes the overlay

---

### PATH 13: Test /keys command
**Status**: PASS (with minor visual bug)

```
╭──────────────────────────────────────────────────────╮
│  API Keys                                            │
│                                                      │
│  ▶ anthropic      ANTHROPIC_API_KEY        ● set     │
│    deepseek       DEEPSEEK_API_KEY         ○ unset   │
│    gemini         GOOGLE_API_KEY           ● set     │
│    groq           GROQ_API_KEY             ○ unset   │
│    kimi           MOONSHOT_API_KEY         ○ unset   │
│    openai         OPENAI_API_KEY           ● set     │
│    openrouter     OPENROUTER_API_KEY       ● set     │
│    qwen           DASHSCOPE_API_KEY        ○ unset   │
│    together       TOGETHER_API_KEY         ○ unset   │
│    xai            XAI_API_KEY              ● set     │
│                                                      │
│  ↑/↓ navigate  enter edit  esc close                 │
╰──────────────────────────────────────────────────────╯
```

- Shows all 10 providers with env var names and status
- Left border `╭` cut off in some orientations (same alignment bug as Level-1 config)

---

### PATH 14: In /keys, navigate to OpenRouter and set key
**Status**: PASS

- Navigation to provider works
- Enter opens key input mode correctly:
```
│  API Keys > openrouter               │
│                                      │
│  OPENROUTER_API_KEY                  │
│                                      │
│  > ▌                                 │
│                                      │
│  enter confirm  ctrl+u clear  esc back│
```
- Keys display in plaintext while typing
- After Enter, confirmation: `"Key saved for openrouter"` in status bar
- Overlay returns to provider list after saving

---

### PATH 15: Test /provider command
**Status**: PASS (with text wrap bug)

```
╭────────────────────────────────────────────╮
│  Routing Gateway                           │
│                                            │
│  ▶ Direct       Use each model's native    │
│  provider                                  │
│    OpenRouter   Route all models via       │
│  openrouter.ai                             │
│                                            │
│  ↑/↓ navigate  enter confirm  esc close    │
╰────────────────────────────────────────────╯
```

**BUG-10**: Gateway descriptions wrap within the border box incorrectly — `"Use each model's native provider"` wraps to next line showing `"provider"` misaligned. Same for OpenRouter description. This is a content-width calculation issue in `viewProviderOverlay()`.

---

### PATH 16: Switch to OpenRouter gateway
**Status**: PASS

- Down arrow moves to OpenRouter
- Enter confirms the selection
- Transient status: `"Gateway: OpenRouter"` (3 seconds)
- Persistent status bar: `"GPT-4.1 ↗OR"` — the `↗OR` indicator works correctly!

---

### PATH 17: Multi-line input test
**Status**: PASS (Ctrl+J only — Shift+Enter unreliable in tmux)

```
❯ Line one of my message
  Line two of the message
```

- `Ctrl+J` adds a newline correctly
- Continuation lines are indented under the prompt
- **Note**: `Shift+Enter` mapped in keys.go but unreliable via tmux; `Ctrl+J` is the reliable method

---

### PATH 18: Slash autocomplete test
**Status**: PASS

Typing `/` shows the autocomplete dropdown:
```
▶ /clear      Clear conversation history
  /context    Show context usage grid
  /export     Export conversation transcript to a markdown file
  /help       Show help dialog
  /keys       Manage provider API keys
  /model      Switch model, gateway, and API keys
  /provider   Switch routing gateway (use /model for per-model config)
  /quit       Quit the TUI
  ... 2 more
```

- Dropdown shows 8 of 10 commands with `... 2 more` overflow indicator
- `/stats` and `/subagents` are the hidden two
- The dropdown correctly filters as more characters are typed
- **UX Issue**: The overflow indicator `... 2 more` doesn't tell which commands are hidden

---

### PATH 19: Test Escape key behavior
**Status**: PASS

- Escape with text in input clears input
- Transient status: `"Input cleared"` (3 seconds)
- Double Escape: first clears input, second is a no-op (correct — does not quit)

---

### PATH 20: Test /export
**Status**: PASS

```
Transcript saved to /Users/dennisonbertram/develop/go-agent-harness/transcript-20260318-215851.md
```

- File is created with correct Markdown format:
```markdown
# Conversation Transcript
Exported: 2026-03-18 21:58:51

---

## User [9:51 PM]
What is 2 plus 2? Answer in one word.

---

## Assistant [9:56 PM]
four
```

- Only successful run content in transcript (errors not included — correct behavior)
- File path shown in status bar confirmation

---

## Bug Report

### BUG-1: Slash commands require two Enter keypresses (autocomplete interaction)
**Severity**: High
**Path**: All slash command paths
**Observed**: When typing a slash command (e.g., `/model`) and pressing Enter, the first Enter accepts the autocomplete suggestion (fills the text buffer with `/model `) without executing. A second Enter is required to actually execute the command.
**Expected**: A single Enter should execute the slash command.
**Root Cause**: In `model.go` Submit handler (line 638): when `m.slashComplete.IsActive()` is true, Enter calls `slashComplete.Accept()` which sets the input value but does NOT submit it. The user must press Enter again to trigger `inputarea.CommandSubmittedMsg`.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` lines 638-645
**Reproduction**: Type `/model` (autocomplete shows), press Enter once (fills buffer, no overlay), press Enter again (opens model switcher).
**Fix Suggestion**: When `slashComplete.Accept()` produces a complete slash command (no additional args needed), immediately submit it as a `CommandSubmittedMsg`.

---

### BUG-2: Help dialog /provider description text wraps outside dialog box
**Severity**: Low
**Path**: PATH 2 (/help)
**Observed**: `/provider` command description `"Switch routing gateway (use /model for per-model config)"` wraps with `"config)"` appearing outside the dialog border on the next line.
**Expected**: Text should either be truncated or the column width should accommodate the description.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/components/helpdialog/` — description column width calculation
**Reproduction**: Open `/help`, observe the `/provider` row.

---

### BUG-3: Help dialog Keybindings and About tabs are inaccessible
**Severity**: Medium
**Path**: PATH 2 (/help)
**Observed**: The help dialog shows three tabs: "Commands | Keybindings | About". No keyboard shortcut exists to switch between tabs. Tried: Tab key, Left/Right arrows — none work. Keys go to the input area, not the dialog.
**Expected**: Tab or Left/Right should navigate between the three help sections.
**Root Cause**: `helpdialog.Model` has `NextTab()` and `PrevTab()` methods implemented but they are never called from `model.go`.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` — help dialog key handling is missing entirely
**Reproduction**: Open `/help`, try Tab, Right arrow, Left arrow — no tab switching occurs.

---

### BUG-4: Help dialog receives no keyboard focus — all keys go to input area
**Severity**: Medium
**Path**: PATH 2 (/help)
**Observed**: When the help dialog is open, any character typed goes into the input area below the dialog. For example, pressing 'g' while help is open appends 'g' to the input buffer.
**Expected**: When a modal overlay is open, keyboard input should be captured by the overlay, not the input area.
**Root Cause**: The `model.go` Update function does not intercept regular `tea.KeyRunes` events when `m.activeOverlay == "help"`. The key falls through to the default branch which passes it to `m.input.Update()`.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` — KeyMsg switch statement lacks a `case m.overlayActive && m.activeOverlay == "help":` branch
**Reproduction**: Open `/help`, type any character — it appears in the input below the dialog.

---

### BUG-5: /stats 'r' period toggle is broken — key goes to input area
**Severity**: Medium
**Path**: PATH 3 (/stats)
**Observed**: The stats panel displays `"Activity (last 7 days)  [r to toggle period]"`. Pressing 'r' while stats is open types 'r' into the input area instead of toggling the period.
**Expected**: 'r' should toggle the stats period between 7-day and 30-day views.
**Root Cause**: `statspanel.Model` has `TogglePeriod()` implemented but `model.go` does not have a key handler for 'r' when `m.activeOverlay == "stats"`.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` — missing stats overlay key handler
**Reproduction**: Open `/stats`, press 'r' — period label stays "last 7 days", 'r' appears in input.

---

### BUG-6: Model list items with long names wrap across two lines in the list
**Severity**: Medium
**Path**: PATH 5, PATH 6 (/model list)
**Observed**: The model `"GPT-4.1 ●"` is displayed as:
```
│ >   GPT-    │
│ 4.1 ●       │
```
The name breaks at a hyphen and the continuation line is NOT indented, making it look like `"4.1 ●"` is a separate item.
**Expected**: Model names should not wrap, or if they do, the continuation should be clearly indented and the cursor `>` should span both lines.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/components/modelswitcher/view.go` — row rendering/width calculation
**Reproduction**: Open `/model`, scroll to OpenAI section, observe "GPT-4.1" and "openai/gpt-4.1-mini" entries.
**Note**: This bug was confirmed in **both** old and new binaries. The width is 220 columns but the model panel has a fixed inner width that clips names.

---

### BUG-7: Level-1 config panel — "enter to update" API Key hint is misleading
**Severity**: High
**Path**: PATH 8 (Level-1 config panel)
**Observed**: When navigating to the API Key section in Level-1 config, the hint shows `"(enter to update)"`. But pressing Enter **confirms and closes** the entire config panel instead of entering key input mode. Only the `K` key (capital K) enters key input mode.
**Expected**: Enter in the API Key section should enter key input mode (as hinted).
**Root Cause**: The Submit key handler (`key.Matches(msg, m.keys.Submit)`) runs before the config panel inner switch can set `m.modelConfigKeyInputMode = true`. The `"Enter in config panel (not in key input) → confirm and close"` code path executes first.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` lines 607-626 (Submit handler) vs 705-710 (config panel K/Enter handler)
**Reproduction**: Open `/model`, Enter to select, Down to API Key section, press Enter — overlay closes instead of entering key input mode.

---

### BUG-8: Level-1 config panel shows duplicate navigation hints in key input mode
**Severity**: Low
**Path**: PATH 8 (Level-1 config panel, key input mode)
**Observed**: When key input mode is active (pressed K), the config panel shows both:
1. `"enter confirm  ctrl+u clear  esc back"` (key input mode footer)
2. `"↑/↓ sections  ←/→ gateway  enter confirm  esc back"` (config panel navigation footer)
Both are visible simultaneously.
**Expected**: When in key input mode, only the key input mode footer should be shown.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` `viewModelConfigPanel()` function — the outer navigation hint renders unconditionally.

---

### BUG-9: Status bar cost display never updates
**Severity**: Medium
**Path**: PATH 11 (post-run cost check)
**Observed**: After a successful LLM run, the status bar shows only the model name (e.g., `"GPT-4.1 ↗OR"`) with no cost information. The `/stats` panel also shows `$0.00` total cost.
**Expected**: Status bar should show cumulative cost (e.g., `"GPT-4.1 ↗OR ~ $0.0012"`) after runs.
**Root Cause**: `statusBar.SetCost()` is never called in `model.go`. The `m.cumulativeCostUSD` field is updated on `usage.delta` events but never forwarded to the status bar component.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` — `case "usage.delta":` handler (lines 1038-1055) doesn't call `m.statusBar.SetCost(m.cumulativeCostUSD)`
**Reproduction**: Run any successful LLM query, check status bar — no cost shown.

---

### BUG-10: /provider overlay — gateway descriptions wrap misaligned
**Severity**: Low
**Path**: PATH 15 (/provider)
**Observed**: The provider overlay box is narrow (44 chars wide). The gateway description text wraps but the continuation line has no indent:
```
│  ▶ Direct       Use each model's native    │
│  provider                                  │
```
**Expected**: Either widen the box or truncate descriptions to fit.
**Code Location**: `/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnesscli/tui/model.go` `viewProviderOverlay()` function — width is hardcoded as 44 which is insufficient for the descriptions.

---

## Additional Observations (Not Bugs, Design Issues)

### OBS-1: Stale binary was deployed — Level-1 config appeared broken
**Severity**: Critical (deployment process issue)
The binary in `./harnesscli` was built at 17:24 but `model.go` was last modified at 18:41. Two commits adding the Level-1 config panel and availability indicators were present in source but not in the binary. The test session consumed significant time investigating a "bug" that was actually a stale binary. **Recommendation**: Add a build step to CI/CD or document that the binary must be rebuilt before testing.

### OBS-2: Initial empty state provides no onboarding guidance
When the TUI starts with no configured model, the user sees only a separator line and prompt. No hint text guides the user to type `/model` to select a model or `/help` to see available commands. A first-run message like `"Type /help for available commands, /model to select a model"` would improve UX significantly.

### OBS-3: Input history navigation is overridden by viewport scroll
The `inputarea` component implements Up/Down arrow key history navigation, but `model.go` intercepts ALL Up/Down key events for viewport scrolling. As a result, input history is inaccessible. The help dialog documents "up" as "scroll up" — this appears intentional — but users familiar with shell history (Up for previous command) may be surprised. Consider `Ctrl+P`/`Ctrl+N` as alternatives (they are also bound to ScrollUp/ScrollDown).

### OBS-4: /clear provides no visual confirmation
Running `/clear` empties the viewport silently. No status message says "Conversation cleared". Compare to Escape which shows `"Input cleared"`. A brief `"Conversation cleared"` status message would be reassuring.

### OBS-5: API key plaintext display in key input mode
When entering API keys via `/keys` or the Level-1 config K-key mode, the key is displayed in full plaintext while typing. Consider masking as `•` characters (or showing only the last 4 chars) for security. This is especially important if the terminal is being screen-shared.

### OBS-6: "gemini-2.5-flash-preview-04-17" shows as raw API ID
In the model list, `"gemini-2.5-flash-preview-04-17"` appears as the raw API identifier rather than a friendly display name. All other models have friendly names (e.g., "Gemini 2.5 Flash"). This raw ID suggests a model was added to the server catalog without a corresponding entry in `DefaultModels` with a `DisplayName`.

### OBS-7: /stats cost shows $0.00 even after successful runs
After the successful OpenRouter run, `/stats` showed `Total cost: $0.00`. Either: (a) the `usage.delta` SSE event didn't include cost data for OpenRouter, or (b) the cost was $0 for a simple query. Worth investigating whether OpenRouter runs ever report cost via `usage.delta`.

### OBS-8: Slash autocomplete "... N more" hint doesn't name the hidden commands
When typing `/`, the autocomplete shows the first 8 commands then `"... 2 more"`. Users can't see which commands are hidden without typing more characters. Consider showing all 10 commands (the dropdown height allows it) or naming the hidden ones.

---

## What Worked Well

1. **Model availability indicators** (`●`/`○`) clearly show which providers are configured without needing to open a separate settings view.
2. **Level-1 config panel** (with new binary) provides a clean gateway + API key + reasoning effort configuration flow.
3. **Error message formatting** is excellent — `✗` prefix, structured JSON displayed, HTTP status codes shown.
4. **OpenRouter gateway** integration works end-to-end (`↗OR` status indicator, model slug mapping, API call routing).
5. **Transcript export** produces well-formatted Markdown with timestamps.
6. **Context grid** shows accurate token usage with visual bar chart.
7. **Slash autocomplete dropdown** with real-time filtering and description text.
8. **Multi-line input** via Ctrl+J works and displays with proper indentation.
9. **Escape semantics** are well-implemented (input clear → overlay close → run cancel hierarchy).
10. **Model search/filter** in the model switcher is responsive and accurate.
11. **API key management** via `/keys` command is clean and functional.
12. **/provider** overlay quickly switches routing gateway.
13. **Stats panel** correctly records and displays run activity history.

---

*Report generated: 2026-03-18*
