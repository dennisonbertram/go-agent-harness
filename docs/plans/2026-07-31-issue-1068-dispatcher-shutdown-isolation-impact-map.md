# Cross-Surface Impact Map: Issue #1068 Dispatcher Shutdown Isolation

## Task

- Task / issue: GitHub #1068
- Plan link: `2026-07-31-issue-1068-dispatcher-shutdown-isolation-plan.md`
- Owner: isolated issue #1068 branch
- Status: review fix implemented and locally verified; promotion pending

## Current Ownership, Callers, and Data Flow

- Entry points: `NewRunner` starts `poolDispatcher` when `WorkerPoolSize > 0`;
  callers terminate it through `Runner.Shutdown`.
- Owning packages/types/functions and source of truth: `Runner.done` signals
  exit and `Runner.dispatcherWG` is the instance-owned completion contract.
- Callers, consumers, events, and downstream data: daemon/app shutdown and
  harness tests; no event or persisted-data flow changes.
- Similar abstractions searched: `inflight`, `shutdownOnce`, worker-pool tests,
  and process-wide goroutine helpers in `runner_shutdown_test.go`.
- Search evidence: `rg -n "poolDispatcher|dispatcherWG|Shutdown|runnerGoroutineStackContains" internal/harness`.
- Duplication/ownership conclusion: the wait group already owns instance
  completion; the global stack substring is a parallel and unsafe assertion.
  Every bounded test fixture must also invoke the public `Shutdown` ownership
  boundary instead of leaving its dispatcher alive after the test returns.

## Config, API, CLI, and Tools

- User-facing config added or changed: none; `WorkerPoolSize` behavior is retained.
- Defaults / fallbacks: none changed.
- Environment variables, config files, or saved settings touched: none.
- Endpoints, request fields, response fields, or server wiring affected: none.
- CLI commands, tools, wire formats, or integrations affected: none.
- Error states / validation changes: none.

## Persistence and Compatibility

- Schemas, migrations, caches, generated data, or ownership changes: none.
- Backward/forward compatibility and versioning: shutdown remains idempotent
  and preserves queue-drain and timeout behavior.
- Partial rollout and mixed-version behavior: not applicable; local lifecycle only.

## Lifecycle, Security, and Reliability

- Concurrency, cancellation, retries, cleanup, and resource ownership: primary
  surface; target dispatcher completion must be distinguished from unrelated
  Runner dispatchers while `done`, `inflight`, and `dispatcherWG` ordering stays
  intact. Worker-pool fixtures must release blocked providers before cleanup
  calls `Shutdown`, including failure paths.
- Authentication, authorization, permissions, trust, privacy, and secrets:
  none; search found no auth or data boundary in this lifecycle path.
- Failure modes, recovery, idempotency, and data repair: guard against a false
  leak report, a real lingering dispatcher, deadlock, or early return; no data repair.

## Product and Integration Surfaces

- Server/runtime: more trustworthy bounded Runner shutdown verification.
- TUI/web/macOS/other clients: no direct contract; app/daemon termination benefits indirectly.
- Provider/model/tool catalog and routing: none; no provider or catalog calls change.
- External systems and automation: GitHub race/full regression gates only.
- UX states, keyboard/focus/accessibility/motion: none; no UI code involved.

## Deployment and Operations

- Deployment/migration order and feature flags: separate PR; no migration or flag.
- Logs, metrics, traces, alerts, and support diagnostics: test failure identifies
  target/control identity instead of a shared function name.
- Rollback triggers and recovery steps: revert if shutdown deadlocks, latency
  regresses, or queue accounting fails.
- Runbooks and operator docs: no operator command changes.

## Regression Tests

- Characterization and first expected red test: two live bounded Runners prove
  target instance exit while the old global scan remains positive.
- New acceptance tests required: `TestRunnerDispatcherShutdownIsInstanceScoped`
  and `TestWorkerPoolTestRunnerCleanupStopsDispatcher`.
- Edge, negative, failure, lifecycle, and security tests: existing queue drain,
  active cancellation timeout, idempotent shutdown, and unbounded-mode tests.
- Integration/e2e/real-path proof: full harness package race stress and repository regression.
- Cross-surface regressions to guard: harness/server normal/race/vet.
- Exact targeted and full commands: cleanup regression normal/race, all
  worker-pool tests normal/race at `-count=100`, complete harness race at
  `-count=5`, harness vet, and the unchanged repository regression gate.

## Documentation and Handoff

- Specs/public docs before code: plan and impact map only; no public docs change.
- Implementation notes/logs/indexes after code: all four logs plus plans index and active plan.
- Training/onboarding/release notes: none; internal bug-fix PR evidence is sufficient.

## Warning Check

- Every impact heading is reconciled; `none` entries include the searched lifecycle rationale.
