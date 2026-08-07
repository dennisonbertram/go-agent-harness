# Issue #1254 Impact Map

## Task

- Task / issue: #1254 mandatory child `task_complete`.
- Plan link: `2026-08-07-issue-1254-task-complete-plan.md`.
- Owner: Runner lifecycle.
- Status: implemented pending verification.

## Current Ownership, Callers, and Data Flow

- Entry points: deferred `spawn_agent` -> `Runner.RunForkedSkill` -> `StartRun` -> step engine.
- Source of truth: `ActivationTracker` is keyed by run ID; `TaskCompleteTool` emits `_task_complete`; terminal transitions own cleanup.
- Consumers: synchronous `spawn_agent` parses the child `ForkResult`; `subagents.Manager` is not called.
- Search evidence: `rg "task_complete|RunForkedSkill|AllowedTools|activations.Cleanup" internal/harness`.
- Conclusion: activate per created child run and interpret its successful tool output in the step engine.

## Config, API, CLI, and Tools

- Config/API/CLI: no fields, endpoints, or schemas change.
- Tool contract: `task_complete` is mandatory only for `ForkDepth > 0`; root remains unavailable.
- Validation: mixed completion plus any sibling is rejected before executing either; invalid completion follows ordinary error handling.

## Persistence and Compatibility

- None: activation is in-memory and terminal cleanup already deletes it. No migration or Manager persistence change.

## Lifecycle, Security, and Reliability

- Child activation is installed after child ID creation and cleaned by existing completed/failed/cancelled paths.
- The sole-control exemption is depth-gated and limited to a non-mutating terminal tool; root deny and child deny lists still win.
- Sentinel validation requires successful tool execution and exact valid JSON marker, preventing malformed/failed completion from ending work.

## Product and Integration Surfaces

- Server/runtime: child completes promptly and parent receives structured summary.
- TUI/native: completed spawn card and final parent transcript should settle without a stuck/failed child.
- Provider/catalog: one fewer provider request after completion; no catalog changes.
- External systems: `/v1/subagents` remains empty because synchronous spawn does not use Manager.

## Deployment and Operations

- No rollout flag or migration. Observe terminal events and absence of `max_empty_responses`.
- Rollback: revert child activation/filter exemption and sentinel branch together.

## Regression Tests

- Red/green: restrictive child activation, terminal marker/no second provider call, root denial, invalid marker non-terminal, mixed turn no sibling mutation, cleanup.
- Integration: deterministic API parent/child transcript plus TUI/native smoke.
- Commands: focused normal/race, API acceptance, `./scripts/test-regression.sh`.

## Documentation and Handoff

- Internal plan/map, durable logs, and indexes are updated. No public behavior claim is added before acceptance evidence.
