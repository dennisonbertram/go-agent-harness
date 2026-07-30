# Issue #1002 — Conversational Cron CRUD

## Context

- Governing GitHub issue: [#1002](https://github.com/dennisonbertram/go-code/issues/1002)
- Parent epic: [#1000](https://github.com/dennisonbertram/go-code/issues/1000)
- Dependency: [#1001](https://github.com/dennisonbertram/go-code/issues/1001), closed and verified as an ancestor of `origin/main` at `fedcf6073135deb7cce1fa49921aa698a9cc7cd7`.
- Problem: the cron service/client already support update, but the deferred model-facing catalog exposes only create, list, get, delete, pause, and resume. An agent cannot edit an existing job in place.
- User impact: an operator can ask the agent to change a recurring job while preserving its stable job ID, scope, and execution history.

## Scope

- In scope: model-facing `cron_update`, partial schedule/execution-config/timeout/tag updates, validation/no-op errors, optimistic update conflict transport, registration, description, tests, and required documentation/log/index updates.
- Out of scope: cron history, remote `cronsd` execution, overlap policy, callback persistence/retry, macOS controls, and any other epic child.

## Documentation Contract

- Feature status: `implemented; reviewable PR pending; full regression blocked by pre-existing Keychain test helper failure`
- Public docs affected: embedded tool description and plan/log artifacts; no user guide route list is changed because this is a model-facing tool.
- Spec docs to update before code: this plan and impact map.
- Implementation notes to add after code: engineering, observational, and system logs with exact red/green/full-gate evidence.

## Test Plan (TDD)

- New failing tests first:
  - `TestCronUpdateChangesOnlyTheFieldsGiven` proves partial request forwarding and stable ID.
  - `TestCronUpdateRejectsANoOp` proves actionable validation.
  - `TestCronUpdateAcceptsExecutionConfigAndExpectedTimestamp` proves config and conflict token serialization.
  - `TestServerUpdateJobRejectsStaleExpectedUpdatedAt` proves the service rejects a stale write.
  - registry and embedded-description tests prove catalog/schema/documentation coverage.
- Existing tests to update: cron registry expected tool set, description manifest, and remote adapter request mapping.
- Regression tests required: invalid JSON, missing ID, invalid timestamp, client/service errors, and omitted-field preservation.

## Cross-Surface Impact Map

See `2026-07-31-issue-1002-conversational-cron-crud-impact-map.md`.

## Implementation Checklist

- [x] Dependency readiness checked against current `origin/main`.
- [x] Current ownership and duplicate-tool search recorded.
- [x] Plan and impact map created before implementation.
- [x] Add failing tests and capture the expected red result.
- [x] Implement the smallest update seam and transport conflict check.
- [ ] Run focused, race, and full regression verification; focused tests pass,
      while the full normal gate is blocked by the documented Keychain helper.
- [ ] Commit and push a reviewable PR with `Closes #1002`; do not merge.

## Risks and Mitigations

- Risk: a model update could overwrite a concurrent operator edit. Mitigation: accept an optional `expected_updated_at` token and return HTTP 409 on mismatch; the tool exposes the field for read-then-update flows.
- Risk: omitted JSON fields could erase existing configuration. Mitigation: pointer fields plus tests that assert nil for omitted values.
- Risk: ownership could become model-mutable. Mitigation: update request has no tenant, agent, conversation, or job-ID mutation fields; existing #1001 scope binding remains authoritative.
