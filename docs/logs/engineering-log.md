# Engineering Log

## 2026-03-04

- Initialized repository scaffold.
- Added operating policy (`AGENTS.md`) with strict TDD, worktree-first, and pre-commit testing requirements.
- Created docs structure with indexes, logs, context, plans, and runbooks.
- Added merge helper script: `scripts/verify-and-merge.sh`.
- Refactored `AGENTS.md` into a bootstrap reference map for faster onboarding.
- Added long-term thinking log (`docs/logs/long-term-thinking-log.md`) with command-intent and user-intent precedence.
- Added UX requirements doc (`docs/design/ux-requirements.md`).
- Added completed bootstrap plan/checklist (`docs/plans/2026-03-04-repo-bootstrap-plan.md`).
- Updated merge workflow to auto-push `main` in `scripts/verify-and-merge.sh`.
- Updated worktree runbook and AGENTS guidance to reflect process-guided enforcement (no hard gating yet).
- Added explicit response-clarity policy requiring `Task status: DONE` / `Task status: NOT DONE`.
- Updated agent completion and nightly-task docs to require status-first reporting.

## 2026-03-04 (OpenAI Harness POC)

- Added Go module and executable service entrypoint: `cmd/harnessd/main.go`.
- Implemented core harness runtime in `internal/harness/`:
  - Deterministic run loop with bounded steps.
  - Event history + live subscriber fanout.
  - In-memory run state with status/output/error tracking.
  - Tool registry with schema metadata and execution dispatch.
- Added default proof-of-concept tools:
  - `list_files` (workspace-scoped listing, recursive/non-recursive).
  - `read_file` (workspace-scoped reads with byte limit + truncation flag).
  - `run_go_test` (bounded timeout + restricted package pattern).
- Implemented OpenAI provider adapter in `internal/provider/openai/client.go` against `/v1/chat/completions` with function-tool schema mapping and tool-call parsing.
- Implemented HTTP server in `internal/server/http.go`:
  - `POST /v1/runs`
  - `GET /v1/runs/{runID}`
  - `GET /v1/runs/{runID}/events` (SSE)
  - `GET /healthz`
- Added tests first, then implemented to green:
  - `internal/harness/runner_test.go`
  - `internal/harness/tools_test.go`
  - `internal/provider/openai/client_test.go`
  - `internal/server/http_test.go`
- Updated README with setup, API contract, event taxonomy, and quick-start usage.

## 2026-03-04 (Toolset Update: read/write/edit/bash)

- Replaced default harness tool registrations in `internal/harness/tools_default.go`:
  - Removed `list_files`, `read_file`, `run_go_test`.
  - Added `read`, `write`, `edit`, `bash`.
- Implemented `write` with create/overwrite/append support and parent directory creation.
- Implemented `edit` with single/replace-all text replacement and explicit error when `old_text` is not found.
- Implemented `bash` command execution with timeout, workspace working directory confinement, and deny-list guardrails for dangerous commands.
- Rewrote `internal/harness/tools_test.go` with failing-first assertions for new tools and safety constraints.
- Ran full suite to confirm no behavior regressions outside toolset update.

## 2026-03-04 (Function Coverage Expansion)

- Added `cmd/harnessd/main_test.go` to cover entrypoint logic and env helpers:
  - `main` success/error exit behavior (via test hooks).
  - `run` delegation behavior.
  - `runWithSignals` missing key, provider failure, and graceful shutdown.
  - `getenvOrDefault` and `getenvIntOrDefault`.
- Refactored `cmd/harnessd/main.go` for testability while preserving runtime behavior:
  - Introduced `runMain`, `exitFunc`, and `runWithSignalsFunc` hooks.
  - Converted fatal exits in internal flow to returned errors handled in `main`.
- Expanded `internal/harness/runner_test.go` with failure-path coverage:
  - Provider error run failure path.
  - `failRun(nil)` default error path.
  - `mustJSON` marshal-failure fallback.
- Expanded `internal/server/http_test.go` with handler error/edge coverage:
  - `GET /healthz`.
  - method-not-allowed checks.
  - invalid JSON handling.
  - not-found run and event stream paths.
- Coverage verification:
  - `go test ./... -coverprofile=coverage.out`
  - `go tool cover -func=coverage.out`
  - Total statement coverage now `81.0%`.
  - All functions report non-zero coverage.

## 2026-03-05 (Regression Guardrails Automation)

- Added coverage-gate library and tests:
  - `internal/quality/coveragegate/gate.go`
  - `internal/quality/coveragegate/gate_test.go`
