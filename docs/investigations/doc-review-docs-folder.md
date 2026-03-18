# Documentation Review: docs/ Folder

**Date**: 2026-03-18
**Scope**: All docs in `docs/` excluding `docs/logs/`, `docs/investigations/`, `docs/process/`
**Method**: Compared documented claims against the actual codebase

---

## Summary

Reviewed 15+ documentation files across runbooks, design docs, and testing docs. Found several outdated or inaccurate claims, concentrated primarily in the MCP runbook (tool counts), the event catalog (missing ~30 event types), the testing runbook (incorrect test command), and the CLI live testing runbook (missing `auth` subcommand). Most design docs and operational runbooks are accurate.

---

## Runbooks

### docs/runbooks/harnesscli-live-testing.md
**Status**: OUTDATED

**Inaccuracies found**:
1. **Missing `auth` subcommand**: The doc does not mention `harnesscli auth login`, which is implemented in `cmd/harnesscli/auth.go`. The CLI has a top-level dispatch function that routes `auth` to `runAuth()` before falling through to the `run()` function.

2. **Relevant Code Paths section is incomplete**: The doc lists `cmd/harnesscli/main.go` as the CLI entrypoint but does not mention `cmd/harnesscli/auth.go` which contains the `dispatch()` function (the actual top-level entrypoint), `runAuth()`, and `runAuthLogin()`.

3. **Default port**: The doc says "usually `http://127.0.0.1:8080`". The actual default in the CLI code is `http://localhost:8080` (flag default in `main.go` line 124). Functionally equivalent but technically different strings.

**What to fix**:
- Add documentation about `harnesscli auth login` subcommand and its flags (`-server`, `-tenant`, `-name`).
- Update Relevant Code Paths to include `cmd/harnesscli/auth.go`.
- Mention the dispatch routing: `auth` subcommand vs default `run` behavior.

---

### docs/runbooks/testing.md
**Status**: OUTDATED (minor)

**Inaccuracies found**:
1. **Common Commands section**: Lists `go test ./...` as the standard command. However, the MEMORY.md and the actual `scripts/test-regression.sh` both confirm the correct package scope is `./internal/... ./cmd/...` (not `./...`). Running `go test ./...` may pull in packages outside the intended test scope.

2. **Regression Gate section**: Correctly references `./scripts/test-regression.sh` and its behavior, but the "Common Commands" section above it contradicts the actual practice. The script runs `go test ./internal/... ./cmd/...`, not `go test ./...`.

**What to fix**:
- Change `go test ./...` to `go test ./internal/... ./cmd/...` in the Common Commands section (3 occurrences).

---

### docs/runbooks/mcp.md
**Status**: OUTDATED (multiple issues)

**Inaccuracies found**:

1. **MCP HTTP server tool count**: Doc says "10 total" but lists 12 tool names in the table. The actual `internal/mcpserver/mcpserver.go` defines exactly 10 tools: `start_run`, `get_run_status`, `list_runs`, `steer_run`, `submit_user_input`, `list_conversations`, `get_conversation`, `search_conversations`, `compact_conversation`, `subscribe_run`. The doc table includes `wait_for_run` and `continue_run` which do NOT exist in the HTTP MCP server -- they only exist in the stdio binary (`internal/harnessmcp/tools.go`).

2. **Tool table is wrong**: The doc table lists `wait_for_run` and `continue_run` as HTTP MCP server tools. They are NOT. These are stdio-binary-only tools. Meanwhile, `steer_run` and `submit_user_input` ARE in the HTTP MCP server but are NOT in the doc table.

3. **Stdio binary tool count**: Doc says the stdio binary exposes 5 tools: `start_run`, `get_run_status`, `wait_for_run`, `continue_run`, `list_runs`. This is CORRECT per `internal/harnessmcp/tools.go`.

**What to fix**:
- Update the HTTP MCP server tool table to list the actual 10 tools.
- Remove `wait_for_run` and `continue_run` from the HTTP server table.
- Add `steer_run` and `submit_user_input` to the HTTP server table.
- Fix the "10 total" count -- it should remain 10, but the listed tools must match reality.

---

### docs/runbooks/deployment.md
**Status**: ACCURATE (generic)

The deployment runbook is deliberately high-level and generic. Nothing in it contradicts the codebase. No specific commands or paths that could go stale.

---

### docs/runbooks/worktree-flow.md
**Status**: ACCURATE

References `scripts/verify-and-merge.sh` which exists. The workflow steps are accurate. One note: the script may push to `origin`, but the project uses `upstream` as the remote per MEMORY.md. This is not technically wrong in the doc (the doc says "when `origin` is configured") but could be confusing.

---

### docs/runbooks/documentation-maintenance.md
**Status**: ACCURATE

Generic process doc. Nothing code-specific to go stale.

---

### docs/runbooks/observational-memory.md
**Status**: ACCURATE

All environment variables listed match the actual code in `cmd/harnessd/main.go` (lines 204-215). The defaults match: `HARNESS_MEMORY_MODE=auto`, `HARNESS_MEMORY_DB_DRIVER=sqlite`, `HARNESS_MEMORY_SQLITE_PATH=.harness/state.db`, `HARNESS_MEMORY_DEFAULT_ENABLED=false`, `HARNESS_MEMORY_LLM_MODEL=gpt-5-nano`, etc. Tool actions match `internal/harness/tools/observational_memory.go`. Event signals match `internal/harness/events.go`.

---

