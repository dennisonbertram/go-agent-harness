# Cross-Surface Impact Map: Issue #1044 AskUserQuestion Status-Test Race

## Task

- Task / issue: synchronize the status regression fixture, #1044.
- Plan: `2026-07-30-issue-1044-ask-status-race-plan.md`.
- Owner: Codex.
- Status: implemented and fully verified locally; hosted checks pending.

## Current Ownership, Callers, and Data Flow

- Entry: `TestDeniedAskUserQuestionDoesNotStrandRunStatus`.
- Production source of truth: `Runner.StartRun` stores a run then dispatches it
  asynchronously; `Runner.GetRun` returns the status snapshot.
- Test flow: `funcProvider` completion step two samples the run status through a
  closure-captured ID that the test publishes after `StartRun` returns.
- Search conclusion: `CompletionRequest` has no run ID, and adjacent fixtures
  do not share this closure.

## Config, API, CLI, and Tools

- Config/env/defaults: none.
- API/CLI/wire/tool behavior: unchanged.
- Error/status contract: retain `running`, reject `waiting_for_user`, and
  require an eventual terminal status.

## Persistence and Compatibility

- State: test-local channel only.
- Schemas/migrations/caches: none.
- Compatibility: no runtime change.

## Lifecycle, Security, and Reliability

- Concurrency: a capacity-one channel establishes the happens-before edge.
- Auth/privacy/secrets: none.
- Failure/recovery: the existing status assertions remain diagnostic.

## Product and Integration Surfaces

- Harness production, API, TUI, web, macOS GUI, and providers: no code change.
- Automation: race-enabled CI becomes repeatable.
- UX/accessibility: none.

## Deployment and Operations

- Deployment/migration/flags: none.
- Rollback: revert if the handoff no longer samples step-two state.
- Observability/operator docs: none.

## Regression Tests

- Red: GitHub Actions race report on the test-local `runID`.
- Green: focused normal/race `-count=100`.
- Controls: denial does not end the run, status is exactly `running` mid-run,
  and the final state is terminal.
- Full: harness normal/race and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Update engineering log, long-term log, plan, impact map, and plans index.
- No public docs.

## Warning Check

- Every runtime/product surface is explicitly unaffected because the change is
  confined to the fixture's identity publication.