- Added coverage-gate CLI and tests:
  - `cmd/coveragegate/main.go`
  - `cmd/coveragegate/main_test.go`
- Added regression contract test for default tool interface:
  - `internal/harness/tools_contract_test.go` (asserts `bash`, `edit`, `read`, `write` contract).
- Added automated regression script:
  - `scripts/test-regression.sh`
  - Runs `go test`, `go test -race`, coverage profile generation, and coverage gate checks.
- Added CI workflow:
  - `.github/workflows/test-regression.yml`
  - Executes regression script on `pull_request` and `push` to `main`.
- Updated testing and worktree runbooks + README development commands to use regression script as default quality gate.
- Verified full regression suite passes locally with coverage gate result:
  - `coveragegate: PASS (total=81.1%, min=80.0%, zero-functions=0)`.

## 2026-03-05 (Hooks + Baseline Tools Expansion)

- Added hook contracts and runner integration in `internal/harness`:
  - New hook types/interfaces in `types.go` (`PreMessageHook`, `PostMessageHook`, `HookAction`, `HookFailureMode`).
  - Runner hook pipeline in `runner.go`:
    - Pre-message hooks executed before provider call.
    - Post-message hooks executed after provider call.
    - Hook events emitted: `hook.started`, `hook.completed`, `hook.failed`.
    - Blocking and mutation semantics with fail-open/fail-closed modes.
- Added hook-focused tests in `internal/harness/hooks_test.go`:
  - Mutation, blocking, fail-open, and fail-closed behavior for pre and post hooks.
- Expanded default toolset in `internal/harness/tools_default.go`:
  - Added baseline tools:
    - `ls`
    - `glob`
    - `grep`
    - `apply_patch`
    - `git_status`
    - `git_diff`
  - Kept existing tools:
    - `read`, `write`, `edit`, `bash`
- Expanded tool tests in `internal/harness/tools_test.go`:
  - New baseline tool behavior and validation/error branches.
  - Additional branch coverage for helper functions and command execution paths.
- Updated default tool contract test in `internal/harness/tools_contract_test.go`.
- Updated README to document hooks and expanded tool list.
- Validation:
  - `go test ./...` passed.
  - `./scripts/test-regression.sh` passed.
  - Coverage gate after changes: `PASS (total=80.8%, min=80.0%, zero-functions=0)`.
- Live OpenAI verification (local key, `gpt-5-nano`, tmux-hosted harness):
  - Confirmed successful run with `run.completed`.
  - Observed tool calls for `ls`, `write`, `apply_patch`, `grep`, `git_status`, `git_diff` in event stream.

## 2026-03-05 (Sample CLI Test Client)

- Added a new CLI client in `cmd/harnesscli/main.go` to test harness connectivity quickly from terminal.
- Implemented CLI flow:
  - Parse flags (`-base-url`, `-prompt`, `-model`, `-system-prompt`).
  - Create run via `POST /v1/runs`.
  - Stream and print lifecycle events from `GET /v1/runs/{id}/events`.
  - Stop on terminal events (`run.completed`, `run.failed`) with explicit terminal summary output.
- Added full TDD coverage in `cmd/harnesscli/main_test.go`:
  - `main` exit delegation.
  - Create-run payload contract validation.
  - SSE block parsing + event decode + terminal detection.
  - End-to-end CLI success path.
  - Non-2xx create/stream regression paths.
  - Invalid SSE data handling path.
- Validation:
  - `go test ./cmd/harnesscli`
  - `go test ./...`
  - `./scripts/test-regression.sh` (pass, coverage gate pass)
- Live OpenAI verification (local key, `gpt-5-nano`, tmux-hosted harness):
  - Ran CLI end-to-end with prompt to create `demo/live-cli-smoke.html`.
  - Observed real `bash`, `write`, and `ls` tool calls in stream.
  - Completed with `terminal_event=run.completed`.
- Added operator documentation:
  - `docs/runbooks/harnesscli-live-testing.md`
  - Includes tmux commands, variable map, expected outputs, known live-run issues, and troubleshooting.

## Entry Template

- Date:
- Task:
- Change summary:
- Tests added/updated:
- Bugs fixed:
- Regression tests added:
- Docs updated:

## 2026-03-05 (Modular Tooling Migration + Crush-Informed Expansion)

- Refactored tool implementation into modular package: `internal/harness/tools/`.
  - Added catalog-driven registration (`catalog.go`) and common shared utilities (`common_paths.go`, `common_exec.go`, `common_result.go`, `policy.go`).
  - Migrated and modularized existing tools (`read`, `write`, `edit`, `bash`, `ls`, `glob`, `grep`, `apply_patch`, `git_status`, `git_diff`).
