# Issue #157 Grooming: feat(demo-cli): Multi-line input with shift+enter

## Summary
Allow users to compose multi-line prompts using Shift+Enter, with a line counter indicator.

## Already Addressed?
**NOT ADDRESSED** — `bufio.Scanner` reads one line at a time. No multi-line input support.

## Clarity Assessment
Very good — specific UX details: 6-line max display, line counter, Ctrl+C behavior, paste handling.

## Acceptance Criteria
- Shift+Enter adds newline without submitting
- Enter submits the full multi-line prompt
- Line counter shows current/max lines
- Ctrl+C clears input
- Paste handling works correctly

## Scope
Medium — requires replacing `bufio.Scanner` with proper terminal input handling.

## Blockers
Would benefit from Bubble Tea migration (#152) for clean implementation, but can be done independently with a raw-mode terminal library.

## Effort
**Medium** (4-6h) — Replace input mechanism + line counter UI + tests.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `ux`. Good.

## Recommendation
**well-specified** — Ready to implement. Consider sequencing after #152 for cleaner implementation.
