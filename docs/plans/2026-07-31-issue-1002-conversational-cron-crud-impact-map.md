# Issue #1002 — Conversational Cron CRUD Impact Map

## Task

- Task / issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Plan link: `2026-07-31-issue-1002-conversational-cron-crud-plan.md`
- Owner: Codex implementation worktree
- Status: in implementation

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/harness/tools/deferred/cron.go`, `NewDefaultRegistryWithOptions`, `internal/cron.Server` PATCH `/v1/jobs/{id}`, and `internal/cron.Client.UpdateJob`.
- Source of truth: `tools.CronClient` and `tools.CronUpdateJobRequest`; the service owns validation/scheduling/persistence.
- Callers/consumers: model tool registry, embedded `harnessd` adapter, remote cron client, cron HTTP server, SQLite store, and transcript-readable tool result serialization.
- Similar abstractions searched: all `cron_*` constructors, registry wiring, `CronClient` implementations, `UpdateJobRequest` call sites, permission/catalog tests, and embedded descriptions. The historical `fix/cron-tools-core` branch contains an unmerged `cron_update` implementation; `cron_history` is explicitly excluded.
- Search evidence: `rg -n "cron_(create|list|status|history|update|pause|resume|delete)|CronClient|CronUpdateJobRequest|UpdateJobRequest" internal cmd .github docs`.
- Conclusion: extend the existing update service and tool seam; do not add a dispatcher or parallel cron abstraction.

## Config, API, CLI, and Tools

- User-facing config: none.
- Defaults: existing service default timeout remains authoritative; update fields are optional and patch only supplied values.
- Environment/config: none.
- API: additive `expected_updated_at` on cron update requests; stale writes return a conflict response. Existing clients without the token remain compatible.
- Tool: add `cron_update` with `id`, optional schedule, execution config/command, timeout, tags, and expected timestamp; pause/resume remain explicit tools.
- Errors: missing ID/no-op/invalid timestamp are actionable model-facing errors; invalid schedule remains service-owned.

## Persistence and Compatibility

- Schema/migrations: no schema change; the existing `updated_at` value is the optimistic token.
- Compatibility: old clients may omit `expected_updated_at`; all existing tool names and stored rows remain readable.
- Mixed-version behavior: an older remote service ignores unknown JSON only if its decoder remains permissive; the local current service enforces the token. No destructive migration or ownership field is introduced.

## Lifecycle, Security, and Reliability

- Concurrency: service compares the expected timestamp with the freshly loaded row before applying the patch; mismatch is a 409 conflict. Scheduler re-arm behavior stays in the existing service.
- Security/privacy: no tenant, agent, conversation, or ownership mutation fields are exposed; scope filtering remains #1001's contract. Config/command values are not logged by the new tool.
- Failure/recovery: failed update leaves the stored job unchanged; caller refreshes with `cron_get` and retries using the returned `updated_at`.

## Product and Integration Surfaces

- Server/runtime: existing cron HTTP server and `harnessd` remote adapter gain only the additive token mapping.
- TUI/web/macOS/other clients: `None — this slice adds a model-facing tool and additive server request field; downstream UI lifecycle work belongs to #1009.`
- Provider/model/tool catalog: registry, permissions metadata, schema, and embedded description gain `cron_update`; no provider routing changes.
- External systems: `None — no new external integration.`
- UX/accessibility: transcript result is the existing structured `CronJob`; no UI surface changes.

## Deployment and Operations

- Order/flags: deploy additively; no feature flag or migration required.
- Observability: existing cron service errors and job timestamps remain the diagnostics; no prompt/config logging added.
- Rollback: remove the registry entry/tool implementation while retaining the additive request field; existing CRUD endpoints and stored jobs continue working.
- Runbooks: `None — no operator procedure changes.`

## Regression Tests

- Characterization/red: deferred tool test fails to compile before `CronUpdateTool`; server stale-token test fails before conflict handling.
- Acceptance: partial update, config/command mapping, no-op rejection, stable ID/result, registry presence, and stale-write conflict.
- Edge/negative: missing ID, malformed JSON/timestamp, client errors, omitted fields, and existing invalid schedule path.
- Integration: cron client/server request mapping and the existing embedded adapter tests.
- Exact commands: focused `go test` in deferred/tools/cron packages, `go test ... -race`, then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs/public docs: this plan and the embedded `cron_update.md` description.
- After code: update plans/logs indexes plus engineering, observational, and system logs.
- Training/release notes: `None — no separate release note system is used for this internal tool catalog change.`

## Warning Check

All surfaces are explicitly mapped; unaffected UI/external/runbook surfaces include search-based rationale.
