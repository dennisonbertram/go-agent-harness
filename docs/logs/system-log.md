# System Log

Use this file to document systems, interfaces, and interactions as they are built.

## 2026-03-04

- System state: foundational workflow and documentation system only.
- Notable interfaces:
  - `AGENTS.md` defines operational policy.
  - `docs/runbooks/*` define execution playbooks.
  - `scripts/verify-and-merge.sh` operationalizes test-gated merges.

## 2026-03-04 (OpenAI Harness POC)

- System/component: `cmd/harnessd` + `internal/harness` + `internal/provider/openai` + `internal/server`.
- Responsibilities:
  - Accept run requests and execute deterministic LLM/tool loop.
  - Expose run status and event stream for external clients (GUI/TUI).
  - Execute bounded workspace tools for coding-oriented actions.
- Inputs/outputs:
  - Input: HTTP JSON request (`POST /v1/runs`), OpenAI API responses, tool arguments.
  - Output: run state (`GET /v1/runs/{runID}`), SSE lifecycle events (`/events`), tool result envelopes back to model.
- Dependencies:
  - OpenAI API (`/v1/chat/completions`) via `OPENAI_API_KEY`.
  - Local Go toolchain for `run_go_test`.
- Failure modes:
  - Provider request failures or malformed model outputs result in `run.failed`.
  - Unknown tool/tool argument errors are returned as tool-output error payloads to continue loop.
  - Slow SSE clients may miss live events but can retrieve persisted event history for the run.
- Operational notes:
  - Runtime state is in-memory only.
  - `HARNESS_MAX_STEPS` bounds loop depth.
  - Tool execution is bounded and event-emitting per run step.

## 2026-03-04 (Toolset Interface Revision)

- System/component: `internal/harness/tools_default.go`.
- Responsibilities:
  - Provide standardized coding tool interface: `read`, `write`, `edit`, `bash`.
  - Enforce workspace path boundaries for file operations.
  - Execute bounded shell commands for command-line workflows.
- Inputs/outputs:
  - Input: structured JSON arguments from model tool calls.
  - Output: JSON result envelopes (`content`, `bytes_written`, `replacements`, `exit_code`, etc.).
- Dependencies:
  - Local filesystem permissions.
  - `/bin/bash` availability for `bash` tool execution.
- Failure modes:
  - `edit` fails when `old_text` cannot be matched.
  - `bash` rejects commands matching danger deny-list patterns.
  - Path traversal attempts fail before filesystem access.
- Operational notes:
  - `bash` command execution remains timeout-bounded and workspace-rooted.
  - Deny-list guardrails are heuristic and should be reviewed before production exposure.

## 2026-03-04 (Entrypoint Testability and Coverage)

- System/component: `cmd/harnessd/main.go` testability boundary.
- Responsibilities:
  - Keep `main` as process entrypoint while allowing deterministic tests for startup/exit behavior.
  - Preserve server startup/shutdown behavior with signal-driven termination.
- Inputs/outputs:
  - Input: environment variables + signal channel.
  - Output: process exit behavior in `main`, error returns from `run`/`runWithSignals`.
- Dependencies:
  - OpenAI provider construction callback.
  - HTTP server lifecycle (`ListenAndServe`, `Shutdown`).
- Failure modes:
  - Missing API key/provider construction failure now return explicit errors through `runWithSignals`.
  - Server startup fatal errors surface through returned error channel.
- Operational notes:
  - Added lightweight test hooks (`runMain`, `exitFunc`, `runWithSignalsFunc`) to isolate process-level behavior in unit tests.

## 2026-03-05 (Regression Quality Gate System)

- System/component: `scripts/test-regression.sh` + `cmd/coveragegate` + `internal/quality/coveragegate`.
- Responsibilities:
  - Execute standard regression suite locally and in CI.
  - Enforce minimum total statement coverage threshold.
  - Enforce non-zero function coverage across codebase.
- Inputs/outputs:
  - Input: coverage profile (`coverage.out`), configured minimum threshold (`MIN_TOTAL_COVERAGE`).
  - Output: pass/fail exit code and gate summary (`PASS` with total and zero-function count).
- Dependencies:
  - Go toolchain (`go test`, `go tool cover`).
  - GitHub Actions runner for CI execution.
- Failure modes:
  - Missing/invalid coverage profile fails gate.
  - Any function at `0.0%` fails gate.
  - Total coverage below threshold fails gate.
- Operational notes:
  - Default threshold is `80.0%`, configurable via environment variable.
  - Workflow file: `.github/workflows/test-regression.yml`.

## 2026-03-05 (Hook Pipeline + Tool Surface Expansion)

