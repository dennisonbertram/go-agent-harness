# Issue #152 Grooming: feat(demo-cli): Migrate to Bubble Tea TUI framework

## Summary
Replace the current raw `bufio.Scanner` demo-cli event loop with a Bubble Tea (Elm-architecture) TUI.

## Already Addressed?
**NOT ADDRESSED** — Current `demo-cli/` uses raw `bufio.Scanner` with ANSI codes. No Bubble Tea dependency in `go.mod`.

## Clarity Assessment
Good — motivation clear, architecture described ("smart model, dumb components" pattern). Missing: whether old `cmd/harnesscli` is deprecated, keyboard input specifics, terminal capability detection.

## Acceptance Criteria
- Bubble Tea `appModel` with SSE event → `tea.Msg` bridge
- All 9 SSE event types rendered
- `/model` and `/help` within Bubble Tea model
- Concurrent resize safety
- Manual tmux validation

## Scope
Large — complete CLI rewrite.

## Blockers
None. Is prerequisite for #153.

## Effort
**Large** (4-5 days) — new Elm architecture, SSE bridge, state machine redesign.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `tui`. Good.

## Recommendation
**needs-clarification** — Clarify: (1) deprecate old `cmd/harnesscli`? (2) keyboard input handling specifics. Otherwise well-specified. This is the foundation for #153-#159.
