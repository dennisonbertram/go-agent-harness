# Plan: Mastra-Style Orchestration Program

## Status Ledger

- `runtime-container`: `implemented`
- `durable-checkpoints`: `implemented`
- `workflow-runtime`: `implemented`
- `memory-layering`: `implemented`
- `agent-networks`: `implemented`

Allowed statuses:

- `planned`
- `in implementation`
- `implemented`
- `deferred`
- `rejected`

## Intent

- Command intent: add Mastra-style orchestration patterns to the harness without creating ghost features or destabilizing the current `/v1/runs*` surface.
- User intent: get a staged, test-first architecture program where documentation is trustworthy, existing behavior stays protected by regression coverage, and new orchestration capabilities are added deliberately.

## Program Rules

- Public docs describe only implemented and test-covered behavior.
- Plan docs define scope, stage order, and dependency order.
- Stage spec docs define exact contracts, non-goals, acceptance criteria, and test matrices before code starts.
- Implementation notes are written only after the matching code lands.
- Every stage must begin with characterization tests for the seam being changed.
- Every new capability must begin with failing tests.
- Every discovered bug must get a permanent regression test before the fix is considered complete.

## Stage Order

1. Runtime container
2. Durable checkpoints
3. Workflow runtime
4. Memory layering
5. Agent networks

## Stage Goals And Non-Goals

### 1. Runtime Container

- Goal: create a narrow composition root in `cmd/harnessd` for HTTP and MCP startup assembly so later orchestration work plugs into one runtime wiring boundary.
- Non-goals:
  - no new user-facing routes
  - no runner behavior changes
  - no README changes

### 2. Durable Checkpoints

- Goal: unify approval/input/external resume pauses behind a persisted checkpoint service with restart-safe state.
- Non-goals:
  - no workflow DSL yet
  - no public README route documentation until the routes and storage are live

### 3. Workflow Runtime

- Goal: add deterministic workflow orchestration above the runner with typed `run`, `tool`, `checkpoint`, and `branch` steps.
- Non-goals:
  - no silent migration of recipes into workflows
  - no general expression language in v1

### 4. Memory Layering

- Goal: separate recent transcript, explicit working memory, and observational recall into a stable prompt contract.
- Non-goals:
  - no vector search or embeddings in v1
  - no public documentation until prompt assembly and storage behavior ship

### 5. Agent Networks

- Goal: define explicit planner/worker/reviewer style multi-agent topologies on top of workflows and existing subagent primitives.
- Non-goals:
  - no second orchestration engine
  - no parallel fan-out/fan-in in v1

## Dependency Order

- Runtime container must land before durable checkpoints so new orchestration services have one wiring seam.
- Durable checkpoints must land before workflows so human-in-the-loop suspension uses one persisted model.
- Workflows must land before memory layering and agent networks so shared orchestration semantics stay consistent.
- Memory layering must land before richer agent networks so roles have an explicit shared-state surface.

## Required Tests Before Code

- Capture the current regression status for:
  - `./cmd/harnessd`
  - `./internal/harness`
  - `./internal/server`
  - `./internal/subagents`
  - relevant TUI/export packages when routing or persisted state changes could affect them
- Add characterization coverage for the currently-observed seam before any structural refactor.
- Add failing tests for the new behavior before implementation.

## Required Documentation Before And After Code

### Before Code

- Umbrella plan exists and lists the stage in the status ledger.
- Stage spec exists with exact scope, exact contracts, acceptance criteria, and test matrix.
- If the stage changes provider/model flow, add an impact map before code starts.

### After Code

- Stage spec status matches reality.
- Public docs still describe only implemented behavior.
- Local doc indexes are updated.
- Engineering/system/observational logs are updated for the landed slice.

## Stage Verification Workflow

1. Update the stage spec with the intended acceptance contract.
2. Add or tighten characterization tests for the seam being changed.
3. Add new failing tests for the capability.
4. Implement the smallest code change that makes the tests pass.
5. Add regression tests for any bug discovered while implementing the slice.
6. Run targeted package tests.
7. Run `./scripts/test-regression.sh` at stage-complete boundaries.
8. Update stage status and docs only to the level actually implemented.

## Current Milestone

- Milestone: land stages 2-5 with documentation-first TDD and additive runtime wiring.
- Exit criteria:
  - stage 2-5 specs are `implemented`
  - checkpoint, workflow, memory, and network routes exist only where they are actually test-covered
  - `go test ./internal/checkpoints ./internal/workflows ./internal/networks ./internal/workingmemory ./internal/harness ./internal/server ./cmd/harnessd` passes

## Linked Stage Specs

- `2026-04-05-runtime-container-spec.md`
- `2026-04-05-durable-checkpoints-spec.md`
- `2026-04-05-workflow-runtime-spec.md`
- `2026-04-05-memory-layering-spec.md`
- `2026-04-05-agent-networks-spec.md`
