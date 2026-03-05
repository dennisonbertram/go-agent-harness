# Observational Log

Use this file for observations about system behavior without immediately prescribing code changes.

## 2026-03-04

- Baseline observation: repository initialized with no implementation code yet.
- Harness observation: a run started through `POST /v1/runs` can be consumed via SSE from `GET /v1/runs/{runID}/events` even if the subscriber attaches after initial events, because event history is replayed before live streaming.
- Tool safety observation: default file tools reject workspace-escape paths and the test runner tool bounds execution with a timeout.
- Toolset observation: replacing tools with `read/write/edit/bash` preserved harness loop behavior and SSE outputs; only tool-call semantics changed.
- Bash observation: deny-list command guardrails reject clearly dangerous inputs (for example `rm -rf /`) while still allowing bounded command execution in workspace context.
- Coverage observation: after adding targeted tests for entrypoint, runner failure paths, and HTTP error handlers, all functions now show non-zero execution coverage in `go tool cover -func`.
- Regression observation: automated regression script now catches both total coverage drops and per-function `0.0%` coverage regressions before merge.
- CI observation: regression workflow is runnable in GitHub Actions without extra repository-specific setup beyond Go toolchain availability.
- Hook observation: hook events are emitted around LLM turns and can be consumed by clients for pre/post policy visibility (`hook.started`, `hook.completed`, `hook.failed`).
- Baseline tools observation: `ls`, `glob`, `grep`, `apply_patch`, `git_status`, and `git_diff` are callable through the same tool loop and appear with full lifecycle events.
- Live-run observation: model-driven `apply_patch` replaced the first matching occurrence in the file (title) when `find` was broad, demonstrating deterministic but occurrence-sensitive patch behavior.
- CLI observation: the new `harnesscli` client can attach to the existing SSE API and reliably terminate on `run.completed`/`run.failed` without hanging, making it a practical test harness for manual integration checks.

## Entry Template

- Date:
- Environment/context:
- Observation:
- Evidence:
- Hypothesis:
- Suggested follow-up:
- Modular-tooling observation: moving tools into `internal/harness/tools/` preserved registry-driven execution semantics while making per-tool changes isolated and easier to test.
- Policy observation: `permissions` mode cleanly blocks mutating/fetch/execute actions with structured `permission_denied`/`permission_error` payloads, while `full_auto` remains fast-path default.
- Live schema observation: OpenAI tool schema validation rejects array properties without `items`; adding explicit `items` on `apply_patch.edits` and `todos.todos` resolved request-time failures.
- Live-run observation: after schema fix, a tmux-hosted `gpt-5-nano` run completed successfully and exercised new `read` pagination/line metadata in event stream outputs.
- AskUserQuestion observation: a run now exposes a deterministic paused state (`waiting_for_user`) with explicit `run.waiting_for_user` and `run.resumed` events, enabling frontend clients to render input prompts without polling ambiguous tool state.
- Broker observation: invalid answer submissions no longer break run execution; they return `400` while preserving pending question state until a valid submission arrives.
- Timeout observation: when no answer is submitted before `HARNESS_ASK_USER_TIMEOUT_SECONDS`, the run fails immediately after the AskUserQuestion tool call with a timeout error, preventing indefinite stalled runs.
