# Plan: Issue #1257 profile source-tier resolver

## Context

- Governing GitHub issue: #1257.
- Problem: detail, `get_profile`, and profile manifests infer a source tier by
  reusing a fallback-capable loader. An existing but empty configured directory
  therefore misreports a built-in as project or user.
- User impact: all discovery surfaces must report the actual defining tier,
  including a child profile that inherits fields from a different tier.

## Scope

- In scope: one profile-layer resolver returning the resolved profile and its
  winning top-level tier; its HTTP detail, deferred-tool, and manifest callers;
  core/server/deferred/manifest regressions.
- Out of scope: profile schema, persistence, CRUD, precedence redesign, cache,
  runtime execution, TUI/GUI redesign, and deployment configuration.

## Test Plan (TDD)

- First red: resolver and three consumer regressions prove an embedded profile
  remains `built-in` with empty-but-existing project/user directories.
- Precedence: project wins over user; user wins over built-in; an inherited
  child reports the tier of the child file, not its base.
- Freshness: add and remove an override between resolver calls and assert the
  tier/model update immediately, proving no cache was introduced.

## Implementation Checklist

- [x] Verify complete issue and record current ownership/impact evidence.
- [x] Create plan and cross-surface impact map before code.
- [ ] Capture focused red core/server/deferred/manifest regressions.
- [ ] Introduce and adopt the shared resolver without changing loader behavior.
- [ ] Run focused package tests and `./scripts/test-regression.sh`.
- [ ] Record logs/indexes and real API multi-turn parity evidence.
- [ ] Commit, push, and open one PR that closes #1257; do not merge.

## Rollout and Rollback

- Rollout: ordinary code deployment; no cache, migration, or data rewrite.
- Rollback: cleanly revert this single source-provenance slice.
