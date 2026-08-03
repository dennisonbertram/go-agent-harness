# Plan: Issue #1125 native run-action ownership

## Context and Scope

- Governing issue: #1125, stacked on #1122 head `d1931ae2`.
- Stop, Composer steer, and ToolWalk timeout previously resolved `currentRunID`
  at action time, so stale A UI could target newly selected B.
- In scope: expected-run cancel/steer APIs, captured identities in native UI and
  ToolWalk, deterministic A-to-B tests, and inherited strict-format repair.
- Out of scope: harness API, callback dispatch, TUI, selection policy, and
  #1122 approval/plan/input fences.

## TDD and Verification

- Expected red: a captured A action can issue B cancel/steer after selection.
- Green: stale A yields zero B endpoint calls; B remains selected and draft is
  preserved; legitimate B actions, #994 delayed ACK/force-stop, and external
  isolation remain covered.
- Gates: strict Swift format, focused native/ToolWalk tests, full Swift package,
  repository regression, then hosted checks and independent review.

## Impact Map

- `2026-08-03-issue-1125-action-owner-impact-map.md`.

## Checklist

- [x] Verify issue and exact #1122 stack base.
- [x] Add expected-run fences, visible captures, and deterministic regressions.
- [x] Repair strict Swift formatting.
- [ ] Run complete gates, push draft `Closes #1125`, and obtain cheap review.
