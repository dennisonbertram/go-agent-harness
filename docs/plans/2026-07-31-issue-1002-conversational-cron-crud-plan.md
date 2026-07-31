# Issue #1002 — Conversational Cron CRUD

## Context

- Governing GitHub issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Parent epic: [#1000](https://github.com/dennisonbertram/go-code/issues/1000)
- Dependency: [#1001](https://github.com/dennisonbertram/go-code/issues/1001), closed and verified as an ancestor of `origin/main` at `fedcf6073135deb7cce1fa49921aa698a9cc7cd7`.
- Problem: the deferred model-facing catalog exposed shell-only creation and lacked a safe in-place update path. An agent could schedule a shell command, but could not create a typed harness continuation or safely edit a recurring job.
- User impact: an operator can ask the agent to create, inspect, change, pause/resume, and delete a recurring conversation job while preserving its stable job ID, immutable scope, execution history, and same-conversation harness behavior.

## Scope

- In scope: model-facing shell-compatible and explicit harness `cron_create`, typed prompt execution config, immutable RunMetadata binding, model-facing `cron_update`, atomic persistence CAS, mandatory update version tokens, timeout validation, lifecycle/concurrency tests, registration, descriptions, and required documentation/log/index updates.
- Out of scope: remote `cronsd` harness transport and auth/readiness (#1003), terminal linkage/overlap (#1004), cron history, callback persistence/retry, macOS controls, and any other epic child.

## Documentation Contract

- Feature status: `implemented on reviewable PR #1057; focused normal/race green; full regression remains a separate pre-existing Keychain blocker`
- Public docs affected: embedded tool description and plan/log artifacts; no user guide route list is changed because this is a model-facing tool.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering, observational, and system logs with exact red/green/full-gate evidence.

## Test Plan (TDD)

- New failing tests first:
  - `TestCronCreateHarnessJobUsesImmutableRunScope` proves typed harness config and scope override rejection.
  - `TestIntegrationCronUpdateCompareAndSwapAllowsOneConcurrentWriter` proves atomic one-winner persistence and no stale active scheduler resurrection.
  - `TestServerUpdateJobRejectsNonPositiveTimeout` proves authoritative unsafe-timeout rejection.
  - `TestEmbeddedCronModelToolsFullScopedLifecycle` proves create/list/get/update/pause/resume/delete through model tools and a stateful scoped adapter.
  - registry, comprehensive tool-list, schema, and embedded-description tests prove catalog coverage.
- Existing tests to update: cron registry/tool lists, description manifest, remote adapter request mapping, and the assembled embedded harness path.
- Regression tests required: invalid JSON, missing ID/version, invalid timestamp/timeout, client/service errors, omitted-field preservation, concurrent CAS, scope isolation, and harness starter correlation.

## Cross-Surface Impact Map

See `2026-07-31-issue-1002-conversational-cron-crud-impact-map.md`.

## Implementation Checklist

- [x] Dependency readiness checked against current `origin/main`.
- [x] Current ownership and duplicate-tool search recorded.
- [x] Plan and impact map created before implementation.
- [x] Add failing tests and capture the expected red result.
- [x] Implement atomic persistence CAS and move scheduler reconciliation after a successful write.
- [x] Add explicit model-facing harness creation while preserving legacy shell creation.
- [x] Enforce authoritative non-empty shell commands, non-empty harness prompts, valid schedules, and positive explicitly supplied timeouts on create/update boundaries.
- [x] Run focused normal and race verification; full regression remains blocked by the documented Keychain helper.
- [x] Commit and push reviewable PR #1057 with `Closes #1002`; do not merge. Audit follow-up head is recorded in the handoff below.

## Risks and Mitigations

- Risk: a model update could overwrite a concurrent operator edit. Mitigation: `cron_update` requires `expected_updated_at`; the service performs a persistence-level compare-and-swap and returns typed/HTTP 409 on zero-row matches. Legacy pause/resume calls also CAS against their loaded version.
- Risk: omitted JSON fields could erase existing configuration. Mitigation: pointer fields plus tests that assert nil for omitted values.
- Risk: ownership could become model-mutable. Mitigation: update request has no tenant, agent, conversation, or job-ID mutation fields; existing #1001 scope binding remains authoritative.
- Risk: harness creation could silently degrade to shell. Mitigation: explicit `execution_type` validation, typed `{prompt}` config, distinct shell/harness inputs, and an assembled starter test; remote cronsd transport remains #1003.
- Risk: malformed execution config or an unsafe timeout could reach persistence. Mitigation: `ValidateExecutionConfig` runs at HTTP and embedded create/update boundaries; explicit HTTP/tool timeout values must be positive while omitted create values retain the 30-second default.
