# Engineering Log

## 2026-03-25 (Issue #431 Conversation Cleaner Startup Failures)

- Fixed the `harnessd` conversation-retention cleaner lifecycle so startup failures after cleaner initialization still cancel the cleaner context before returning.
- Added a narrow test seam in `cmd/harnessd/main.go` so the startup path can be characterized without changing runtime behavior:
  - `runWithSignals(...)` now delegates to `runWithSignalsWithOptions(...)`.
  - the cleaner factory stays unexported and defaults to `harness.NewConversationCleaner(...)`.
  - cleaner cancellation is deferred once initialized, with the normal shutdown path still cancelling early and clearing the handle.
- Added regression coverage in `cmd/harnessd/main_test.go` for:
  - clean shutdown with conversation cleaner enabled
  - startup failure after cleaner initialization cancels the cleaner context
- Added planning/logging artifacts for the issue-specific work:
  - `docs/plans/2026-03-25-issue-431-cleaner-startup-failure-plan.md`
  - `docs/logs/long-term-thinking-log.md`
  - `docs/plans/INDEX.md`
- Verification:
  - `GOCACHE=$PWD/.tmp/gocache go test ./cmd/harnessd -run 'TestStartupFailureCancelsConversationCleaner|TestShutdownConversationCleanerCancellation'`
  - `GOCACHE=$PWD/.tmp/gocache go test ./cmd/harnessd`
  - `GOCACHE=$PWD/.tmp/gocache go vet ./internal/... ./cmd/...`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/gocache ./scripts/test-regression.sh` (fails for unrelated existing zero-coverage functions, including `cmd/harnesscli/tui/cmd_parser.go:73`, `cmd/harnesscli/tui/components/tooluse/model.go:44`, and `internal/profiles/loader.go:101`)

## 2026-03-25 (Harness Review Bug Tickets)

- Reviewed the harness runtime and transport paths with focus on cancellation propagation, forked-run failure reporting, tool-allowlist integrity, and bootstrap cleanup.
- Created four bug issues with implementation-ready agent prompts, explicit TDD requirements, and regression-test expectations:
  - `#428` Cancel timed-out subruns instead of leaving them running
  - `#429` Propagate forked child-run failures instead of reporting success
  - `#430` Preserve `allowed_tools` restrictions on agent and skill fallback paths
  - `#431` Close the conversation cleaner on `harnessd` startup failures
- Verification:
  - `gh issue create` created issues `#428` through `#431`
  - no runtime code changed in this pass

## 2026-03-25 (Architecture Review Backlog)

- Reviewed the harness architecture with focus on config authority, persistence ownership, and monolithic orchestration boundaries.
- Converted the review into a dependency-ordered GitHub issue stack with TDD-first implementation guidance and explicit regression-test expectations:
  - `#421` Make merged harness config the authoritative runtime contract
  - `#422` Consolidate run persistence ownership into the runner boundary
  - `#423` Extract runner preflight orchestration from `execute()`
  - `#424` Extract runner event journal and sink path from `runner.go`
  - `#425` Extract the core step engine from the runner monolith
  - `#426` Split `harnessd` bootstrap into modular app wiring
  - `#427` Continue decomposing `internal/server/http.go` by feature
- Execution order captured in the issue bodies:
  - Start with config contract and persistence ownership so runtime boundaries are explicit.
  - Then split the runner monolith in slices: preflight, event journal, step engine.
  - Run `harnessd` bootstrap decomposition and `internal/server` transport decomposition alongside or after the runner work as dependencies allow.
- Verification:
  - Created GitHub issues `#421` through `#427`
  - No runtime code changed in this pass

## 2026-03-25 (Backend OpenRouter Discovery)

- Added additive backend discovery support in `internal/provider/catalog/discovery.go`:
  - live OpenRouter fetch from `https://openrouter.ai/api/v1/models`
  - in-memory TTL caching
  - stale-cache fallback when a refresh fails
