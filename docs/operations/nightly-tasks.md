# Nightly Tasks

Use this checklist for automated nightly agent execution.

## Nightly Checklist

- [ ] Run full test suite and capture failures.
- [ ] Identify flaky tests and create issues.
- [ ] Scan open issues and suggest priority updates.
- [ ] Verify docs indexes are current when files changed that day.
- [ ] Summarize changes to engineering, observational, and system logs.
- [ ] Confirm no unlogged bug fixes exist.

## Output Requirements

Every nightly run must return the completion format from `agent-completion-template.md` with exact files, commands, outcomes, and blockers.
The response must start with an explicit status line: `Task status: DONE` or `Task status: NOT DONE`.
