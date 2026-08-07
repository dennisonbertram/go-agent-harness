# Plan: Issue #1272 fetched bootstrap source provenance

## Context

- Governing GitHub issue: #1272.
- Problem: `scripts/init.sh` could reuse an older task worktree without
  refreshing or recording the requested bootstrap source; a mutable
  `FETCH_HEAD` also made source selection vulnerable to unrelated fetches.
- User impact: acceptance and operator worktrees could not reliably state
  which merged-main revision they were bootstrapped from.
- Constraints: preserve legitimate task commits in reused worktrees; do not
  reset branches or broaden the script beyond bootstrap provenance.

## Scope

- In scope: resolve unqualified and `origin/` branch refs through freshly
  fetched remote-tracking refs; accept existing commit SHAs and explicit local
  `refs/heads/` refs; verify newly created `-b` worktrees; record requested
  source and actual worktree revisions in the generated environment file.
- Out of scope: changing task-branch reuse semantics, remote topology,
  harness runtime, API/TUI/GUI behavior, or deployment.

## Documentation Contract

- Feature status: `implemented` in this PR, pending review and merge.
- Public docs affected: none; this is a developer bootstrap contract.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering and long-term logs.

## Test Plan (TDD)

- New failing test first: a remote advances after initial task-worktree
  creation; reuse must preserve task HEAD while recording refreshed source;
  explicit `origin/main`, SHA, and `refs/heads/main` sources must select their
  exact commits.
- Existing tests: retain fresh default-origin-main and metadata provenance
  coverage.
- Regression tests: focused bootstrap acceptance suite and full repository
  regression gate.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1272-init-fetched-base-impact-map.md`.

## Implementation Checklist

- [x] Confirm structured issue #1272 and current script ownership.
- [x] Add and run deterministic red regression.
- [x] Resolve stable source SHAs and record provenance.
- [x] Preserve reusable task worktree HEADs.
- [x] Update documentation and indexes.
- [ ] Run full regression, commit, push, and open the closing PR.

## Risks and Mitigations

- Risk: treating reuse as a reset destroys task commits. Mitigation: verify
  only fresh `git worktree add -b` creation and record reuse HEAD separately.
- Risk: concurrent fetch changes `FETCH_HEAD`. Mitigation: resolve the
  fetched `refs/remotes/origin/<branch>` commit before worktree creation.
