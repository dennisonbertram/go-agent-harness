# Cross-Surface Impact Map: Issue #1194

## Task

- Task / issue: #1194 `git_blame_context` porcelain metadata parsing.
- Plan: `2026-08-05-issue-1194-blame-parser-plan.md`.
- Owner: issue-1194 worktree.
- Status: red-first implementation pending.

## Current Ownership, Callers, and Data Flow

- Entry point: `GitBlameContextTool` runs `git blame --porcelain`, calls
  `parsePorcelainBlame`, best-effort calls `git show`, then marshals the
  existing response.
- Source of truth: `internal/harness/tools/deferred/git_deep.go`; tests live in
  `git_deep_test.go`.
- Consumers: the generic tool registry/provider transcript path; GUI/TUI render
  tool output through existing transcript code.
- Search evidence: `rg -n 'GitBlameContextTool|parsePorcelainBlame|git_blame_context'
  internal/harness/tools/deferred cmd` found no second parser or client adapter.
- Conclusion: one parser owns header identity; avoid parallel parse logic.

## Config, API, CLI, and Tools

- Config/defaults/environment: none.
- API/CLI/tool schema: unchanged `git_blame_context` request and result fields.
- Error handling: main `git blame` errors remain tool failures; only optional
  `git show` enrichment is suppressed when nonzero/timed out.

## Persistence and Compatibility

- Persistence/schema/cache: none.
- Compatibility: valid 40/64-hex porcelain records retain existing output;
  malformed metadata is ignored rather than reclassified as a commit.
- Mixed rollout: stateless binary replacement; no stored data repair.

## Lifecycle, Security, and Reliability

- Concurrency/cancellation: existing context/timeouts stay intact. A timed-out
  enrichment cannot populate content.
- Security/privacy: no new command, path, authorization, or secret surface;
  suppressing failed stderr avoids surfacing misleading diagnostics as history.
- Recovery: each hash enrichment remains independent and best effort.

## Product and Integration Surfaces

- Harness runtime: corrected tool JSON emitted to the existing run/SSE flow.
- TUI/GUI: no client code change; existing transcript displays corrected data.
- Provider/model/catalog/external systems: none.
- UX/accessibility: a real subject/line replaces unusable `fatal`/line-zero
  output without layout or interaction changes.

## Deployment and Operations

- Deployment: normal stateless harness rollout; no flag or ordering.
- Diagnostics: tool output is corrected; existing command error paths remain.
- Rollback: one commit revert, no migration/runbook data repair.

## Regression Tests

- First red: valid literal header plus `previous` metadata is parsed as two
  records by the old code and overwrites the line/hash.
- Acceptance: 40/64 headers, malformed long metadata, and real two-commit
  rewrite all retain one real commit and positive line/subject.
- Failure: failed/timed-out `git show` does not contribute stderr subject.
- Integration: exact current-main fake-provider API/SSE continuation after tool
  response; targeted normal/race and canonical-temp full regression.

## Documentation and Handoff

- Public/spec docs: none because wire behavior already exists.
- Durable docs: plan/map, plan/log indexes, engineering/observational/system
  logs, and long-term-thinking success definition after green evidence.
- PR: one focused `Closes #1194` PR, no #1195 diff-range changes.
