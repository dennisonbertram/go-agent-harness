# Issue #1009 — macOS scheduled-task lifecycle and controls

## Context

- Governing GitHub issue: #1009 (child of #1000).
- Problem: `/v1/tasks` currently exposes only a generic task row and the macOS
  Activity page renders it read-only.  Cron jobs and callbacks therefore lack
  visible timing/result/run linkage and cannot be controlled from the app.
- User impact: a deployment watcher must be observable, controllable, and
  reconciled to the server rather than represented by a model text claim.
- Constraints: additive wire contract; server remains authoritative for every
  action; do not alter scheduler or retry behavior; preserve generic task rows.

## Scope

- In scope: additive cron/callback lifecycle fields in task rows; typed,
  forward-compatible Swift task values; client requests for pause/resume/delete
  and cancel; Activity detail/action UX with accessibility; reconciliation and
  regression coverage.
- Out of scope: scheduler persistence/retry algorithm, a general notification
  system, TUI changes, and final #1010 live proof.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: native macOS and API behavior documentation only after
  implementation is tested.
- Implementation notes: engineering log and plan indexes record the actual
  additive contract.

## Test Plan (TDD)

- First red: server task-union contract asserts cron next/last execution/run
  fields, callback due time, and authoritative actions; Swift client tests
  assert typed unknown decoding and exact control requests.
- Existing tests updated: task API and Activity session tests.
- Review repair red: stale cron pause/resume/delete requests must return 409
  without mutation; versioned Swift requests must preserve an empty body for
  older rows; active versus terminal linked runs must not share controls.
- Timestamp-wire repair red: a nanosecond `updated_at` token must decode and
  return to the action endpoint byte-for-byte; a client-truncated token must
  receive 409 without mutating the cron job.
- Regression: focused Go server normal/race, focused Swift package tests, full
  `./scripts/test-regression.sh`, and `swift test --package-path macapp`.

## Cross-Surface Impact Map

See `2026-08-04-issue-1009-macapp-task-lifecycle-impact-map.md`.

## Implementation Checklist

- [x] Verify issue contract and architecture/search evidence.
- [x] Create plan and impact map.
- [x] Capture red task serialization/client-action tests.
- [x] Implement additive server and typed client contract.
- [x] Add accessible Activity actions and server reconciliation.
- [x] Update logs/indexes; issue evidence follows final regression.
- [x] Pass focused and full regression gates.
- [x] Preserve opaque cron action version tokens through final review repair.
- [x] Keep no-store callback terminal rows and their lifecycle timestamps in
  the all-state task inventory without changing legacy active-only lists.

## Risks and Mitigations

- Stale UI actions could claim success: each action refreshes `/v1/tasks` in a
  `defer` path and surfaces the server error. Cron actions additionally send
  optional `expected_updated_at` as the opaque server string (rather than a
  reformatted `Date`); stale versions and invalid state transitions return 409
  without mutation.
- Mixed server versions omit fields/actions: optional fields decode as absent,
  unknown enum values remain displayable, and controls remain hidden.
- Destructive deletion: confirmation is required in the Activity UI.
- Linked run authority: opening a task loads its durable conversation first;
  only a non-terminal event accepted by the run reducer can make its linked
  run a live control target.
- No-store callback terminal visibility: `ListAllCallbacks` snapshots retained
  manager rows for the task API, while `List` and `ListCallbacks` remain backed
  by the active `byConv` index; every legacy cancel/fire/shutdown terminal
  transition advances `updated_at`.
