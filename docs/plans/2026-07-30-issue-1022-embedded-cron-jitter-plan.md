# Plan: Issue #1022 — wire resolved jitter into embedded cron

## Context

- Governing GitHub issue: https://github.com/dennisonbertram/go-code/issues/1022
- Problem: `harnessd` resolves cron jitter values from TOML and environment
  configuration, but `buildCronBootstrap` discards them and constructs the
  embedded scheduler with default jitter.
- User impact: configured timing is ignored, `next_run_at` can pass without a
  fire, and UI/operator status is misleading.
- Constraints: preserve current defaults, remote cron behavior, persistence,
  job schemas, and scheduler shutdown semantics.

## Scope

- In scope: thread the resolved `config.CronConfig` through embedded cron
  bootstrap into the existing `cron.SchedulerConfig.Jitter`, with
  deterministic bootstrap coverage and manual continuation proof.
- Out of scope: new endpoints, jitter algorithm changes, callback behavior,
  next-run schema changes, remote cronsd, or UI design.

## Documentation Contract

- Feature status: `implemented; full regression green and manual verification in progress`
- Public docs affected: None; existing documented config becomes effective.
- Spec docs to update before code: this plan and its impact map.
- Implementation notes to add after code: engineering log and GitHub evidence.

## Test Plan (TDD)

- New failing test first: a `buildCronBootstrap` call with jitter disabled must
  construct a scheduler whose registered job fires without invoking the
  configured jitter sleep.
- Existing tests to update: bootstrap callers receive resolved cron config.
- Regression tests: configured min/max/avoid/log values map exactly; omitted
  configuration preserves defaults; remote cron remains store/scheduler-free.

## Cross-Surface Impact Map

- See `2026-07-30-issue-1022-embedded-cron-jitter-impact-map.md`.

## Implementation Checklist

- [x] Link a contract-complete structured GitHub issue.
- [x] Record current architecture, callers, consumers, and search evidence.
- [x] Document status and cross-surface impact before code.
- [x] Write the failing regression test.
- [x] Implement the minimal bootstrap wiring.
- [x] Update logs and indexes.
- [x] Run focused, race, and full-regression verification.
- [ ] Complete the real harness and native GUI verification.
- [ ] Merge and push to `main`.

## Risks and Mitigations

- Risk: a zero-value `config.CronConfig` could unintentionally disable the
  scheduler's established defaults.
- Mitigation: configuration loading supplies explicit defaults in production;
  bootstrap tests pin both resolved defaults and an explicit disable override.
- Risk: changing bootstrap arguments could drift test-only and production
  callers.
- Mitigation: use one typed config argument at every call site and compile all
  daemon tests.
