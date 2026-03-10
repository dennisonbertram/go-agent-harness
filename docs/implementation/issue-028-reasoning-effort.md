# Issue #28: Stream thinking/reasoning content from thinking models

## Status: DONE

## Summary

Added `reasoning_effort` request-side configuration for o-series reasoning models. The streaming of reasoning/thinking content via `assistant.thinking.delta` events was already implemented; this completes the missing piece: the ability to configure the reasoning budget per-run.

## What Was Already Done

- Provider (`client.go`): `chatCompletionMessageDelta.ReasoningContent` parses `reasoning_content` from streaming chunks and emits `CompletionDelta{Reasoning: ...}`.
- Runner (`runner.go`): `emitCompletionDelta` already emitted `EventAssistantThinkingDelta` when `delta.Reasoning != ""`.
- Event system (`events.go`): `EventAssistantThinkingDelta` already existed.
- Tests existed for both provider and runner paths.

## What This PR Adds

### 1. `CompletionRequest.ReasoningEffort` (`internal/harness/types.go`)

```go
ReasoningEffort string `json:"reasoning_effort,omitempty"`
```

Controls the thinking budget forwarded to the provider. For OpenAI o-series models, valid values are `"low"`, `"medium"`, `"high"`. Empty means provider default.

### 2. `RunRequest.ReasoningEffort` (`internal/harness/types.go`)

```go
ReasoningEffort string `json:"reasoning_effort,omitempty"`
```

Exposed in the HTTP API so callers can set the reasoning budget per-run via JSON body.

### 3. Runner wiring (`internal/harness/runner.go`)

The `execute` function now copies `req.ReasoningEffort` into every `CompletionRequest` built during the run loop.

### 4. OpenAI provider (`internal/provider/openai/client.go`)

Added `ReasoningEffort string` to `completionRequest` with `json:"reasoning_effort,omitempty"` so it is serialized to the OpenAI API when set, and absent from the request body when empty.

## Tests Added

- `TestClientPassesReasoningEffortToProvider` — verifies `reasoning_effort:"high"` appears in the HTTP request body sent to OpenAI.
- `TestClientOmitsReasoningEffortWhenEmpty` — verifies the field is absent from the body when not set.
- `TestRunnerPassesReasoningEffortToProvider` — verifies `RunRequest.ReasoningEffort = "medium"` reaches `CompletionRequest.ReasoningEffort`.
- `TestRunnerOmitsReasoningEffortWhenNotSet` — verifies empty `ReasoningEffort` is not injected by the runner.

## Files Changed

- `internal/harness/types.go` — added `ReasoningEffort` to `CompletionRequest` and `RunRequest`
- `internal/harness/runner.go` — wire `req.ReasoningEffort` into `completionReq`
- `internal/provider/openai/client.go` — added `ReasoningEffort` to `completionRequest`, wire from `req.ReasoningEffort`
- `internal/harness/runner_test.go` — 2 new tests
- `internal/provider/openai/client_test.go` — 2 new tests

## Test Results

```
ok  go-agent-harness/internal/harness       2.023s
ok  go-agent-harness/internal/provider/openai  1.409s
```

All packages pass. Only pre-existing `demo-cli` build failure (unrelated).
