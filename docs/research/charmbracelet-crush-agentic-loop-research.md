# crush Agent Architecture and Core Agentic Loop

**Source snapshot date:** 2026-03-04 (commit `v0.47.0`, `8bcca78520e5dd082bdf254a4a915a1505bc5c29`).
**Repository star/push metadata:** verified via GitHub API on 2026-03-04 (`20,856` stars, `pushed_at=2026-03-04T14:42:08Z`).
**Scope:** Source-backed architecture and execution-loop analysis for the core `SessionAgent` runtime and coordinator orchestration.

## 1) Repository touchpoints

Core files are:
- [main.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/main.go)
- [internal/cmd/root.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/cmd/root.go)
- [internal/app/app.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/app/app.go)
- [internal/agent/coordinator.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/coordinator.go)
- [internal/agent/agent.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agent.go)
- [internal/agent/loop_detection.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/loop_detection.go)
- [internal/agent/agent_tool.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agent_tool.go)
- [internal/agent/agentic_fetch_tool.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agentic_fetch_tool.go)
- [internal/session/session.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/session/session.go)
- [internal/message/message.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/message/message.go)
- [internal/permission/permission.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/permission/permission.go)
- [internal/ui/model/ui.go](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/ui/model/ui.go)

## 2) Orchestration stack

`main.go` starts CLI and `cmd/root.go` wires into app construction. Interactive and non-interactive modes diverge briefly and then converge on `AgentCoordinator.Run`.

Interactive path (`crush` without `run`) flows through app bootstrap and Bubbletea UI in `app.New` and `ui.New`.

Non-interactive path (`crush run`) flows through `App.RunNonInteractive`, which creates a dedicated session, auto-approves permissions for that session, and invokes `AgentCoordinator.Run` in a goroutine while streaming assistant output from `message` events.

The active execution boundary is:

**Coordinator policy plane → SessionAgent runtime plane → fantasy stream callbacks → DB message/session services → UI/pubsub/event plane.**

## 3) Core runtime: `SessionAgent.Run` execution sequence

This is the actual loop for `charmbracelet/crush`.

