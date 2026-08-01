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

Current status: Issue #1002 conversational cron CRUD acceptance repair is
implemented locally, review-clear, and rebased onto current `origin/main`
`3506e01c` on `codex/issue-1002-repair-v2`. Model
CRUD/history is ID-only, operator name lookup is ambiguity-safe, and ownership
is enforced in persistence predicates. Create/resume are paused-first; active
replacement is inert `Prepare` → durable CAS → infallible `Commit`. Monotonic
registration identities are rechecked at a scheduler-locked execution admission
point, so a completed pause/delete/replacement cannot be followed by a stale
execution row. Legacy scoped-name migration has durable history/integrity proof.
The exact rebased candidate passes focused normal/race and the complete
foreground regression at 85.7% coverage with zero uncovered functions. A real
OpenAI-backed 11-run conversation proved model create/get/update/list/history/
pause/resume/delete, stale-CAS rejection, two scheduled same-conversation
continuations visible in SSE and transcript, and final deletion (empty list and
404 read). Guarded PR update, hosted checks, and production merge remain.

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

Current status: Issue #1056 terminal assistant-message reconciliation is
rebased onto production main `c10c085d` in an isolated worktree. The
required two-turn final-only behavior test failed before the reducer change and
now passes; focused and complete TUI normal/race suites are green. The full
repository gate passes at 85.7% with zero uncovered production functions, and
the exact-candidate real PTY/API/SQLite/reconnect matrix preserves the exact
four-message conversation without duplicates. Two independent source/diff
reviews found no P1/P2 issue. Focused TUI validation is being rerun on the
rebased candidate. An exact-head review then found contentless `/resume`
completion could duplicate the prior assistant transcript row; the actual
resume-path completed/failed red is green after resetting assistant ownership
at `RunStartedMsg`. The focused resume, new-content, replay, and two-turn matrix
passes normal and race. The isolated review fix is ready for promotion; hosted
and production-main gates remain before PR #1059 may merge.

Current status: Issue #1067 terminal publication atomicity shipped through PR
#1070 on production main `b45b4334`; local pre/post full regressions and hosted
PR fast/race checks passed. Its issue is closed.

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
Issue #1003 remote cronsd dispatch is in independent PR review after a clean
rebase onto `origin/main`; the review gate replaced process-local replay
dedupe with a durable restart-safe run binding, and a subsequent live review
found and repaired the missing fresh-run-store API-key bootstrap migration;
the final review also repaired reserved-run persistence ordering, per-job
deadline composition, unbounded completed single-flight entries, and
same-process accepted-mark/concurrent-resume duplicate-dispatch windows. The
exact-head follow-up also covers accepted queued shutdown recovery,
nonpositive PATCH timeout normalization, and stalled-body timeout/cancel.
The current review repair also adds mandatory single-tenant cronsd ingress
authentication, authenticated readiness, tenant-scoped CRUD/history and
scheduler startup, plus explicit harnessd/cronctl client credentials. Focused
review then required durable historical conversation-owner backfill and a
linearizable legacy shell tenant claim across servers; both repairs are under
focused normal/race verification. Exact-head verification/push/re-review
remains pending.
Config-driven hooks epic #737 is implemented and verified on branch
`codex/config-hooks-epic-737` with PR #784 open (fast gate + full regression
PASS). Active adapter-first eval work has cleared the repo regression gate,
passed a real-provider Terminal-Bench smoke, and promoted a real smoke baseline.
Remaining work before merge is final verification and any requested
review/cleanup.

Current active plans:
- `2026-07-31-issue-1056-terminal-assistant-message-plan.md`
- `2026-07-31-issue-1067-terminal-status-event-atomicity-plan.md`
- `2026-07-31-issue-1076-workflow-initial-write-exit-plan.md`
- `2026-07-31-issue-1068-dispatcher-shutdown-isolation-plan.md`
- `2026-07-30-issue-1052-provider-key-capture-sync-plan.md`
- `2026-07-30-issue-1054-waiting-pending-order-plan.md`
- `2026-07-30-issue-1023-feedback-intake-plan.md`
- `2026-07-31-issue-1003-remote-cronsd-plan.md`
- `2026-06-26-adapter-first-eval-harness-plan.md`
- `2026-04-05-orchestration-program-plan.md`
- `2026-03-29-training-test-fix-plan.md`
- `2026-04-01-per-run-sandbox-boundary-plan.md`
- `2026-04-13-autoresearch-testing-plan.md`

Most recent completed plan: `2026-07-29-issue-987-issue-driven-workflow.md` (PR #990)
Previous completed plan: `2026-06-28-config-driven-hooks-epic-737-plan.md` (PR #784)
