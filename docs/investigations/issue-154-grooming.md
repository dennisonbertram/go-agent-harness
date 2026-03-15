# Issue #154 Grooming: feat(demo-cli): Glamour markdown rendering for assistant responses

## Summary
Use the glamour library to render assistant message content as styled markdown instead of raw text.

## Already Addressed?
**NOT ADDRESSED** — `display.go` prints raw markdown as text. No glamour dependency in `go.mod`.

## Clarity Assessment
Clear. Specific acceptance criteria.

## Acceptance Criteria
- glamour renders assistant messages (code blocks, bold, lists, headers)
- Thinking/reasoning deltas displayed in muted style
- Graceful fallback if terminal doesn't support styling
- Tests for rendered output

## Scope
Atomic. Can be implemented independently of Bubble Tea migration.

## Blockers
None (can work with current or Bubble Tea CLI).

## Effort
**Small** (2-4h) — Add glamour dep, integrate into `PrintDelta` path.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `tui`. Good.

## Recommendation
**well-specified** — Ready to implement.