- Extended `internal/provider/catalog/registry.go` so runtime provider resolution and merged model listing can use cached live OpenRouter data while preserving static catalog metadata as the overlay authority.
- Updated `internal/server/http.go` so `GET /v1/models` serializes the merged registry view when a provider registry is configured.
- Wired `cmd/harnessd/main.go` to enable OpenRouter discovery automatically when the loaded model catalog includes an `openrouter` provider, without making startup perform a live fetch.
- Added focused regression coverage in:
  - `internal/provider/catalog/discovery_test.go`
  - `internal/provider/catalog/discovery_registry_test.go`
  - `internal/server/http_models_test.go`
  - updated `internal/harness/runner_test.go`
- Verification:
  - `go test ./internal/provider/catalog ./internal/server ./internal/harness ./cmd/harnessd`

## 2026-03-18 (Issue #316 Context Grid Coverage)

- Added direct package coverage for `cmd/harnesscli/tui/components/contextgrid` in `cmd/harnesscli/tui/components/contextgrid/model_test.go`:
  - default total-token fallback when `TotalTokens <= 0`
  - used-token clamping for negative and over-limit values
  - width fallback / max-width bar sizing
  - rendered header, counts, percentage text, and bar glyph assertions
- Tightened `cmd/harnesscli/tui/components/contextgrid/model.go` so the progress bar fits within the requested width after accounting for the surrounding brackets:
  - `barWidth` now uses `width - 2`
  - narrow widths clamp to at least one cell instead of forcing a 10-cell overflow
- Verification:
  - `TMPDIR=$PWD/.tmp GOCACHE=$PWD/.tmp/go-build go test ./cmd/harnesscli/tui/components/contextgrid`
  - `TMPDIR=$PWD/.tmp GOCACHE=$PWD/.tmp/go-build go test -cover ./cmd/harnesscli/tui/components/contextgrid`
- Regression status:
  - package coverage for `cmd/harnesscli/tui/components/contextgrid` is now `93.1%`
  - full `./scripts/test-regression.sh` is blocked in this sandbox because many existing tests cannot bind local ports (`httptest.NewServer`, `listen tcp :0`, `127.0.0.1:0`) under the current environment; the failures are not isolated to the context-grid package

## 2026-03-18 (Issue #332 Runner Orchestration Coverage)

- Added direct orchestration regression tests in `internal/harness/runner_orchestration_test.go` for:
  - `SubmitInput` mapping broker validation failures to `ErrInvalidRunInput`
  - `SubmitInput` mapping missing pending-question submissions to `ErrNoPendingInput`
  - terminal-history and stream-closure wait semantics
  - failed `RunForkedSkill` terminal result mapping
- Refactored the shared wait logic in `internal/harness/runner.go` into `waitForTerminalResult(...)` so `RunPrompt` and `RunForkedSkill` keep the same behavior while the history/stream branches become directly testable.
- Verification:
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/harness -run 'TestSubmitInput_MapsBrokerValidationFailure|TestSubmitInput_MapsMissingPendingQuestion|TestWaitForTerminalResult_UsesTerminalHistory|TestWaitForTerminalResult_ReturnsOnStreamClose|TestRunForkedSkill_ReturnsFailedForkResult|TestRunPrompt_ReturnsOutput|TestRunPrompt_RespectsContextCancellation'`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build go test ./internal/harness`
  - `TMPDIR=$PWD/.tmp/tmp GOCACHE=$PWD/.tmp/go-build ./scripts/test-regression.sh`
- Regression status:
  - targeted harness tests and full `internal/harness` package tests passed.
  - the repo-wide regression script failed for unrelated environment/sandbox reasons: multiple packages panic or error when `httptest.NewServer`, `net.Listen`, or `listen tcp 127.0.0.1:0` attempt to bind a localhost port in this sandbox (examples include `internal/cron`, `internal/mcp`, `internal/observationalmemory`, `internal/server`, `cmd/harnesscli`, `cmd/harnesscli/tui`, `cmd/harnessd`, `cmd/cronsd`, and `internal/workspace`).
  - no issue-`#332` failure remained in the harness package after the new tests/refactor landed.

## 2026-03-18 (Ownership And Copy-Semantics Hardening)

