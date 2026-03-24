# Issue #373 Grooming: TUI Thinking-Delta Display

## Already Addressed?
**YES** — Fully implemented in branch `issue-369-wire-messagebubble` (commit `b16e36c`).

## Evidence
- `ThinkingDeltaMsg` type introduced in tui package
- Integration test: `cmd/harnesscli/tui/thinkingbar_integration_test.go` validates thinking deltas show as "Thinking: ..." indicator
- Root model `appendThinkingDelta()` handles SSE `assistant.thinking.delta` events
- Test verifies thinking clears when assistant output begins
- Not yet merged to main; PR needed

## Clarity
GOOD — Scope explicit: wire assistant.thinking.delta events into visible user-facing path.

## Acceptance Criteria
Adequate — tests confirm rendering + clear-on-output-start behavior.

## Scope
ATOMIC — Event routing and UI display in model.go, no side effects.

## Blockers
NONE — branch exists and implementation is complete.

## Recommended Labels
- `already-resolved`
- `enhancement`
- `medium`
