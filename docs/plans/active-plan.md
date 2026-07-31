# Active Plan

Current status: Issue #1067 terminal publication atomicity is implemented on
isolated branch `codex/issue-1067-terminal-status-event-atomicity`. The hosted
durability regression has a deterministic local red-green repair; final gates
on the branch rebased to current `main` remain in progress before parent
promotion to open PR #1070. PR #1060 and PR #1055 remain excluded.

Current status: Issue #1023 anytime contextual `/feedback` intake is implemented
test-first and verified in its isolated worktree; targeted, full normal/race,
coverage-gate, and real TUI bundle checks pass, with merge pending.
Config-driven hooks epic #737 is implemented and verified on branch
`codex/config-hooks-epic-737` with PR #784 open (fast gate + full regression
PASS). Active adapter-first eval work has cleared the repo regression gate,
passed a real-provider Terminal-Bench smoke, and promoted a real smoke baseline.
Remaining work before merge is final verification and any requested
review/cleanup.

Current active plans:
- `2026-07-31-issue-1067-terminal-status-event-atomicity-plan.md`
- `2026-07-30-issue-1023-feedback-intake-plan.md`
- `2026-06-26-adapter-first-eval-harness-plan.md`
- `2026-04-05-orchestration-program-plan.md`
- `2026-03-29-training-test-fix-plan.md`
- `2026-04-01-per-run-sandbox-boundary-plan.md`
- `2026-04-13-autoresearch-testing-plan.md`

Most recent completed plan: `2026-07-29-issue-987-issue-driven-workflow.md` (PR #990)
Previous completed plan: `2026-06-28-config-driven-hooks-epic-737-plan.md` (PR #784)
