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
  tests, and engineering documentation.
- Out of scope: remote cronsd transport, overlap policy, callback persistence,
  and macOS UI.

## Documentation Contract

- Feature status: `implemented; repository coverage gate remains blocked by baseline gaps`
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
- Existing tests to update: positional `RunStarter` fixtures and executor
  mocks, keeping direct shell executor calls backward-compatible.
- Regression tests required: SQLite round-trip for scoped harness config and
  scheduler execution ID propagation, plus tenant-isolation coverage for two
  jobs sharing a conversation string.

## Cross-Surface Impact Map

- Config: None; no user-facing configuration or environment variable changes.
- Server API: Internal cron execution config gains optional typed harness scope
  fields; the authenticated HTTP create path continues to stamp tenant scope.
- TUI state: None; cron runs enter the existing runner request boundary and do
  not alter TUI state or persisted client settings.
- Regression tests: `internal/cron` and `cmd/harnessd` contract, compatibility,
  persistence, and isolation tests.

## Implementation Checklist

- [x] Define acceptance criteria in the issue and this plan.
- [x] Document feature status and exact contract before code.
- [x] Review ownership/copy semantics for persisted request/config data.
- [x] Write failing tests first.
- [x] Implement minimal code changes.
- [x] Update engineering log and indexes.
- [x] Run focused tests and repository regression suite.
- [x] Commit the issue-scoped files.

## Verification Note

- Focused tests, `go test ./...`, and the normal and race phases of
  `./scripts/test-regression.sh` pass.
- The final coverage gate exits nonzero on seven pre-existing zero-coverage
  functions in `internal/harness/job_bridge.go`, `internal/modelstore`, and
  `internal/server/http_catalog.go`; the issue-scoped cron dispatcher methods
  are covered and do not appear in that zero-function list.

## Risks and Mitigations

- Risk: changing the shared executor interface breaks shell and test callers.
  Mitigation: preserve the existing `Executor.Execute` contract and add an
  execution-aware optional seam for typed harness dispatch.
- Risk: old rows lack new scope fields. Mitigation: optional JSON fields decode
  to explicit empty defaults; no migration rewrites or deletes existing data.
- Risk: prompt/config data attempts to override stored tenant scope.
  Mitigation: construct the runner request from stored job/config fields only;
  do not accept scope overrides in the model-facing tool schema.
