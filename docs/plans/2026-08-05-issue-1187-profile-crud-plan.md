# Plan: Issue #1187 isolated harnessd profile CRUD

## Context

- Governing GitHub issue: #1187.
- Problem: profile CRUD exists below the daemon boundary but current harnessd
  composition omits the profile directories, leaving agent tools absent and
  HTTP mutation routes unconfigured.
- User impact: operators cannot safely exercise profile creation or mutation
  without writing to their real user profile directory.
- Constraints: the override must be absolute, opt-in, preserve built-in /
  project / user lookup precedence, and never require a `HOME` override.

## Scope

- In scope: resolve `HARNESS_PROFILES_DIR`; wire one derived project/user/
  mutation path into the registry, runner, and HTTP server; add listener-aware
  real-daemon API and fake-provider multi-turn acceptance evidence.
- Out of scope: profile schema migration, profile-selection UI semantics, and
  changing default user directory behavior.

## Documentation Contract

- Feature status: `in implementation`.
- Public docs affected: `docs/runbooks/profile-authoring.md`.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering, observational, system,
  and long-term-thinking logs and their indexes.

## Test Plan (TDD)

- New failing tests first: absolute override resolution; bootstrap propagation
  to registry/runner/server; and a listener-aware daemon acceptance that
  creates, reads, updates, reads, deletes, and observes not-found in the same
  isolated runtime.
- Existing tests to update: bootstrap forwarding and profile documentation
  examples.
- Regression tests required: default path unchanged, relative override denied,
  built-in protection, traversal rejection, atomic profile write, precedence,
  normal/race full regression.

## Cross-Surface Impact Map

See `2026-08-05-issue-1187-profile-crud-impact-map.md`.

## Implementation Checklist

- [x] Read issue #1187 and current ownership/search evidence.
- [x] Document contract and cross-surface impact before code.
- [x] Add failing acceptance and propagation tests.
- [x] Add minimal production path resolver and composition wiring.
- [x] Prove real isolated daemon API + fake-provider multi-turn flow.
- [x] Update public docs, durable logs, and indexes.
- [x] Run canonical full normal/race/coverage regression.
- [x] Repair exact-head MCP stdio composition review finding and rerun gates.
- [ ] Push one reviewable `Closes #1187` PR; do not merge.

## Risks and Mitigations

- Risk: a configuration path can silently write to a real user directory.
  Mitigation: accept only an absolute explicit override, preserve the default
  only when omitted, and assert acceptance writes only below `t.TempDir()`.
- Risk: registry, runner, and server drift to different profile tiers.
  Mitigation: derive paths once at startup and pass the same values through all
  three composition boundaries with direct forwarding tests.