- Added an explicit clone contract for mutable exported/state-storing harness types:
  - `internal/harness/types.go`
    - `ToolDefinition.Clone()` now deep-copies schema maps.
    - existing `Message.Clone()` remains the owner of `ToolCalls` copy semantics.
  - `internal/harness/clone.go`
    - centralized deep-copy helpers for payload maps, string slices, and message slices with preserved nil semantics.
- Hardened registry ownership boundaries in `internal/harness/registry.go`:
  - clone tool definitions on registration
  - clone definitions on `Definitions()`, `DefinitionsForRun()`, and `DeferredDefinitions()`
  - deep-copy MCP-discovered tool schemas before storing them
- Normalized remaining runner message snapshot reads onto `copyMessages(...)` in `internal/harness/runner.go` so internal readers stop using ad hoc shallow slice copies.
- Fixed nil/empty conversation semantics in `internal/harness/runner.go`:
  - persisted empty conversations are now distinguishable from missing conversations via store owner lookup
  - `copyMessages(...)` preserves non-nil empty slices instead of collapsing them to `nil`
- Added TDD coverage in `internal/harness/registry_test.go` for:
  - caller mutation after `Register(...)`
  - returned-definition mutation after `Definitions()` / `DefinitionsForRun()`
  - `ToolDefinition.Clone()` nil semantics
- Added the reusable checklist runbook and wired it into the planning flow:
  - `docs/runbooks/ownership-copy-semantics.md`
  - `docs/runbooks/INDEX.md`
  - `docs/plans/PLAN_TEMPLATE.md`
  - `docs/runbooks/worktree-flow.md`
- While running the repo regression gate, fixed two unrelated pre-existing blockers so the gate got further:
  - `cmd/harnesscli/tui/components/statspanel/model.go` plus three golden snapshots now anchor snapshot rendering to the latest fixture date instead of wall-clock time
  - `internal/subagents/manager.go` now synchronizes worktree auto-cleanup so `Get()` no longer races or reports cleanup complete before the filesystem destroy finishes
- Validation:
  - `go test ./internal/harness ./internal/subagents ./cmd/harnesscli/tui/components/statspanel`
  - `go test ./internal/subagents -run 'TestManagerCreateWorktreeSubagent(DestroyOnSuccess|Preserve)' -race`
  - `./scripts/test-regression.sh` executed via `tmux`
- Regression status:
  - repo-wide regression script still exits non-zero because the existing coverage gate reports many zero-coverage functions in unrelated packages (for example `cmd/forensics/main.go:18`, `cmd/harnesscli/main.go:347`, `cmd/harnesscli/tui/api.go:99`, `internal/config/config.go:511`, `internal/provider/openai/client.go:749`, `internal/subagents/manager.go:164`)
  - no new repo-wide behavioral test failure remained after the `statspanel` and `subagents` fixes above

## 2026-03-18 (Runner Concurrency Invariants)

- Made the runner's concurrency/lifecycle invariants explicit in `internal/harness/runner.go`:
  - `emit()` owns canonical event ordering.
  - `state.messages` is the single source of truth for run context.
  - payload ownership must stay isolated across caller/history/subscriber/recorder boundaries.
- Strengthened recorder behavior in `internal/harness/runner.go`:
  - `startRecorderGoroutine()` now buffers out-of-order arrivals and flushes JSONL in `Seq` order.
  - `recorder.drop_detected` markers now carry the dropped event's `Seq`, keeping the ledger position explicit if a drop is surfaced.
- Added invariant-focused regression coverage in `internal/harness/runner_forensics_test.go`:
  - `TestEventLedgerInvariant_JSONLMatchesInMemoryHistory`
- Reframed existing compaction tests in `internal/harness/runner_context_compact_test.go` around the `state.messages` source-of-truth contract.
- Verification:
  - `GOCACHE=/tmp/go-build-cache go test ./internal/harness -run 'TestEventLedgerInvariant_JSONLMatchesInMemoryHistory|TestCompactRunSurvivesConcurrentExecute|TestCompactRunAtStepBoundary|TestMessageExportMutationIsolation|TestAccountingStructPointerFieldIsolation'`
  - `GOCACHE=/tmp/go-build-cache go test -race ./internal/harness -run 'TestEventLedgerInvariant_JSONLMatchesInMemoryHistory|TestCompactRunSurvivesConcurrentExecute|TestCompactRunAtStepBoundary|TestMessageExportMutationIsolation|TestAccountingStructPointerFieldIsolation'`
  - Full repo regression suite not run in this pass.

