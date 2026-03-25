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

## 2026-03-25 (Backend OpenRouter Model Discovery)

- Command intent: Implement a backend model discovery layer with OpenRouter live discovery, TTL caching, static-overlay merge behavior, runtime routing support, `/v1/models` integration, tests, and docs.
- User intent: Make backend model selection and model listing behave like the already-improved startup/TUI paths, so dynamic OpenRouter slugs work without depending on a fully hardcoded catalog.
- Success definition:
  - Backend discovery exists as an additive layer over the existing provider catalog.
  - OpenRouter live models can be fetched from `https://openrouter.ai/api/v1/models` with in-memory TTL caching.
  - Static catalog metadata continues to win when present for pricing, aliases, quirks, and context defaults.
  - Runtime provider resolution can route `moonshotai/kimi-k2.5` through OpenRouter when OpenRouter is configured and no explicit provider is set.
  - `GET /v1/models` includes live OpenRouter models and falls back safely to cache or static catalog when live discovery fails.
  - Existing static-catalog providers remain unchanged.
  - Focused regression tests cover fetch decode, cache behavior, merged listing, dynamic routing, and fallback behavior.
- Non-goals:
  - Generalizing discovery for every provider in this pass.
  - Replacing the static catalog or startup bootstrap behavior outright.
  - Making startup block on network discovery.
- Guardrails/constraints:
  - Keep changes small and reviewable.
  - Follow strict TDD.
  - Use cached data when possible and static fallback otherwise.
  - Do not break existing catalog-driven providers or `/v1/models` consumers.
- Open questions:
  - Whether the backend `list_models` tool should be discovery-aware now or in a follow-up once `/v1/models` and routing are stabilized.
- Next verification step: add failing tests for discovery/cache/merge/routing, implement the minimal backend layer, then run targeted packages and the regression suite.

## 2026-03-24 (Worktree Bootstrap Script)

- Command intent: Build a reusable setup script that creates a fresh agent worktree and leaves it ready for local development and verification.
- User intent: Give agents a consistent, low-friction bootstrap path so they do not have to assemble the worktree environment by hand.
- Success definition:
  - `scripts/init.sh` creates or reuses a dedicated worktree under `.codex-worktrees/`.
  - `scripts/bootstrap-worktree.sh` remains as a compatibility wrapper only.
  - The script downloads Go dependencies and builds local binaries inside the worktree instead of dirtying the main checkout.
  - The script writes a sourceable env file with the key workspace paths and binary locations.
  - The script can optionally start `harnessd` in tmux for long-running local development.
  - `AGENTS.md`, `CLAUDE.md`, and the worktree runbook point agents at the canonical init script.
- Non-goals:
  - Replacing the full worktree policy or test-gated merge workflow.
  - Adding new runtime behavior to `harnessd`.
- Guardrails/constraints:
  - Long-running processes must still run in tmux.
  - Keep the script safe to rerun on an existing worktree.
  - Do not overwrite unrelated user changes.
- Open questions:
  - Whether future bootstrap automation should also start a smoke-test session by default.
- Next verification step: run the script in `--check` mode, verify the shell syntax, and confirm the docs reference the new entrypoint.

## 2026-03-18 (Issue #316 Context Grid Coverage)

- Command intent: Take one open backlog issue to completion by adding direct regression coverage for the TUI context usage grid component and merging the work.
- User intent: Close a clearly scoped backlog item end to end with strict TDD, proving the `/context` usage grid’s rendering contract directly instead of relying on indirect overlay tests.
- Success definition:
  - Issue `#316` is the only issue worked in this run.
  - Dedicated tests exist for `cmd/harnesscli/tui/components/contextgrid`.
  - Tests cover default total fallback, used-token clamping, width fallback/bar limits, and rendered usage text.
  - The repo regression gate passes before merge.
  - A PR is opened and merged, or a concrete GitHub permission blocker is reported.
- Non-goals:
  - Refactoring unrelated TUI code.
  - Expanding scope to additional coverage-only issues.
