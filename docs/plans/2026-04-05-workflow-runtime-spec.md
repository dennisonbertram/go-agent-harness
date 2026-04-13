# Spec: Workflow Runtime

Feature status: `implemented`

## Intent

- Add deterministic orchestration above the runner for repeatable multi-step flows.

## Exact Public Contract

- New internal types:
  - `WorkflowDefinition`
  - `WorkflowRun`
  - `WorkflowStepState`
- Implemented configuration:
  - `HARNESS_WORKFLOWS_DIR`
- Implemented HTTP routes:
  - `GET /v1/workflows`
  - `GET /v1/workflows/{name}`
  - `POST /v1/workflows/{name}/runs`
  - `GET /v1/workflow-runs/{id}`
  - `GET /v1/workflow-runs/{id}/events`
  - `POST /v1/workflow-runs/{id}/resume`
- Implemented v1 step kinds:
  - `run`
  - `tool`
  - `checkpoint`
  - `branch`

## Explicit Non-Goals

- No silent conversion of recipes into workflows.
- No parallel graph execution in v1.
- No general-purpose expression language in branches.

## Acceptance Criteria

- Workflow definitions are loaded from YAML before execution.
- Workflow runs persist per-step outputs, run state, and event history in the runtime SQLite state.
- `run` steps launch real harness runs.
- `tool` steps execute registered tool handlers directly.
- `checkpoint` steps suspend until resumed through the checkpoint subsystem.
- `branch` steps support simple field-based routing only.

## Test Matrix

- Characterization:
  - current recipe behavior remains unchanged
- New failing-first tests:
  - step execution order
  - suspend/resume behavior
  - workflow event stream behavior
- Regression:
  - `go test ./internal/harness`
  - `go test ./internal/server`
  - `./scripts/test-regression.sh`

## Rollout And Documentation Rules

- Keep workflow routes out of public docs until they are implemented and tested.
- Document only the v1 step kinds that actually ship.
- If recipe behavior changes, update both this spec and the recipe docs before continuing.
