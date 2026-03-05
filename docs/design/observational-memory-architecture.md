# Observational Memory Architecture

## Goals

- Provide optional, tool-controlled observational memory for coding runs.
- Keep local standalone operation simple (`SQLite` + in-process ordering).
- Keep schema and interfaces migration-safe for future Postgres + remote coordination.

## Scope Keys

Memory state is keyed by:

- `tenant_id`
- `conversation_id`
- `agent_id`

Defaults:

- `tenant_id=default`
- `agent_id=default`
- `conversation_id=run_id` when omitted

## Core Components

- `internal/observationalmemory/manager.go`
  - Main orchestration and policy decisions.
- `internal/observationalmemory/store_sqlite.go`
  - Durable local state (`om_memory_records`, `om_operation_log`, `om_markers`).
- `internal/observationalmemory/coordinator.go`
  - Per-scope ordered local mutation execution.
- `internal/observationalmemory/observer.go`
  - Model-backed observation synthesis.
- `internal/observationalmemory/reflector.go`
  - Model-backed memory reflection.
- `internal/harness/tools/observational_memory.go`
  - Tool control plane (`enable|disable|status|export|review|reflect_now`).

## Runner Integration

- `internal/harness/runner.go` keeps run transcript snapshots in run state.
- Before each model turn, runner asks memory manager for a snippet and prepends:

```text
<observational-memory>
...
</observational-memory>
```

- After each turn/tool cycle, runner submits transcript deltas to `Observe`.
- Runner emits memory lifecycle events:
  - `memory.observe.started`
  - `memory.observe.completed`
  - `memory.observe.failed`
  - `memory.reflection.completed`

## Tool Context Access

Tools now receive read-only runtime context:

- `RunMetadata`
- `TranscriptReader` with `Snapshot(limit, includeTools)`

No tool gets mutable transcript access.

## Modes

- `HARNESS_MEMORY_MODE=off|auto|local_coordinator`
- `auto` resolves to `local_coordinator` in v1.
- Postgres store exists as a compile-ready stub for future activation.

## Data Model

Main tables:

- `om_memory_records`: canonical current state per scope.
- `om_operation_log`: ordered mutation/audit trail.
- `om_markers`: observation/reflection markers.

JSON payload columns remain text-based for sqlite/postgres portability.

## Export and Review

- `export` action writes JSON/Markdown snapshots into workspace-safe paths.
- `review` action delegates analysis to configured `AgentRunner` when available.

## v1 Constraints

- Remote coordinator transport is not implemented yet.
- Postgres store is intentionally non-functional until phase-2/3 rollout.
- Token counting uses deterministic approximation (`runes/4`).