## 2026-03-18 (Provider/Model Impact Map Guardrail)

- Added a new one-page planning artifact for provider/model flow work:
  - `docs/plans/IMPACT_MAP_TEMPLATE.md`
  - Requires explicit sections for config, server API, TUI state, and regression tests.
  - Makes blank headings an explicit warning; unaffected surfaces must be documented as `None` with rationale.
- Added a focused runbook:
  - `docs/runbooks/provider-model-impact-mapping.md`
  - Defines when the impact map is required and how to use it before implementation starts.
- Updated workflow entry points to surface the requirement early:
  - `AGENTS.md`
  - `docs/context/critical-context.md`
  - `docs/plans/PLAN_TEMPLATE.md`
  - `docs/runbooks/worktree-flow.md`
- Updated planning metadata:
  - `docs/plans/2026-03-18-provider-model-impact-map-guardrail-plan.md`
  - `docs/plans/active-plan.md`
  - `docs/plans/INDEX.md`
  - `docs/runbooks/INDEX.md`
- Verification:
  - Planned as doc cross-reference verification in this pass; no runtime code changed.

## 2026-03-06 (Issue #18 Head-Tail Buffer for Long Command Output)

- Added bounded head-tail output capture in `internal/harness/tools/head_tail_buffer.go`:
  - concurrency-safe writer that stores leading and trailing output bytes
  - explicit middle omission marker: `...[truncated output]...`
- Integrated bounded capture in command execution paths:
  - `internal/harness/tools/bash_manager.go` for foreground `bash` and background jobs (`job_output`)
  - `internal/harness/tools/common_exec.go` so command-backed helper tools also avoid unbounded output buffering
- TDD evidence (failing first, then green):
  - failing first: `GOCACHE=/tmp/go-build-cache go test ./internal/harness/tools -run TestJobManagerOutputHeadTailBuffer` (compile failure before implementation: missing `maxOutputBytes`)
  - passing after implementation:
    - `GOCACHE=/tmp/go-build-cache go test ./internal/harness/tools -run TestJobManagerOutputHeadTailBuffer`
    - `GOCACHE=/tmp/go-build-cache go test ./internal/harness -run TestBashToolOutputUsesHeadTailBuffer`
- Full regression gate:
  - executed via tmux: `GOCACHE=/tmp/go-build-cache ./scripts/test-regression.sh`
  - failed due unrelated pre-existing repo issues:
    - `cmd/harnesscli/main_prompt_test.go` references undefined `httpClient`
    - existing harness test failure: `TestApplyPatchToolAcceptsUnifiedPatchPayload`
- Commit/merge status:
  - blocked by required full regression gate failure (no commit/merge performed).

## 2026-03-05 (Provider Token Streaming)

- Added incremental provider-to-runner streaming contract in `internal/harness/types.go` via `CompletionRequest.Stream` and `CompletionDelta`.
- Updated runner execution to emit live SSE-visible delta events before turn completion:
  - `assistant.message.delta`
  - `tool.call.delta`
- Implemented OpenAI streaming chat completions assembly in `internal/provider/openai/client.go`:
  - sends `stream: true`
  - requests streamed usage via `stream_options.include_usage`
  - assembles assistant text and tool calls from chunked deltas
- Added TDD coverage:
  - streamed assistant/tool-call assembly in `internal/provider/openai/client_test.go`
  - runner delta event emission in `internal/harness/runner_test.go`
- Validation:
  - `go test ./internal/provider/openai` passed
  - targeted runner tests in `go test ./internal/harness -run 'TestRunner(EmitsAssistantMessageDeltaEvents|EmitsToolCallDeltaEventsBeforeExecution|ExecutesToolCallsAndPublishesEvents|FailsWhenProviderErrors|EmitsUsageDeltaAndPersistsTotals|FailedRunIncludesPartialUsageTotals)'` passed
