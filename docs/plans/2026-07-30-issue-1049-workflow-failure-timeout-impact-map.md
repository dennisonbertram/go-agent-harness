# Cross-Surface Impact Map: Issue #1049 Workflow Failure-Event Timeout

## Task

- Task / issue: stabilize the live workflow failure-event test, #1049.
- Plan: `2026-07-30-issue-1049-workflow-failure-timeout-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry: `TestEngineDefinitionSubscribeAndFailureEvents`.
- Production flow: Engine persists failed workflow state and publishes
  `workflow.failed` to its subscribers.
- Test flow: subscribe, wait for stored terminal state, then drain the live
  stream until the failure event.
- Search conclusion: the two-second timer is test-local; production API,
  workflow, and provider deadlines are separate.

## Config, API, CLI, and Tools

- Config/env/defaults: none.
- API/CLI/wire/tool behavior: unchanged.
- Event contract: continue requiring exact `workflow.failed` delivery.

## Persistence and Compatibility

- Stores/schemas/migrations/caches: unchanged.
- Compatibility: no runtime change.

## Lifecycle, Security, and Reliability

- Concurrency: allow race-instrumented shared CI enough scheduling time.
- Cleanup: stop the timer and cancel the subscription.
- Auth/privacy/secrets: none.
- Failure diagnostics: retain the complete observed event list.

## Product and Integration Surfaces

- Harness/workflow runtime, API, TUI, web, macOS GUI, providers: no code change.
- Automation: full race and hosted gates become host-speed tolerant.
- UX/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Rollback: revert if a genuinely absent failure event no longer fails.
- Observability/operator docs: none.

## Regression Tests

- Red: full race gate timed out after two seconds with the first two events.
- Green: focused normal/race `-count=100`.
- Controls: stored status and error remain failed; live stream must still
  contain `workflow.failed`.
- Full: workflows normal/race and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update engineering log, long-term log, plan, impact map, and plans index.
- No public docs.

## Warning Check

- All runtime/product surfaces are explicitly unaffected because the repair is
  confined to one test-owned timer.