- System/component: `internal/harness/runner.go` hook pipeline and `internal/harness/tools_default.go` baseline tools.
- Responsibilities:
  - Execute hook chain before and after each provider turn.
  - Allow hook-driven request/response mutation or blocking.
  - Emit hook lifecycle events for UI/TUI observability.
  - Provide repository-oriented baseline tools for traversal, search, patching, and git inspection.
- Inputs/outputs:
  - Input: hook implementations in `RunnerConfig`, model tool-call arguments.
  - Output: updated requests/responses, run failures on blocked/error hooks (depending on mode), tool JSON outputs.
- Dependencies:
  - Local filesystem and git binary availability for `git_status`/`git_diff`.
  - Provider call loop in runner execution.
- Failure modes:
  - Hook fail-closed mode converts hook errors into `run.failed`.
  - Hook fail-open mode emits `hook.failed` and continues run.
  - Tool validation errors are returned as tool error payloads and surfaced in `tool.call.completed`.
- Operational notes:
  - Hook failure mode defaults to `fail_closed`.
  - Baseline tool names now include:
    - `ls`, `glob`, `grep`, `apply_patch`, `git_status`, `git_diff`
    - plus `read`, `write`, `edit`, `bash`.

## 2026-03-05 (CLI Test Client)

- System/component: `cmd/harnesscli`.
- Responsibilities:
  - Provide a minimal operator-facing CLI to test the harness API without manual `curl` orchestration.
  - Start a run and stream run events until terminal completion/failure.
- Inputs/outputs:
  - Input: command flags (`-base-url`, `-prompt`, `-model`, `-system-prompt`).
  - Output: run id and line-by-line event stream in terminal, plus terminal event summary.
- Dependencies:
  - Harness HTTP API endpoints (`POST /v1/runs`, `GET /v1/runs/{id}/events`).
  - JSON SSE event payload format from server.
- Failure modes:
  - Non-2xx create/stream responses return non-zero exit with API error context.
  - Invalid SSE `data` payload returns non-zero exit (`invalid sse data`).
  - Missing prompt returns immediate validation error.
- Operational notes:
  - Stream reader handles framed SSE blocks and stops explicitly on `run.completed` or `run.failed`.

## Entry Template

- Date:
- System/component:
- Responsibilities:
- Inputs/outputs:
- Dependencies:
- Failure modes:
- Operational notes:

## 2026-03-05 (Modular Tool Registry + Approval Modes)

- System/component: `internal/harness/tools` modular tool subsystem + compatibility wrapper in `internal/harness/tools_default.go`.
- Responsibilities:
  - Provide a catalog-based, pluggable tool registration flow.
  - Isolate each tool into its own implementation unit.
  - Apply approval policy middleware (`full_auto` or `permissions`) at tool handler boundary.
- Inputs/outputs:
  - Input: `BuildOptions` (workspace root, approval mode, integrations, HTTP client, sourcegraph config).
  - Output: sorted tool catalog with wrapped handlers and JSON result envelopes.
- Dependencies:
  - Optional external integrations for LSP (`gopls`), Sourcegraph HTTP endpoint/token, MCP registry, agent runner, and web fetcher.
- Failure modes:
  - In `permissions` mode, mutating/fetch/execute actions emit structured denial payloads when policy denies or errors.
  - Missing external dependencies produce deterministic runtime errors from the affected tool handlers.
  - Invalid tool JSON schema (for arrays without `items`) causes provider-side request rejection; fixed for current arrays.
- Operational notes:
  - Default server mode remains `full_auto` via `HARNESS_TOOL_APPROVAL_MODE` default.
  - Run-scoped context key (`run_id`) is now injected for tool execution to support run-local state (`todos`).

## 2026-03-05 (AskUserQuestion Pause/Resume Interface)

- System/component: `internal/harness/tools/ask_user_question.go`, `internal/harness/ask_user_broker.go`, `internal/harness/runner.go`, `internal/server/http.go`.
- Responsibilities:
  - Allow model turns to issue structured user clarification requests through `AskUserQuestion`.
  - Pause a run in `waiting_for_user` state until answers are submitted.
  - Resume execution after valid answers or fail the run on timeout.
- Inputs/outputs:
  - Input: tool args `{questions:[...]}` and API submissions `{answers:{...}}`.
  - Output: tool result JSON `{questions:[...], answers:{...}}`, run state transitions, and wait/resume events.
- Dependencies:
  - In-memory `AskUserQuestionBroker` shared by runner and tool layer.
  - HTTP input endpoints (`GET/POST /v1/runs/{id}/input`) for user answer submission.
