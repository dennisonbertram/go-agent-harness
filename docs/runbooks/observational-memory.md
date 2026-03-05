# Observational Memory Runbook

## Purpose

Operate optional observational memory in local harness deployments and prepare for future scale-out.

## Environment Variables

- `HARNESS_MEMORY_MODE=off|auto|local_coordinator`
- `HARNESS_MEMORY_DB_DRIVER=sqlite|postgres`
- `HARNESS_MEMORY_DB_DSN` (used for postgres mode)
- `HARNESS_MEMORY_SQLITE_PATH` (default `.harness/state.db`)
- `HARNESS_MEMORY_DEFAULT_ENABLED` (default `false`)
- `HARNESS_MEMORY_OBSERVE_MIN_TOKENS` (default `1200`)
- `HARNESS_MEMORY_SNIPPET_MAX_TOKENS` (default `900`)
- `HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS` (default `4000`)

## Recommended Local Setup

```bash
export HARNESS_MEMORY_MODE=auto
export HARNESS_MEMORY_DB_DRIVER=sqlite
export HARNESS_MEMORY_SQLITE_PATH=.harness/state.db
export HARNESS_MEMORY_DEFAULT_ENABLED=false
```

## Tool Actions

Use `observational_memory` tool with actions:

- `status`
- `enable`
- `disable`
- `reflect_now`
- `export`
- `review`

Example payload:

```json
{
  "action": "enable",
  "config": {
    "observe_min_tokens": 1200,
    "snippet_max_tokens": 900,
    "reflect_threshold_tokens": 4000
  }
}
```

## Operational Notes

- Memory is scoped by `tenant_id + conversation_id + agent_id`.
- Defaults are `tenant_id=default`, `agent_id=default`, `conversation_id=run_id`.
- When disabled, no snippet is injected and observe calls are no-ops.
- Exports are workspace-scoped and path-validated.

## Event Signals

Watch SSE for memory events:

- `memory.observe.started`
- `memory.observe.completed`
- `memory.observe.failed`
- `memory.reflection.completed`

## Recovery

- Local startup requeues stale processing operations in `om_operation_log`.
- If sqlite lock contention appears, ensure WAL mode is active and avoid parallel external sqlite writers.

## Scale Path (Future)

- Postgres adapter and remote coordinator transport are planned but not enabled in v1.
- Keep operation ordering semantics stable by scope key when moving off local coordinator mode.
