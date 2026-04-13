# Spec: Agent Networks

Feature status: `implemented`

## Intent

- Add explicit, replayable agent-network topologies on top of workflows and existing subagent primitives.

## Exact Public Contract

- New internal types:
  - `NetworkDefinition`
  - `NetworkRun`
  - `RoleNode`
- Implemented configuration:
  - `HARNESS_NETWORKS_DIR`
- Implemented HTTP routes:
  - `GET /v1/networks`
  - `GET /v1/networks/{name}`
  - `POST /v1/networks/{name}/runs`
- Implemented topology support in v1:
  - sequential roles
- Implemented role contract:
  - roles compile into workflow-backed sequential `run` steps
  - role prompts inject structured child-result instructions
  - later roles receive prior role output through workflow step templating

## Explicit Non-Goals

- No separate orchestration engine outside workflows.
- No parallel fan-out/fan-in in v1.
- No hidden freeform prompt-only delegation graph.

## Acceptance Criteria

- Network definitions compile into workflow definitions.
- Structured child-result instructions are enforced in compiled role prompts.
- Sequential role execution is test-covered through the workflow runtime.

## Test Matrix

- Characterization:
  - current subagent structured result and handoff behavior
- New failing-first tests:
  - role sequencing
  - workflow-backed compilation
- Regression:
  - `go test ./internal/subagents`
  - `go test ./internal/harness`
  - `./scripts/test-regression.sh`

## Rollout And Documentation Rules

- Keep network routes out of README and operator docs until they exist.
- Keep parallel execution explicitly deferred until a later spec changes that status.
- If network execution stops compiling through workflows, update this spec before continuing.