- Guardrails/constraints:
  - Strict TDD: failing tests first, then minimal implementation.
  - Keep changes inside the current worktree/branch.
  - Preserve existing behavior unless acceptance-criteria coverage exposes a small required fix.
- Open questions:
  - Whether any production code change is needed, or the issue resolves with tests only.
- Next verification step: Add the new package tests, run them red then green, and execute `./scripts/test-regression.sh` before opening the PR.

## 2026-03-18 (Repo-Wide Zero-Coverage Gate)

- Command intent: Fix the repo-wide zero-coverage regression gate so pushes are no longer blocked.
- User intent: Make the required regression script pass end to end without weakening the coverage protections that are supposed to catch real test erosion.
- Success definition:
  - `./scripts/test-regression.sh` completes successfully.
  - Coverage collection reflects repo-wide execution instead of package-local blind spots where appropriate.
  - Remaining zero-covered functions in `./internal/...` and `./cmd/...` are exercised by targeted regression tests rather than ignored.
  - Any incidental regression blockers encountered while reaching the coverage gate are resolved or made deterministic.
- Non-goals:
  - Lowering the minimum coverage threshold.
  - Disabling the zero-function coverage rule.
  - Broad refactors unrelated to the current push blocker.
- Guardrails/constraints:
  - Keep runtime behavior unchanged unless a deterministic test fix requires a minimal correction.
  - Prefer small focused tests over sweeping placeholder coverage tests.
  - Update the repo docs/logs to reflect the coverage-gate behavior change.
- Open questions:
  - Whether the race-path harness failure is a one-off flake or needs a deterministic fix in this pass.
- Next verification step: Run a repo-wide coverage pass with `-coverpkg`, add the missing targeted tests, and rerun `./scripts/test-regression.sh`.

## 2026-03-18 (Runner Concurrency Invariants)

- Command intent: Implement the review feedback by making the runner's concurrency and lifecycle invariants explicit and test-enforced.
- User intent: Preserve the recorder/message-state fixes by making future changes defend clear ownership, serialization, and state-transition rules instead of relying on race-clean runs alone.
- Success definition:
  - The runner code documents the concurrency invariants for recorder ordering, message-state ownership, and payload isolation.
  - Regression coverage explicitly checks the JSONL ledger matches in-memory event history.
  - Existing compaction and forensic-isolation tests are aligned with the invariant framing.
- Non-goals:
  - Redesigning the runner concurrency model.
  - Introducing new behavior beyond invariant enforcement/documentation.
- Guardrails/constraints:
  - Keep implementation scoped to the runner/test surface touched by the review.
  - Preserve current runtime behavior.
  - Do not overwrite unrelated user changes in the worktree.
- Open questions:
  - Whether the team later wants a dedicated invariant checklist in review docs beyond code comments and tests.
- Next verification step: Run targeted harness tests for recorder ordering/completeness and compaction source-of-truth behavior, then record the result in the logs.

## 2026-03-18 (Provider/Model Impact Map Guardrail)

- Command intent: Implement the repo review finding by requiring a cross-surface impact map for provider/model flow work.
- User intent: Prevent feature slices from landing with missing integration coverage across config, server wiring, TUI behavior, or regression tests.
- Success definition:
  - A reusable impact-map template exists in `docs/plans/`.
  - The bootstrap, plan template, and worktree flow all direct contributors to create the artifact before implementation.
  - The four required headings are explicit: config, server API, TUI state, regression tests.
  - Blank headings are called out as a warning, with `None` plus rationale required when a surface is truly unaffected.
- Non-goals:
  - Adding CI enforcement in this pass.
  - Retrofitting older tasks with new impact maps.
- Guardrails/constraints:
  - Keep the artifact lightweight and one-page.
  - Only require it for provider/model flow work rather than every task.
  - Fit the rule into the repo's existing planning workflow.
- Open questions:
  - Whether future automation should lint for missing impact maps on provider/model changes.
- Next verification step: Confirm the new template and runbook are reachable from `AGENTS.md`, `PLAN_TEMPLATE.md`, and `docs/runbooks/worktree-flow.md`.

