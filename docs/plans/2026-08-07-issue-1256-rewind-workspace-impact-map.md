# Issue #1256 Cross-Surface Impact Map

## Task

- Task / issue: #1256 — trusted workspace metadata for rewind.
- Plan link: `2026-08-07-issue-1256-rewind-workspace-plan.md`.
- Owner: Runner and SQLite conversation-store boundary.
- Status: in implementation.

## Current Ownership, Callers, and Data Flow

- Entry points: `Runner.StartRun`/step engine mutating tool calls; server
  `POST /v1/conversations/{id}/rewind`; fork endpoint.
- Source of truth: configured `RunnerConfig.WorkspaceBaseOptions.RepoPath`,
  persisted in `conversations.workspace`; SQLite `ForkConversation` copies it.
- Data flow: runner -> `UpdateConversationMeta` -> owner lookup -> restore root.
- Search evidence: `rg -n 'WorkspaceBaseOptions|RepoPath|UpdateConversationMeta|ForkConversation|rewind' internal/harness internal/server cmd`.
- Conclusion: reuse the existing metadata row and trusted config; no new store.

## Config, API, CLI, and Tools

- User config: none; the already configured runner workspace becomes durable.
- Defaults/fallbacks: default tenant stores an empty tenant ID but the same
  configured workspace; no CWD/client/point fallback.
- API/CLI: no schema or wire change; existing 404 and `/rewind <id> confirm`
  behavior remain authoritative.
- Error states: absent or empty owner workspace stays `404 not_found`.

## Persistence and Compatibility

- No migration: existing nullable/text `workspace` column is populated for new
  rewindable conversations.
- Existing empty rows remain fail-closed, including old conversations/forks.
- Fork copies the stored canonical path using existing transactional logic.

## Lifecycle, Security, and Reliability

- Ordering: metadata must be stored before `CaptureRewindPreImage` creates a
  usable point, not only at terminal message persistence.
- Trust: only runner configuration crosses to a destructive restore root;
  HTTP requests never supply a workspace.
- Failure behavior: metadata persistence errors are logged; the point cannot be
  treated as a trusted rewindable success by the acceptance path.

## Product and Integration Surfaces

- Server/runtime: owner lookup and restore retain strict root validation.
- TUI: unchanged command; 30x100 PTY proves the existing route succeeds with
  test-owned stored metadata.
- Provider/model/tool catalog: none — existing mutating-tool classification.
- External systems/UX: none; confirmation remains required.

## Deployment and Operations

- Order: single backwards-compatible binary deploy; no migration/flag.
- Observability: existing metadata-persistence error logger remains the failure
  diagnostic; PTY artifacts bind terminal evidence to test workspace state.
- Rollback: revert early metadata persistence; no stored-data repair is needed.
- Runbooks: none; session-rewind runbook remains correct.

## Regression Tests

- Red: immediate default/named metadata plus capture ordering fails on
  `36237749` because workspace is empty until terminal persistence.
- Acceptance: secure fork restores `before` only inside its trusted workspace.
- Negative: empty owner returns 404 and leaves its file plus an external file
  untouched.
- Real path: 30x100 isolated PTY invokes `rewind safe-point confirm` and
  proves content becomes `before`.
- Commands: focused `go test ./internal/harness ./internal/server -run Issue1256`,
  matching race run, then `./scripts/test-regression.sh`.

## Documentation and Handoff

- No public docs change; add durable implementation evidence after tests pass.
- Add the two artifacts to `docs/plans/INDEX.md` and record logs/index entries.

## Warning Check

- Every surface is mapped; unaffected surfaces explicitly retain current behavior.
