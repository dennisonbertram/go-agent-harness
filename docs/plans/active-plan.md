# Active Plan

Current status: Issue #1236 plural workflow subscription handoff is in its
isolated `codex/workflow-subscribe-gap-1236` worktree, rebased to pushed main
`03284bb8`. Deterministic snapshot/register, burst, Store-error, cancellation,
and durable-restart red/green coverage is complete; focused normal/race/stress
pass. The branch is rebased to pushed main `03284bb8`; the required
external-cache full regression and independent exact-head review remain before
#1238 is updated. Adjacent terminal-history SSE completion is now merged via
#1239, while the separate checkpoint restart gap remains #1240.

Current status: Issue #1208 has deterministic non-interactive native
fake-provider scenario support implemented on its isolated branch. It
preflights the core two-message tool, cron linked-conversation, and callback
exactly-once linked-conversation fixtures without launching the owner
lifecycle. Rendered execution remains explicitly pending operator foreground
and TCC authorization.

Current status: Issue #1199 makes authored skill creation synchronously reload the ordinary registry and routes all verification through persistence then reload. Existing SKILL.md format/O_EXCL behavior and independent skill-pack ownership remain unchanged; full regression remains required.
Current status: Issue #1089's fail-closed rendered-native proof validator and
launcher-owned isolated collection are implemented in
`codex/issue-1089-native-rendered-matrix`. The native gate now requires exactly
one final PASS per applicable case, canonical contained regular artifacts, and
nonce/temp-root/repository-driver/app-build/loopback-child-daemon provenance.
The full regression is pending; no installed-app foreground proof has been
claimed because that requires explicit operator approval and a driver-owned
isolated GUI session.

Current status: Issue #1198 resolves `HARNESS_SKILLS_DIR` once as an absolute
global skill root (or preserves `$HARNESS_GLOBAL_DIR/skills` when unset) and
threads it through harnessd loader, create/verify registry, watcher, and Go
workflow skill discovery plus the TUI's local slash catalog. Harnessd rejects
relative values before a listener; TUI fails closed rather than presenting the
legacy global catalog. Earlier full regression passed at 85.6%/zero uncovered;
the review repair has focused normal/race evidence and awaits amended full
regression, review, hosted checks, and PR handoff.

Current status: Issue #1190 gives each production `dialHTTP` connection an
owned clone of the default HTTP transport, shared with the test constructor;
idempotent connection close releases only its own idle pool. A nonparallel
gated production authentication regression holds the dial, closes an unrelated
`httptest` server/global pool, then proves the strict `ErrUnauthorized` result.
Focused normal/race evidence is green; MCP package and full regression remain.

Current status: Issue #1174 binds `/init` persistence to its accepted run,
so the real `assistant.message` -> `SSEDoneMsg(run.completed)` lifecycle writes
only matching output. Target re-stat and atomic replacement prevent silent
tool-created-file overwrite; focused normal/race and canonical-temp full
regression pass locally, with independent review and hosted CI pending.

Current status: Issue #1175 canonicalizes only bootstrap-provenance fixture
paths before asserting Git's child worktree identity on macOS. The underlying
bootstrap provenance guard is unchanged; focused normal/race and full
regression pass, with draft-PR handoff pending.

Current status: Issue #1173 separates durable run-ID replay from rollout-file
simulation. The new authenticated per-run route starts a terminal source's
distinct same-conversation run with its effective provider; bare TUI IDs use
the lifecycle path while file-shaped arguments remain simulation. Focused
normal/race and full regression checks are green; fake-provider PTY acceptance
and draft PR handoff remain.

Current status: Issue #1161 scheduled-run routing preservation is implemented
on `origin/main` `c991a725`. Origin model, provider, fallback permission, and
ordered fallback providers now cross embedded cron, remote cronsd, and durable
callback boundaries; queued durable runs also persist their requested provider
before asynchronous dispatch. Tenant/auth/scope, secrets, retry ownership, and
clients are unchanged. Final validation is the complete affected normal/race
suites plus the repository regression gate before one closing, unmerged PR.

