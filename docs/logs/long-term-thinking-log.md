# Long-Term Thinking Log

Purpose: keep durable intent and success criteria visible so agents can make good decisions without re-discovery.

Decision rule: when uncertain, default to `command intent` and `user intent` below.

## Entry Template

- Date:
- Command intent:
- User intent:
- Success definition:
- Non-goals:
- Guardrails/constraints:
- Open questions:
- Next verification step:

## 2026-03-04

- Command intent: Set up a new git repository with a strong documentation system, strict TDD workflow, worktree-based delivery, test-gated merge discipline, and operational runbooks.
- User intent: Make the project easy for multiple agents to understand and execute quickly, while keeping technical rigor without over-engineering beyond MVP needs.
- Success definition:
  - Repo initialized on `main`.
  - Documentation folders and indexes exist.
  - Engineering, observational, and system logs exist.
  - Plans/checklist workflow exists and is required.
  - UX requirements and nightly task guidance exist.
  - Agent policy points to these documents and explains intent precedence.
- Non-goals:
  - Full enterprise process stack.
  - Premature scaling optimization.
- Guardrails/constraints:
  - Security best practices remain mandatory.
  - Tests must be meaningful and run before commit.
  - Bugs must produce regression tests and issue tracking.
- Open questions:
  - Final CI/test tooling conventions once implementation code exists.
  - Deployment target/platform details.
- Next verification step: Validate all indexes and cross-references after each new documentation file is added.

## 2026-03-04 (Workflow Adjustment)

- Command intent: Keep the workflow lightweight and practical for early-stage execution, with automatic merge/push to `main`.
- User intent: Reduce operational friction from branch tracking while retaining test-first discipline and clear docs.
- Success definition:
  - Merge helper script auto-pushes `main` on success.
  - No hard enforcement gates are introduced yet.
  - Process expectations remain clear in docs.
- Non-goals:
  - Hook/CI enforcement during early-stage setup.
- Guardrails/constraints:
  - Continue strict TDD and meaningful test requirements.
  - Keep regression-test + issue + logging discipline for bugs.
- Open questions:
  - When to transition from process-guided to hard-gated enforcement.
- Next verification step: Revisit enforcement level once contributor volume and deployment risk increase.

## 2026-03-04 (OpenAI Harness POC)

- Command intent: Design and implement a proof-of-concept Go coding harness powered by OpenAI as a service/server that emits events for easy GUI/TUI integration.
- User intent: Validate the architecture quickly with a minimal but real tool-calling runtime and a streamable event surface.
- Success definition:
  - Runnable Go server exists with API endpoints for run creation, status lookup, and event streaming.
  - Harness loop calls OpenAI and executes a small coding-oriented toolset.
  - Event stream exposes lifecycle/tool/assistant events suitable for client rendering.
  - Tests cover harness loop behavior, tool behavior, and HTTP/SSE behavior.
- Non-goals:
  - Durable persistence across process restarts.
  - Production-hardening of permissions, authn/authz, and multi-tenant isolation.
- Guardrails/constraints:
  - Keep implementation scope small and deterministic.
  - Preserve workspace boundaries for file tools.
  - Enforce bounded execution (`max_steps`, tool command timeout).
- Open questions:
  - Should future iterations expose token-level streaming deltas from provider responses?
  - Should run queueing/cancellation become session-aware in v2?
- Next verification step: Run an end-to-end manual check with a live API key (`POST /v1/runs` + `GET /v1/runs/{id}/events`) and confirm event consumption in a prototype client.

## 2026-03-04 (Toolset Rename and Capability Adjustment)

- Command intent: Update harness tools to include `read`, `write`, `edit`, and `bash`.
- User intent: Make the coding harness expose a more practical editing and shell-command interface for interactive clients.
- Success definition:
  - Default registry only exposes requested tool names.
  - File tools remain workspace-scoped and reject traversal attempts.
  - `edit` provides deterministic text replacement behavior.
  - `bash` executes commands with timeout and basic safety rejection.
  - Tests validate new toolset behavior.
- Non-goals:
  - Full sandboxing/authorization model for arbitrary shell execution.
  - Advanced patch semantics beyond exact text replacement.
- Guardrails/constraints:
  - Keep command execution bounded by timeout.
  - Prevent obvious dangerous shell patterns.
  - Preserve existing run loop and SSE API.
