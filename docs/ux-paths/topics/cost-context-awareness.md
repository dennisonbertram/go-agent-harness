# UX Stories: Cost & Context Awareness

**Topic**: Monitoring real-time token spend, reading cumulative USD cost in the status bar, checking context window fill via `/context`, and reviewing historical activity with `/stats`.

**Application**: go-agent-harness TUI (`harnesscli --tui`)

**Status**: Generated 2026-03-23

---

## STORY-CA-001: Watching the Status Bar Cost Counter Update in Real Time

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer using the TUI for the first time after a cost surprise in a previous project
**Goal**: Know exactly how much each exchange costs without leaving the conversation
**Preconditions**: TUI is open, no run is active. Status bar shows the active model name. No cost has been incurred yet — `$0.0000` is absent (cost segment is hidden until the first `usage.delta` event arrives because `costUSD > 0` is required to render it).

### Steps

1. User types a prompt and presses **Enter** → The run starts; `statusbar.Model.running` becomes `true`; the status bar renders `...` (dimmed) next to the model name to indicate activity.
2. Server streams the first `usage.delta` SSE event with `cumulative_cost_usd: 0.0012` → The TUI parses the event, calls `m.statusBar.SetCost(0.0012)`, and re-renders the status bar. The cost segment `$0.0012` appears in the bottom bar (dimmed style, 4 decimal places).
3. Server streams a second `usage.delta` event with `cumulative_cost_usd: 0.0048` → Status bar updates in place; `$0.0048` replaces `$0.0012` without any flicker or delay.
4. Run completes (`SSEDoneMsg`) → The `...` running indicator disappears. The cost figure `$0.0048` remains visible as the session total.
5. User types another prompt and presses **Enter** → A new run starts. When the next `usage.delta` arrives with `cumulative_cost_usd: 0.0091`, the status bar updates to `$0.0091` (cumulative session total, not per-run delta).

### Variations

- **Very long run with many tool calls**: Each tool round-trip generates a `usage.delta` event; the cost counter ticks upward with every event, providing a live meter feel.
- **Narrow terminal**: If the terminal is too narrow to display all segments, lower-priority segments drop first (branch, workdir, MCP failures) before cost. The model name (priority 1) and running indicator (priority 2) drop before cost (priority 3) only at extreme narrowness. Cost reliably stays visible in most real-world terminal widths.

### Edge Cases

- **Zero-cost event**: If a `usage.delta` arrives with `cumulative_cost_usd: 0.0`, the cost segment remains hidden (the `costUSD > 0` guard prevents showing `$0.0000`). This is intentional — the bar is clean until real cost has accrued.
- **Invalid JSON in usage.delta**: The TUI silently ignores the malformed event; the cost counter keeps its last valid value. No error is shown in the status bar.
- **Model name truncated**: The model name is truncated to 24 characters with `...` when it exceeds that length. The cost field is not affected.

---

## STORY-CA-002: Understanding Per-Run Cost vs. Session-Total Cost

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Engineer running multiple short tasks in one TUI session, trying to budget usage
**Goal**: Understand whether the cost shown is "this message" or "this whole session"
**Preconditions**: TUI is open. One prior exchange has already run and cost `$0.0050`. The status bar currently shows `$0.0050`.

### Steps

1. User reads the status bar: `gpt-4o ~ $0.0050` → This is the **session cumulative total**, not the cost of the most recent run. The value accumulates from the first run until `/clear` or TUI restart.
2. User sends a second prompt → Run completes. The next `usage.delta` event carries `cumulative_cost_usd: 0.0098` → Status bar updates to `$0.0098`. The increment of `$0.0048` was the cost of just that second run.
3. User wants to see what that second run cost in isolation → There is no per-run breakdown in the status bar itself. The user opens `/stats` (see STORY-CA-004) to see total cost over a time window, or mentally subtracts the prior value.
4. User types `/clear` → Conversation history and transcript are cleared. The cumulative cost counter is **not** reset; the status bar still shows `$0.0098`. Cost tracks the full session, not just the visible conversation history.

### Variations

- **Multi-day usage**: The cost counter persists for the lifetime of the TUI process. If the user leaves the TUI open overnight and resumes, the counter continues from where it left off. Historical cost-per-day data lives in the stats panel (STORY-CA-004), populated from `usageDataPoints`.

### Edge Cases