## 2026-03-18 (Ownership And Copy-Semantics Hardening)

- Command intent: Build and apply a concrete ownership/copy-semantics checklist grounded in the repo's runner review history.
- User intent: Stop repeating shallow-copy regressions by making clone boundaries explicit in code and documentation instead of rediscovering them in review loops.
- Success definition:
  - Exported or state-storing harness types with mutable fields have explicit clone behavior.
  - Registry and runner snapshot paths stop relying on ad hoc shallow struct copies where shared maps/slices can leak through.
  - A reusable internal checklist exists for reviewing slices, maps, pointers, and nil semantics before code review.
  - Ownership-focused tests pass for the touched surfaces.
- Non-goals:
  - Solving every historical runner concurrency issue in the same pass.
  - Refactoring unrelated packages just to use clone helpers.
- Guardrails/constraints:
  - Preserve existing nil semantics where callers may distinguish nil from empty.
  - Keep the change narrow, reviewable, and grounded in current code rather than generic guidance.
  - Run the package tests and the repo regression gate before considering the task complete.
- Open questions:
  - Which additional exported types outside `internal/harness` should adopt the same contract in a follow-up pass.
- Next verification step: Run `go test ./internal/harness` and `./scripts/test-regression.sh`, then record the concrete pass/fail result in the engineering log.

## 2026-03-18 (Issue #332 Runner Orchestration Coverage)

- Command intent: Complete GitHub issue `#332` by adding direct regression coverage for `SubmitInput`, `RunPrompt`, and `RunForkedSkill`.
- User intent: Make runner orchestration extraction safer by pinning the public helper semantics that currently rely on incidental coverage.
- Success definition:
  - `SubmitInput` error mapping is asserted directly.
  - terminal-history, stream-closure, and terminal-result mapping behavior are covered through deterministic orchestration tests.
  - `go test ./internal/harness` passes with the new regression coverage in place.
- Non-goals:
  - broader runner refactors beyond what is needed to expose the wait-path contract.
  - fixing unrelated packages that fail only because the sandbox forbids opening localhost listeners.
- Guardrails/constraints:
  - Keep behavior unchanged while making orchestration wait semantics directly testable.
  - Follow strict TDD and stop if the full repo regression gate is blocked by unrelated failures.
- Open questions:
  - Whether the repo regression script should eventually detect sandboxed localhost restrictions and skip listener-based packages in this environment.
- Next verification step: run the targeted harness tests, then `go test ./internal/harness`, then `./scripts/test-regression.sh` and record the blocker if the sandbox still prevents listener-based tests.

## 2026-03-17 (Untested Feature Issue Backlog)

- Command intent: Identify implemented features that are missing test coverage and create GitHub issues for them.
- User intent: Turn the remaining untested feature surface into concrete, trackable work items instead of leaving test gaps implicit.
- Success definition:
  - Remaining feature areas with no meaningful tests are identified from the current codebase.
  - GitHub issues are created with scope, impact, and acceptance criteria for each missing-test feature area.
  - The issue set is grounded in the current implementation rather than stale documentation.
- Non-goals:
  - Writing the missing tests in this pass.
  - Reworking features that are already adequately covered.
- Guardrails/constraints:
  - Prefer feature-level gaps over file-by-file nitpicks.
  - Use the repo code and test layout as the source of truth.
  - Keep issue scope specific enough for a remote agent to execute directly.
- Open questions:
  - Whether the unimplemented `thinkingbar` should be treated as a missing-test issue only or folded into a broader implementation issue later.
- Next verification step: Confirm the created issues map to packages with zero direct test coverage and record the issue numbers in the task handoff.

## 2026-03-19 (Post-Review Stabilization Backlog)

- Command intent: Convert the harness/TUI review into a concrete, dependency-ordered GitHub issue backlog.
- User intent: Work through the next tranche of high-value improvements methodically without guessing what should happen next or over-investing in low-value new features.
- Success definition:
  - Review findings are turned into a small ordered set of implementation issues.
  - Each issue names the target behavior, tests required, regression coverage required, and any dependency order.
  - The backlog favors stabilization/productization over speculative feature growth.
