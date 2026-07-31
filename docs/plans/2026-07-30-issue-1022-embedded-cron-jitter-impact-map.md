# Cross-Surface Impact Map — Issue #1022 embedded cron jitter wiring

## Task

- Task / issue: https://github.com/dennisonbertram/go-code/issues/1022
- Plan link: `2026-07-30-issue-1022-embedded-cron-jitter-plan.md`
- Owner: harnessd runtime composition
- Status: in implementation

## Current Ownership, Callers, and Data Flow

- Entry points: TOML/env config loading into `config.Config.Cron`, followed by
  `runWithSignals` and `buildCronBootstrap`.
- Source of truth: `internal/config.CronConfig` for resolved values and
  `internal/cron.JitterConfig` for scheduler behavior.
- Downstream: embedded `cron.Scheduler`, cron API/tool-created jobs, task/UI
  status, and harness continuations.
- Search evidence: `rg` over `buildCronBootstrap`, `SchedulerConfig`,
  `HARNESS_CRON_JITTER_ENABLED`, and `CronConfig` found one production
  constructor that hard-codes only `MaxConcurrent`.
- Conclusion: repair the existing composition boundary; no new abstraction.

## Config, API, CLI, and Tools

- Config changed: existing jitter enabled/min/max/avoid/log fields become
  effective for embedded cron.
- Defaults: preserve resolved 60–300 second jitter and existing avoid marks.
- Endpoints/tools/wire formats/errors: None; schemas and validation unchanged.

## Persistence and Compatibility

- Schema/cache/migrations: None.
- Compatibility: jobs and mixed-version databases remain readable; timing
  changes only when an operator already supplied non-default config.
- Partial rollout: safe because scheduler construction is process-local.

## Lifecycle, Security, and Reliability

- Lifecycle: jitter waits retain the existing interruptible shutdown path.
- Security/privacy/secrets: None; no new data or logging.
- Recovery/idempotency: restart with prior config; no data repair.

## Product and Integration Surfaces

- Server/runtime: embedded scheduler construction only.
- TUI/web/macOS: no code change, but displayed timing and conversation
  continuation now follow configuration.
- Provider/model/tool catalog: None.
- External automation: deterministic cron timing is restored.
- Accessibility/motion: None.

## Deployment and Operations

- Order/flags: normal binary rollout; existing config is the flag.
- Observability: verify `last_run_at`, conversation append, execution record,
  and lifecycle logs.
- Rollback: revert wiring commit; no migration.
- Runbooks: no new command.

## Regression Tests

- First red: bootstrap with jitter disabled still computes/default-sleeps.
- Acceptance: exact config mapping plus immediate scheduled fire using a fake
  clock/sleep seam.
- Edge/failure: production defaults and remote cron path remain unchanged.
- Real path: minute cron with jitter disabled advances a persisted conversation,
  appears in tasks/UI, and can be deleted.
- Commands: focused daemon/cron tests, affected race tests, and
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Before code: Issue #1022, this map, and its plan.
- After code: engineering log, plan status/checklist, GitHub verification.
- Public/release docs: None; this is a correctness repair for existing config.

## Warning Check

- Every applicable surface is mapped; explicit `None` entries are supported by
  the source searches above.
