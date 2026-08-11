# Issue #1161 Scheduled Routing Preservation Impact Map

## Current ownership and data flow

- Origin request: `harness.RunRequest` owns `Model`, `ProviderName`,
  `AllowFallback`, and `FallbackProviders`; `Runner.startRun` resolves model and
  stores run/provider state, while the step engine injects `tools.RunMetadata`.
- Cron creation: `deferred.CronCreateTool` derives immutable ownership from
  `RunMetadata` and writes prompt JSON into `cron.Job.ExecConfig`.
- Embedded dispatch: `HarnessExecutor.ExecuteOutcomeWithID` decodes config into
  `cron.RunStartRequest`; `cmd/harnessd.cronRunStarter` maps that request back
  to `harness.RunRequest`.
- Remote dispatch: `RemoteRunStarter` maps the typed request to
  `POST /v1/cron/runs`; `server.handleCronRun` authenticates and maps it to
  Runner admission; `cronRunRequestFingerprint` owns replay equivalence.
- Callback dispatch: `SetDelayedCallbackTool` maps `RunMetadata` into
  `SetRequest`; `CallbackInfo` is the durable state-machine payload;
  `callbackRunStarter.StartCallback` maps it to reserved-ID Runner admission.

## Cross-surface analysis

- Call sites/data flow: affected only at the existing metadata, cron, remote
  start, callback, and harnessd adapter boundaries above. Shell cron remains
  untouched.
- Config/env/defaults: no new environment or daemon configuration. Empty
  routing retains Runner defaults.
- API/CLI/wire/tools: additive internal cron-run JSON fields; no new model-facing
  tool arguments and no caller-supplied ownership. Existing clients decode
  absent fields normally.
- Persistence/schema/cache: delayed callbacks receive additive safe routing
  columns. Cron routing remains in its existing JSON execution config. No
  credential persistence. Cron start fingerprints include routing policy.
- Concurrency/lifecycle/retries: immutable copied slices are captured at
  scheduling time; callback token/lease state and cron execution/retry logic do
  not change. Restart replay uses persisted values.
- Security/auth/privacy/secrets: tenant derives from the existing authenticated
  context; tenant/conversation/agent checks remain authoritative. Only names,
  model ID, boolean policy, and ordered names cross the boundary—never keys,
  endpoints, headers, or provider configuration. Logs remain ID-only.
- TUI/web/macOS: no client source or rendering change. These clients benefit
  from the existing same-conversation assistant continuation once routing
  succeeds; interaction proof is downstream and not a reason to alter UI.
- Provider/model/tool catalog: no catalog mutation or resolution redesign. The
  scheduled request reuses the exact origin policy against the same Runner.
- Deployment/observability/runbooks: additive SQLite migration and wire fields
  are rollback-compatible. Existing failed-run/event/history signals verify
  recovery; no alert or runbook contract changes.
- Backward compatibility/versioning: old cron config, old callbacks, and old
  remote clients yield zero values. A new cronsd talking to an old harnessd
  has fields ignored by Go JSON; an old cronsd retains historical defaults.
- Tests/fixtures: embedded cron regression first; typed executor, remote client,
  authenticated server/fingerprint, callback tool/store/restart, harnessd
  adapters; focused normal/race and full repository gate.
- Documentation/onboarding: plan, impact map, active plan, four durable logs,
  and plans/logs indexes. No public API docs because the endpoint is an
  internal authenticated daemon boundary and the change is additive.

## Risks and controls

- Risk: resolved provider name could replace requested fallback intent.
  Control: capture model plus explicit provider/fallback request policy from the
  immutable run admission state, not credentials or mutable daemon defaults.
- Risk: slice aliasing changes a scheduled policy after acknowledgement.
  Control: copy fallback provider slices into metadata and durable payloads.
- Risk: same idempotency key silently reuses a run after routing changed.
  Control: fingerprint every routing field, including ordered fallback names.
- Risk: migration breaks legacy callback DBs. Control: additive columns with
  `NOT NULL DEFAULT` values and explicit legacy migration tests.