- Non-goals:
  - Implementing the fixes in this pass.
  - Expanding the feature surface beyond what is needed to make the current system coherent.
- Guardrails/constraints:
  - Prefer issues that remove architectural friction, deployment friction, or user-facing rough edges.
  - Separate harness and TUI concerns clearly.
  - Make each ticket executable by a remote agent without additional grooming.
- Open questions:
  - Whether the TUI command/render consolidation should be delivered as one PR or a short stack of smaller PRs.
- Next verification step: Create the GitHub issues, then capture the resulting issue numbers and dependency order in the handoff.

## 2026-03-19 (Issue #361 Golden Path Deployment Contract)

- Command intent: Implement issue `#361` by making the documented golden-path deployment actually bootable and by backing it with repeatable regression coverage plus a live smoke entrypoint.
- User intent: Work through the backlog in dependency order with real TDD, so the harness has one trustworthy deployment path before more feature work lands.
- Success definition:
  - `harnessd` has a real, repo-supported `full` startup contract instead of a broken documented profile path.
  - Regression tests fail first and then pass for profile resolution and persistence-backed startup/readback.
  - The smoke script validates health, provider/model discovery, run creation, event streaming, at least one tool call, terminal completion, and persistence readback.
  - The golden-path runbook matches the actual startup contract.
- Non-goals:
  - Adding CI enforcement for live-provider smoke.
  - Expanding the golden path to S3, extra MCP servers, or third-party integrations.
- Guardrails/constraints:
  - Strict TDD.
  - Keep the path provider-agnostic where practical after the #362 bootstrap fix.
  - Preserve the current harness API surface unless a startup contract bug requires a small fix.
- Open questions:
  - Whether the cleanest supported `full` contract should resolve through config-layer builtins or project-level profile discovery.
- Next verification step: add the failing startup/profile regression test, reproduce the smoke-script failure locally, then implement the smallest fix that makes `--profile full` and the persistence-backed smoke path real.

## 2026-03-17 (Docs And Contract Sync)

- Command intent: Update the user-facing documentation so it matches the current harness codebase.
- User intent: Make the README, agent guidance, and live CLI runbook reflect the actual routes, run payload, event surface, tool catalog, and configuration behavior.
- Success definition:
  - README describes the current HTTP routes, run request shape, event families, tool surface, and configuration knobs.
  - CLAUDE.md no longer says provider support is only planned.
  - The harness CLI runbook reflects the current flags and live-testing flow.
  - The long-term thinking log records the docs-sync effort.
- Non-goals:
  - Changing runtime behavior.
  - Adding new APIs or tools.
- Guardrails/constraints:
  - Treat the implementation as the source of truth.
  - Avoid documenting unsupported flags, routes, or environment variables.
- Open questions:
  - Whether the README should later split the long environment list into a dedicated config reference doc.
- Next verification step: Reconcile any future API or config changes against these docs before release.

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

## 2026-03-06 (Issue #18 Head-Tail Buffer for Long Command Output)

- Command intent: Take a tracked GitHub issue, plan it according to project rules, implement it with tests, and merge when the full test gate passes.
- User intent: Improve harness reliability by preventing unbounded command-output growth while preserving useful diagnostics.
- Success definition:
  - Command output handling keeps both leading and trailing content for oversized output.
  - `bash` foreground and background `job_output` paths use bounded output capture.
  - Tests are written first and cover truncation behavior explicitly.
  - Regression gate passes before merge.
- Non-goals:
  - Token streaming changes.
  - Persistent archival of full command logs.
- Guardrails/constraints:
  - Preserve existing tool result schema fields.
  - Keep omission explicit so users know output was truncated.
  - Follow strict TDD and documentation/index maintenance.
- Open questions:
  - Whether additional command-backed tools should share the same bounded output helper immediately.
- Next verification step: Add failing tests for oversized output in both foreground/background flows, implement bounded buffer, then run `./scripts/test-regression.sh`.