1. Preflight checks. `SessionAgent.Run` rejects empty user prompt/attachments and missing session id immediately (`ErrEmptyPrompt`, `ErrSessionMissing`). See validation at [internal/agent/agent.go#L148-L154](https://github.com/charmbracelet/crush/blob/8bcca78520e5dd082bdf254a4a915a1505bc5c29/internal/agent/agent.go#L148-L154).
2. Per-session single-flight admission. If `IsSessionBusy(sessionID)` is true, the call is appended into `messageQueue[sessionID]` and returns `nil` without executing. Queue replay is handled after the current run finalizes.
3. State snapshot. Mutable state is copied under lock from `csync` containers: `largeModel`, `tool list`, prompt text, and prefix.
4. MCP instruction merge. Connected MCP servers contribute `<mcp-instructions>` into the system prompt (`agent.go:174-186`).
5. Tool/agent setup. The model is wrapped with `fantasy.NewAgent(...)` and tool cache-control is applied to the last tool message.
6. Context hydration. Session metadata is loaded by id, prior messages are reconstructed via `getSessionMessages`, and title generation is spawned asynchronously on first-message sessions.
7. Incoming user payload is persisted as a `message.User` row via `createUserMessage` before any model call.
8. Execution context is injected with runtime metadata keys for downstream tools: session id, assistant message id, image support, model name (`tools.SessionIDContextKey`, `tools.MessageIDContextKey`, etc.).
9. `activeRequests` stores a cancel function for the session so `Cancel(sessionID)` can abort run-time processing.
10. `agent.Stream(...)` is executed with rich callbacks and stop conditions.
11. `event.PromptSent` fires before stream starts; `event.PromptResponded` fires once stream returns.

If `SessionAgent.Run` is called recursively from queue replay, the same method is re-entered after current run cleanup. The queue is only popped once the currently executing stream has exited.

## 4) `PrepareStep`: first-step mutation and request shaping

`PrepareStep` is the only point where the assistant step list is rewritten before model execution per request.

It prepends queued prompts, injects queued user prompts from `messageQueue` directly as user turns, applies provider media workaround, injects prefix system message and creates an assistant message placeholder.

Queue injection is FIFO because `append` preserves call order and the queued slice is deleted before continuing. This guarantees earlier calls are replayed in arrival order before continuing the active request.

The provider-media workaround is conditional: `workaroundProviderMediaLimitations` rewrites tool message media into user attachment messages for providers that are not Anthropic/Bedrock-compatible (`agent.go:1077-1143`).

Cache-control options are attached to the latest messages (`agent.go:271-285`) so only trailing context carries hints.

For every run invocation, a fresh assistant DB message is created in `PrepareStep` (`agent.go:291-299`) and request context is updated with `assistantMsg.ID`, `supports_images`, and model name.

## 5) Callback state machine inside `agent.Stream`

`SessionAgent.Run` registers multiple callbacks; together they describe the observable loop.

`OnReasoningStart`, `OnReasoningDelta`, `OnReasoningEnd` append reasoning and signatures into the active assistant message. `OnTextDelta` appends user-visible text, with a leading newline trim on first delta to avoid visible gap.

`OnToolInputStart` creates a tool call record with `ProviderExecuted=false` and `Finished=false`. `OnToolCall` marks the same call as finished. This split exists because tool-call streaming can begin with partial input before completion.

`OnToolResult` persists a `tool`-role message by converting fantasy tool result content into internal `message.ToolResult` shape.

`OnStepFinish` maps fantasy reasons (`stop`, `length`, `tool_calls`) into internal `finish` reasons, reloads the session from storage, updates usage/cost, and persists session state.

`StopWhen` enforces two stop guards:

1. context-pressure guard using `closeToContextWindow` logic (`largeContextWindowThreshold=200000`, `largeContextWindowBuffer=20000`, `smallContextWindowRatio=0.2`) and compares `currentSession.PromptTokens + currentSession.CompletionTokens` against threshold
2. loop guard via `hasRepeatedToolCalls` with `loopDetectionWindowSize=10`, `loopDetectionMaxRepeats=5`

The loop guard computes SHA-256 signatures over tool-call/value pairs in trailing steps. Signatures are hash of ordered `(tool name, input, output)` per call; it ignores non-tool steps. Tests confirm repeated pattern must exceed `maxRepeats` (6 of 10 triggers, 5 does not).

## 6) Error finalization and completion semantics

If streaming returns an error or cancellation, the runtime performs deterministic finalization:

- active reasoning is closed (`FinishThinking`).
- every unfinished tool call in the in-memory assistant message is marked finished.
- for each unfinished tool call with no persisted result, a synthetic `message.ToolResult` is appended. Error body changes by cause:
  - `context.Canceled` → "Tool execution canceled by user"
  - `permission.ErrorPermissionDenied` → "User denied permission"
  - otherwise generic provider/tooling error
- assistant finish state is appended with a concrete title/reason pair.

A notable branch: canceled context on summarize deletes a partially created summary message in `Summarize` (`agent.go:641-643`) to avoid orphaned summary rows.

`ErrRequestCancelled` is surfaced via `agent.ErrRequestCancelled` in app-level non-interactive flow handling.

## 7) Summarize + queued replay phase

When context pressure triggers `shouldSummarize`, active request context is removed from `activeRequests` first, then `Summarize` runs with `summaryPrompt` and provider options.

`Summarize` creates a dedicated assistant summary message (`IsSummaryMessage=true`), streams reasoning + text into it, records usage, sets `SummaryMessageID`, and resets prompt tokens (`currentSession.PromptTokens=0`) while retaining completion metrics for the summary turn.

If stream ended mid-tooling and unfinished calls remain, `Run` enqueues a synthetic follow-up prompt referencing the interruption; that prompt is then part of the normal run flow.

After normal completion, active requests are removed, queue length is checked, and the oldest queued request is recursively re-run. This gives deterministic per-session FIFO replay.

## 8) Coordinator: policy and model/tool lifecycle

`Coordinator.Run` first waits for agent bootstrap (`readyWg.Wait`) and then refreshes models before each call.

`UpdateModels` rebuilds the large/small model bindings and re-runs tool construction against latest config, then swaps them onto the existing `SessionAgent` with `SetModels` and `SetTools`.

Unauthorized provider responses are retried once by refreshing OAuth token or resolving API key templates (`isUnauthorized` + `refreshOAuth2Token` / `refreshApiKeyTemplate`). Retry path is call-local to keep the run loop stable.

`buildTools` assembles toolset in phases: conditional `agent`/`agentic_fetch` add-ons, core tools, LSP tools (if configured or auto), MCP tools, then allowed-tool filtering. MCP filtering supports per-agent allowlists (`AllowedMCP`) and short-circuits to zero MCP tools when `AllowedMCP` is empty.

Provider creation in `buildProvider` handles per-config provider-specific construction. For anthropic-thinking models, special beta header injection occurs before request dispatch.

## 9) Sub-agent and nested tool execution

`agent` and `agentic_fetch` tools both route into `runSubAgent`. `runSubAgent` creates a task session and invokes another `SessionAgent.Run` with a derived session id.

Child session id format is `messageID$$toolCallID` via `CreateAgentToolSessionID` in `session/service`.

`runSubAgent` passes explicit model settings, call options, and max tokens into the child run, then accumulates child cost into the parent using `updateParentSessionCost`.

`Session.IsAgentToolSession` and `ParseAgentToolSessionID` allow UI components to map child events back to parent tool call IDs for nested tool rendering.

`agentic_fetch` differs by constructing its own temporary workspace, building an explicit web-fetch/search prompt, and auto-approving permission for that child session. It creates sub-agent-only tool lists (`web_fetch`, `web_search`, `glob`, `grep`, `sourcegraph`, `view`) and runs both large/small bindings as the selected small model.

Coordinator tests verify `runSubAgent` returns an `isError` `ToolResponse` instead of a Go error when the child agent returns an execution error, and verify cost propagation to parent sessions.

## 10) Session and message persistence contracts

`session.CreateTaskSession` stores `ParentSessionID` and a custom ID supplied by the caller, enabling hierarchical sessions for tools.

`message.Create` auto-appends `Finish{Reason:"stop"}` to every non-assistant role (`internal/message/message.go:62-67`). This means raw tool or user messages are always terminated consistently for renderer assumptions.

`getSessionMessages` handles summary truncation: if `SummaryMessageID` exists it starts replay from summary onward and rewrites summary message role to user context.

`Summarize` and title generation both persist usage, and costs are aggregated with token-cost formulas; OpenRouter override costs are used when provider metadata exposes explicit usage cost.

## 11) Permission and cancellation behavior in execution path

Tool calls route permission checks through `permissions.Request`, which publishes UI-visible requests and then blocks on a response channel.

`AutoApproveSession` short-circuits all future requests for that session. non-interactive mode sets this for the run session immediately.

Request handling is serialized by `requestMu` and supports allowlist shortcuts by exact key (`tool:action`) or tool name. Per-session remembered grants are applied for repeated path-level actions.

Cancellation behavior:

`Cancel(sessionID)` cancels the active stream and active summarize stream if present, then clears queued calls.

`CancelAll` iterates all active keys and waits up to 5 seconds for shutdown, with polling on `IsBusy()`.

## 12) Failure modes to watch

The loop intentionally prefers bounded failure behavior over unbounded autonomy.

1) Context pressure is the normal interruption mechanism, not hard failure.
2) Tool loop repetition detection caps runaway tool-only cycles.
3) Provider/tool runtime errors are converted into durable tool result messages so the model can observe failures.
4) If active run dies before assistant creation, user message is still persisted but no assistant placeholder exists, so UI sees last completed message boundary.
5) Queue is in-memory only; process restart loses queued calls.

## 13) UI propagation for nested tool runs

The main session chat loads message items from `sessionID` and calls `loadNestedToolCalls` recursively for tool items that implement `NestedToolContainer`.

For each agent-like tool call, UI builds `messageID$$toolCallID` and loads that nested session, maps nested tool call history, and attaches it back into the parent item tree.

`handleChildSessionMessage` handles live child events, parses child session ids, validates tool-call ownership, and updates nested tool call/result lists in place.

This is how tool call tool-calls are visualized as expandable nested traces rather than flat tool chatter.

## 14) Why the loop is structured this way

The loop is not just “LLM in, tool results out.” It is a bounded, replay-capable state machine: admission control, deterministic queue semantics, callback persistence on every observable event, bounded context safety, loop protection, durable failure synthesis, and parent/child cost accounting.

The architecture is intentionally split so policy and orchestration (`Coordinator`) stay mutable per config while execution and durability (`SessionAgent`) stay stable and queue-safe.
