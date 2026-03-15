# Issue #156 Grooming: feat(demo-cli): Tool output visibility toggle

## Summary
Add a toggle to show/hide full tool output, switching between compact summary and full verbose display.

## Already Addressed?
**NOT ADDRESSED** — `display.go` always shows 120-char truncated tool summary. No toggle exists.

## Clarity Assessment
Clear — compact vs verbose modes with `/details` toggle command.

## Acceptance Criteria
- Default: compact mode (120-char summary, current behavior)
- `/details` command toggles verbose mode (full output)
- Toggle state persists for session
- Test for both modes

## Scope
Atomic.

## Blockers
None.

## Effort
**Small** (1-2h) — Toggle flag + conditional formatting in display + `/details` command.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `ux`. Good.

## Recommendation
**well-specified** — Ready to implement.