- Open questions:
  - Should `bash` evolve to an allow-list instead of a deny-list?
  - Should `edit` support multi-hunk line-range operations in a future revision?
- Next verification step: Execute a live run that uses all four tools and confirm client-side event rendering with final file state validation.

## 2026-03-04 (All Functions Tested Request)

- Command intent: Test all functions in the current harness codebase.
- User intent: Increase confidence that each function has at least one executed test path.
- Success definition:
  - Every function in `go tool cover -func` reports non-zero coverage.
  - Tests include entrypoint/runtime failure paths and HTTP error handlers, not only happy paths.
- Non-goals:
  - 100% statement/branch coverage.
  - Live external integration tests.
- Guardrails/constraints:
  - Keep runtime semantics unchanged while enabling testability.
  - Avoid introducing behavior-only-for-tests beyond lightweight hook points.
- Open questions:
  - Whether to enforce minimum package-level statement coverage thresholds in CI.
- Next verification step: Decide CI coverage gate policy (for example minimum total + per-package thresholds) and wire into pipeline.

## 2026-03-05 (Regression Enforcement for Ongoing Development)

- Command intent: Ensure complete testing and regression protection as the harness grows.
- User intent: Prevent future feature additions from reducing test confidence.
- Success definition:
  - Single regression script runs core tests + race checks + coverage gates.
  - CI workflow executes same regression script for PRs/pushes.
  - Gate fails on low total coverage and on any function with `0.0%` coverage.
  - Default tool contract has explicit regression test.
- Non-goals:
  - External integration test coverage of third-party systems.
  - Branch protection policy administration.
- Guardrails/constraints:
  - Keep thresholds configurable while default is strict enough to catch regressions.
  - Ensure local and CI use the exact same gate command.
- Open questions:
  - Whether to add per-package minimum coverage thresholds in addition to total threshold.
- Next verification step: Observe CI behavior across next few PRs and tune `MIN_TOTAL_COVERAGE` only if signal/noise ratio is poor.

## 2026-03-05 (Hooks and Baseline Tooling Completion)

- Command intent: Implement pre/post message hook support and add baseline tools (`ls`, `glob`, `grep`, `apply_patch`, `git_status`, `git_diff`) with full TDD and live OpenAI verification.
- User intent: Make the harness extensible around message flow and practical for basic coding/repo tasks with strong regression discipline.
- Success definition:
  - Hook pipeline integrated in runner with event emissions and tested blocking/mutation/error modes.
  - Baseline tools added in harness registry and covered by tests.
  - Regression suite remains green under enforced coverage gate.
  - Live `gpt-5-nano` task succeeds with `run.completed` and real tool usage.
- Non-goals:
  - Production-grade sandbox policy engine for all shell/file operations.
  - Persistent storage for hook execution audit beyond event stream history.
- Guardrails/constraints:
  - Keep run loop deterministic and bounded by `HARNESS_MAX_STEPS`.
  - Maintain workspace boundary checks for path-based tools.
  - Preserve threshold-based regression gating.
- Open questions:
  - Whether `apply_patch` should support targeted nth-occurrence/hunk semantics to reduce accidental first-match replacements.
  - Whether to add hook registration via HTTP API instead of code-level config only.
- Next verification step: Add a focused follow-up for richer patch targeting semantics and optional per-tool policy hooks.

## 2026-03-05 (Sample CLI Test Harness)

- Command intent: Build a small CLI test tool that connects to the harness service and validates run/event behavior quickly.
- User intent: Have an easy way to test the server from terminal and use it for real live smoke tasks.
- Success definition:
  - CLI creates runs through `POST /v1/runs`.
  - CLI streams events through `GET /v1/runs/{id}/events` and exits on terminal events.
  - Unit tests cover payload contract, SSE parsing, success path, and error paths.
  - Full regression suite remains green.
  - Live OpenAI-backed run succeeds with real tool usage.
- Non-goals:
  - Interactive shell/TUI behavior.
  - Persisted local history in the CLI.
- Guardrails/constraints:
  - Keep implementation minimal and deterministic.
  - Reuse current API contracts without introducing server-side changes.
  - Maintain regression gates and coverage threshold.
- Open questions:
  - Whether to add `--run-id` attach mode for streaming existing runs started by another client.
  - Whether to support JSONL/raw-output mode for easier machine parsing.
- Next verification step: Evaluate whether GUI/TUI prototypes should consume CLI output directly or connect to SSE endpoint natively.