Current status: Issue #1141 isolates a callback deadline-release fixture race from #1138 hosted CI. The three tests use causal admitted, blocked-renewal, deadline, and cancellation-aware admission gates before releasing the blocked renewal; production callback/API/TUI/native behavior is explicitly out of scope. Focused normal/race x20 and the full regression remain required.

Current status: Issue #1135 isolates the hosted cron recovery fixture race.
The two recovery tests now block only their terminal mock-store return, prove
the persisted-but-not-returned row still denies the exact same scope, release
it, join `reconcileWG`, verify the recovered scope and lease are gone, then
admit. The remote fixture additionally gates its terminal response instead of
racing an atomic status flip. No production scheduler, persistence, remote,
or client behavior changes. Focused normal/race x100, complete cron normal/race,
and the tmux-hosted repository normal/race/coverage gate pass at 85.5% coverage
with zero uncovered functions; independent review, hosted checks, and promotion
remain.

Current status: Issue #1132 isolates the hosted compaction-after-wait fixture.
The test now subscribes immediately after `StartRun`, observes the public
`run.waiting_for_user` event through replay/live subscription, and only then
asserts pending input/state before preserving the complete compaction/resume
contract. It makes no production change. Focused normal/race x100, complete
harness normal/race, and the foreground full regression pass at 85.5% coverage
with zero uncovered functions; independent review, hosted checks, and promotion
remain.

Current status: Issue #1124 isolates the post-#1107 hosted retry-wait recovery
fixture. A mutex-protected test-only fake clock replaces the 60 ms deadline and
15 ms sleep: recovery holds a durable one-hour retry checkpoint, an explicit
early fire must not admit or mutate it, then an explicit post-deadline fire
must reuse the reserved run identity exactly once. Callback manager, SQLite,
API/task visibility, TUI, and native GUI behaviour remain unchanged. Final
focused/package/full validation, review, hosted checks, and promotion remain.
Current status: Issue #1130 is implemented locally on #1128 `654b7da`. It
Current status: Issue #1133 is implemented locally on #1131 `63cf9fcd`. It
corrects the residual #1130 wait-policy defect: B displacement revokes A
controls but not A outcome observation. ToolWalk now passively waits for A
terminal/failure through its deadline, while a displaced timeout can POST only
the exact locally owned A without mutating selected B. Eight URLSession-gated
`RunSession.submit()` + `Runner` tests cover B-before-A terminal, EOF, timeout,
B -> C -> A timeout authority, one-shot/terminal/failure/reset revocation,
and delayed acknowledgement with zero B/C actions. Strict formatting and full
Swift pass; full repository regression awaits the independent #1135 baseline
fixture repair, followed by review, hosted checks, and stacked PR promotion.

Historical status: Issue #1130 was implemented locally on #1128 `654b7da`. It
separates A's submission lifecycle from displacement, makes late
acknowledgement/EOF/failure identity-safe, and gives ToolWalk typed
terminal/failure/displaced/timeout outcomes so only timeout cancels A. Strict
formatting, focused suites, full Swift, and repository normal/race/coverage
pass; review, draft publication, and hosted checks remain required before
promotion.

Current status: Issue #1128 stacks on #1127 `d2cd29cf` and replaces the last
dynamic submission identity lookup. `RunSubmission` resolves only from the
local `startRun` response, retains A-only events/transcript/terminal state, and
is displaced rather than permitted to operate selected scheduled B. Composer
captures its action mode once; ToolWalk uses the handle for every wait,
automatic interaction, timeout, and verdict. Strict format and complete Swift
tests pass; repository gate, draft publication, review, and hosted checks
remain.

