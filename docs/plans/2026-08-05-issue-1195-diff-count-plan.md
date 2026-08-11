# Plan: Issue #1195 git diff-range summary counts

## Context

- Governing GitHub issue: #1195.
- Problem: `parseStatSummary` looks for `changed` in the second token of a
  summary clause. Git emits `1 file changed` and `N files changed`, placing it
  in the third token, so the public `files_changed` value is always zero.
- User impact: the deferred `git_diff_range` tool's stat string contradicts
  its structured aggregate in API/TUI/GUI transcripts.
- Constraints: retain the current JSON schema, `stat_only` contract, no-diff
  zero values, insertion/deletion counts, and command execution boundary.

## Scope

- In scope: strict summary parser correction, literal parser tests, a controlled
  two-commit tool fixture, exact-current fake-provider API/SSE continuation
  proof, and required logs/indexes.
- Out of scope: Git command invocation, request/result schema changes, other
  deep-git tools, persistence, scheduler, clients, and #1194 blame behavior.

## Documentation Contract

- Feature status: bug fix in implementation.
- Public docs affected: none; the established structured result is corrected.
- Implementation notes: this plan/map, logs, and indexes retain the parsing
  contract, acceptance evidence, and rollback boundary.

## Test Plan (TDD)

1. Add literal red tests for singular, plural, and files-only stat summaries;
   assert no-diff remains zero.
2. Add a red two-commit tool fixture asserting `files_changed=1`, additions,
   deletions, and exact returned stat for normal and `stat_only` calls.
3. Implement the smallest checked-token parser change.
4. Run focused normal/race tests, exact-current fake-provider API/SSE
   multi-message proof, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-05-issue-1195-diff-count-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1195 and exact post-#1196 base.
- [x] Record plan and impact map before source changes.
- [x] Capture parser and controlled tool-fixture red failures.
- [x] Implement minimal parser correction.
- [x] Prove parser/tool/API-SSE normal/race/full-regression behavior.
- [x] Update durable logs and indexes; publish one `Closes #1195` PR.

## Risks and Rollback

- Risk: accepting a non-summary line as a summary. Mitigation: require a
  leading integer and exact `file/files changed` clause before assigning count.
- Risk: silent regression of stat-only/no-diff values. Mitigation: regression
  assertions retain all aggregates for both calls and a no-diff literal.
- Rollback: revert parser and associated tests/docs. No migration, data repair,
  or client rollback is required.
