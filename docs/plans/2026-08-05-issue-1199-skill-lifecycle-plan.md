# Issue #1199 Skill Lifecycle Plan

Status: implemented locally; verification pending full regression.

1. Red: characterize watcher-disabled create followed immediately by verify/list.
2. Green: reload the authored-skill registry synchronously after a successful `create_skill` write.
3. Route core, deferred, and HTTP verification through the adapter persistence boundary: write `SKILL.md`, then reload the registry; a reload failure is returned as failure.
4. Preserve O_EXCL create semantics, SKILL.md compatibility, and separate pack registry ownership.
5. Validate focused normal/race and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

Rollback: revert lifecycle wiring. Existing SKILL.md files need no migration.