- **TUI restart**: After closing and reopening the TUI, the cost counter resets to zero (no persistence across processes). Historical per-day data in the stats panel is loaded from the server, not the in-memory counter.
- **Multiple `usage.delta` events per run**: The event carries `cumulative_cost_usd`, which is the server's running total for the run. The TUI takes the last value received — it does not sum deltas. If events arrive out of order, the last one wins.

---

## STORY-CA-003: Opening the Context Grid to Check Token Fill

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer working on a long codebase exploration task, worried about hitting the context limit
**Goal**: See how much of the 200k-token context window has been consumed
**Preconditions**: TUI is open. Several tool-heavy exchanges have run. The cumulative token count is in the tens of thousands. No overlay is currently open.

### Steps

1. User types `/context` → The autocomplete dropdown appears momentarily (filtered to `/context`), then the command auto-executes. The context grid overlay opens.
2. The overlay displays:
   ```
   Context Window Usage

     [████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]

     Used:  24576 tokens
     Total: 200000 tokens
     Usage: 12.3%
   ```
   The progress bar is filled proportionally (`█` for used, `░` for empty). Width is capped at 60 cells regardless of terminal width.
3. User reads: 24,576 tokens used, 175,424 remaining. At 12.3%, the context window is far from full.
4. User presses **Escape** → The overlay closes. The main chat view is restored. The input area regains focus.

### Variations

- **Using slash command with autocomplete**: Typing `/con` + Tab auto-completes to `/context ` and executes the command — the overlay opens without the user pressing Enter.
- **Opening via keyboard shortcut**: There is no dedicated single-key shortcut for `/context`; it is always opened via slash command.

### Edge Cases

- **Zero tokens used**: If no run has started yet, `UsedTokens` is `0`. The progress bar is entirely empty (`░░░░...`). `Used: 0 tokens`, `Usage: 0.0%`. The overlay still renders correctly.
- **Token count exceeds total**: The `contextgrid.Model` clamps `UsedTokens` to `TotalTokens` before rendering, so the bar never overflows past 100%. `Usage: 100.0%` is the maximum displayed percentage.
- **Default context window**: The `TotalTokens` field defaults to `200000` when not explicitly set, matching the model's actual context window. If the server reports a different model with a different window, `TotalTokens` is updated accordingly; otherwise the user always sees 200k as the denominator.

---

## STORY-CA-004: Opening the Stats Panel to Review Historical Activity

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer who has been using the harness daily for two weeks and wants a sense of usage trends
**Goal**: Get a visual overview of run frequency and spending over the past week
**Preconditions**: TUI has been used on multiple days. `usageDataPoints` contains historical DataPoints from prior `usage.delta` events. No overlay is currently open.

### Steps

1. User types `/stats` + **Enter** → The stats panel overlay opens. The heatmap defaults to the **last 7 days** (`PeriodWeek`) period.
2. The overlay displays:
   ```
   Activity (last 7 days)  [r to toggle period]

   Mon  ░░░░░░░
   Tue  ░░░▒░░░
   Wed  ░░░░░░░
   Thu  ░▒░░░░░
   Fri  ░░░░░░░
   Sat  █░░░░░░
   Sun  ░░░▓░░░

   Total runs: 23   Total cost: $0.41
   ```
   Each cell represents one day. Intensity blocks (`░`, `▒`, `▓`, `█`) are assigned by percentile rank across non-zero counts within the window: bottom quartile = `░`, up to 95th percentile = `▓`, above 95th = `█`.
3. User sees `Total runs: 23   Total cost: $0.41` at the bottom — the aggregate over the visible window.
4. User reads the hint text `[r to toggle period]` and presses **r** → Period cycles to **last 30 days** (`PeriodMonth`). The heatmap re-renders with 30 columns. The total line updates: `Total runs: 87   Total cost: $1.73`.
5. User presses **r** again → Period cycles to **last 365 days** (`PeriodYear`). The heatmap now shows ~52 columns (one per week, 7 rows per column = days of the week). Total line shows full-year totals.
6. User presses **r** one more time → Period wraps back to **last 7 days**.
7. User presses **Escape** → The overlay closes. The main chat view is restored.

### Variations

- **First run ever**: If `usageDataPoints` is empty, the grid renders all `░` cells. `Total runs: 0   Total cost: $0.00`. The header and `r` hint are still shown. This is the valid empty state.
- **All days equally active**: When all data points have the same count, all rank above the 95th percentile (because the sort-and-rank algorithm uses `count+1` as upper bound). All active cells render as `█`.