- Failure modes:
  - Invalid tool question shape returns tool-call error payload (run continues unless timeout path).
  - Invalid submitted answers return `400 invalid_request` and keep question pending.
  - Missing pending input returns `409 no_pending_input`.
  - Timeout returns typed error and transitions run to `run.failed`.
- Operational notes:
  - `HARNESS_ASK_USER_TIMEOUT_SECONDS` controls per-question wait timeout (default 300s).
  - Event stream now includes `run.waiting_for_user` and `run.resumed` for UI/CLI orchestration.

## 2026-03-05 (Observational Memory Subsystem)

- System/component: `internal/observationalmemory` + runner/tool integration.
- Responsibilities:
  - Persist optional observational memory by `(tenant_id, conversation_id, agent_id)` scope.
  - Inject bounded memory snippets into model turns when enabled.
  - Execute ordered per-scope memory mutations in local coordinator mode.
  - Expose operator/model control via `observational_memory` tool.
- Inputs/outputs:
  - Input: run transcript snapshots, tool actions (`enable|disable|status|export|review|reflect_now`), environment memory settings.
  - Output: memory records/operations/markers in DB, SSE memory lifecycle events, optional export files.
- Dependencies:
  - SQLite store in v1 (`modernc.org/sqlite`).
  - Existing provider for observer/reflector model calls (tools disabled).
- Failure modes:
  - Observer/reflector failures emit `memory.observe.failed` and preserve run continuity.
  - Misconfigured memory store startup fails harness boot with explicit error.
  - Postgres mode currently returns explicit not-implemented errors.
- Operational notes:
  - `HARNESS_MEMORY_MODE=off|auto|local_coordinator`.
  - `auto` resolves to local coordinator behavior in v1.
  - Transcript is exposed to tools as read-only snapshot through context interfaces.

## 2026-03-05 (System Prompt Composition Pipeline)

- System/component: `internal/systemprompt` + runner integration in `internal/harness/runner.go`.
- Responsibilities:
  - Resolve static prompt layers by intent/model/extensions at run creation.
  - Inject per-turn runtime context as ephemeral system message.
  - Emit prompt-resolution telemetry events for clients.
- Inputs/outputs:
  - Input: `RunRequest` prompt fields (`agent_intent`, `task_context`, `prompt_profile`, `prompt_extensions`) and `prompts/catalog.yaml` assets.
  - Output: provider-facing system messages and run events (`prompt.resolved`, `prompt.warning`).
- Dependencies:
  - YAML catalog parser (`gopkg.in/yaml.v3`).
  - Prompt asset files under `prompts/`.
- Failure modes:
  - Invalid prompt catalog/paths fail harness startup.
  - Unknown intent/profile/behavior/talent fails `POST /v1/runs` as `invalid_request`.
  - Reserved `skills` field is ignored with warning event.
- Operational notes:
  - `system_prompt` request field bypasses prompt engine completely.
- Runtime context includes `run_started_at_utc`, `current_time_utc`, `elapsed_seconds`, `step`, and phase-1 cost placeholder.
- New config vars: `HARNESS_PROMPTS_DIR`, `HARNESS_DEFAULT_AGENT_INTENT`.

## 2026-03-05 (Usage and Cost Accounting Pipeline)

- System/component: `internal/provider/openai`, `internal/provider/pricing`, `internal/harness/runner`, `internal/systemprompt/runtime_context`.
- Responsibilities:
  - Normalize per-turn provider usage into harness accounting fields.
  - Compute per-turn USD cost when pricing metadata/catalog is available.
  - Accumulate run-level usage/cost totals and expose them to APIs/events.
  - Inject live accounting fields into runtime context on every turn.
- Inputs/outputs:
  - Input: provider completion response usage fields, optional explicit provider cost fields, optional pricing catalog JSON.
  - Output:
    - `usage.delta` event per completion turn.
    - `run.completed` / `run.failed` payload totals (`usage_totals`, `cost_totals`).
    - `GET /v1/runs/{id}` totals in run state.
    - runtime context fields (`prompt_tokens_total`, `cost_usd_total`, etc.).
- Dependencies:
  - Optional env-configured pricing catalog path: `HARNESS_PRICING_CATALOG_PATH`.
  - OpenAI usage response schema (`prompt_tokens`, `completion_tokens`, details objects).
- Failure modes:
  - Missing usage from provider does not fail run; accounting defaults to zero with `provider_unreported`.
  - Missing model pricing does not fail run; cost remains zero with `unpriced_model`.
  - Invalid pricing catalog path/content fails startup with explicit load error.
- Operational notes:
  - No bundled default price table is required; pricing is opt-in via catalog path.
  - `CostUSD` remains populated for backward compatibility while richer cost structure is also exposed.