Current status: Issue #1125 stacks on #1122 `d1931ae2` to fence the remaining
native actions. Stop, Composer steer, and ToolWalk timeout carry their rendered
or decision run identity to an expected-run guard, so stale A cannot issue B's
endpoint or mutate B's state. Strict format and focused native/ToolWalk tests
pass; complete Swift/repository gates and draft handoff remain.

Current status: Issue #1122 is a stacked native ownership repair on draft PR
#1118 head `34219699`. It stamps approval/plan affordances with their SSE
`run_id`, clears every pending interaction synchronously when visible run
ownership changes or retires, and makes Chat/ToolWalk retain that identity for
approval, plan, and answer actions. Deterministic A-to-B, terminal/fallback,
and foreign-terminal regressions are green. Exact final Swift is 222 tests / 43
suites; full repository regression passes at 85.5% coverage with zero uncovered
functions. Independent review remains before the stacked draft is handed off.

Current status: Issue #1007 rebases external scheduled cron/callback control
ownership onto main's accounting and request-generation reducer. The combined
reducer selects control authority before accounting, retains terminal
tombstones, invalidates displaced asynchronous action ownership, and resumes a
fallback only after rendering the selected terminal. Focused Swift (6), full
macapp (217 tests/43 suites), and complete normal/race/coverage regression
(85.5%, zero uncovered functions) are green. Independent review, hosted CI,
and merge reconciliation remain pending.

Current status: Issue #1120 is a test-and-documentation-only blocked-heartbeat
fixture repair merged through #1121 and #1119 into the current #1106 stack. It preserves production callback semantics
while proving the ordered process-fence/deadline/exact-token-release handoff.
Focused normal/race x100, complete tools normal/race, and isolated full
regression are green; independent review is complete and final #1106 hosted
promotion remains.

Current status: Issue #1117 is a test-only callback fixture stabilization
merged through #1119 into the current #1106 stack. It replaces a load-sensitive
30 ms-lease/100 ms-wait assertion with normal lease ownership evidence and
adds exact-one-starter coverage to transient SQLite claim contention. It made
no production callback change and passed independent review; final #1106
promotion remains.

Current status: Issue #1108 repairs only the native callback replay test
fixture after hosted `live-harnessd` observed terminal C accounting before its
asynchronous durable-message reconciliation. The regression now holds C's
second `/messages` response behind an application-level gate, proves C
ownership/completed state/exact usage before release, then awaits rendered C
durable text with those same invariants. Adjacent stale-terminal fixtures now
await rendered durable A/B rows after their gates rather than raw request
issuance. No `RunSession`, API, persistence, cron/callback, or TUI product
behavior changes are in scope. The expected delayed-durable red, strict format,
focused stream suite (11), C regression x20, full Swift suite (190), live
RunSession suite (2), and Go normal/race/coverage regression (85.5%, zero
uncovered functions) now pass. Commit `da489f67`, PR #1109 (`Closes #1108`),
and independent review are complete; the final cheap re-review and merge
remain.

Current status: Issue #1102 isolates the hosted `TestRunnerAskUserQuestionWaitsAndResumes` race baseline. The broker exposes pending input before the separate runner status/event notifier completes, so the existing polling fixture has no causal boundary. The test-only repair waits for the existing `run.waiting_for_user` event then retains the immediate status, submit, and full order assertions; no runtime lifecycle behavior is in scope.

Current status: Issue #1098 isolates the merged #1004 zero-function coverage
gate for deleted-job recovery. The worktree is freshly aligned to
`origin/main` `224d667a`; plan and impact mapping establish that recovery must
persist an unavailable terminal execution without touching a soft-deleted job,
preserve its RunID, and release the recovered lease only after persistence.
Strict red tests, focused normal/race coverage, and the full regression remain.

Outcome: after #1096 merged, #1098 was rebased to `5c4ed8c8`. Direct tests now
cover both definitive missing-job errors, durable release ordering, no-touch,
RunID retention, and persistence-failure retention. Targeted normal/race x20,
function coverage (91.7%/100%), and full normal/race/coverage regression pass;
the candidate is test-only and ready for independent review.

