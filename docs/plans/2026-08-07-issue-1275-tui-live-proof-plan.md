# Plan: Issue #1275 replay-boundary live TUI proof

## Context

- Governing issue: #1275.
- Problem: the replay-boundary acceptance test required historic, queued, and
  live markers to coexist in an auto-scrolled `View`. A valid short viewport
  made the predicate time out despite decoded/reduced live SSE.
- User impact: race CI reported a lost scheduled continuation without proving
  whether delivery or the test predicate was at fault.
- Constraints: test-only; no timeout increase or product bridge/reducer change.

## Scope

- In scope: make fixture release depend on exact-once snapshot transcript
  entries; stop only after post-update decoded `live:3` assistant event is
  visibly rendered; retain transcript and no-history-fetch assertions and add
  causal diagnostics.
- Out of scope: bridge/model production code, user-visible behavior, timeouts,
  or weakening snapshot/live assertions.

## Test Plan (TDD)

- Expected red: short viewport makes the old simultaneous-View condition
  timeout before live release.
- Green: focused normal/race x10, TUI race package, full regression.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1275-tui-live-proof-impact-map.md`.

## Implementation Checklist

- [x] Capture short-viewport red reproduction.
- [x] Gate fixture release on transcript exact-once state.
- [x] Prove decoded/reduced live `live:3` event and retain assertions.
- [x] Run focused normal/race x10.
- [ ] Run package race, full regression, commit, push, and closing PR.

## Risks and Mitigations

- Risk: hiding a real bridge loss. Mitigation: stop only on the decoded
  `SSEEventMsg` after `Update` renders live content; diagnostics distinguish
  snapshot, decode, and reduction stages.