### Edge Cases

- **Data points with zero-time dates**: Zero-value `time.Time` structs in `usageDataPoints` are silently skipped during aggregation. They do not appear in the grid.
- **Data outside the current window**: If a DataPoint's date is older than the selected period, it is excluded from both the heatmap cells and the totals line. Switching to a longer period may reveal it.
- **Single run today in an otherwise empty history**: Only today's cell shows a non-`░` character. The period total shows `Total runs: 1   Total cost: $X.XX`.

---

## STORY-CA-005: Using Stats to Decide Whether to Start a New Conversation

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer near the end of a multi-hour debugging session, conscious of LLM costs and context limits
**Goal**: Decide whether to continue the current conversation or clear it and start fresh
**Preconditions**: TUI has been running for ~2 hours. Status bar shows `$0.34`. The user has used a significant amount of context.

### Steps

1. User opens `/context` → Context grid shows `Used: 142,000 tokens`, `Total: 200,000 tokens`, `Usage: 71.0%`. The progress bar is about 70% filled. The user can see there is still headroom, but the session is getting heavy.
2. User presses **Escape** to close the context overlay.
3. User opens `/stats` → Stats panel shows the last 7 days heatmap. Today's cell is the densest (`█`). `Total runs: 41   Total cost: $0.34` for the week.
4. User presses **r** → Switches to last 30 days. The weekly pattern is visible; prior weeks show lighter cells. The user identifies that this is an unusually heavy day.
5. User presses **Escape** to close the stats overlay.
6. User evaluates:
   - Context is 71% full — there is still room, but with 29% remaining the model will start to lose early context or degrade performance on very long remaining tasks.
   - Today has been unusually expensive ($0.34 in one day vs. $0.05–$0.12 on typical days as visible in the heatmap).
   - Decision: start a new conversation with `/clear` to reset context and cost tracking for the next subtask.
7. User types `/clear` → Conversation is cleared. The status bar cost counter is NOT reset (it is a session total, not a conversation total). The user starts fresh with a new prompt but carries forward the same session-level cost visibility.

### Variations

- **Context only 10% full**: The user decides to continue without clearing — plenty of headroom remains, and clearing would lose valuable conversation state (prior code context, file contents, etc.).
- **Cost much lower than expected**: The user glances at the status bar, sees `$0.02`, and continues without concern. `/stats` is not needed for simple sanity checks.

### Edge Cases

- **After /clear, context grid still shows old token count**: The `/clear` command does not reset `m.totalTokens` or `m.contextGrid.UsedTokens`. Token count only resets when a new `usage.delta` event arrives from the server and overwrites the cumulative value. This means the context grid may appear "still full" briefly after `/clear`. The user can verify by running a new prompt and watching the token count update.
- **Stats panel shows no data for prior days**: If the user just started using the harness, prior days have no data. This is a valid empty state — the user can still make decisions based on today's real-time cost from the status bar.

---

## STORY-CA-006: Watching the Context Bar Progress Toward Full

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Developer running a large agentic task that processes many files, expecting context pressure
**Goal**: Monitor context fill during the run, ready to intervene before the model degrades
**Preconditions**: TUI is open. A long-running run has been active for several minutes. The user is monitoring the status bar and periodically checking the context grid.

### Steps

1. Run starts. User opens `/context` early: `Used: 8,192 tokens`, `Usage: 4.1%`. The bar is nearly empty. User closes with **Escape**.
2. Several tool calls later — file reads, bash executions, assistant responses. Each `usage.delta` event updates `m.totalTokens` and `m.contextGrid.UsedTokens` in the main model.
3. User opens `/context` again: `Used: 95,000 tokens`, `Usage: 47.5%`. The bar is half-filled. The user notes the rapid growth and anticipates hitting the limit if the task continues at this rate.
4. User continues. More tool calls, more file content accumulated. The `usage.delta` events stream in.
5. User opens `/context` again: `Used: 178,500 tokens`, `Usage: 89.3%`. The progress bar is nearly full — only about 21,500 tokens of headroom remain. The user can visually see the bar is close to the right edge.
6. User decides to finish the current subtask and then run `/clear` before the next one to avoid context exhaustion.
7. User presses **Escape** and monitors the status bar. The run completes. The user issues `/clear` and begins the next task from a clean context state.

### Variations