- Note: full `go test ./internal/harness` is currently blocked by an unrelated existing failure in `TestApplyPatchToolAcceptsUnifiedPatchPayload`.

## 2026-03-05

### Architecture Decision: REST over GraphQL

**Decision**: Stick with REST for all API endpoints. Do not adopt GraphQL.

**Rationale**:
- The API is command-and-control for orchestrating agent runs, not a complex query interface
- Current surface is 6 endpoints with clean REST sub-resource patterns (`/runs/{id}/events`, `/runs/{id}/input`)
- SSE for event streaming is REST-native; GraphQL subscriptions (WebSocket-based) would add complexity for no benefit
- New endpoints (`/steer`, `/continue`) are imperative actions, not data mutations — REST verbs express this naturally
- Go stdlib makes REST trivial; GraphQL requires schema/codegen layer (gqlgen etc.) that's overkill here
- No client needs complex field selection, cross-resource queries, or varied data shapes

**When to revisit**: If a dashboard or analytics layer needs to query across many runs with filters, pagination, and field selection — a read-heavy client with varied data needs. That would be a separate read API, not a replacement for the core run orchestration API.

### Issues Created

- [#1](https://github.com/dennisonbertram/go-agent-harness/issues/1) — Stream tool output incrementally during execution
- [#2](https://github.com/dennisonbertram/go-agent-harness/issues/2) — Audit SSE events for completeness and consistency
- [#3](https://github.com/dennisonbertram/go-agent-harness/issues/3) — Make max steps tunable per-run, default to unlimited
- [#4](https://github.com/dennisonbertram/go-agent-harness/issues/4) — Implement deferred (lazy-loaded) tools via ToolSearch meta-tool
- [#5](https://github.com/dennisonbertram/go-agent-harness/issues/5) — Add run continuation for multi-turn conversations
- [#6](https://github.com/dennisonbertram/go-agent-harness/issues/6) — Add mid-run steering for user guidance during execution

### Architecture Direction: Platform Backend (CLI + GUI)

Established that the harness is a **Go backend platform** supporting multiple frontends (CLI, web GUI, desktop app). Must work transparently in both local and remote modes — remote execution should feel like local, and vice versa.

Key architectural pieces identified:
- **Persistence layer** (#7) — foundational, everything else depends on it
- **Workspace abstraction** (#8) — transparent local/remote via `Workspace` interface + optional proxy agent on user's machine
- **Client auth** (#9) — API keys, tenant isolation, scoped permissions
- **Cost/safety controls** (#10) — cost ceilings, idle detection, spending limits (critical once max steps goes unlimited)
- **Multi-provider** (#11) — Anthropic alongside OpenAI, auto-detect from model name, prompt caching

### Codex CLI Architecture Study

Researched OpenAI Codex CLI (Rust, 65+ crates, Apache-2.0) for architectural patterns. Findings in `docs/research/codex-cli-architecture.md`. Created issues for the most impactful patterns:

- [#15](https://github.com/dennisonbertram/go-agent-harness/issues/15) — Two-axis permission model (sandbox × approval policy)
- [#16](https://github.com/dennisonbertram/go-agent-harness/issues/16) — JSONL rollout recorder for replay/fork/audit
- [#17](https://github.com/dennisonbertram/go-agent-harness/issues/17) — Conversation compaction for unlimited-step sessions
- [#18](https://github.com/dennisonbertram/go-agent-harness/issues/18) — Head-tail buffer for long process output
- [#19](https://github.com/dennisonbertram/go-agent-harness/issues/19) — Bidirectional MCP (client + server)
- [#20](https://github.com/dennisonbertram/go-agent-harness/issues/20) — Layered configuration cascade with cloud/team overrides

Skipped creating separate issues for Op/EventMsg protocol (already covered by SSE event audit #2 and the existing architecture) and Codex's skills/memories system (observational memory already covers this).

### Research

- Deferred tools design doc written to `docs/research/deferred-tools-design.md` — covers Claude Code's ToolSearch pattern, Go implementation strategy, token savings analysis (40-60%), and comparison of alternatives (intent filtering, tiered packs, description compression, dynamic pruning). Recommended approach: ToolSearch + tiered packs.

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

## 2026-03-05 (Token Counting + Cost Tracking)

- Added additive accounting types in `internal/harness/types.go`:
  - `CompletionUsage` optional detail fields.
  - `CompletionCost`, `UsageStatus`, `CostStatus`.
  - Run-level totals: `RunUsageTotals`, `RunCostTotals`.
- Added pricing module in `internal/provider/pricing/`:
  - file-backed JSON catalog loader.
  - provider/model resolver with alias support.
  - unit tests for load/resolve/validation behavior.
- Extended OpenAI adapter (`internal/provider/openai/client.go`):
  - parses usage + detail fields.
  - normalizes missing usage to zero + `provider_unreported`.
  - computes cost from explicit response cost when present, otherwise resolver-driven pricing.
  - emits `unpriced_model` when pricing is unavailable.
- Extended runner accounting (`internal/harness/runner.go`):
  - per-turn accumulation of usage/cost totals.
  - new `usage.delta` event each model turn.
  - `run.completed` and `run.failed` now include usage/cost totals payloads.
  - run state includes persisted totals exposed by `GET /v1/runs/{id}`.
- Updated runtime context (`internal/systemprompt/runtime_context.go`):
  - replaced phase-1 cost placeholder with live token/cost fields.
  - default `cost_status: pending` before first completion.
- Wired pricing config in server startup (`cmd/harnessd/main.go`):
  - `HARNESS_PRICING_CATALOG_PATH` enables resolver-backed cost computation.
- Updated tests:
  - `internal/provider/openai/client_test.go`
  - `internal/provider/pricing/catalog_test.go`
  - `internal/harness/runner_test.go`
  - `internal/harness/runner_prompt_test.go`
  - `internal/systemprompt/engine_test.go`
  - `internal/server/http_test.go`
- Validation:
  - `go test ./...` passed.
  - `go test ./... -race` passed.
  - `./scripts/test-regression.sh` passed (`coveragegate: PASS`, total `80.1%`, zero-functions `0`).

## 2026-03-05 (Token/Cost Documentation Pass)

- Updated `README.md` to fully document:
  - `GET /v1/runs/{id}` usage/cost totals fields.
  - `usage.delta` payload contract.
  - missing-usage and missing-pricing behavior.
  - pricing catalog JSON format and configuration.
- Updated `docs/runbooks/harnesscli-live-testing.md`:
  - added `HARNESS_PRICING_CATALOG_PATH`.
  - documented expectation that `usage.delta` appears during runs.
- Updated `docs/design/system-prompt-architecture.md` heading/scope text to reflect OpenAI-first implementation status.
- Updated `docs/plans/INDEX.md` to mark token/cost plan as completed.

## 2026-03-05 (Optional Observational Memory: Local-First Foundation)

- Added new subsystem package: `internal/observationalmemory/`.
  - Core manager orchestration and state model (`manager.go`, `types.go`).
  - Model-backed observer + reflector implementations (`observer.go`, `reflector.go`).
  - Local per-scope coordinator (`coordinator.go`).
  - SQLite durable store with migration-safe schema (`store_sqlite.go`, migrations).
  - Postgres compile-ready stub for future activation (`store_postgres.go`).
- Added transcript/runtime context seams in tool layer:
  - `RunMetadata` and read-only `TranscriptReader` in `internal/harness/tools/types.go`.
- Added new tool: `observational_memory` in `internal/harness/tools/observational_memory.go`.
  - Actions: `enable`, `disable`, `status`, `export`, `review`, `reflect_now`.
- Wired tool catalog/default registry to include observational memory manager.
- Updated runner integration in `internal/harness/runner.go`:
  - Stores run transcript snapshots.
  - Injects `<observational-memory>` snippet before model turns when enabled.
  - Calls memory observe flow after each turn/tool cycle.
  - Emits memory lifecycle events (`memory.observe.*`, `memory.reflection.completed`).
  - Passes run metadata + transcript reader into tool execution context.
- Expanded run API metadata fields in `internal/harness/types.go`:
  - `tenant_id`, `conversation_id`, `agent_id` on `RunRequest` and `Run`.
- Updated server bootstrap in `cmd/harnessd/main.go`:
  - Added memory env config parsing and manager creation.
  - Wired shared manager into registry + runner.
- Added/updated tests for new surfaces:
  - `internal/harness/tools/observational_memory_test.go`
  - `internal/harness/runner_test.go` memory snippet/event coverage
  - Tool contract/catalog/default-registry expected tool list updates.
- Added architecture and runbook docs:
  - `docs/design/observational-memory-architecture.md`
  - `docs/runbooks/observational-memory.md`
- Updated roadmap/index/readme docs to include observational memory and configuration.

## 2026-03-05 (Modular System Prompt Subsystem)

- Added new prompt engine module in `internal/systemprompt/`:
  - `catalog.go`: YAML catalog loading/validation and prompt asset indexing.
  - `matcher.go`: deterministic model profile routing with fallback signaling.
  - `engine.go`: static prompt composition for base/intent/model/extensions/custom layers.
  - `runtime_context.go`: per-turn ephemeral runtime context formatter.
  - `types.go`, `errors.go`, `validation.go` for subsystem contracts.
- Added file-driven prompt assets under `prompts/`:
  - `catalog.yaml`
  - `base/main.md`
  - `intents/{general,code_review,frontend_design}.md`
  - `models/{default,openai_gpt5}.md`
  - starter behavior/talent extensions.
- Expanded run request model in `internal/harness/types.go`:
  - `agent_intent`, `task_context`, `prompt_profile`, `prompt_extensions`.
  - reserved `skills` field retained for forward compatibility and ignored in phase 1.
- Updated runner integration in `internal/harness/runner.go`:
  - resolve prompt context at `StartRun`.
  - preserve `system_prompt` override bypass behavior.
  - rebuild provider messages each turn using static prompt + ephemeral runtime context + transcript.
  - emit `prompt.resolved` and `prompt.warning` events.
  - keep runtime context non-persistent in transcript state.
- Updated server bootstrap in `cmd/harnessd/main.go`:
  - startup loads prompt engine from `HARNESS_PROMPTS_DIR` (with default auto-discovery).
  - added `HARNESS_DEFAULT_AGENT_INTENT` config.
  - startup fails fast on invalid prompt catalog/files.
- Updated CLI in `cmd/harnesscli/main.go`:
  - new flags for intent/profile/extensions (`-agent-intent`, `-task-context`, `-prompt-profile`, `-prompt-behavior`, `-prompt-talent`, `-prompt-custom`).
- Added/updated tests:
  - `internal/systemprompt/{catalog,matcher,engine}_test.go`
  - `internal/harness/runner_prompt_test.go`
  - `internal/server/http_prompt_test.go`
  - `cmd/harnesscli/main_prompt_test.go`
- Validation:
  - Focused suites passed: `go test ./internal/systemprompt ./internal/harness ./internal/server ./cmd/harnesscli ./cmd/harnessd`.

## 2026-03-06 (Terminal Bench Periodic Smoke Suite)

- Added a private Terminal Bench integration under `benchmarks/terminal_bench/`.
- Added custom benchmark agent bridge in `benchmarks/terminal_bench/agent.py`:
  - Copies the current repository into each task container.
  - Builds `harnessd` and `harnesscli` inside the container.
  - Starts the harness in tmux and drives tasks through the real HTTP API.
- Added three stable smoke tasks:
  - `go-retry-schedule-fix`
  - `staging-deploy-docs`
  - `incident-summary-shell`
- Added local runner script:
  - `scripts/run-terminal-bench.sh`
  - Uses `tb` when installed or falls back to `uv tool run terminal-bench`.
- Added scheduled workflow:
  - `.github/workflows/terminal-bench-periodic.yml`
  - Runs nightly and on manual dispatch, then uploads benchmark artifacts.
- Added operator documentation:
  - `docs/runbooks/terminal-bench-periodic-suite.md`
- Updated README, nightly tasks, plan tracker, and indexes to reflect the new benchmark path.
- Validation:
  - Not run in this change set.
