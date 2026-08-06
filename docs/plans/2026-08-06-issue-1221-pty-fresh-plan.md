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
  FIRST_REPLY -> Escape -> user -> SECOND_REPLY -> /quit`, a Go-owned PTY
  master with a single append-only collector, immutable per-action VT screens plus offset/hash/run
  frame records, raw SSE, API/store probe, hashes, and cleanup.
- Out of scope: all command variants, native UI, cron/callback proof, and
  product runtime behavior.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: None; this is test/acceptance infrastructure.
- Spec docs to update before code: this plan and its linked impact map.
- Implementation notes to add after code: durable logs and their indexes.

## Test Plan (TDD)

- New failing tests to add first: fresh runner contract requires a geometry-aware
  flushing launcher, monotonic immutable frame seals, and all visible milestones
  plus two ordered durable runs.
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
- [x] Capture/re-read/hash terminal, ordered VT frames, keys, SSE, and API/store evidence.
- [x] Update logs/indexes and record exact verification.
- [ ] Re-run focused normal/race and full regression after ordered-frame repair.
- [ ] Rebase, amend, and force-push the existing single `Closes #1221` PR; do not merge.

## Risks and Mitigations

- Risk: pipe-backed `script` inherits zero geometry. Mitigation: reuse the
  `stty rows 30 cols 100` launcher and assert its exact argv in tests.
- Risk: raw ANSI text is stale or hidden. Mitigation: one collector alone reads
  the Go-owned PTY master and persists its append-only transcript; each action seals an immutable
  `[start,end)` prefix, input/prefix/render hashes, and a VT-interpreted frame.
- Risk: a second prompt is combined with the first. Mitigation: the sequencer
  waits for the first rendered reply and completed durable run, seals its frame,
  then types `/search`; it repeats that ordering through Escape and turn two.
