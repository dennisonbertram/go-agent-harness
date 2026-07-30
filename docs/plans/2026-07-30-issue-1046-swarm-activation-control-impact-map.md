# Cross-Surface Impact Map: Issue #1046 Swarm Activation Control

## Task

- Task / issue: stabilize the unrestricted swarm activation control, #1046.
- Plan: `2026-07-30-issue-1046-swarm-activation-control-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry: `TestAgentSwarmDeniedForMemberRuns`.
- Production source of truth: `ActivationTracker` owns per-run activations;
  terminal Runner paths call `Cleanup(runID)`.
- Test flow: the member run exhausts `capturingProvider.turns`; the following
  unrestricted run can complete immediately, racing activation inspection.
- Search conclusion: adjacent swarm tests do not reuse an exhausted provider
  for a live activation control.

## Config, API, CLI, and Tools

- Config/env/defaults: none.
- API/CLI/wire/tool behavior: unchanged.
- Tool contract: keep `agent_swarm` hidden for denied members and visible after
  activation for unrestricted runs.

## Persistence and Compatibility

- State: test-local release channel only.
- Schemas/migrations/caches: none.
- Compatibility: no runtime change.

## Lifecycle, Security, and Reliability

- Concurrency: block the control provider until after definitions are read.
- Cleanup: release and wait for terminal status; Runner cleanup remains intact.
- Auth/privacy/secrets: none.

## Product and Integration Surfaces

- Harness production, API, TUI, web, macOS GUI, and providers: no code change.
- Automation: fast and race-enabled CI become repeatable.
- UX/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Rollback: revert if the control no longer runs through normal Runner paths.
- Observability/operator docs: none.

## Regression Tests

- Red: hosted missing-definition failure after terminal cleanup.
- Green: focused normal/race `-count=100`.
- Controls: denied member remains hidden and uncallable; live unrestricted run
  sees the activated definition; released control reaches terminal state.
- Full: harness normal/race and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update engineering log, long-term log, plan, impact map, and plans index.
- No public docs.

## Warning Check

- Every runtime/product surface is explicitly unaffected because the repair is
  confined to the test provider lifecycle.