## 2026-03-05 (Incremental Modular Tooling Implementation)

- Command intent: Implement the full incremental migration plan to modular, crush-informed tooling with strict TDD and regression gates.
- User intent: Make tools cleanly organized so adding a new tool is low-friction, while expanding tool coverage and preserving quality.
- Success definition:
  - Tool logic moved into `internal/harness/tools/` with catalog-driven registration.
  - Default harness registry remains backward-compatible while exposing expanded tool surface.
  - Approval mode seam exists with `full_auto` default and strict `permissions` behavior available.
  - Regression suite and coverage gate remain passing after migration.
  - Live OpenAI smoke run succeeds with new modular stack.
- Non-goals:
  - UI-driven permission prompts in this iteration.
  - Production-hardened external integration backends for every optional tool.
- Guardrails/constraints:
  - Keep tool contracts deterministic and JSON-schema compatible with OpenAI function calling.
  - Maintain no-zero-function-coverage enforcement.
  - Keep unsupported integrations dependency-gated instead of silently stubbed in runtime.
- Open questions:
  - Whether to default-enable optional external integrations when adapters become available at runtime.
  - Whether to evolve `permissions` mode from policy hook to interactive approval broker in a future iteration.
- Next verification step: add one integration test pack for real MCP adapter wiring and strict-mode policy behavior under active harness runs.

## 2026-03-05 (AskUserQuestion Interactive Clarification Flow)

- Command intent: Implement Claude-compatible `AskUserQuestion` behavior with full server/runner support, strict TDD coverage, and documented operational contracts.
- User intent: Allow upstream clients to drive structured user clarification prompts mid-run and resume safely, without ad hoc protocol handling.
- Success definition:
  - `AskUserQuestion` tool is available in default registry with compatible question/answer schema.
  - Runner supports `waiting_for_user` status and emits explicit wait/resume events.
  - Input API endpoints exist for fetching pending prompts and submitting answers.
  - Timeout is configurable and enforced with deterministic run failure.
  - Tests cover tool validation, broker lifecycle, runner transitions, and HTTP error semantics.
- Non-goals:
  - Interactive CLI prompt UX in this iteration.
  - Persistent pending-question storage across process restarts.
- Guardrails/constraints:
  - Keep structured JSON contracts deterministic for client UI builders.
  - Preserve existing run/event semantics outside the new waiting-input flow.
  - Maintain regression gate discipline and non-zero function coverage constraints.
- Open questions:
  - Whether to add CLI interactive answer collection behind a flag in a follow-up iteration.
- Next verification step: Run full regression gate (`go test`, `go test -race`, `./scripts/test-regression.sh`) and verify event payload shapes in a live harness session.

## 2026-03-05 (Provider Token Streaming)

- Command intent: Check the tracked streaming issues and implement token-by-token model streaming through the harness event surface.
- User intent: Allow clients to render assistant output progressively instead of waiting for a whole provider turn to complete.
- Success definition:
  - Runner accepts incremental provider deltas and emits SSE-visible assistant/tool-call delta events.
  - OpenAI provider uses streaming chat completions and assembles final content/tool calls correctly.
  - Existing turn completion, tool execution, usage accounting, and final assistant message behavior remain intact.
  - Tests cover streamed text, streamed tool-call assembly, and runner event emission order.
- Non-goals:
  - Streaming stdout/stderr from long-running tools.
  - Reworking client UX beyond exposing events.
- Guardrails/constraints:
  - Keep existing REST endpoints unchanged.
  - Do not execute tools until streamed tool-call arguments are fully assembled.
  - Maintain deterministic final run state and regression gate coverage.
- Open questions:
  - Whether to expose separate event types for tool-call creation vs argument deltas in a later iteration.
- Next verification step: Run provider and runner tests, then full regression suite to confirm new streaming events do not break existing clients.

## 2026-03-05 (Optional Observational Memory, Local-First with Scale Path)

- Command intent: Implement observational memory with local standalone viability first, while keeping architecture migration-safe for many-agent and future production deployment.
- User intent: Avoid premature optimization, but build with explicit interfaces, logs, and docs so scaling to many/thousands of agents is a planned expansion rather than a rewrite.
- Success definition:
  - Memory is optional and tool-controlled per scope.
  - Local sqlite + in-process ordered writes work end-to-end.
  - Runner can inject memory snippets and observe transcript deltas.
  - Documentation and logs clearly describe current behavior and scale path.
