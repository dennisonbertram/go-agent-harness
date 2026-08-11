# Cross-Surface Impact Map: Issue #1247 cron core-tool contract

## Task

- Task / issue: #1247 — align cron core-tool documentation and regression.
- Plan link: `2026-08-07-issue-1247-cron-core-registry-plan.md`.
- Owner: default registry documentation/regression slice.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `NewDefaultRegistryWithOptions` constructs the model-visible
  default registry; `DefinitionsForRun` filters only deferred tools by
  activation.
- Owning source of truth: `internal/harness/tools_default.go` appends the eight
  cron definitions through `catalogTools` to `coreTools` when the scoped cron
  client is configured.
- Callers/consumers: Runner passes `DefinitionsForRun` to providers; public
  website docs describe that contract.
- Similar abstractions searched: callback core tests and generic registry
  `DeferredDefinitions`; `find_tool` unit fixtures intentionally model
  arbitrary deferred tools and are not default-registry evidence.
- Search evidence: `rg -n "cron_(create|list|get|delete|pause|resume|update|history)|find_tool|deferred" internal cmd docs website`.
- Conclusion: tier is assigned at default-registry assembly, not by the tool
  constructors whose reusable definitions retain a deferred default.

## Config, API, CLI, and Tools

- Config: none; existing `CronClient` conditional registration is unchanged.
- API/CLI/wire: none.
- Tools: documentation and test contract explicitly identify all eight as core
  and state that `find_tool(select:...)` is not required for initial use.
- Errors/validation: none; no change to execution paths.

## Persistence and Compatibility

- Schemas/migrations/caches: none.
- Compatibility: correcting docs aligns existing clients and providers with
  production; no runtime version behavior changes.
- Partial rollout: documentation and tests ship together; rollback is a single
  documentation/test revert.

## Lifecycle, Security, and Reliability

- Lifecycle/concurrency/retries/cleanup: none; no runtime change.
- Auth/authorization/privacy/secrets: unchanged scoped cron client and policy
  enforcement; tests do not bypass either.
- Recovery/data repair: none.

## Product and Integration Surfaces

- Server/runtime: only default-registry schema assertions; no handler change.
- TUI/web/macOS: no client change; their agent runtimes receive the existing
  initial core schema.
- Provider/model/tool catalog: validates existing initial tool catalog contract
  and no deferred cron definitions.
- External systems/UX: cronsd execution, UI status, and schedule behavior are
  explicitly excluded.

## Deployment and Operations

- Deployment/flags: none.
- Diagnostics: test failures name the missing core or unexpectedly deferred
  tool; no new telemetry.
- Rollback: revert docs/test-only change; no data or service action.
- Runbooks: no operational change.

## Regression Tests

- First expected red: a documentation contract rejects current stale website
  language that says six cron tools are deferred and require `find_tool`.
- Acceptance: one default registry with a stub scoped client exposes exactly
  all eight names on `DefinitionsForRun("run", nil)` and exposes none through
  `DeferredDefinitions`.
- Negative/security/lifecycle: no configured client still registers no cron
  tools in existing coverage; authorization/execution remain covered by their
  existing tests.
- Integration/e2e: #1247 intentionally does not claim cron execution; the
  parent acceptance suite is rerun after this repair.
- Commands: focused normal/race `go test ./internal/harness -run
  'TestCronToolsAreCoreNotDeferred|TestCronDocumentation'`; then
  `./scripts/test-regression.sh` with isolated external caches.

## Documentation and Handoff

- Public docs: update both stale deferred scheduling statements and the tool
  tier list with all eight current names.
- Implementation notes: append durable logs and indexes after green tests.
- Training/release: no release note; hand off exact direct-core contract to
  scheduled-tool matrix work.

## Warning Check

- Every surface is addressed. Unaffected surfaces are explicitly marked none
  because this slice changes no runtime behavior.
