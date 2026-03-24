# Issue #371 Grooming: TUI Wire Diffview Component

## Already Addressed?
**YES** — Fully implemented in branch `issue-369-wire-messagebubble` (commit `b16e36c`).

## Evidence
- `diffview.Model.View()` delegates to shared diff renderer (complete, not stub)
- Integration test: `cmd/harnesscli/tui/diffview_integration_test.go` validates unified diffs detected and routed through diffview
- Root model checks for unified diff prefix and routes to diffview component
- Not yet merged to main; PR needed from `issue-369-wire-messagebubble`

## Clarity
GOOD — Scope clearly defined: make diffview.Model.View() a real rendering adapter.

## Acceptance Criteria
Adequate — integration test verifies diffview is used for unified diff rendering.

## Scope
ATOMIC — Component adapter integration confined to model.go + diffview/model.go.

## Blockers
NONE — branch exists and implementation is complete.

## Recommended Labels
- `already-resolved`
- `enhancement`
- `small`