- **Checking context every few minutes as a habit**: Some users open `/context` regularly during long tasks, similar to checking a battery indicator. The overlay takes one keystroke to open and one to close, making it low-friction.
- **Task that stays small**: If the agent is working on a narrow, self-contained question, context usage may peak at 5–10% and never become a concern.

### Edge Cases

- **Context hits 100%**: The `contextgrid.Model` clamps `UsedTokens` to `TotalTokens`, so the display reads `Usage: 100.0%` with a fully-filled bar. The server may truncate or fail the run at this point; the TUI would show a `run.failed` error appended to the viewport. The context grid itself does not warn or change color — it is a read-only display.
- **Model with different context window**: If the server communicates a `total_tokens` value different from 200,000 (e.g., a model with a 128k window), the `TotalTokens` field on `contextgrid.Model` is updated accordingly. The percentage and bar reflect the correct ratio.

---

## STORY-CA-007: Transient Status Bar Messages During Cost Operations

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer who relies on the status bar for quick feedback
**Goal**: Understand the relationship between persistent cost display and transient status messages
**Preconditions**: TUI is open. Status bar shows `gpt-4o ~ $0.0082`. No run is currently active.

### Steps

1. User types `/export` + **Enter** → The conversation is exported to a timestamped markdown file (e.g., `transcript-20260323-142501.md`). The status bar briefly shows a transient message: `Exported: transcript-20260323-142501.md` for 3 seconds.
2. During those 3 seconds, the **transient message replaces** the normal status bar content (model + cost). The user cannot see the current cost while the export message is visible.
3. After 3 seconds, the auto-dismiss fires → The transient message clears. The status bar returns to its normal state: `gpt-4o ~ $0.0082`. The cost figure is exactly as it was before the export.
4. User types an unknown command like `/foo` → Status bar shows a transient hint for 3 seconds: the unknown command message. Then it reverts to `gpt-4o ~ $0.0082`.
5. User presses **Escape** on an empty input (nothing to cancel or clear) → Status bar shows `Input cleared` for 3 seconds. Then reverts.

### Variations

- **Run interrupted**: When the user presses Ctrl+C during a run, a transient `Interrupted` message appears in the status bar for 3 seconds. During this time, the cost counter is not visible. After the message clears, the cost counter reappears showing the cumulative cost including the interrupted run.
- **Export failed**: If the export encounters a filesystem error, the transient message reads `Export failed` instead of a file path. The cost counter is unaffected and returns after 3 seconds.

### Edge Cases

- **Multiple transient messages in quick succession**: Each new transient message replaces the previous one and resets the 3-second timer. If the user issues two quick commands, only the second message is visible (the first is overwritten before it dismisses).
- **Cost update during transient message display**: If a `usage.delta` SSE event arrives while a transient message is showing, the underlying `costUSD` field is updated immediately via `SetCost()`. The transient message remains visible until it auto-dismisses, at which point the cost reflects the latest value — not an intermediate value.

---

## STORY-CA-008: Comparing Costs Across Time Periods in the Stats Panel

**Type**: medium
**Topic**: Cost & Context Awareness
**Persona**: Engineering team lead reviewing AI tool usage to report monthly spend to their team
**Goal**: Compare weekly vs. monthly usage patterns and total costs without leaving the TUI
**Preconditions**: TUI has been in use for at least 30 days. `usageDataPoints` contains a rich history. Stats panel has been populated via accumulated `usage.delta` events over many sessions.

### Steps

1. User opens `/stats` → Stats panel opens in **last 7 days** (`PeriodWeek`) mode. The heatmap shows 1 column × 7 rows. Heavy days (this week included a large codebase review) show `█` cells. `Total runs: 67   Total cost: $1.22`.
2. User presses **r** → Switches to **last 30 days** (`PeriodMonth`). The heatmap now has more columns. Sparse weeks are visible — last month had two quiet weeks and one very heavy week. `Total runs: 183   Total cost: $3.47`.
3. User mentally divides: `$3.47 / 30 days = ~$0.12/day` average. This week's `$1.22 / 7 = ~$0.17/day` is above average.
4. User presses **r** again → Switches to **last 365 days** (`PeriodYear`). The heatmap shows ~52 columns. Most cells are `░` (the tool was only adopted 2 months ago). The populated weeks are visible as a cluster on the right side of the heatmap. `Total runs: 214   Total cost: $4.01`.
5. User notes that the year total nearly matches the 30-day total, confirming the tool is relatively new. Presses **r** to wrap back to week view.
6. User presses **Escape** → Overlay closes.