- Added Phase 1/2/3 tool contracts and implementations with dependency-gated registration:
  - `job_output`, `job_kill`
  - `fetch`, `download`
  - `todos`
  - `lsp_diagnostics`, `lsp_references`, `lsp_restart`
  - `sourcegraph` (registered when endpoint configured)
  - `list_mcp_resources`, `read_mcp_resource`, dynamic `mcp_<server>_<tool>` (registered when MCP registry provided)
  - `agent`, `agentic_fetch`, `web_search`, `web_fetch` (registered when integrations provided)
- Added approval-mode seam and compatibility wiring:
  - New harness types for `ToolApprovalMode`, `ToolPolicy`, policy input/output.
  - Added `HARNESS_TOOL_APPROVAL_MODE` env wiring in `cmd/harnessd/main.go`.
  - Added `NewDefaultRegistryWithPolicy(...)` while preserving `NewDefaultRegistry(...)` compatibility.
- Updated runner tool execution context to include run ID for run-scoped tools (used by `todos`).
- Expanded test coverage heavily for modular package and compatibility wrappers:
  - `internal/harness/tools/catalog_test.go`
  - `internal/harness/tools/coverage_boost_test.go`
  - `internal/harness/tools/coverage_extra_test.go`
  - `internal/harness/tools_default_test.go`
  - Updated `internal/harness/tools_contract_test.go` expected tool surface.
  - Updated `cmd/harnessd/main_test.go` for approval-mode env parser.
- Fixed live OpenAI schema issue discovered during tmux smoke test:
  - Added explicit `items` schemas for array properties in `apply_patch.edits` and `todos.todos`.
- Validation:
  - `go test ./...` passed.
  - `./scripts/test-regression.sh` passed.
  - Coverage gate after migration: `PASS (total=80.0%, min=80.0%, zero-functions=0)`.
- Live OpenAI verification (tmux-hosted harness + `gpt-5-nano`):
  - Confirmed `run.completed` with real tool usage (`ls`, `write`, `read`) and generated file verification.

## 2026-03-05 (Claude-Compatible AskUserQuestion Tool)

- Added a new first-class `AskUserQuestion` tool in `internal/harness/tools/ask_user_question.go` with Claude-compatible schema and result payload (`questions` + `answers`).
- Added tool-side validation and answer normalization helpers:
  - 1-4 questions, 2-4 options per question.
  - required `question/header/options/multiSelect` fields.
  - unique question text and option labels.
  - multi-select answer normalization to comma-separated labels.
- Added broker interfaces and context helpers in `internal/harness/tools/types.go`:
  - `AskUserQuestionBroker`, `AskUserQuestionRequest`, `AskUserQuestionPending`.
  - `ContextKeyToolCallID` / `ToolCallIDFromContext`.
- Added in-memory broker implementation in `internal/harness/ask_user_broker.go`:
  - one pending question per run.
  - blocking wait in `Ask`.
  - typed timeout error path.
  - submission validation with invalid-input preservation.
- Updated tool catalog/default registry wiring:
  - `AskUserQuestion` now registers in default toolset.
  - new registry options support broker + timeout injection.
- Updated runner behavior:
  - new status `waiting_for_user`.
  - emits `run.waiting_for_user` and `run.resumed` events.
  - fails run immediately on typed AskUserQuestion timeout.
  - adds tool call id into tool execution context.
  - new runner methods for input API: `PendingInput` and `SubmitInput`.
- Updated HTTP server API in `internal/server/http.go`:
  - `GET /v1/runs/{runID}/input`
  - `POST /v1/runs/{runID}/input`
  - error contracts: `404` missing run, `409` no pending input, `400` invalid JSON/request.
- Updated runtime wiring in `cmd/harnessd/main.go`:
  - new env var `HARNESS_ASK_USER_TIMEOUT_SECONDS` (default `300`).
  - shared in-memory broker injected into both registry and runner.
- Added/updated tests:
  - `internal/harness/tools/ask_user_question_test.go`
  - `internal/harness/ask_user_broker_test.go`
  - `internal/harness/runner_test.go` (wait/resume and timeout paths)
  - `internal/server/http_test.go` (input endpoint lifecycle and error semantics)
  - `internal/harness/tools/catalog_test.go` and `internal/harness/tools_contract_test.go` (tool contract update)
  - `cmd/harnessd/main_test.go` (ask-user timeout env parsing)
