# Plan: Issue #1221 geometry-aware fresh PTY conversation evidence

## Context

- Governing GitHub issue: #1221 (parent #1088).
- Problem: an ad-hoc piped `script(1)` session inherited zero terminal rows and
  columns, so it could prove durable API/SSE state while the TUI viewport was
  intentionally empty. The official continuation runner already sets 30x100,
  but has no fresh-conversation scenario.
- User impact: real terminal evidence must distinguish a viewport setup defect
  from a missing chat reply, and establish the first reusable multi-turn
  evidence primitive for the slash-command matrix.
- Constraints: isolated fake provider, caller-owned artifacts, existing
  BSD/util-linux launcher only; no TUI, server, provider, or API product change.

## Scope

- In scope: a fail-closed 30x100 PTY launch, `user -> FIRST_REPLY -> /search
  FIRST_REPLY -> user -> SECOND_REPLY`, VT screens, raw SSE, API/store probe,
  hashes, and cleanup.
- Out of scope: all command variants, native UI, cron/callback proof, and
  product runtime behavior.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: None; this is test/acceptance infrastructure.
- Spec docs to update before code: this plan and its linked impact map.
- Implementation notes to add after code: durable logs and their indexes.

## Test Plan (TDD)

- New failing tests to add first: fresh runner contract requires a geometry-aware
  launcher and all four visible milestones plus two ordered durable runs.
- Existing tests to update: PTY launcher args must reject the zero-geometry
  fresh form; continuation behavior stays characterized.
- Regression tests required: focused normal/race `ptyrunner`, then full
  `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

See `2026-08-06-issue-1221-pty-fresh-impact-map.md`.

## Implementation Checklist

- [x] Verify #1221/#1088 contract and existing official PTY seam.
- [x] Record plan and cross-surface impact analysis before runner code.
- [x] Add failing fresh evidence contract.
- [x] Implement the smallest official-runner-only fresh scenario.
- [x] Capture/re-read/hash terminal, VT, keys, SSE, and API/store evidence.
- [x] Update logs/indexes and record exact verification.
- [x] Run focused normal/race and full regression.
- [ ] Commit, push, and open a single `Closes #1221` PR; do not merge.

## Risks and Mitigations

- Risk: pipe-backed `script` inherits zero geometry. Mitigation: reuse the
  `stty rows 30 cols 100` launcher and assert its exact argv in tests.
- Risk: raw ANSI text is stale or hidden. Mitigation: persist a VT-interpreted
  frame for every user-visible milestone.
- Risk: a second prompt is combined with the first. Mitigation: wait for the
  first rendered reply and its completed durable run before typing `/search`
  or turn two.