Current status: Issue #1093 deterministically owns the conversation-cleaner lifecycle. A cleaner now acknowledges exit through its `Start` result; bootstrap cancels and awaits it exactly once before closing conversation persistence on normal shutdown or any startup-failure unwind. Channel-controlled signal and startup-failure regressions, repeated race runs, and affected package suites pass. Foreground full-regression/hosted promotion remains the acceptance gate.

Current status: Issue #1086's registry-derived tool/TUI inventory, versioned
proof schema, registry-owned present provenance, and paired live unavailable
resolver evidence are implemented and focused normal/race verified in its
dedicated worktree. Evidence identity/applicability and report rendering now
fail closed against the hashed inventory. Exact-head command provenance,
mandatory resolver evidence, unidentified-resolution 503 behavior, and
selected-surface completeness are repaired and focused normal/race verified.
The v2 schema now also requires canonical-plus-alias PTY invocations,
class-appropriate runtime identities, independently verified typed
postconditions, digested/redaction-declared artifacts, and inventory-bound
required synthetic scenario catalogs.
Native applicability is now a mandatory inventory-bound mapping: every resolved
item is either native-available or explicitly not-applicable with source and UX
rationale. Native passes additionally require screenshot, AX, raw SSE/event,
API/store proof, and exact isolated build/daemon/workspace metadata. The
final-v2 foreground full regression passed normal, full race, and coverage at
85.7% with zero uncovered functions. Swift is inapplicable because no
macOS/ToolWalk consumer changed. Commit,
exact-diff review, and promotion remain parent-lane gates. No surface-runner or
scheduled-work convergence claim is included.

Current status: Issue #1006's callback dispatch state machine is implemented
locally on merged #1005 base `72fef8ab`. Set reserves one callback-derived run
ID; token-fenced SQLite claims, heartbeat/reclaim, bounded retry, cancel-winner
semantics, and `Runner.EnsureRunWithIDContext` reconcile that identity through
started/failed terminal linkage. Safe all-state task/list fields and lifecycle
events are implemented. Exact reds, focused repeated race, an assembled
recovered-manager-to-Runner test, complete affected-package normal/race tests,
and the full repository gate pass at 85.5% coverage with zero uncovered
functions. Independent review found and repaired post-terminal lifecycle event
loss, restart replay, partial-success durable listing, and unsafe summary
exposure. Focused and complete affected normal/race gates pass on the repair;
the final repository gate passes at 85.5% with zero uncovered functions.
Commit and promotion remain. Native controls remain #1007 and the live
cross-surface matrix remains #1010.

Current status: Issue #1005 adds an additive SQLite callback store at
`.harness/callbacks.db`. Creation persists before acknowledgement, shutdown
stops timers without cancellation, and runtime recovery happens only after the
real callback starter is bound. Retry/idempotency/run-link policy is reserved
for #1006; focused store/restart tests and full regression remain required.
Current status: Issue #1115's test-only workflow subscriber repair is implemented and locally verified. The chatty script is gated until subscription completes, then fills all 64 channel slots and proves ordered buffered delivery followed by close; focused normal/race `-count=100`, complete workflow normal/race, and the authoritative foreground repository regression pass at 85.6% coverage with zero uncovered functions. Production already closes/deregisters registered subscribers under the shared engine mutex, so runtime and late-subscriber replay semantics remain unchanged. Exact-tree rerun, independent review, hosted checks, and promotion remain.

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

Current status: Issue #1180 replaces the linked-worktree Git-environment build
override with an exact-revision, clean ephemeral staging clone because Go
1.26.4 buildvcs discovery does not read a linked worktree's `.git` indirection
file. Candidate validation and atomic publish remain fail-closed. Focused
normal/race acceptance and a serial full regression are green under the
documented canonical temporary-directory isolation; hosted review remains.

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
- `2026-08-03-issue-1115-workflow-subscriber-plan.md`
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
