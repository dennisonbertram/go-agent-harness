# Issue #34 Grooming: conversation-persistence: Add retention policy / auto-cleanup

## Summary
Automatically delete conversations older than a configurable retention period (default: 30 days). Pin mechanism to preserve important conversations.

## Already Addressed?
**NOT ADDRESSED** — No retention logic, no `HARNESS_CONVERSATION_RETENTION_DAYS` env var, no `pinned` flag, and no background cleanup goroutine found in the codebase.

## Clarity Assessment
Requirements are explicit. Missing implementation detail: when/how often cleanup runs (startup + periodic? startup only?).

## Acceptance Criteria
- `HARNESS_CONVERSATION_RETENTION_DAYS` env var (default: 30, 0=disabled)
- `pinned` field in conversations table (pinned conversations skipped)
- Cleanup runs on startup + periodically (recommend hourly via `time.Ticker`)
- SQLite `DELETE WHERE created_at < threshold AND NOT pinned`
- Tests under `-race`

## Scope
Atomic.

## Blockers
None. Minor clarification needed on cleanup trigger timing.

## Effort
**Medium** (2-3h) — Background goroutine + SQL delete + tests.

## Label Recommendations
Current: `enhancement`. Recommended: `enhancement`, `small`

## Recommendation
**well-specified** — Ready to implement. Suggest using startup + hourly ticker as default trigger if not clarified.
