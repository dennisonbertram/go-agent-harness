# Active Plan

Current status: Issue #1067 terminal publication atomicity is implemented on
isolated branch `codex/issue-1067-terminal-status-event-atomicity`. Post-review
hardening now defines and tests the one-way durable failure contract, bounded
store waits, per-run status serialization, per-conversation lock reclamation,
recorder drain for suppressed terminal events, event-plus-status safe pruning,
finite fail-closed admission during terminal persistence outage, and a
prune-wide reservation for validated continuation sources. The concurrent red,
durability-retention focused suite, affected-package normal/race/vet, and full
foreground repository gate pass at 85.7% coverage with zero uncovered
production functions on the prior follow-up. PR #1070 reached rebased head
`5f106bef`, where hosted race run `30656467482` exposed a stale settled-test
assumption; the audited shared event
collector now independently awaits terminal status under the same deadline.
Affected normal/race x100, hosted-equivalent `make test-race`, and the final
outside-sandbox foreground repository gate pass at 85.7% coverage with zero
uncovered production functions. The test-only repair remains local and
unpushed; PR #1060 remains excluded and no merge is authorized.

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
