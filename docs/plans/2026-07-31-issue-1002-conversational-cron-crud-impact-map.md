# Issue #1002 — Conversational Cron CRUD Impact Map

## Task

- Task / issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Plan link: `2026-07-31-issue-1002-conversational-cron-crud-plan.md`
- Owner: Codex implementation worktree
- Status: acceptance-audit repair implemented locally on
  `codex/issue-1002-repair-v2`, review-clear and rebased onto `origin/main`
  `3506e01c`; exact rebased full regression and a real-provider
  same-conversation CRUD/fire canary are green. Guarded push and merge remain.

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/harness/tools/deferred/cron.go` model-facing CRUD tools, `NewDefaultRegistryWithOptions`, `internal/cron.Server` create/PATCH/DELETE routes, `internal/cron.Client`, and the embedded `harnessd` adapter.
- Source of truth: `tools.CronClient`, typed create/update request payloads, and the cron store; the model-facing scoped client owns RunMetadata authorization, while the service owns validation, persistence CAS, and scheduler reconciliation.
- Callers/consumers: model tool registry, embedded `harnessd` adapter, remote cron client, cron HTTP server, SQLite store, and transcript-readable tool result serialization.
- Similar abstractions searched: all `cron_*` constructors, registry wiring, `CronClient` implementations, `RunStartRequest`, `HarnessExecutor`, `DispatchExecutor`, create/update request call sites, permission/catalog tests, and embedded descriptions. The historical `fix/cron-tools-core` branch contains an unmerged `cron_update` implementation; this slice owns the model CRUD/history catalog, while #1003 remote authentication and dispatch remain explicitly excluded.
- Search evidence: `rg -n "cron_(create|list|status|history|update|pause|resume|delete)|CronClient|CronUpdateJobRequest|UpdateJobRequest" internal cmd .github docs`.
- Conclusion: extend the existing update service and tool seam; do not add a dispatcher or parallel cron abstraction.

## Config, API, CLI, and Tools

- User-facing config: model schema now distinguishes legacy `shell` command creation from explicit `harness` prompt creation.
- Defaults: omitted create `execution_type` remains legacy shell; omitted create timeout remains 30; explicitly supplied create and all update timeouts must be positive. Shell configs require a non-empty `command`; harness configs require a non-empty `prompt`.
- Environment/config: none.
- API: additive `expected_updated_at` on cron update and versioned delete requests; persistence CAS returns typed conflict and HTTP 409 on zero matching rows. `/v1/jobs/{id}` is now ID-only; explicit operator lookup is `/v1/jobs/by-name?name=...`, whose query encoding preserves slash, spaces, percent, and Unicode. Ownership scope is propagated across the internal client/server request without adding #1003 authentication. Empty raw operator DELETE remains compatible.
- Tool: `cron_create` supports legacy shell or explicit harness prompt config; get/update/history/pause/resume/delete schemas explicitly require job IDs and reject name semantics. Every model mutation of an existing row requires `updated_at` from `cron_get`. `cron_get` always reports whether its recent-history query was available and includes a warning on failure, so an unavailable query cannot masquerade as a successful empty history.
- Errors: missing ID/no-op/invalid timestamp are actionable model-facing errors; global operator name collisions return typed `ErrJobAmbiguous`/HTTP 409.

## Persistence and Compatibility

- Schema/migrations: replace legacy global `UNIQUE(name)` with a partial unique index over `(tenant_id, conversation_id, agent_id, name)` for non-deleted rows. Transactional rebuild uses SQLite `index_list`/`index_xinfo` metadata rather than DDL text, recognizing inline/named/quoted/collated single-column constraints while excluding composite and partial indexes; jobs/executions are preserved.
- Compatibility: old stored rows and histories remain readable with exact timestamps; a second migration is a no-op. The former implicit GET-by-name route is deliberately replaced by a distinct operator route so model calls cannot become name lookups.
- Mixed-version behavior: an older remote service may accept name fallback or unversioned PATCH; the current model client requires IDs/version tokens. Raw remote authentication remains #1003.

## Lifecycle, Security, and Reliability

- Concurrency: service writes through `UpdateJobCAS` and model deletion through `DeleteJobCAS`; mutation paths serialize store and scheduler transitions. Active replacement is `Prepare` (inert candidate, old entry retained) → durable CAS → infallible in-memory `Commit`; prepare/CAS failure aborts only the candidate. Registration identities are monotonic and checked again after jitter and durable reload, so queued pre-pause/pre-replacement callbacks cannot execute after a completed transition.
- Validation: `ValidateExecutionConfig` is shared by HTTP and embedded service boundaries, so shell-only rows cannot be created with empty/unknown command config and harness rows cannot be created or updated with an incomplete prompt. Model-tool timeout presence is pointer-valued so explicit zero is not confused with the omitted default.
- Security/privacy: no tenant, agent, conversation, or ownership mutation fields are exposed. Remote server and embedded adapter select by exact tenant + conversation + agent predicates before CRUD/history; the wrapper remains a fail-closed outer boundary. Raw cronsd authentication is not added here.
- Failure/recovery: active replacement prepare or CAS failure leaves the prior durable and live state unchanged. Create and paused→active resume persist/register through paused-first handling, so registration/activation failure leaves a durable paused row that restart will not arm. The remote server and embedded adapter share this policy.

## Product and Integration Surfaces

- Server/runtime: `NewDefaultRegistryWithOptions` installs one idempotent scoped cron wrapper for every top-level, worktree per-run, and subagent model registry; operator/server wiring retains raw adapters. Cron HTTP server and embedded/remote `harnessd` adapters gain atomic update/delete handling; embedded bootstrap already routes `ExecTypeHarness` through `HarnessExecutor` and the model tool now produces its typed prompt config.
- TUI/web/macOS/other clients: `None — this slice adds a model-facing tool and additive server request field; downstream UI lifecycle work belongs to #1009.`
- Provider/model/tool catalog: registry, permissions metadata, schema, and embedded descriptions cover both `cron_create` execution modes and `cron_update`; no provider routing changes.
- External systems: `None — no new external integration.`
- UX/accessibility: transcript result is the existing structured `CronJob`; no UI surface changes.

