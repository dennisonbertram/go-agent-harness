# Cross-Surface Impact Map: Issue #1174

## Task

- Task / issue: #1174 TUI `/init` SSE persistence.
- Plan link: `2026-08-05-issue-1174-init-sse-plan.md`.
- Owner: Codex. Status: implemented locally; pending independent review and hosted CI.

## Current Ownership, Callers, and Data Flow

- Entry points: slash parser -> `executeInitCommand`; `RunStartedMsg`; `SSEEventMsg`; `SSEDoneMsg` and `RunCompletedMsg` in `cmd/harnesscli/tui/model.go`.
- Owner: `init_agents.go` owns content extraction and workspace commit; `Model` owns pending run identity/lifecycle.
- Flow: `/init` prompt -> accepted run ID -> `assistant.message` -> matching `run.completed` -> workspace `AGENTS.md` -> next system-prompt resolution.
- Search evidence: `rg -n 'pendingInitAgentsMd|SSEDoneMsg|RunCompletedMsg|AGENTS.md' cmd/harnesscli/tui` found the synthetic-only completion branch.

## Config, API, CLI, and Tools

- User-facing: `/init [confirm]` now reports actual-SSE persistence or rejected conflict/failure.
- Config/API/tools: none; no route or wire format changes. Plain final markdown remains authoritative.

## Persistence and Compatibility

- Persistence: workspace-local `AGENTS.md` only; no schema/cache/migration.
- Compatibility: existing file is replaced only after pre-generation `confirm`; replacement preserves its mode. A newly appeared file is not overwritten.
- Mixed versions: client-local repair; older TUIs retain prior behavior until updated.

## Lifecycle, Security, and Reliability

- Lifecycle: only a matching accepted terminal can write; failed/fatal/reconnect-loss and local Ctrl+C/Escape cancellation consume or retain state safely without a commit. A foreign terminal cannot commit; subsequent matching completion remains valid only when the pending request was not locally cancelled.
- Security: target remains configured workspace; no home/profile/global data touched.
- Reliability: target re-stat, temporary file close/sync, atomic rename, parent directory sync, and cleanup protect the file.

## Product and Integration Surfaces

- Server/runtime: none; consumes existing SSE contract.
- TUI: write/conflict/failure status is visible.
- GUI/macOS/web: none; they do not execute the TUI slash handler.
- Provider/model/tool routing: none; plain `assistant.message` is provider-agnostic.

## Deployment and Operations

- Deployment: normal TUI binary update; no flag/migration.
- Diagnostics: existing TUI status line reports path or no-write cause.
- Rollback: isolated client revert; no data repair/runbook change.

## Regression Tests

- First red: `TestInitCommand_SSEAssistantMessageThenCompletedWritesAgentsMd` failed because `SSEDoneMsg` omitted the write helper.
- Acceptance: matching success writes once; fenced content remains supported.
- Negative: failed/fatal/foreign/file-race terminals, an unbound startup failure (including a malformed or whitespace-only successful response without `run_id`), exhausted reconnect loss, and late messages after local cancellation never overwrite; confirmed mode persists. Accepted run IDs are normalized before identity fencing.
- Commands: focused normal/race and canonical-temp `./scripts/test-regression.sh`.

## Documentation and Handoff

- Specs: this plan/map precede code.
- Logs/indexes: updated with final verification evidence before PR.
- Public training/release notes: none; repair to existing behavior.
