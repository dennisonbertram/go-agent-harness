# Plan: Issue #1128 submitted-run ownership

## Context and Scope

- Governing issue: #1128, stacked on #1127 head `d2cd29cf`.
- A composer closure captured only its rendered run ID but chose steer versus
  submit using live state. ToolWalk similarly re-read global run/transcript
  state after submitting, allowing a scheduled B to replace local A.
- In scope: immutable composer action selection, one A-only `RunSubmission`
  handle returned by both native submit layers, ToolWalk lifecycle ownership,
  and deterministic native regressions.
- Out of scope: harness API/persistence, callback or cron execution, TUI, and
  scheduled-run selection policy beyond recording an existing local run's
  first lifecycle timestamp.

## TDD and Verification

- Red: B selected before A's response can be mistaken for A; after A capture,
  B can receive A's timeout action; a stale steer can fall through to submit.
- Green: the handle resolves only from `startRun`, retains A-only events and
  terminal evidence, becomes displaced on selected B, and ToolWalk exits with
  no B action. A real terminal A still receives its normal verdict.
- Gates: strict format, focused submission/external-control/ToolWalk tests,
  full Swift package, repository normal/race/coverage gate, then independent
  cheap review and hosted checks.

## Impact Map

- `2026-08-03-issue-1128-submission-handle-impact-map.md`.

## Checklist

- [x] Verify #1128 acceptance criteria and stacked #1127 base.
- [x] Add immutable composer action and A-only `RunSubmission` evidence.
- [x] Add deterministic B-before-response, B-after-capture, A-terminal, and
  start-failure/reset regressions.
- [x] Run strict format and complete Swift package.
- [x] Run repository regression: normal/race passed; coverage passed at 85.5%
  with zero uncovered functions.
- [ ] Publish stacked draft `Closes #1128` and obtain independent cheap review
  plus hosted checks.
