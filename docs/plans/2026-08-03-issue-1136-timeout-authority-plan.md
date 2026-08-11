# Plan: Issue #1136 immutable timeout authority

## Context and scope

- Governing issue: #1136, stacked after #1133 on the #1131 native ownership
  line.
- Problem: a mutable session pointer cannot prove that an A timeout remains
  owned by A after B terminals and C starts.
- In scope: private A-handle capability authority, reset/load invalidation,
  one-shot transport dispatch, and deterministic native proof.
- Out of scope: harness endpoints, selected-run reducer behavior, tool grammar,
  and production scheduling semantics.

## TDD and implementation

- Red: B -> C -> A timeout lost authority when only mutable session pointers
  were consulted.
- Repair: each `RunSubmission` captures a private owner token and session
  generation. `RunSession` exposes no public handle-based timeout cancel.
  ToolWalk binds its configured immutable `Duration` at submission; GUI uses
  the parameter-free submit path. `RunSubmission.markStarted` derives the
  sole absolute deadline, and `RunSession.submissionTimeoutGate(for:)` alone
  mints a package-visible opaque ticket with a fileprivate constructor at that
  deadline. `Runner.waitForTerminal` accepts only its poll interval: it cannot
  reinterpret a timeout duration after submission. No caller supplies a
  post-submit duration and no raw
  transport API exists. Ticket consumption is transport-only and atomically rechecks started owner,
  generation, and one-shot state; terminal, failure, reset, and load revoke it.
  Reset/load cancel every live submission stream by immutable handle, including
  displaced A plus selected C.
- Deterministic proof: direct capability dispatch proves exactly one A cancel
  after B -> C, zero B/C actions, no cancel after terminal/failure/reset, and
  physical A+C stream detachment. #1133 continues to prove ToolWalk timeout
  policy and passive terminal/failure observation.

## Status and gates

- [x] Write red and repair the authority model.
- [x] Add deterministic capability/revocation/detachment tests.
- [x] Re-run strict format and full Swift after the final submission-bound
  deadline repair: formatter passed; 252 tests in 46 suites passed.
- [x] Focused `PassiveSubmissionOutcomeIntegrationTests`: 14/14 cases on the
  final stacked head (the earlier 4/4 and 8/8 counts were intermediate slices).
- [x] Run `./scripts/test-regression.sh` after the final repair: normal, race,
  and coverage passed (85.5% total; zero uncovered production functions).
- [x] Capture the API-removal red: former direct handle callers no longer
  compile after `cancelTimedOutSubmission(_:)` is removed.
- [x] Add deadline-ticket proof for pre-expiry absence, B -> C -> A exact-one
  dispatch, terminal/failure/reset revocation, and duplicate refusal.
- [x] Correct the review-found package raw-transport bypass: GoCodeUI owns the
  ticket and fileprivate transport closure, while the submission-bound gate
  returns the same immutable gate to repeated callers. A test-only internal
  monotonic-now seam shared by `markStarted` and the gate advances epsilon and
  exact-deadline state without wall-clock sleeps, proving no ticket/action
  before expiry and then one A-only ticket/dispatch;
  source-surface tests forbid both raw transport and `armSubmissionTimeout`.
- [x] Run exact strict Swift format, full Swift, and full repository regression
  on the final opaque-ticket head.
- [ ] Publish the separate stacked draft PR with `Closes #1136`.
