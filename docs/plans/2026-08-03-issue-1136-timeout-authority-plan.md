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
  generation. `RunSession` atomically consumes a started-only capability once;
  terminal, failure, reset, and load revoke it. Reset/load cancel every live
  submission stream by immutable handle, including displaced A plus selected C.
- Deterministic proof: direct capability dispatch proves exactly one A cancel
  after B -> C, zero B/C actions, no cancel after terminal/failure/reset, and
  physical A+C stream detachment. #1133 continues to prove ToolWalk timeout
  policy and passive terminal/failure observation.

## Status and gates

- [x] Write red and repair the authority model.
- [x] Add deterministic capability/revocation/detachment tests.
- [x] Re-run strict format (0/7 touched Swift files) and full Swift (245 tests
  / 46 suites) after the final proof update.
- [x] Focused `PassiveSubmissionOutcomeIntegrationTests`: 10/10 cases on the
  final stacked head (the earlier 4/4 and 8/8 counts were intermediate slices).
- [x] Run `./scripts/test-regression.sh` after the #1135 baseline repair:
  normal, race, and coverage passed (85.5% total; zero uncovered production
  functions).
- [x] Publish the separate stacked draft PR with `Closes #1136`.
