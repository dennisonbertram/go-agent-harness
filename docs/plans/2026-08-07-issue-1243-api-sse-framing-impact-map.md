# Cross-Surface Impact Map: Issue #1243 SSE framing proof

## Task

- Task / issue: #1243.
- Plan link: `2026-08-07-issue-1243-api-sse-framing-plan.md`.
- Owner: `cmd/harnessd/filesystem_git_api_acceptance_test.go` test-only proof.
- Status: implemented and full-regression green on #1232's exact head; stacked
  PR and independent exact-head review remain.

## Current Ownership, Callers, and Data Flow

- Entry: `TestIssue1231FilesystemAndGitToolsUseOneDurableConversation` retains
  each daemon stream then calls `assertFilesystemGitToolCalls`.
- Source of truth: production `internal/server.writeSSE` writes `id:`,
  `retry:`, `event:`, and JSON `data:` fields; the #1231 decoder must prove
  rather than recreate that relation.
- Callers/consumers: this acceptance helper only. Search evidence: `rg -n
  "writeSSE|decodeFilesystemGitSSE|assertFilesystemGitToolCalls" internal cmd`
  locates the production writer and this one test consumer.
- Conclusion: replace only the evidence decoder/validator, not server writer.

## Config, API, CLI, and Tools

- Config/defaults/env: none; existing private artifact root is unchanged.
- Endpoints/wire format/CLI/catalog: none changed. Existing SSE headers and
  envelope fields are asserted for equality.
- Error states: malformed retained test evidence fails closed with diagnostics.

## Persistence and Compatibility

- Schemas/migrations/caches: none.
- Compatibility/mixed versions: none; this reads one emitted stream only.

## Lifecycle, Security, and Reliability

- Concurrency: preserve the #1241 keyed pending-call matcher and legal
  concurrent A/B lifecycle completion order.
- Cancellation/retry/cleanup: unchanged.
- Security/privacy: private retained raw evidence and digest behavior unchanged;
  rejecting identity substitution avoids false provenance.

## Product and Integration Surfaces

- Server/runtime: no code change; direct harnessd stream remains proof source.
- TUI/web/macOS, providers/catalog, external systems/UX: none; search evidence
  shows no caller outside this acceptance-only decoder.

## Deployment and Operations

- Deployment/flags/runbooks: none.
- Diagnostics: exact header/JSON mismatch errors make reviewable artifacts
  fail closed.
- Rollback: revert this test-only slice.

## Regression Tests

- Red/negative: missing header ID/event, empty JSON ID/type, mismatched ID/type,
  and header-only identity are rejected; comment-only ping is ignored.
- Positive: normal and multi-data frames plus concurrent A/B lifecycle frames
  preserve valid ordering.
- Real path: the #1231 one-daemon/four-turn test validates actual writer output.
- Commands: focused normal/race `go test ./cmd/harnessd -run
  '^(TestFilesystemGitSSEDecoder|TestFilesystemGitToolCallLifecycle|TestIssue1231FilesystemAndGitToolsUseOneDurableConversation)$'`, followed by
  `./scripts/test-regression.sh`.

## Documentation and Handoff

- Public docs: none; no product change.
- Records: plan/map, plan index, engineering/observational/system logs and
  their indexes.
- PR: stacked on `codex/issue-1231-api-filesystem-git`, `Closes #1243`, with
  exact head/base, red and green evidence, real-daemon proof, and full gate.

## Warning Check

- Every cross-surface heading is explicitly no-op or test-only; product SSE
  writer changes are out of scope.
