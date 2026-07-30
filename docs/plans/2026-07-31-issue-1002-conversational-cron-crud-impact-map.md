# Issue #1002 — Conversational Cron CRUD Impact Map

## Task

- Task / issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Plan link: `2026-07-31-issue-1002-conversational-cron-crud-plan.md`
- Owner: Codex implementation worktree
- Status: implemented on PR #1057; focused normal/race verified; full regression has the pre-existing Keychain blocker

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/harness/tools/deferred/cron.go` model-facing create/update tools, `NewDefaultRegistryWithOptions`, `internal/cron.Server` create/PATCH routes, `internal/cron.Client`, and the embedded `harnessd` adapter.
- Source of truth: `tools.CronClient`, typed create/update request payloads, and the cron store; the service owns validation, persistence CAS, and scheduler reconciliation.
- Callers/consumers: model tool registry, embedded `harnessd` adapter, remote cron client, cron HTTP server, SQLite store, and transcript-readable tool result serialization.
- Similar abstractions searched: all `cron_*` constructors, registry wiring, `CronClient` implementations, `RunStartRequest`, `HarnessExecutor`, `DispatchExecutor`, create/update request call sites, permission/catalog tests, and embedded descriptions. The historical `fix/cron-tools-core` branch contains an unmerged `cron_update` implementation; `cron_history` and #1003 remote transport are explicitly excluded.
- Search evidence: `rg -n "cron_(create|list|status|history|update|pause|resume|delete)|CronClient|CronUpdateJobRequest|UpdateJobRequest" internal cmd .github docs`.
- Conclusion: extend the existing update service and tool seam; do not add a dispatcher or parallel cron abstraction.

## Config, API, CLI, and Tools

- User-facing config: model schema now distinguishes legacy `shell` command creation from explicit `harness` prompt creation.
- Defaults: omitted create `execution_type` remains legacy shell; service default timeout remains 30 for create; update timeout must be positive and update fields patch only supplied values.
- Environment/config: none.
- API: additive `expected_updated_at` on cron update requests; persistence CAS returns typed conflict and HTTP 409 on zero matching rows. Existing pause/resume clients remain compatible by CASing their loaded version.
- Tool: `cron_create` supports legacy shell or explicit harness prompt config; `cron_update` requires `id` plus `expected_updated_at` and accepts optional schedule, execution config/command, positive timeout, and tags; pause/resume remain explicit tools.
- Errors: missing ID/no-op/invalid timestamp are actionable model-facing errors; invalid schedule remains service-owned.

## Persistence and Compatibility

- Schema/migrations: no schema change; the existing `updated_at` value is the optimistic token.
- Compatibility: old HTTP clients may omit `expected_updated_at`; the service still atomically CASes against the version it loaded. Existing shell tool calls remain valid; stored rows and legacy shell execution remain readable.
- Mixed-version behavior: an older remote service may accept an unversioned HTTP PATCH, but the current model-facing tool requires the token. No destructive migration or ownership field is introduced.

## Lifecycle, Security, and Reliability

- Concurrency: service writes through `UpdateJobCAS`, whose SQL `UPDATE ... WHERE job_id AND updated_at` row count is authoritative. Scheduler side effects happen only after CAS success; active jobs are re-registered with the updated config, paused jobs are removed.
- Security/privacy: no tenant, agent, conversation, or ownership mutation fields are exposed; scope filtering remains #1001's contract. Config/command values are not logged by the new tool.
- Failure/recovery: failed CAS leaves persistence and scheduler state unchanged; caller refreshes with `cron_get` and retries using the returned `updated_at`. Harness creation uses `DispatchExecutor` in-process; remote transport/auth/readiness remain #1003.

## Product and Integration Surfaces

- Server/runtime: cron HTTP server and embedded/remote `harnessd` adapters gain atomic update handling; embedded bootstrap already routes `ExecTypeHarness` through `HarnessExecutor` and the model tool now produces its typed prompt config.
- TUI/web/macOS/other clients: `None — this slice adds a model-facing tool and additive server request field; downstream UI lifecycle work belongs to #1009.`
- Provider/model/tool catalog: registry, permissions metadata, schema, and embedded descriptions cover both `cron_create` execution modes and `cron_update`; no provider routing changes.
- External systems: `None — no new external integration.`
- UX/accessibility: transcript result is the existing structured `CronJob`; no UI surface changes.

## Deployment and Operations

- Order/flags: deploy additively; no feature flag or migration required.
- Observability: existing cron service errors and job timestamps remain the diagnostics; no prompt/config logging added.
- Rollback: remove the registry entry/tool implementation while retaining the additive request field; existing CRUD endpoints and stored jobs continue working.
- Runbooks: `None — no operator procedure changes.`

## Regression Tests

- Characterization/red: deferred create harness test initially showed shell-only creation; CAS concurrency test initially allowed serial read/check/write behavior; timeout/version/lifecycle tests were added before their fixes.
- Acceptance: typed harness create, immutable scope, same-conversation starter, partial update, atomic stale conflict, no-op/timeout rejection, stable ID/result, registry presence, and full lifecycle.
- Edge/negative: missing ID/version, malformed JSON/timestamp/timeout, client errors, omitted fields, mixed shell/harness inputs, and existing invalid schedule path.
- Integration: cron client/server CAS request mapping, real SQLite concurrency, model-facing tools through a stateful scoped embedded adapter, and assembled harness starter path.
- Exact commands: affected normal suite `go test ./internal/cron ./internal/harness/tools/... ./internal/harness ./cmd/harnessd -count=1`; focused race slice with `-race`; repository full gate remains independently blocked by the existing Keychain helper failure.

## Documentation and Handoff

- Specs/public docs: this plan and the embedded `cron_update.md` description.
- After code: update plans/logs indexes plus engineering, observational, and system logs.
- Training/release notes: `None — no separate release note system is used for this internal tool catalog change.`

## Warning Check

All surfaces are explicitly mapped; unaffected UI/external/runbook surfaces include search-based rationale.
