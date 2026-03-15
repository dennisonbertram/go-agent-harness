# Issue #153 Grooming: feat(demo-cli): Three-panel layout with input area and sidebar

## Summary
Add a three-panel TUI layout: chat history panel, fixed input area at bottom, metadata sidebar.

## Already Addressed?
**NOT ADDRESSED** — Current demo-cli uses single linear output. No lipgloss in `go.mod`.

## Clarity Assessment
Excellent — 5 concrete acceptance criteria, responsive behavior defined (hide sidebar <120 cols).

## Acceptance Criteria
- Chat history panel with scroll
- Fixed input area at bottom
- Metadata sidebar (cost, model, step count)
- Responsive: sidebar hidden <120 cols
- Resize-safe concurrent updates

## Scope
Medium.

## Blockers
**BLOCKED by #152** — Requires Bubble Tea migration as foundation.

## Effort
**Medium** (3-4 days after #152).

## Label Recommendations
Current: `enhancement`, `demo-cli`, `tui`. Good.

## Recommendation
**blocked** — Implement after #152 is merged. Well-specified once unblocked. Add clarification: sidebar width breakpoint definition, chat scroll anchoring behavior.
