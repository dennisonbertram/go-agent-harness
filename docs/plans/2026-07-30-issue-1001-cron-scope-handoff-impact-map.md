# Impact Map: Issue #1001 Cron Scope Handoff Review Repairs

## Task

- Task / issue: GitHub #1001, preserve scoped embedded cron continuations and
  repair review findings.
- Plan link: `2026-07-30-issue-1001-cron-scope-handoff-plan.md`.
- Owner: issue #1001 implementation branch.
- Status: Review repairs implemented; repository gate green.

## Current Ownership and Data Flow

- `cron_create` derives tenant, conversation, and agent from `RunMetadata`.
- `tools.CronCreateJobRequest` crosses either `embeddedCronAdapter` or
  `cronClientAdapter`.
- Embedded jobs persist through `cron.SQLiteStore`, then
  `Scheduler -> DispatchExecutor -> HarnessExecutor -> cronRunStarter ->
  Runner.StartRun`.
- Standalone jobs cross `cron.Client -> cron.Server -> Store`; the server must
  not discard fields already present in the additive request contract.

## Config and Deployment

- User-facing config: None.
- Defaults/fallbacks: embedded scheduling remains the default when no cron URL
  is configured. Legacy rows retain empty tenant/agent and the historical
  config-level conversation fallback.
- Environment/config files: unchanged.
- Migration: additive SQLite `conversation_id` and `agent_id` columns only.
- Rollback: code can be reverted without deleting the additive columns or
  rewriting existing rows.

## APIs, Tools, and Clients

- Server API: `CreateJobRequest` already exposes optional tenant, conversation,
  and agent fields. The standalone server must persist and return all three.
- Auth: the harness-facing cron route continues to stamp tenant from the
  authenticated context. This repair does not broaden model-controlled scope.
- Tool catalog: no names, permissions, or schemas change.
- CLI/TUI/macOS/web: no behavior or state change; additive job fields remain
  forward-compatible.

## Persistence, Lifecycle, and Concurrency

- SQLite job rows own the scalar scope fields after creation.
- No reference-typed fields are introduced, so no clone helper is required.
- Scheduler timing, overlap, retry, and terminal run linkage are unchanged and
  remain assigned to later epic slices.
- The composed regression uses the new in-process `TriggerJob` method. It
  reloads the authoritative active job and enters the normal fire path; no
  remote or user-facing manual-fire endpoint is added.

## Security and Privacy

- Stored scope is authoritative at execution time.
- Model arguments cannot supply tenant, conversation, or agent ownership.
- Standalone API behavior becomes lossless; remote authentication and remote
  harness dispatch remain out of scope.
- Lifecycle logs contain job/execution IDs only, not prompts or credentials.

## Verification

- Red first: standalone server scoped POST/store/GET round trip.
- Red first: persisted scoped job through dispatcher/executor/starter into a
  real Runner.
- Behavior tests for every function reported by the zero-function coverage
  gate.
- Focused package tests, race checks, `go test ./...`, and
  `./scripts/test-regression.sh` must all pass.
- Final result: `coveragegate: PASS (total=85.6%, min=80.0%,
  zero-functions=0)` and `[regression] PASS`.

## Documentation

- Update the plan, plans index, long-term thinking log, and engineering log.
- No public feature documentation changes until later user-facing cron slices.