### docs/runbooks/terminal-bench-periodic-suite.md
**Status**: ACCURATE

References scripts and paths that exist. Environment variables are consistent with usage.

---

### docs/runbooks/tool-usability-testing.md
**Status**: ACCURATE

Framework and cron test suite are self-contained test documentation. The tool descriptions and source file references match the codebase. `internal/harness/tools/cron.go` and `internal/harness/tools/descriptions/cron_create.md` both exist.

---

### docs/runbooks/issue-triage.md
**Status**: ACCURATE

Generic process doc. No code-specific claims.

---

## Design Documents

### docs/design/event-catalog.md
**Status**: SIGNIFICANTLY OUTDATED

**Inaccuracies found**:

The catalog documents 23 event types across 9 categories. The actual `internal/harness/events.go` defines **57 event types** in `AllEventTypes()`. The catalog is missing **34 event types**:

**Missing run lifecycle events**:
- `run.cost_limit_reached`
- `run.step.started`
- `run.step.completed`

**Missing LLM turn events**:
- `assistant.thinking.delta`
- `reasoning.complete`

**Missing tool execution events**:
- `tool.activated` (deferred tool activated via find_tool)
- `tool.output.delta` (incremental output from running tool)

**Missing provider events**:
- `provider.resolved`

**Missing accounting events**:
- `cost.anomaly`

**Missing tool hook events** (separate from message hooks):
- `tool_hook.started`
- `tool_hook.failed`
- `tool_hook.completed`

**Missing callback events**:
- `callback.scheduled`
- `callback.fired`
- `callback.canceled`

**Missing skill events**:
- `skill.constraint.activated`
- `skill.constraint.deactivated`
- `tool.call.blocked`
- `skill.fork.started`
- `skill.fork.completed`
- `skill.fork.failed`

**Missing meta-message events**:
- `meta.message.injected`

**Missing steering events**:
- `steering.received`

**Missing auto-compaction events**:
- `auto_compact.started`
- `auto_compact.completed`

**Missing context management events**:
- `compact_history.completed`
- `context.reset`
- `context.window.snapshot`
- `context.window.warning`

**Missing forensics events**:
- `tool.decision`
- `tool.antipattern`
- `tool.hook.mutation`
- `llm.request.snapshot`
- `llm.response.meta`
- `error.context`
- `audit.action`
- `causal.graph.snapshot`

**Missing retry events**:
- `llm.empty_response.retry`

**Missing dynamic rule events**:
- `rule.injected`

**Missing recorder events**:
- `recorder.drop_detected`

**What to fix**: Major update needed. Add all 34 missing event types with their payload schemas. Consider reorganizing into the categories already visible in `events.go`.

---

### docs/design/system-prompt-architecture.md
**Status**: ACCURATE

The folder layout, composition order, runtime context fields, request surface, and event descriptions all match the current implementation. The `RunRequest` fields match `cmd/harnesscli/main.go` and the prompt engine code. Cost tracking and pricing sections are accurate.

---

### docs/design/tool-roadmap.md
**Status**: ACCURATE

All tools listed as `implemented` exist in the codebase. The catalog.go reference is correct. The tool naming and status descriptions are accurate. No tools are listed that do not exist, and no major implemented tools are missing.

---

### docs/design/plugins.md
**Status**: ACCURATE

The hook model (PreMessageHook, PostMessageHook, PreToolUseHook, PostToolUseHook) matches the code. The conclusion-watcher plugin reference is accurate -- `plugins/conclusion-watcher/` exists with the documented files. The config integration pattern matches `internal/config/config.go`. The wiring example from `cmd/harnessd/main.go` matches the actual code (lines 549-566). Import discipline constraints are accurate.

---

### docs/design/observational-memory-architecture.md
**Status**: ACCURATE

Component file paths, scope keys, modes, environment variables, and data model all match the implementation. The `internal/observationalmemory/` package structure is confirmed. The v1 constraints about remote coordinator and postgres are still accurate.

---

### docs/design/design-notes.md
**Status**: ACCURATE (empty template)

Just a template with no content to go stale.

---

### docs/design/ux-requirements.md
**Status**: ACCURATE (baseline only)

Contains only a baseline requirement (UX-000). Nothing to contradict.

---

## Testing Documents

### docs/testing/manual-curl-smoke-test-v3-20260313.md
**Status**: ACCURATE (historical record)

This is a test run log from 2026-03-13. The endpoints, behaviors, and responses described match the current server implementation. The state machine, error handling, and API surface are all consistent with the current codebase. Listed as historical rather than prescriptive.

---

## Context Documents

### docs/context/critical-context.md
**Status**: ACCURATE

Generic working model and contributor guidance. Nothing code-specific.

---

## Priority Fixes

### High Priority (active reference docs with factual errors)

1. **docs/design/event-catalog.md** -- Missing 34 of 57 event types. This is the primary reference for SSE events and is severely incomplete.

2. **docs/runbooks/mcp.md** -- HTTP MCP server tool table lists wrong tools (`wait_for_run`, `continue_run` instead of `steer_run`, `submit_user_input`).

### Medium Priority (outdated but not misleading)

3. **docs/runbooks/harnesscli-live-testing.md** -- Missing `auth` subcommand documentation.

4. **docs/runbooks/testing.md** -- `go test ./...` should be `go test ./internal/... ./cmd/...`.

### Low Priority (cosmetic or historical)

5. **docs/runbooks/worktree-flow.md** -- Consider noting that the remote is `upstream` not `origin`.
