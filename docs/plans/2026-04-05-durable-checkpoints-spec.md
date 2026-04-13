# Spec: Durable Checkpoints

Feature status: `implemented`

## Intent

- Unify run approvals, ask-user pauses, and workflow suspension behind a persisted checkpoint service.

## Exact Public Contract

- New internal types:
  - `Checkpoint`
  - `CheckpointStore`
  - `CheckpointStatus`
  - `CheckpointKind`
- Implemented HTTP routes:
  - `GET /v1/checkpoints/{id}`
  - `POST /v1/checkpoints/{id}/resume`
- Implemented compatibility behavior:
  - existing `/v1/runs/{id}/approve`
  - existing `/v1/runs/{id}/deny`
  - existing `/v1/runs/{id}/input`
  - these routes remain stable and resolve the current pending checkpoint for the run
- Implemented persistence:
  - additive checkpoint tables in the shared runtime SQLite state database

## Explicit Non-Goals

- No workflow DSL in this stage.
- No public README route documentation until the routes exist and are covered by tests.
- No replacement of the current run API shape.

## Acceptance Criteria

- Approval and ask-user flows are backed by persisted checkpoints instead of in-memory-only broker state.
- Checkpoint records survive process restart in SQLite and can be resumed through the checkpoint API.
- Existing run pause/resume routes remain compatible.
- Existing run-level waiting/resume events remain intact.

## Test Matrix

- Characterization:
  - current approve/deny/input/timeout behavior
- New failing-first tests:
  - persisted checkpoint creation
  - persisted resume payloads
  - compatibility shims for current run routes
- Regression:
  - `go test ./internal/harness`
  - `go test ./internal/server`
  - `./scripts/test-regression.sh`

## Rollout And Documentation Rules

- Keep checkpoint behavior in stage/spec docs only until the routes and storage are real.
- Update public operator docs in the same change that lands implementation and tests.
- If compatibility with existing run routes changes, update this spec before further code changes.
