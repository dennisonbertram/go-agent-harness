# Plan: Issue #1194 porcelain blame parser

## Context

- Governing GitHub issue: #1194.
- Problem: `parsePorcelainBlame` accepts arbitrary long metadata as a header,
  so `previous <hash> <path>` overwrites real blame identity. Failed `git show`
  output can also become a displayed commit subject.
- User impact: `git_blame_context` can report line zero, an invented commit,
  and fatal Git output instead of the file's actual authoring history.
- Constraints: retain the existing JSON schema and best-effort enrichment; an
  individual enrichment failure must not fail the tool.

## Scope

- In scope: strict porcelain-header recognition, non-success/timed-out
  enrichment suppression, literal parser and real two-commit rewrite tests,
  and exact fake-provider API/SSE acceptance evidence.
- Out of scope: Git tool schemas, persistence, scheduler behavior, profiles,
  TUI/GUI implementation, and changes to #1195's diff-range seam.

## Documentation Contract

- Feature status: implementation after red-green verification.
- Public docs affected: none; the output shape is unchanged.
- Implementation notes: plans/logs/indexes record the parser boundary,
  best-effort enrichment rule, acceptance evidence, and rollback.

## Test Plan (TDD)

1. Add literal porcelain regressions: valid 40/64-hex headers followed by
   `previous` and malformed long metadata must retain the valid hash, positive
   line, content, and one unique commit. Confirm these fail against the old
   parser.
2. Add a real two-commit rewrite tool test proving a correct subject and no
   `fatal`/`previous` subject.
3. Require enrichment only when `git show` exits zero and does not time out;
   retain empty best-effort fields otherwise.
4. Run targeted normal/race tests, the exact fake-provider API/SSE continuation
   proof, then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Cross-Surface Impact Map

- See `2026-08-05-issue-1194-blame-parser-impact-map.md`.

## Implementation Checklist

- [x] Verify issue #1194 and inspect the current parser/tool path.
- [x] Record plan and impact map before source changes.
- [ ] Capture the expected-red literal parser test.
- [ ] Implement strict header and enrichment-result validation.
- [ ] Prove literal, two-commit, API/SSE, normal/race, and full regression paths.
- [ ] Update durable logs/indexes and open one PR with `Closes #1194`.

## Risks and Rollback

- Risk: overly strict recognition drops valid porcelain records. Mitigation:
  cover both supported object-id lengths and exact optional group-count form.
- Risk: a diagnostic Git failure is exposed as user content. Mitigation: accept
  enrichment only on zero-exit, non-timeout execution.
- Rollback: revert parser/enrichment checks and tests. No migration, stored
  data repair, or wire rollback is required.
