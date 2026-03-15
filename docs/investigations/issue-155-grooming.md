# Issue #155 Grooming: feat(demo-cli): Prompt history navigation with up/down arrows

## Summary
Add arrow-key-based prompt history navigation (up/down), persisted to `~/.config/harnesscli/history`.

## Already Addressed?
**NOT ADDRESSED** — Current input uses `bufio.Scanner` with no history tracking.

## Clarity Assessment
Excellent — specific requirements: 100 max entries, disk persistence, arrow key navigation.

## Acceptance Criteria
- Up/down arrows cycle through history
- Max 100 entries in memory ring buffer
- Persisted to `~/.config/harnesscli/history` between sessions
- No duplicates (dedup consecutive identical entries)

## Scope
Atomic.

## Blockers
None (works with current CLI; may need rework if #152 Bubble Tea migration happens first).

## Effort
**Small** (2-4h) — Ring buffer + file I/O + key handling.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `ux`. Good.

## Recommendation
**well-specified** — Ready to implement. Consider sequencing before or after #152.