## Deployment and Operations

- Order/flags: deploy additively; the scoped-name SQLite migration runs at startup. Back up the stopped process's SQLite database before first rollout; no feature flag is required.
- Observability: existing cron service errors and job timestamps remain the diagnostics; no prompt/config logging added.
- Rollback: remove the registry entry/tool implementation while retaining the additive request field; existing CRUD endpoints and stored jobs continue working.
- Runbooks: `None — no operator procedure changes.`

## Regression Tests

- Characterization/red: deferred create harness test initially showed shell-only creation; CAS concurrency test initially allowed serial read/check/write behavior; timeout/version/lifecycle tests were added before their fixes.
- Acceptance: typed harness create, immutable scope, same-conversation starter, partial update, atomic stale conflict, no-op/timeout rejection, stable ID/result, registry presence, and full lifecycle.
- Edge/negative: missing ID/version, malformed JSON/timestamp/timeout, client errors, omitted fields, mixed shell/harness inputs, and existing invalid schedule path.
- Integration: real assembled default registries over embedded and remote client/server/SQLite/scheduler adapters prove automatic RunMetadata scope, raw operator compatibility, stale pause/resume/delete conflicts, owned CRUD/history, two-scope same-name isolation, explicit operator ambiguity, paused-first create/resume, prepare/CAS/commit replacement, stale-callback suppression, and concurrent update/delete no-rearm.
- Latest focused commands for this repair:
  `go test [ -race ] ./internal/cron ./internal/harness ./internal/harness/tools ./internal/harness/tools/deferred ./cmd/harnessd -count=1`.
  Both pass after the history-availability repair. The exact rebased candidate
  also passes `./scripts/test-regression.sh` at 85.7% total coverage and zero
  uncovered functions. A real OpenAI-backed conversation exercised all eight
  model tools and two scheduled same-chat continuations before deletion. Push
  and merge remain promotion gates.

## Documentation and Handoff

- Specs/public docs: this plan and the embedded `cron_update.md` description.
- After code: plan/index plus long-term-thinking, engineering, observational, and system logs record the audit repair and exact focused evidence.
- Training/release notes: `None — no separate release note system is used for this internal tool catalog change.`

## Warning Check

All surfaces are explicitly mapped; unaffected UI/external/runbook surfaces include search-based rationale.
