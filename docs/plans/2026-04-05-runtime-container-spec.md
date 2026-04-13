# Spec: Runtime Container

Feature status: `implemented`

## Intent

- Add a narrow internal composition root for `harnessd` startup so HTTP and MCP assembly stop living inline in `main.go`.
- Preserve all current runtime behavior while making later orchestration stages easier to add and test.

## Exact Contract

- Add internal helper types/functions in `cmd/harnessd` only:
  - `buildMCPStdioRuntime(...)`
  - `buildHTTPRuntime(...)`
- The extracted helpers must assemble:
  - MCP stdio tool catalog and stdio server
  - HTTP runner, subagent manager, handler, and `http.Server`
- `runMCPStdio(...)` and `runWithSignals(...)` remain the stable entrypoints.
- No new public HTTP routes, config keys, or README/API documentation changes in this stage.

## Explicit Non-Goals

- No workflow/checkpoint/memory/network behavior.
- No changes to runner semantics, approval behavior, or event ordering.
- No move into a new cross-package `internal/runtime` package in this slice.

## Acceptance Criteria

- `runMCPStdio(...)` delegates stdio server assembly to `buildMCPStdioRuntime(...)`.
- `runWithSignals(...)` delegates runner/subagent/server assembly to `buildHTTPRuntime(...)`.
- Callback starter and lazy summarizer binding still happen after the runner exists.
- Existing `cmd/harnessd` startup/shutdown tests still pass.
- New tests directly cover the extracted runtime assembly helpers.

## Test Matrix

- Characterization:
  - existing `cmd/harnessd` run/startup/shutdown coverage remains green
- New failing-first tests:
  - helper builds MCP runtime with catalog + server
  - helper builds HTTP runtime with runner + subagent manager + server
  - helper still wires callback starter and lazy summarizer to the built runner
- Regression:
  - `go test ./cmd/harnessd`

## Rollout And Documentation Rules

- Keep this stage internal-only; do not add public docs beyond implementation logs.
- If the extraction grows beyond `cmd/harnessd`, stop and update this spec before continuing.
- If any user-visible behavior changes, that is a spec violation for this stage and must be split into a later stage.
