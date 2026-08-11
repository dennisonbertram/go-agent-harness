# Cross-Surface Impact Map: Issue #1042 Default Registry Repeatability

## Task

- Task / issue: isolate repeated default-registry test invocations, #1042.
- Plan: `2026-07-30-issue-1042-default-registry-repeat-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified; merge pending.

## Current Ownership, Callers, and Data Flow

- Entry: `TestDefaultRegistry_Functions`.
- Source of truth: package-level `defaultRegistry`; top-level
  `Register/List/New` delegate to it.
- Search: all `workspace.Register`, default-registry tests, init registration,
  and reset/unregister APIs.
- Conclusion: production correctly retains registrations; only the fixed test
  identity incorrectly assumes one invocation per process.

## Config, API, CLI, and Tools

- Config/env/defaults: none.
- API/CLI/wire/tools: production top-level registry API remains exercised.
- Errors: retain explicit `ErrAlreadyExists` and provisioning checks.

## Persistence and Compatibility

- State: process-local test registry only.
- Schemas/migrations/caches: none.
- Compatibility: no runtime change.

## Lifecycle, Security, and Reliability

- Concurrency: process-local atomic counter assigns invocation ownership.
- Auth/privacy/secrets: none.
- Failure/recovery: assertion output includes the generated name.

## Product and Integration Surfaces

- Server/TUI/web/macOS/providers: none.
- Automation: normal, race, repeated, and coverage gates become deterministic.
- UX/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Rollback: revert if first/duplicate registration is no longer distinguished.
- Operator docs: none.

## Regression Tests

- Red: focused `-count=2` fixed-name failure.
- Green: focused normal/race `-count=100`.
- Controls: List contains the invocation's name; New provisions exactly its
  factory; duplicate still fails.
- Full: workspace normal/race and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update engineering log, long-term log, plan, impact map, and plans index.
- No public docs.

## Warning Check

- All runtime/product surfaces are explicitly unaffected because the change is
  confined to fixture identity.
