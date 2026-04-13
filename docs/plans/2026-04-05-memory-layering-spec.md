# Spec: Memory Layering

Feature status: `implemented`

## Intent

- Separate recent transcript, explicit working memory, and observational recall into a stable prompt contract.

## Exact Public Contract

- New internal types:
  - `WorkingMemoryStore`
  - `WorkingMemoryScope`
  - `MemoryBundle`
- Implemented configuration:
  - working-memory storage uses the existing runtime persistence location
- Implemented tool/API surface:
  - a scoped working-memory tool with `get`, `set`, `delete`, and `list`
- Prompt assembly contract:
  - explicit working memory system snippet first
  - observational recall system snippet second
  - recent transcript after system context

## Explicit Non-Goals

- No embeddings or vector search in v1.
- No public docs until the storage contract and prompt assembly behavior are implemented.
- No replacement of observational memory; this stage layers on top of it.

## Acceptance Criteria

- Explicit working-memory CRUD exists with stable scope isolation.
- Prompt assembly uses the documented layer order.
- Observational memory still operates as the long-lived reflective layer.
- Workflow and network stages can share scoped working memory without relying on transcript mutation.

## Test Matrix

- Characterization:
  - current observational-memory behavior
- New failing-first tests:
  - working-memory CRUD
  - scope isolation
  - prompt composition order
  - coexistence with observational memory
- Regression:
  - `go test ./internal/harness`
  - `./scripts/test-regression.sh`

## Rollout And Documentation Rules

- Do not describe memory layering publicly until both prompt assembly and storage are shipping.
- Keep embeddings/vector recall explicitly marked out of scope until a later spec replaces that decision.
- If the prompt layer order changes, update this spec before more code is written.