### Variations

- **Cost is higher than expected**: The user sees `Total cost: $12.80` over the past month and decides to review which tasks were most expensive. The stats panel does not show per-run breakdowns, but the heatmap identifies the high-activity days. The user can correlate those days with their memory of what they were working on or with exported transcripts from those days.
- **Team usage reporting**: The stats panel only shows the current user's local session data (accumulated in-memory). There is no server-side aggregation across users exposed via the stats panel. For team-level reporting, the user would need to query the server API directly.

### Edge Cases

- **Heatmap shows no activity**: If no `usage.delta` events have been received yet in this session (or the slice was reset), all cells render as `░`. This can happen if the user just started the TUI and has not yet sent any prompts. `Total runs: 0   Total cost: $0.00`.
- **Very high run count**: The `intensityBlock()` function uses percentile ranking, not absolute thresholds. A day with 500 runs and a day with 5 runs are both evaluated relative to the rest of the window. If 500 is the maximum in the window, it renders as `█`; if 5 is average, it might render as `▒`. This ensures the heatmap remains readable even at high usage scales.

---

## STORY-CA-009: First Run — Cost Goes From Hidden to Visible

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: New user launching `harnesscli --tui` for the first time
**Goal**: Understand when and how the cost counter first appears
**Preconditions**: TUI has just been launched. No run has occurred yet. The status bar shows only the model name (e.g., `gpt-4o`). The cost segment is absent because `costUSD` is `0.0` and the `costUSD > 0` guard prevents rendering a `$0.0000` label.

### Steps

1. User observes the status bar: `gpt-4o`. No cost is shown. This is the clean initial state.
2. User types their first prompt — `"What files are in the current directory?"` — and presses **Enter** → A run starts. The status bar adds `...` (running indicator): `gpt-4o ~ ...`.
3. The agent calls the bash tool, which completes quickly. The server emits the first `usage.delta` event with `cumulative_cost_usd: 0.0003`.
4. The TUI receives the event, calls `m.statusBar.SetCost(0.0003)` → The status bar now shows `gpt-4o ~ ... ~ $0.0003`. Three segments are visible: model, running indicator, cost.
5. Run completes → `SSEDoneMsg` clears the running indicator. Status bar becomes: `gpt-4o ~ $0.0003`. The cost counter is now permanent for the session.
6. User sends a second prompt → Cost ticks upward with each `usage.delta` event. The user now has a persistent reference for total session spend.

### Variations

- **User never sends a prompt**: The cost segment never appears. The status bar stays at just the model name. There is no visual clutter from a `$0.00` label.
- **Model name not yet set**: On TUI initialization before the model name is resolved from the server, the status bar may be blank. Once the model is selected (either from config or via `/model`), it appears.

### Edge Cases

- **Very first event carries a cost of exactly 0**: The segment remains hidden. This would be unusual (normally the server sends non-zero cumulative cost), but the guard handles it correctly.
- **Terminal too narrow to show cost on first appearance**: If the terminal is extremely narrow, the cost segment may be dropped in favor of the higher-priority model name and running indicator. The user can widen the terminal to reveal it.

---

## STORY-CA-010: Closing and Reopening Stats Overlay Mid-Session

**Type**: short
**Topic**: Cost & Context Awareness
**Persona**: Developer who checks the stats panel between tasks during a multi-hour session
**Goal**: Verify that the stats panel accumulates today's data across the full session and is accurate when reopened multiple times
**Preconditions**: TUI has been running for 3 hours. Several runs have been completed. `usageDataPoints` has been updated multiple times via `upsertTodayDataPoint`. Stats panel was opened once at the start of the session.

### Steps

1. User opens `/stats` early in the session → Sees `Total runs: 4   Total cost: $0.07` for today.
2. User presses **Escape** → Overlay closes. Work continues.
3. User completes 12 more runs over the next 2 hours. Each run generates `usage.delta` events that call `upsertTodayDataPoint`, which **replaces** today's cost (cumulative, not additive for cost) and **increments** today's run count.
4. User opens `/stats` again → Today's cell is now the densest in the heatmap (`█` since today is by far the most active day). `Total runs: 16   Total cost: $0.31` for today (week total may differ).
5. User presses **r** to check the month view → Can see the current day stands out dramatically from prior days. `Total runs: 16   Total cost: $0.31` appears in the totals line (assuming all runs were today).
6. User presses **Escape** → Returns to the main chat view.

