# Plan: Issue #1001 — preserve embedded cron harness scope

## Context

- Problem: the embedded cron harness handoff currently passes only prompt and
  conversation ID, so tenant and agent ownership are lost when a scheduled
  continuation starts.
- User impact: a scheduled continuation can be reconstructed with incomplete
  ownership metadata, weakening isolation and observability.
- Constraints: additive persistence compatibility, shell cron behavior must not
  change, stored ownership is authoritative, and prompt/tool input cannot
  replace it.

## Scope

- In scope: typed cron start request, harness execution-config decoding,
  execution/job correlation IDs, embedded `harnessd` adapter wiring, regression
  tests, standalone cron API round-trip compatibility for the additive scope
  fields, and engineering documentation.
- Out of scope: remote cronsd transport, overlap policy, callback persistence,
  and macOS UI.

## Review Repair (2026-07-30)

- Review finding 1: the standalone cron HTTP server decodes the new
  `conversation_id` and `agent_id` request fields but drops them when it builds
  the persisted `Job`.
- Review finding 2: tests prove the scheduler, executor, and runner adapter as
  separate seams, but no acceptance test composes the persisted scoped job
  through the dispatcher into a real runner.
- Review finding 3: the repository regression script exits nonzero at the
  zero-function coverage gate. Repository policy requires those baseline gaps
  to be fixed before this slice is complete.
- Impact map: `2026-07-30-issue-1001-cron-scope-handoff-impact-map.md`.

## Documentation Contract

- Feature status: `review repairs implemented and repository gate green`
- Public docs affected: None; this is an internal correctness contract.
- Spec docs to update before code: this plan and the issue acceptance criteria
  are the governing contract.
- Implementation notes to add after code: engineering log entry describing
  the typed boundary and legacy empty-scope behavior.

## Test Plan (TDD)

- New failing tests to add first:
  - `internal/cron`: a harness job produces a typed start request containing
    prompt, tenant, conversation, agent, job, and execution IDs.
  - `internal/cron`: legacy harness config without optional scope remains
    runnable with explicit empty defaults; shell execution remains unchanged.
  - `cmd/harnessd`: the cron starter maps the typed request into every
    corresponding `harness.RunRequest` field and ignores prompt-supplied scope.
  - `internal/cron`: POSTing a scoped `CreateJobRequest` through the standalone
    server persists and returns tenant, conversation, and agent unchanged.
  - `cmd/harnessd`: a scoped SQLite job crosses
    `DispatchExecutor`/`HarnessExecutor`/`cronRunStarter` and creates a real
    runner run with the same scope.
  - Coverage packages: behavior tests exercise every function named by the
    repository zero-function gate; the gate itself is the acceptance check.
- Existing tests to update: positional `RunStarter` fixtures and executor
  mocks, keeping direct shell executor calls backward-compatible.
- Regression tests required: SQLite round-trip for scoped harness config and
  scheduler execution ID propagation, plus tenant-isolation coverage for two
  jobs sharing a conversation string.

## Cross-Surface Impact Summary

- Ownership/callers: model tool metadata and authenticated HTTP callers feed
  the cron client; embedded and standalone adapters must persist the same scalar
  scope contract.
- Config/env/defaults: None; the existing embedded-vs-remote selection remains
  unchanged.
- API/CLI/wire formats: additive `tenant_id`, `conversation_id`, and `agent_id`
  fields round-trip through `CreateJobRequest`; no CLI flag or tool schema adds
  model-controlled ownership.
- Persistence/migrations: existing additive SQLite columns remain authoritative;
  no destructive rewrite or down migration.
- Concurrency/lifecycle/recovery: scheduled behavior is unchanged; a narrow
  in-process `TriggerJob` method loads the authoritative active job and uses
  the same asynchronous fire path, making the composed handoff deterministic
  and providing an operator-valid manual-run primitive without a new endpoint.
- Security/auth/privacy: tenant stamping at the authenticated harness route
  remains authoritative; the model-facing tool derives all scope from run
  metadata; logs exclude prompts and credentials.
- Product clients: TUI, web, and macOS are unaffected because the response
  fields are additive and already optional.
- Provider/model/tool catalog: no provider or model behavior changes; cron tool
  names and schemas remain unchanged.
- Deployment/observability: existing migration and lifecycle log are retained;
  standalone API data loss is removed.
- Compatibility/versioning: legacy rows retain empty defaults and the historical
  config conversation fallback; shell jobs remain behaviorally unchanged.
- Tests/evals/fixtures: focused cron/store/server/runner tests plus the complete
  repository regression script and race phase.
- Documentation: this plan, its impact map, plans index, and engineering log.
- Copy semantics: all new persisted fields are immutable strings; no slice,
  map, pointer, or aliasing contract is introduced.

## Implementation Checklist

- [x] Define acceptance criteria in the issue and this plan.
- [x] Document feature status and exact contract before code.
- [x] Review ownership/copy semantics for persisted request/config data.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Update engineering log and indexes.
- [x] Add review-repair failing tests before their fixes.
- [x] Preserve scope through the standalone cron API.
- [x] Add the composed persisted-job-to-runner regression.
- [x] Cover every function reported by the repository gate.
- [x] Run focused tests and repository regression suite to a fully green gate.
- [x] Commit the issue-scoped files.

## Verification Note

- Focused affected-package tests pass for `internal/cron`, `cmd/harnessd`,
  `internal/harness`, `internal/modelstore`, and `internal/server`.
- `./scripts/test-regression.sh` passes all normal tests, the complete race
  suite, coverage generation, and the repository gate:
  `coveragegate: PASS (total=85.6%, min=80.0%, zero-functions=0)`.
- On macOS, the two real-Keychain tests pass directly but `security(1)` waits
  on the controlling terminal when the suite itself runs inside tmux. The
  accepted full run executed in the logged-in launchd context, with a tmux
  monitor retaining the required long-running-test visibility.

## Risks and Mitigations

- Risk: changing the shared executor interface breaks shell and test callers.
  Mitigation: preserve the existing `Executor.Execute` contract and add an
  execution-aware optional seam for typed harness dispatch.
- Risk: old rows lack new scope fields. Mitigation: optional JSON fields decode
  to explicit empty defaults; no migration rewrites or deletes existing data.
- Risk: prompt/config data attempts to override stored tenant scope.
  Mitigation: construct the runner request from stored job/config fields only;
  do not accept scope overrides in the model-facing tool schema.
