# Cross-Surface Impact Map: Issue #1195

## Task

- Task / issue: #1195 `git_diff_range` files-changed aggregate.
- Plan: `2026-08-05-issue-1195-diff-count-plan.md`.
- Owner: issue-1195 isolated worktree.
- Status: red-first implementation pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `GitDiffRangeTool` invokes `git diff --stat`, calls
  `parseStatSummary`, and marshals existing fields.
- Source of truth: `internal/harness/tools/deferred/git_deep.go`; unit/tool
  tests are in `git_deep_test.go`.
- Consumers: the existing tool registry, provider output, run events, and
  generic TUI/native transcript renderers.
- Search evidence: `rg -n 'parseStatSummary|GitDiffRangeTool|git_diff_range'
  internal cmd docs` found one parser and no client-specific adapter.
- Duplication conclusion: correct the single parser; do not duplicate summary
  parsing in tool callers or clients.

## Config, API, CLI, and Tools

- Config/defaults/environment: none.
- Endpoint/request/result schema: unchanged; existing `files_changed`,
  `insertions`, `deletions`, `stat`, and `stat_only` fields retain names/types.
- CLI/TUI/API behavior: corrected numeric aggregate travels through existing
  tool-result/SSE rendering path without command or UI changes.
- Error handling: unchanged command validation and git errors.

## Persistence and Compatibility

- Schemas/migrations/caches: none.
- Compatibility: no-diff remains 0/0/0; valid singular/plural Git summaries
  now match their returned stat text; mixed-version behavior is stateless.
- Partial rollout: normal binary replacement, no stored data repair.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation/retries: none; parsing occurs after existing
  bounded command execution.
- Auth/authorization/privacy/secrets: no new command, path, capability, or
  output surface.
- Failure/recovery/idempotency: malformed/non-summary text leaves zero values
  rather than affecting command success.

## Product and Integration Surfaces

- Harness runtime: corrected aggregate in current tool JSON and SSE transcript.
- TUI/GUI: no code change; existing generic renderers display the corrected
  structured value.
- Provider/model/tool catalog: tool name and registration unchanged.
- External systems/UX/accessibility: none.

## Deployment and Operations

- Deployment/flags: stateless normal rollout; no feature flags.
- Observability: existing tool output becomes internally consistent with Git
  stat, improving diagnostics.
- Rollback/recovery: revert one parser/test/docs commit; no migration.
- Runbooks: no operator procedure change.

## Regression Tests

- First red: literal singular/plural/files-only parser counts are zero under
  the old token-position logic.
- Acceptance: two-commit fixture asserts all counts and stat for normal and
  `stat_only`; no-diff remains 0/0/0.
- Negative/failure: parser ignores non-summary/no-diff input.
- Integration: exact-current fake-provider API/SSE multi-message proof proves
  tool result and continuation are observable.
- Commands: focused normal/race `go test ./internal/harness/tools/deferred/...`,
  then `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Public/spec docs: none; no public wire contract is added.
- Durable docs: plan/map, `docs/plans/INDEX.md`, engineering and observational
  logs, plus their index remain aligned with verified behavior.
- PR: one focused `Closes #1195` PR, no merge in this slice.
