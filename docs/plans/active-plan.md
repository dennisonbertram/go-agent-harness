# Active Plan

Current status: Issue #1083 approval-publication readiness is implemented and focused-verified in a dedicated worktree. Pending registration now precedes tool and plan event publication in the shared broker lifecycle, while timeout, cancellation, duplicate, checkpoint, and fail-closed semantics remain characterized. Parent promotion owns full regression, hosted, and live SSE canary gates.

Current status: Issue #1081 isolates the exact-main hosted Ubuntu coverage-gate
failure where the platform-neutral `keychainParts` helper receives no execution
because Linux never reaches Darwin Keychain CRUD. The strict red is hosted run
`30672776651`, not a fabricated behavior failure. The planned repair is one
portable table-driven parser validation test; production Keychain code and the
Darwin real-Keychain integration test remain unchanged. Targeted normal/race
and the authoritative foreground full regression are green at 85.7% with zero
uncovered production functions. Exact-head review, hosted Ubuntu proof, and
parent promotion remain pending.

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
uncovered production functions. Exact-head review of local commit `8757e8a3`
then found the settled helper accepted missing or status-mismatched terminal
events and its phase test lacked an entry handshake. The test-only correction
requires exactly one matching terminal event and passes focused normal/race
x100, hosted-equivalent race, and the full repository gate at 85.7% coverage
with zero uncovered production functions. The repair remains local and
unpushed; PR #1060 remains excluded and no merge is authorized.

Current status: Issue #1076 isolates the source-workflow lifecycle race where a
child exits before the initial `start` write and the early EPIPE return skips
wait plus bounded stderr arbitration. Natural-exit and live-child cleanup reds
are green, focused normal/race x100, workflow normal/race, and `make test-race`
pass. The unchanged full regression also passes at 85.6% coverage with zero
uncovered functions, and the repair is committed locally. Parent promotion
remains; this ships separately before #1070 is rebased.

Current status: Issue #1077 D0 macOS app logo is implemented on current
`origin/main`; focused and complete Swift tests plus the GoCode build pass. The
repository regression gate remains blocked by two unrelated real-keychain tests
being killed; the user authorized a waiver on 2026-07-31, and the change is
being committed for merge.

Current status: Issue #1068 dispatcher shutdown isolation and the review-found
bounded worker-pool fixture cleanup are implemented and locally verified on a
dedicated branch. The aggregate 4/5 red, deterministic two-Runner red/green,
cleanup red/green, worker-pool normal/race x100, complete harness race x5, vet,
and unchanged regression gate are recorded. PR #1069 remains open and unmerged;
the local review-fix commit still requires parent promotion and hosted reruns.
PR #1060, PR #1055, and issue #1067 remain excluded.

Current status: Issue #1052 has removed the provider API-key capture test's
unrelated three-second HTTP readiness dependency. Focused repeated normal/race,
complete package normal/race, and repository normal/race/coverage gates pass;
promotion is pending before the cron/callback repair chain can merge.

Current status: Issue #1054 now makes pending AskUserQuestion input readable
before `waiting_for_user` status/events become visible. Focused repeated
normal/race, complete harness normal/race, and repository normal/race/coverage
gates pass; promotion is pending before the cron/callback repair chain.

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
- `2026-07-31-issue-1076-workflow-initial-write-exit-plan.md`
- `2026-07-31-issue-1068-dispatcher-shutdown-isolation-plan.md`
- `2026-07-30-issue-1052-provider-key-capture-sync-plan.md`
- `2026-07-30-issue-1054-waiting-pending-order-plan.md`
- `2026-07-30-issue-1023-feedback-intake-plan.md`
- `2026-06-26-adapter-first-eval-harness-plan.md`
- `2026-04-05-orchestration-program-plan.md`
- `2026-03-29-training-test-fix-plan.md`
- `2026-04-01-per-run-sandbox-boundary-plan.md`
- `2026-04-13-autoresearch-testing-plan.md`

Most recent completed plan: `2026-07-29-issue-987-issue-driven-workflow.md` (PR #990)
Previous completed plan: `2026-06-28-config-driven-hooks-epic-737-plan.md` (PR #784)