- Non-goals:
  - Remote coordinator transport implementation in this phase.
  - Full postgres runtime support in this phase.
- Guardrails/constraints:
  - Keep message transcript access read-only for tools.
  - Keep defaults safe (`memory disabled` unless enabled).
  - Preserve existing run loop behavior when memory is inactive.
- Open questions:
  - Remote coordinator wire protocol shape (HTTP vs queue) for multi-instance mode.
  - Postgres locking strategy and operational SLOs for high-write contention.
- Next verification step: Execute local run smoke coverage for `enable -> observe -> export -> review` and confirm event stream + sqlite state transitions.

## 2026-03-05 (System Prompt Modularity and Intent Routing)

- Command intent: Implement a clean modular system prompt architecture with intent-driven startup prompts, model-specific overlays, and runtime context injection.
- User intent: Make system prompt behavior easy to find, audit, and evolve while enabling harness-coordinated specialist agents (for example code review vs frontend design).
- Success definition:
  - Prompt system has its own module and file assets.
  - Run API supports intent/profile/extension fields.
  - Unknown prompt references fail early (`invalid_request`).
  - Runtime context is refreshed per turn without transcript bloat.
  - Prompt-resolution and warning events are visible in run streams.
- Non-goals:
  - Claude Skills runtime integration in this iteration.
  - Real usage/cost injection in this iteration.
- Guardrails/constraints:
  - Preserve `system_prompt` override semantics.
  - Keep startup deterministic and fail-fast on invalid prompt catalog.
  - Keep phase-1 cost reporting explicit (`unavailable_phase1`) rather than implicit estimates.
- Open questions:
  - Final phase-2 approach for provider usage/cost normalization across model providers.
  - Governance workflow for prompt extension additions and review ownership.
- Next verification step: Run full regression script and validate `prompt.resolved` / `prompt.warning` event payloads in an end-to-end live run.

## 2026-03-05 (Token Counting and Cost Tracking Design)

- Command intent: Think through and document a concrete approach to add token counting and cost tracking as a dedicated architecture subsection.
- User intent: Make phase-2 usage/cost work implementation-ready, auditable, and explicit rather than leaving high-level placeholder notes.
- Success definition:
  - Design doc contains a standalone token/cost subsection with data model, provider normalization, pricing strategy, runtime integration, and test coverage.
  - Runtime context replacement path for `cost_status: unavailable_phase1` is clearly defined.
  - Failure states (`estimated`, `unpriced_model`, `provider_unreported`) are explicit for clients/operators.
- Non-goals:
  - Implementing runtime code changes in this documentation update.
  - Finalizing provider pricing numbers in this pass.
- Guardrails/constraints:
  - Keep provider-reported usage as primary source when available.
  - Preserve deterministic run behavior when usage/cost data is unavailable.
  - Keep runtime context ephemeral and avoid transcript bloat.
- Open questions:
  - Canonical location and update policy for pricing catalog ownership.
  - Whether to expose detailed token classes in public API by default or behind optional fields.
- Next verification step: Implement usage normalization + pricing resolver with fixture-based tests, then validate end-to-end events and runtime context output in a live run.

## 2026-03-06 (Periodic Terminal Bench Harness Suite)

- Command intent: Create a Terminal Bench-based test suite that can periodically exercise the real harness end-to-end.
- User intent: Catch regressions that only show up when the harness performs actual terminal tasks, without depending only on unit tests or ad hoc live checks.
- Success definition:
  - Private Terminal Bench tasks exist in-repo and are stable enough for recurring runs.
  - A custom agent bridge runs the current `go-agent-harness` checkout inside task containers.
  - A local runner script exists for operators.
  - A scheduled GitHub Actions workflow can run the suite and keep artifacts.
- Non-goals:
  - Full public benchmark coverage or leaderboard submission.
  - PR-blocking on paid benchmark runs.
- Guardrails/constraints:
  - Keep the suite small, deterministic, and inexpensive.
  - Test the real harness API path (`harnessd` + `harnesscli`), not a mocked adapter.
  - Preserve existing repo regression workflow as the primary pre-merge gate.
- Open questions:
  - Whether to expand the suite beyond smoke coverage once failure patterns stabilize.
  - Whether to add result summarization or alerting beyond artifact upload.
- Next verification step: Run `./scripts/run-terminal-bench.sh` with a real API key and inspect per-task artifacts under `.tmp/terminal-bench/`.
