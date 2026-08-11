# Cross-Surface Impact Map: Issue #1241 raw SSE lifecycle ordering

## Task

- Task / issue: #1241 per-tool API acceptance lifecycle order.
- Plan link: `2026-08-07-issue-1241-api-sse-order-plan.md`.
- Owner: test-only `cmd/harnessd` acceptance helper.
- Status: implemented and pushed as stacked PR #1242 at code head
  `8f8c58b6a2486acd3b8ef31f117be744ba6baf35`; fresh exact-head review pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `TestIssue1231FilesystemAndGitToolsUseOneDurableConversation`
  calls `assertFilesystemGitToolCalls` once per retained run stream.
- Owning source: `cmd/harnessd/filesystem_git_api_acceptance_test.go`; raw SSE
  becomes `[]harness.Event` in `decodeFilesystemGitSSE`.
- Consumers: only the #1231 test and its retained artifacts; no product caller.
- Search evidence: `rg -n "assertFilesystemGitToolCalls|EventToolCallStarted|EventToolCallCompleted" cmd/harnessd internal` found the helper and canonical event constants.
- Conclusion: replace only this helper's evidence matcher; do not duplicate or
  alter the server/Runner event protocol.

## Config, API, CLI, and Tools

- User-facing config added or changed: None.
- Defaults/fallbacks/environment: existing private artifact-root setting only;
  its meaning is unchanged.
- HTTP endpoints/wire formats/CLI/tool catalog: None; raw event payload fields
  (`run_id`, `call_id`, `tool`, `arguments`, `output`, `error`) are asserted,
  not changed.
- Error states: acceptance fails closed with test diagnostics only.

## Persistence and Compatibility

- Schemas/migrations/caches: None.
- Compatibility: no product artifacts or persisted records change.
- Mixed-version behavior: None; this validates one daemon's emitted stream.

## Lifecycle, Security, and Reliability

- Concurrency: matcher accepts concurrent starts/completions by keyed
  unmatched state; duplicate ID detection is scoped to one run.
- Security/privacy: retained private `0700` evidence and its digests unchanged.
- Failure/recovery: orphan, duplicate, wrong-run/name/ID mismatch, and
  unfinished calls fail deterministically before evidence can pass.

## Product and Integration Surfaces

- Server/runtime: no implementation change; the direct real daemon remains
  the integration proof source.
- TUI/web/macOS: None; no client code or rendering changes.
- Provider/model/tool catalog: None; existing fake turns/default registry stay
  exactly as #1231 defines them.
- External systems/UX: None.

## Deployment and Operations

- Deployment/migration/flags: None; test-only PR.
- Diagnostics: existing retained raw-SSE artifacts and explicit lifecycle
  failure messages improve reviewability.
- Rollback: revert this test-only PR; no runtime state needs repair.
- Runbooks: acceptance-inventory wording stays accurate; no operator workflow
  changes.

## Regression Tests

- First red: completion-before-start frames falsely pass current split-slice
  matching.
- New acceptance: valid A/B interleave; rejected orphan, duplicate start,
  duplicate completion, tool/ID mismatch, wrong run, and unfinished start.
- Real path: the #1231 one-daemon/four-turn test proves the matcher against
  actual HTTP/SSE output.
- Commands: focused normal/race `go test ./cmd/harnessd -run
  '^(TestFilesystemGitToolCallLifecycle.*|TestIssue1231FilesystemAndGitToolsUseOneDurableConversation)$'`, then `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public specs: None.
- Implementation records: plan/index; engineering, observational, and system
  logs plus log index.
- Handoff: PR #1242 says `Closes #1241`, declares its #1232 base, and records
  the focused plus 85.1% full-gate evidence. It must receive a fresh review at
  the documentation-amended head before the stack is promoted.

## Warning Check

- Every surface is either scoped to the acceptance matcher or explicitly
  unaffected with source-search rationale; no product behavior is implied.
