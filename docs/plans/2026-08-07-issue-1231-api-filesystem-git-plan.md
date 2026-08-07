# Plan: Issue #1231 default-registry filesystem and Git API/SSE acceptance

## Context

- Governing GitHub issue: #1231.
- Problem: the generic API/SSE runner proves transport/lifecycle but not the real effects of the default filesystem and Git tools.
- User impact: an agent must be able to make a later turn in the same conversation inspect and modify a disposable repository, while operators retain independent API/SSE/store and artifact evidence.
- Constraints: one isolated initialized Git repository; default registry; fake scripted provider; no user workspace, network, GUI/TUI, cron, or callback work.

## Scope

- In scope: reusable real-daemon API/SSE sequence driver and acceptance coverage for `ls`, `glob`, `grep`, `read`, `write`, `edit`, `apply_patch`, `file_inspect`, `git_status`, `git_diff`, `git_diff_range`, `git_log_search`, `git_file_history`, `git_blame_context`, and `git_contributor_context`.
- Out of scope: product tool behavior changes, all other tools, GUI/TUI, scheduling, and remote services.

## Documentation Contract

- Feature status: `review-repaired and fully regression-tested locally; independent review pending`.
- Public docs affected: None; this is a test-only acceptance capability.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering log and acceptance-inventory status/index references.

## Test Plan (TDD)

- New failing tests to add first: a real default-daemon fixture that rejects a sequence without the ordered tool-call records and external fixture Git/filesystem postconditions.
- Existing tests to update: `internal/acceptance/apisserunner` unit coverage for multi-run same-conversation sequencing and exact tool-event validation.
- Regression tests required: targeted normal/race `cmd/harnessd` and `internal/acceptance/apisserunner`, then the external-cache repository regression suite.

## Cross-Surface Impact Map

- Required map: `2026-08-07-issue-1231-api-filesystem-git-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1231 and source ownership/search evidence.
- [x] Create plan and complete cross-surface map.
- [x] Add the deliberately red real-daemon sequence test.
- [x] Implement only reusable acceptance-driver support required by the test.
- [x] Record the canonical live `/v1/tools` inventory hash and per-tool owner/condition/provenance rows.
- [x] Retain and digest raw SSE, terminal run, conversation-store, fixture, and external Git-probe artifacts.
- [x] Retain artifacts beyond test return under a configured private root and record explicit fixture-cleanup evidence.
- [x] Assert exact non-empty durable assistant replies in order, conversation-scoped distinct tool-call IDs, ordered starts/completions, and terminal SSE events.
- [x] Run targeted normal/race and external-cache full regression.
- [x] Update logs/indexes; amend PR with exact verification evidence.
- [x] Commit, push, and open a closing PR; do not merge.

## Risks and Mitigations

- Risk: a scripted completion can make a tool event look successful while the tool did nothing. Mitigation: require exact SSE tool identity/arguments/result plus independent fixture probes after each ordered turn.
- Risk: shell/Git fixture setup leaks or mutates user state. Mitigation: temporary root, explicit Git identity, explicit `RemoveAll` plus retained cleanup evidence; the evidence directory is separate, private, and intentionally retained.
- Risk: proof is lost when Go removes `t.TempDir`. Mitigation: `HARNESS_ISSUE_1231_ARTIFACT_ROOT` selects a private `0700` parent (default: a private directory under the system temp root); every run creates a unique private child and intentionally does not delete it.
