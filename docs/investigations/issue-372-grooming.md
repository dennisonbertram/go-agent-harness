# Issue #372 Grooming: TUI Wire Tooluse Component

## Already Addressed?
**YES** — Fully implemented in branch `issue-369-wire-messagebubble` (commit `b16e36c`).

## Evidence
- `tooluse.Model` expanded from ~25 lines to ~165 lines with full state machine and lifecycle routing
- Integration test: `cmd/harnesscli/tui/tooluse_integration_test.go` validates bash output, error states via component path
- Root model `appendToolUseView()` creates tooluse.Model, calls View(), routes result to viewport
- Tests cover command label display, truncation hints, error indicators via component
- Not yet merged to main; PR needed

## Clarity
GOOD — Scope precise: make tooluse.Model.View() real entry point, route lifecycle events through component hierarchy.

## Acceptance Criteria
Adequate — integration tests cover all tool states (pending, running, completed, error).

## Scope
ATOMIC — Tool event routing in model.go + tooluse/model.go.

## Blockers
NONE — branch exists and implementation is complete.

## Recommended Labels
- `already-resolved`
- `enhancement`
- `medium`
