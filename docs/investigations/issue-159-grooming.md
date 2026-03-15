# Issue #159 Grooming: feat(demo-cli): /clear command and session cost summary on exit

## Summary
Add `/clear` to clear the display (preserving server-side context) and print a session cost summary on exit.

## Already Addressed?
**PARTIALLY ADDRESSED** — Cost tracking infrastructure exists: `PrintUsage` in `display.go` captures `cumulative_cost_usd` from `usage.delta` events inline during streaming. However:
- No `/clear` command
- No session exit cost summary (only inline per-event display)

## Clarity Assessment
Good — clear requirements with specific format: `Session cost: $0.0042 | Tokens: 1,234 in / 567 out`.

## Acceptance Criteria
- `/clear` clears all rendered chat history from display
- Server-side conversation context NOT reset by `/clear`
- On clean exit (Ctrl-C or `quit`), cost summary printed after TUI tears down
- Summary shows total USD, input tokens, output tokens
- Summary omitted if no tokens used

## Scope
Atomic.

## Blockers
None.

## Effort
**Small** (1-2h) — `/clear` command + accumulate total cost across session + print on exit.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `ux`. Good.

## Recommendation
**well-specified** — Ready to implement. Cost infrastructure already in place; just needs accumulation + exit print.
