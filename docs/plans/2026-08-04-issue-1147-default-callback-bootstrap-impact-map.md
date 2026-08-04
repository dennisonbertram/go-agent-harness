# Cross-Surface Impact Map: #1147 default callback bootstrap

## Task

- Task / issue: #1147 default callback bootstrap cannot admit reserved continuation runs.
- Plan link: `2026-08-04-issue-1147-default-callback-bootstrap.md`.
- Owner: Codex.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `cmd/harnessd/main.go` initializes callback store/manager/starter before `buildHTTPRuntime`; `callbackRunStarter.StartCallback` calls `Runner.EnsureRunWithIDContext`.
- Source of truth: `buildPersistenceBootstrap` owns run-store creation; `Runner` owns reserved ID persistence/admission; `CallbackManager` owns durable callback state and retry.
- Consumers: delayed-callback tool, callback event bridge, conversation SSE, `/v1/tasks`, TUI, macOS transcript.
- Search evidence: `rg 'buildPersistenceBootstrap|EnsureRunWithIDContext|callbackRunStarter|HARNESS_RUN_DB' cmd internal`; no-DB branch leaves `bootstrap.runStore` nil, reserved admission explicitly rejects a nil store.
- Conclusion: repair existing bootstrap ownership; do not add a callback-specific runner/store abstraction.

## Config, API, CLI, and Tools

- Config/defaults: callback-enabled default workspace gains `.harness/runs.db`; explicit `HARNESS_RUN_DB` continues to select its path.
- API/tools: no wire/API/tool schema change; actual callback lifecycle already flows through task/SSE contracts.
- Error states: bootstrap must report store creation/migration failure before scheduling is available.

## Persistence and Compatibility

- Persistence: additive SQLite run store/migrations in the workspace; callback DB unchanged.
- Compatibility: explicit DB path and auth semantics are pinned; existing workspaces gain a store only when callbacks are enabled.
- Mixed rollout: safe because local/workspace state is independent and run records are additive.

## Lifecycle, Security, and Reliability

- Lifecycle: preserve one callback-reserved run ID across retry/recovery; close newly owned store on bootstrap cleanup.
- Security: internal storage must not accidentally enable mandatory auth or alter tenant/agent/conversation ownership checks.
- Recovery: callback manager remains source of retries; no data repair beyond normal migration.

## Product and Integration Surfaces

- Server/runtime: `harnessd`, runner, callback manager/bridge, stores.
- TUI/macOS: no source changes; they gain a genuine later assistant event to render. #1148 and #1009 remain separate owners for visibility gaps.
- Provider/model: provider-independent; fake provider drives deterministic test.
- UX/accessibility: no direct UI copy/control change.

## Deployment and Operations

- Deployment: provision/migrate store during callback-enabled bootstrap before serving.
- Diagnostics: log redacted effective store path and callback admission state; task/SSE state remains observable.
- Rollback: disable callback dispatch but retain callback/run SQLite files; never remove pending records.
- Runbooks: callback operator docs and engineering log/indexes.

## Regression Tests

- First red: `go test ./cmd/harnessd -run TestDefaultBootstrapDelayedCallbackStartsContinuation -count=1` must show callback admission failure before code repair.
- Acceptance: no explicit run DB -> one same-conversation continuation; explicit run DB/auth unaffected; restart/retry/cancel controls.
- Commands: targeted package normal/race; `go test ./cmd/harnessd ./internal/harness ./internal/server -race -count=1`; `./scripts/test-regression.sh`; live API then TUI/current-GUI matrix.

## Documentation and Handoff

- Add engineering-log entry, callback persistence/operator docs and indexes; PR records red/green commands, current SHA, rollout/rollback evidence.
