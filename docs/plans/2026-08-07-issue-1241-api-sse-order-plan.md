# Plan: Issue #1241 per-tool raw SSE lifecycle validator

## Context

- Governing GitHub issue: #1241; stacked on unmerged #1232 at
  `7128c855999e60fc504771a1a7fb6f7b84a18b16`.
- Delivery: pushed stacked PR #1242
  (`codex/issue-1241-api-sse-order` ->
  `codex/issue-1231-api-filesystem-git`) at code head
  `8f8c58b6a2486acd3b8ef31f117be744ba6baf35`.
- Problem: #1231's acceptance helper collects `tool.call.started` and
  `tool.call.completed` frames into independent slices, so it can falsely pass
  a completion that preceded its start or belonged to another tool call.
- User impact: the retained real-daemon evidence must prove each filesystem/Git
  tool actually followed a valid raw-SSE lifecycle, including concurrent calls.
- Constraints: acceptance-test code only; leave product SSE/tool behavior and
  #1232's branch unchanged; #1241 PR targets #1232's branch.

## Scope

- In scope: ordered raw-SSE validation in
  `cmd/harnessd/filesystem_git_api_acceptance_test.go`, deterministic unit
  regressions, and durable engineering/observation/system logs.
- Out of scope: runner/server event production, transport changes, tool
  implementation, GUI/TUI/scheduled work, and rebasing or editing #1232.

## Documentation Contract

- Feature status: implemented and pushed; fresh exact-head review pending.
- Public docs affected: None; this is a test-only evidence repair.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: logs and their indexes.

## Test Plan (TDD)

- New failing tests first: deterministic raw frames that the old split-slice
  validator falsely accepts when completion arrives before start; orphan,
  duplicate-start, duplicate-completion, name/ID mismatch, wrong-run, and
  valid A/B interleaving controls.
- Existing tests to update: the #1231 direct real-daemon acceptance keeps the
  same four durable turns and validates their raw streams through the new
  matcher.
- Regression tests required: focused normal/race `cmd/harnessd`, direct
  #1231 real-daemon acceptance, then `./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- Required map: `2026-08-07-issue-1241-api-sse-order-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1241, #1232 dependency head, and current
  assertion ownership.
- [x] Create plan and complete the cross-surface impact map.
- [x] Add and capture deterministic red tests.
- [x] Replace split-slice matching with run-scoped lifecycle matching.
- [x] Preserve valid concurrent interleaving and per-run duplicate detection.
- [x] Run focused normal/race and direct #1231 real-daemon acceptance.
- [x] Update logs/indexes and exact PR evidence.
- [x] Run full regression (`85.1%`, zero uncovered functions).
- [x] Commit, push, and open stacked closing PR #1242 (`Closes #1241`).
- [ ] Obtain a fresh independent read-only review of the documentation-amended
  exact head; do not merge this stacked PR directly to `main`.

## Evidence

- Red: `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1241-red go test
  ./cmd/harnessd -run '^TestFilesystemGitToolCallLifecycleRejectsCompletionBeforeStart$' -count=1`
  failed at the explicit false-pass sentinel.
- Green focused normal/race: lifecycle table plus the real #1231 one-daemon,
  four-turn acceptance passed.
- Full: `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1241-full-cache2
  ./scripts/test-regression.sh` passed normal, race, coverage, 85.1% total,
  and zero uncovered functions; retained log:
  `/private/tmp/gocode-1241-full.log`.

## Risks and Mitigations

- Risk: a global call-ID uniqueness rule rejects a protocol-valid reuse in a
  later run. Mitigation: track IDs by `(runID, callID)` and reject duplicates
  only inside one raw run transcript.
- Risk: serial matching rejects legal concurrent calls. Mitigation: retain an
  unmatched-start map so `start A, start B, complete B, complete A` succeeds.
- Risk: acceptance-only code is mistaken for product behavior. Mitigation:
  document the boundary and retain the existing real-daemon test unchanged.
