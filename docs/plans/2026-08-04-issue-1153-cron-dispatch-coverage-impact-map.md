# Cross-Surface Impact Map: Issue #1153

## Task

- Task / issue: #1153 durable cron lease polling/cancellation coverage.
- Plan link: `2026-08-04-issue-1153-cron-dispatch-coverage-plan.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: authenticated `POST /v1/cron/runs` enters
  `Server.getOrStartCronRun` in `internal/server/cron_run_idempotency.go`.
- Owner/source of truth: `CronRunStartStore` is the durable tenant/key/run lease
  binding; `Runner.StartRunWithIDContext` admits the reserved run.
- Callers/data: four `getOrStartCronRun` contention branches call
  `waitForCronRunDispatch`; existing two-server tests normally acquire directly.
- Search evidence: `rg -n "waitForCronRunDispatch|getOrStartCronRun|CronRunStartStore" internal/server internal/store --glob '*.go'`.
- Conclusion: test wrapper belongs in server cron tests; no new production owner.

## Config, API, CLI, and Tools

- User-facing config/defaults/environment/endpoints/CLI/tools: None. The test
  sets the existing private poll interval only; the production default remains
  10ms.
- Errors/validation: cancellation must retain typed
  `errCronRunIdempotencyUnavailable` wrapping context cancellation.

## Persistence and Compatibility

- Schemas/migrations/caches: None. Tests retain the existing durable
  tenant/idempotency reservation and `Accepted` semantics.
- Mixed version/rollout: None; test-only coverage does not alter wire or storage.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: verify retry polling can acquire after foreign
  contention and cancelled callers stop before runner/provider dispatch.
- Security/auth/privacy: no change; tenant scope remains present in every
  scripted store call.
- Failure/recovery/idempotency: preserve one reserved run and one admission.

## Product and Integration Surfaces

- Server/runtime: directly covered through the real `getOrStartCronRun` path.
- TUI/web/macOS: no contract change; they consume the resulting run as before.
- Provider/tool/external/UX: no behavior change; cancellation avoids an unseen
  scheduled provider dispatch.

## Deployment and Operations

- Deployment/flags: none.
- Observability: exact red/green and stress commands are recorded in logs/PR.
- Rollback: revert this test-only seam if it changes production behavior.
- Runbooks: none beyond existing regression gate.

## Regression Tests

- First red: retry and cancellation tests in `internal/server/cron_run_idempotency_test.go`; expected failure is a zero dispatch-poll coverage function.
- Acceptance: scripted first non-acquire then acquire; seeded foreign lease plus cancelled context.
- Negative/lifecycle: assert no `CreateRun`/provider completion on cancellation.
- Real path: real Runner + MemoryStore wrapper, preserving store operations.
- Commands: `go test ./internal/server -run 'TestCronRunDispatchPoll' -count=100`, the same with `-race`, `go test ./internal/server -race -count=1`, and `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: none.
- Notes/indexes: plan index, logs index, and all durable logs record outcome.
- Training/release notes: none.

## Warning Check

Every surface is either mapped above or explicitly unchanged with searched rationale.
