# Plan: Issue #1198 isolated harness skill directory

## Context

- Governing GitHub issue: #1198.
- Problem: `HARNESS_SKILLS_DIR` is advertised by `create_skill` but ignored by
  harnessd's loader, registry, watcher, and workflow wiring; authored skills
  escape an operator's intended isolated directory.
- User impact: acceptance environments and operators cannot safely author,
  reload, list, or verify skills without writing to the default global root.
- Constraints: one absolute global skill root must be resolved once; unset
  behavior stays `$HARNESS_GLOBAL_DIR/skills`; relative overrides fail startup.

## Scope

- In scope: resolve and thread `HARNESS_SKILLS_DIR` through the skill loader,
  registry/create tool, watcher, and Go-workflow skill directories; tests and
  operator documentation.
- Out of scope: workspace-local skills, profile paths, plugin skills, schema
  migrations, or changes to skill format/precedence.

## Documentation Contract

- Feature status: in implementation.
- Public docs affected: skills integration, environment reference, workflow
  SDK path table, and skills/profiles concepts.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: durable engineering, observational,
  system, and long-term-thinking logs plus their indexes.

## Test Plan (TDD)

- New failing tests to add first: absolute override loads only override skills;
  relative/whitespace-relative override exits before listener acquisition;
  unset fallback remains `$HARNESS_GLOBAL_DIR/skills`; watcher reload observes
  a created override skill; TUI local slash catalog sees the override and does
  not fall back to global skills for an invalid override.
- Existing tests to update: custom global-directory matrix becomes an explicit
  unset-fallback assertion.
- Regression tests required: fake-provider HTTP/SSE multi-message acceptance
  creates a skill only under the override then proves catalog/GET/watcher
  observation and no default-root write; normal, race, and full suite.

## Cross-Surface Impact Map

- See `2026-08-05-issue-1198-skills-dir-impact-map.md`.

## Implementation Checklist

- [x] Verify contract-complete issue #1198 and search existing skill wiring.
- [x] Record current source-of-truth and impact map before code.
- [x] Add and capture red tests.
- [x] Resolve one absolute global skill directory and thread all consumers.
- [x] Update public docs and durable logs/indexes.
- [x] Run fake-provider acceptance and `TMPDIR=/private/tmp ./scripts/test-regression.sh` (normal, race, coverage 85.6%, zero uncovered functions).
- [x] Repair review-found TUI local-catalog consumer with focused normal/race regression; final amended full regression pending.
- [ ] Open a single PR with `Closes #1198`; do not merge in this slice.

## Risks and Mitigations

- Risk: one consumer silently retains the default root. Mitigation: test loader,
  registry, watcher, workflow inputs, and real authored-file path together.
- Risk: a relative override can be interpreted from an unsafe CWD. Mitigation:
  reject it before any daemon resource/listener is created.
- Rollback: unset the override or revert this wiring; no persisted migration is
  introduced and the former `$HARNESS_GLOBAL_DIR/skills` fallback remains.
