# Plan: Issue #1247 cron core-tool documentation and registry regression

## Context

- Governing GitHub issue: #1247.
- Problem: production registers all eight cron tools as initial-turn core tools,
  but the public docs still call a six-tool subset deferred and direct a model
  to `find_tool`. That stale contract caused the invalid #1245 premise.
- User impact: an agent needs to see the whole CRUD/history cron surface in its
  initial schema, without attempting deferred discovery for it.
- Constraints: preserve existing scoped-client ownership, authorization, and
  execution behavior. This is documentation and regression coverage only.

## Scope

- In scope: website documentation, an explicit documentation contract test,
  and default-registry assertions for all eight core cron tools and their
  absence from deferred definitions.
- Out of scope: moving cron registrations, changing `find_tool`, cron clients,
  authorization, job execution, persistence, API, or product clients.

## Documentation Contract

- Feature status: implemented.
- Public docs affected: `website/docs/integrations/cron-scheduling.md` and
  `website/docs/concepts/tools-and-permissions.md`.
- Spec docs to update before code: this plan and impact map record the current
  production contract.
- Implementation notes to add after code: durable engineering, observational,
  and system logs plus their index entries.

## Test Plan (TDD)

- New failing test first: a repository documentation contract that rejects the
  stale deferred phrasing and requires all eight names plus direct/core wording.
- Existing test update: `TestCronToolsAreCoreNotDeferred` must prove every
  cron tool is visible with no activation and none is returned by
  `DeferredDefinitions`.
- Regression tests required: direct initial-turn visibility and exact
  core-versus-deferred boundary only; no generic `find_tool` behavior change.

## Cross-Surface Impact Map

- See `2026-08-07-issue-1247-cron-core-registry-impact-map.md`.

## Implementation Checklist

- [x] Verify structured issue #1247 and closed Sol verdict on #1245.
- [x] Record architecture search evidence and current production base.
- [x] Complete plan and impact map.
- [x] Add red documentation and default-registry regression coverage.
- [x] Update public docs to the implemented eight-tool core contract.
- [x] Record durable logs and maintain indexes.
- [x] Run focused normal/race and full regression.
- [ ] Push a reviewable PR that closes #1247; do not merge.

## Risks and Mitigations

- Risk: a test accidentally enforces direct execution behavior rather than
  initial schema visibility. Mitigation: assert `DefinitionsForRun` with a nil
  activation tracker and `DeferredDefinitions`, not generic registry execution
  or `find_tool` selection.
- Risk: stale public docs elsewhere keep suggesting deferred cron discovery.
  Mitigation: search the website tree for both `cron` and `deferred` before
  final verification.