### Variations

- **Stats panel opened during an active run**: The overlay opens on top of the running conversation. The heatmap shows data from prior events. If a new `usage.delta` event arrives while the overlay is open, the underlying `m.statsPanel` is updated immediately (the model is rebuilt via `statspanel.New(m.usageDataPoints)`), but the overlay re-renders on the next `Update` cycle. The user sees current data.
- **Multiple opens in one session**: Each open shows the latest state. There is no caching or stale data — the panel is rebuilt from `m.usageDataPoints` every time a `usage.delta` event arrives.

### Edge Cases

- **Clock crosses midnight while TUI is open**: `upsertTodayDataPoint` uses `time.Now()` to determine "today". After midnight, a new DataPoint is inserted for the new day. Yesterday's data remains in the slice and is visible in the heatmap if the selected period includes it. Today's new data starts accumulating fresh.
- **Stats panel opened with no runs today but prior history**: If the user opens the TUI fresh in the morning without sending any prompts, today's cell is `░`. Prior days may show activity. The status bar also shows no cost (`gpt-4o` only, no `$` segment). This is the expected morning-fresh state.

---

## Summary

| **Story** | **Interaction** | **Key Insight** |
|-----------|-----------------|-----------------|
| STORY-CA-001 | Cost counter updates in real time | `usage.delta` SSE events drive `SetCost()`; counter hidden until first non-zero event |
| STORY-CA-002 | Per-run vs. session-total cost | Status bar always shows the session cumulative total; `/clear` does not reset it |
| STORY-CA-003 | `/context` opens context grid | Shows token progress bar: used / 200k, percentage, and visual fill indicator |
| STORY-CA-004 | `/stats` opens heatmap overlay | 7-row day-of-week grid; `r` key cycles week/month/year; totals line at bottom |
| STORY-CA-005 | Using both panels to decide on `/clear` | Context grid (71% full) + stats (unusually expensive day) inform the decision |
| STORY-CA-006 | Watching context fill during a long run | Token count grows with each `usage.delta`; grid shows approaching limit |
| STORY-CA-007 | Transient messages vs. persistent cost | 3-second transient messages temporarily replace the cost display; it returns after dismiss |
| STORY-CA-008 | Comparing costs across time periods | Week ($1.22), month ($3.47), year ($4.01) aggregates in one overlay with one `r` key |
| STORY-CA-009 | First run — cost segment appears | Cost segment hidden until `costUSD > 0`; first `usage.delta` makes it visible |
| STORY-CA-010 | Stats panel accuracy across reopens | `upsertTodayDataPoint` increments run count and replaces cost on every event; panel always current |

---

## Design Notes

### Cost Format
The status bar renders cost as `$%.4f` (4 decimal places), e.g., `$0.0082`. This format is chosen because most LLM API costs per request are in the sub-cent range and 4 decimal places provides meaningful precision without scientific notation.

### Cumulative vs. Delta
The `usage.delta` SSE event carries `cumulative_cost_usd` — the server's running total for the entire run, not a per-event increment. The TUI takes the last value received. This means the status bar always shows the most recent server-reported cumulative value, not a sum of client-side delta calculations. If events arrive out of order (rare in SSE), the last one wins.

### Context Window Default
`contextgrid.Model` defaults `TotalTokens` to `200000` when unset. This matches the context window of commonly-used frontier models (GPT-4o, Claude 3.5 Sonnet). If the active model has a different window, the server should populate `TotalTokens` accordingly via the usage event or a model metadata endpoint.

### Stats Panel Period Toggle
The `r` key is the only interaction within the stats overlay (besides **Escape** to close). The cycle is Week → Month → Year → Week, implemented by `statspanel.Model.TogglePeriod()`. Pressing `r` immediately re-renders the heatmap with the new period's data window.

---

## Related Topics

- **First Launch & Chat**: The status bar is introduced the moment the TUI starts; cost becomes visible after the first run.
- **Slash Commands & Autocomplete**: Both `/context` and `/stats` are opened via slash command with autocomplete support.
- **Conversation Management**: `/clear` clears conversation history but preserves the session cost counter — relevant context for STORY-CA-002 and STORY-CA-005.
- **Model & Provider Selection**: The active model name is shown alongside the cost in the status bar. Changing models via `/model` updates the model name segment.

---

**Story Count**: 10
**Last Updated**: 2026-03-23
