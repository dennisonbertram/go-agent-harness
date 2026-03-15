# Issue #158 Grooming: feat(demo-cli): /file command for attaching file context

## Summary
Add a `/file <path>[:<start>-<end>]` command to attach file content (optionally a line range) to the next prompt.

## Already Addressed?
**NOT ADDRESSED** — No `/file` command in `handleCommand`. No file attachment state tracking.

## Clarity Assessment
Excellent — specific line range syntax, attachment list display, behavior on prompt submission.

## Acceptance Criteria
- `/file path/to/file` attaches full file content
- `/file path:10-50` attaches lines 10-50
- Attachment list shown in input area
- Attachments prepended to next prompt submission
- `/file clear` clears all attachments
- Tab completion for file paths

## Scope
Medium.

## Blockers
None.

## Effort
**Medium** (4-6h) — File I/O + line range parsing + tab completion + attachment state + UI.

## Label Recommendations
Current: `enhancement`, `demo-cli`, `ux`. Good.

## Recommendation
**well-specified** — Ready to implement.
