# Issue #136: Mid-Run Model Switching

**Status:** Design Research
**Filed:** 2026-03-18
**Related:** `internal/harness/runner.go`, `internal/harness/types.go`, `internal/config/config.go`

---

## Summary

The runner loop currently resolves its primary model once at the start of `execute()` and holds it fixed for the entire run. This document researches the mechanism needed to allow callers and the runner itself to switch models between LLM steps — for example, routing tool-heavy steps to a cheaper model and reserving a more capable model for planning steps.

---

## 1. Current Architecture — How the Model Is Fixed Today

### 1.1 Resolution at run start

Model selection happens in `execute()` in three sequential phases:

```go
// internal/harness/runner.go, inside execute()

// Phase 1: resolve the nominal model string
model := req.Model
if model == "" {
    model = r.config.DefaultModel      // falls back to "gpt-4.1-mini"
}

// Phase 2: resolve RoleModels overrides
roleModels := r.resolveRoleModels(req)
primaryModel := model
if roleModels.Primary != "" {
    primaryModel = roleModels.Primary  // config-level or per-request Primary override
}

// Phase 3: resolve the provider that will serve this model
activeProvider, providerName, err := r.resolveProvider(
    runID, model, req.ProviderName, req.AllowFallback)
```

After this triple resolution, both `primaryModel` (the model identifier string) and `activeProvider` (the `Provider` interface value) are captured as **local variables** in `execute()`. They never change for the lifetime of the run.

### 1.2 Where primaryModel is consumed

`primaryModel` is passed verbatim into every `CompletionRequest` built in the step loop:

```go
completionReq := CompletionRequest{
    Model:           primaryModel,   // same string every step
    Messages:        turnMessages,
    Tools:           r.filteredToolsForRun(runID),
    ReasoningEffort: req.ReasoningEffort,
    ...
}
```

The provider's `Complete()` method receives this model string and forwards it to the API. Both the OpenAI and Anthropic clients honour `req.Model` over their own config-level default:

```go
// openai/client.go
model := req.Model
if model == "" {
    model = c.model
}

// anthropic/client.go
model := req.Model
if model == "" {
    model = c.model
}
```

So the provider itself has no model lock-in; it already supports per-call model overrides. The issue is purely that `execute()` never changes the `primaryModel` local variable.

### 1.3 Existing partial mechanism: RoleModels

The system already has a lightweight two-role model scheme (`RoleModels`) that allows a separate model for compaction/summarisation:

```go
// internal/harness/types.go
type RoleModels struct {
    Primary    string `json:"primary,omitempty"`    // main step loop
    Summarizer string `json:"summarizer,omitempty"` // context compaction
}
```

`RoleModels` is set once at run start from `RunRequest.RoleModels` (per-request) or `RunnerConfig.RoleModels` (server-level). It solves the fixed-role problem for two known roles, but it is not dynamic — the Primary cannot change between steps, and there is no mechanism to add new roles at runtime.

### 1.4 Existing partial mechanism: SteerRun

The `SteerRun` API allows injecting a text string as a user message between LLM steps:

```go
func (r *Runner) SteerRun(runID, message string) error {
    // writes to state.steeringCh (buffered cap=10)
}
```

This influences LLM reasoning but does not change the model used for subsequent calls. A natural extension would be a `SteerModel` variant, but the current architecture needs changes to support it.

### 1.5 Provider is resolved once

A subtle issue: even if `primaryModel` were mutable, the `activeProvider` variable in `execute()` might be the wrong provider for the new model. For example, switching from `gpt-4.1-mini` (OpenAI) to `claude-opus-4-6` (Anthropic) mid-run requires the runner to also swap providers. `resolveProvider()` handles provider resolution from the catalog but it is currently called only once.

---

## 2. Message History Compatibility Across Providers

### 2.1 Internal message format

The runner stores all conversation turns in `[]harness.Message` using a normalised provider-agnostic schema:

```go
type Message struct {
    Role             string     // "user" | "assistant" | "tool" | "system"
    Content          string
    ToolCalls        []ToolCall // assistant -> tool calls
    ToolCallID       string     // tool -> which call this result is for
    Name             string
    IsMeta           bool
    IsCompactSummary bool
    CorrelationID    string
    ConversationID   string
    Reasoning        string
}
```

This schema is intentionally provider-neutral: it captures the same semantic information that both OpenAI and Anthropic APIs need, and each client's `mapMessages()` function translates it to provider-specific wire format at call time.

### 2.2 OpenAI message translation

`openai.mapMessages()` performs a direct structural mapping:

- `user` → `{"role":"user","content":"..."}`
- `assistant` with ToolCalls → `{"role":"assistant","tool_calls":[...]}`
- `tool` → `{"role":"tool","content":"...","tool_call_id":"..."}`
- `system` → `{"role":"system","content":"..."}`

### 2.3 Anthropic message translation

`anthropic.mapMessages()` applies more transformation:

- `system` messages are **extracted** from the list and placed in a top-level `system` field
- `tool` role messages are remapped to `user` role with a `tool_result` content block
- Consecutive same-role messages must be **merged** (Anthropic requires strict user/assistant alternation)
- Tool call arguments are re-serialised as `input` JSON in `tool_use` content blocks

This means a message history produced by an OpenAI-backed run (with `tool_call_id` on result messages) is correctly handled when translated to Anthropic format — the `mapMessages()` function handles the semantic remapping.

### 2.4 Cross-provider switch safety

**Within a single run, switching from OpenAI to Anthropic mid-run is theoretically safe at the message history level.** The internal `[]harness.Message` slice is provider-agnostic, and each provider's client translates it freshly on every call. Neither client stores history internally; both are stateless with respect to conversation history.

**However, there is one structural concern:** tool call IDs. OpenAI uses UUIDs for tool call IDs. Anthropic uses its own format. When a history created under OpenAI (with OpenAI-format tool call IDs) is replayed to Anthropic, Anthropic's `mapMessages()` embeds those IDs inside `tool_use`/`tool_result` content blocks as opaque strings. This is valid — Anthropic only requires that IDs match within a call pair; it does not validate the format externally. Empirically this is safe but should be tested.

**Cross-provider mid-run switching is lower priority than same-provider model tier switching** (e.g., `gpt-4.1` → `gpt-4.1-mini`), which carries zero compatibility risk.

---

## 3. Proposed Per-Step Model Resolution

### 3.1 Core change: mutable model on runState

The simplest structural change is to add a `currentModel` field to `runState` that `execute()` reads at the top of each step:

```go
// proposed addition to runState
type runState struct {
    // ... existing fields ...

    // currentModel is the model used for the next LLM step.
    // Set at run start from req.Model / DefaultModel / RoleModels.Primary.
    // Can be overridden mid-run via ModelOverride steering message or PATCH endpoint.
    currentModel string

    // currentProviderOverride, if non-empty, overrides provider resolution for
    // the next LLM step. Useful when switching to a model from a different provider.
    currentProviderOverride string
}
```

In `execute()`, instead of capturing `primaryModel` as a local variable, the step loop reads from `state.currentModel`:

```go
for step := 1; ...; step++ {
    // Read current model from runState so mid-run switches take effect.
    r.mu.RLock()
    st := r.runs[runID]
    stepModel := st.currentModel
    stepProviderOverride := st.currentProviderOverride
    r.mu.RUnlock()

    // Resolve provider for this step (may differ from the initial provider).
    stepProvider, _, err := r.resolveProvider(runID, stepModel, stepProviderOverride, req.AllowFallback)
    ...
    completionReq := CompletionRequest{
        Model:    stepModel,
        ...
    }
    result, err := stepProvider.Complete(ctx, completionReq)
    ...
}
```

`r.resolveProvider()` is already idempotent and cheap (it does a catalog lookup, not an HTTP call). Calling it once per step adds negligible overhead.

### 3.2 Initialisation

At the start of `execute()`, `currentModel` is set from the same logic that currently sets the `primaryModel` local variable:

```go
r.mu.Lock()
if state, ok := r.runs[runID]; ok {
    state.currentModel = primaryModel
}
r.mu.Unlock()
```

No change to the `RoleModels.Primary` logic — it simply sets the initial `currentModel`.

### 3.3 Mid-turn vs between-turn boundary

Model switches should take effect **between turns only** — i.e., at the step boundary, after the prior step's tool results have been appended to history. Attempting a mid-turn switch (while a streaming `Complete()` call is in flight) would require interrupting the HTTP stream, which is complex and not worth the complexity.

The proposed design enforces between-turn semantics automatically because `execute()` reads `state.currentModel` at the top of each step, after the previous step's tool calls have completed.

---

## 4. API Design

Three surfaces are considered:

### 4.1 Option A: Extend SteerRun with a model directive (recommended for Phase 1)

The existing `POST /v1/runs/{id}/steer` endpoint accepts a JSON body with a `message` field. The simplest extension is to add an optional `model` field:

```json
POST /v1/runs/{id}/steer
{
  "message": "optional user text",
  "model": "gpt-4.1-mini"
}
```

If `model` is non-empty and `message` is empty, the body is a pure model switch with no user-visible text injection. If both are set, the model changes and the message is injected simultaneously.

**Runner-level change:**

```go
// SteerRun gains an optional model parameter
func (r *Runner) SteerRun(runID, message, model string) error { ... }

// steeringCh carries a directive struct instead of a plain string
type steeringDirective struct {
    Message string
    Model   string
}
```

**Emitted event:** `model.switched` (new EventType) with fields `step`, `from_model`, `to_model`, `reason: "steer"`.

**HTTP handler change:**

```go
var req struct {
    Message string `json:"message"`
    Model   string `json:"model,omitempty"`
}
```

### 4.2 Option B: New PATCH endpoint

A new `PATCH /v1/runs/{id}` endpoint (or `POST /v1/runs/{id}/model`) allows richer mutation semantics:

```json
PATCH /v1/runs/{id}
{
  "model": "gpt-4.1-mini",
  "reasoning_effort": "low"
}
```

This is cleaner from a REST perspective but requires routing changes and a new handler. It also gives room to add other mutable run-time fields (e.g., `max_steps`, `reasoning_effort`) in the future.

### 4.3 Option C: Model schedule in RunRequest (compile-time, not dynamic)

A `ModelSchedule` field on `RunRequest` allows pre-declaring model changes at specific step numbers:

```json
{
  "prompt": "...",
  "model": "gpt-4.1",
  "model_schedule": [
    {"from_step": 3, "model": "gpt-4.1-mini"},
    {"from_step": 8, "model": "gpt-4.1"}
  ]
}
```

This is useful for budget-capped workflows with predictable step profiles but is inflexible for dynamic workloads. It can be layered on top of the per-step model resolution proposed in §3 without API changes to SteerRun.

**Recommendation:** Implement Option A first (extending SteerRun). It reuses the existing steering channel infrastructure, requires the smallest diff, and is already familiar to callers. Option C can be added as a compile-time shorthand on top of the same per-step resolution. Option B is worth adding later as part of a broader run mutation API.

---

## 5. Cost Accounting Implications

### 5.1 Current accounting

`recordAccounting()` accumulates `CostUSDTotal` across steps using pricing data returned by the provider in `CompletionResult.Cost` or resolved from the pricing catalog:

```go
state.costTotals.CostUSDTotal += turnCostUSD
state.costTotals.LastTurnCostUSD = turnCostUSD
state.costTotals.CostStatus = costStatus
```

Cost per step comes from `result.CostUSD` (if the provider returns it) or from `pricing.Resolver` (catalog-based). Both paths use the model string in `CompletionResult` to look up pricing.

### 5.2 Mid-run model switch impact

Because pricing is resolved per-step from `CompletionResult`, model switches are **already handled correctly** at the pricing level — each step's cost reflects the model actually called, not the run's initial model. No changes to `recordAccounting()` are needed.

### 5.3 Audit trail and forensics

The audit trail currently writes the model at `run.started` time. After a mid-run switch, the audit log would be incomplete unless we also write a `model.switched` event into the hash chain. The existing `AuditRecord` struct supports arbitrary `Payload` maps, so this is straightforward.

The `llm.request.snapshot` forensic event does not currently include the model string. It should be added (`"model": stepModel`) when the mid-run switch feature is implemented.

### 5.4 Cost ceiling enforcement

The per-run cost ceiling check (`r.exceedsCostCeiling()`) is model-agnostic — it compares `state.costTotals.CostUSDTotal` against `state.maxCostUSD`. It will continue to work correctly regardless of which model is active for any given step.

### 5.5 Usage totals aggregation and cross-model attribution

`RunUsageTotals` accumulates `PromptTokensTotal`, `CompletionTokensTotal`, etc., as a single aggregate across the entire run. When models switch mid-run, different models may have different context window sizes and pricing tiers, so aggregate token counts are less meaningful in isolation.

**Recommended enhancement:** Add a `ModelBreakdown []ModelStepCost` field to `RunCostTotals` (emitted in `usage.delta` and `run.completed` events) that records per-model token and cost subtotals. This is optional for Phase 1.

---

## 6. Implementation Plan

### Phase 1: Minimal per-step model resolution (runner-only change)

1. Add `currentModel string` and `currentProviderOverride string` to `runState`.
2. Set `state.currentModel = primaryModel` at the start of `execute()` after the existing resolution.
3. Change the step loop to read `stepModel` and `stepProvider` from `state.currentModel` at the top of each iteration.
4. Add a new `EventModelSwitched` (`model.switched`) EventType with payload `{step, from_model, to_model, reason}`.
5. Write unit tests: single-step model switch, switch back, switch with provider change.

**Estimated diff:** ~60 lines in `runner.go`, ~10 lines in `events.go`, ~100 lines of tests.

### Phase 2: Extend SteerRun / steeringCh

1. Change `steeringCh` element type from `string` to a `steeringDirective` struct.
2. Update `drainSteering()` to apply model overrides from the directive to `state.currentModel`.
3. Extend `SteerRun()` to accept a `model string` parameter.
4. Update `handleRunSteer()` HTTP handler to accept and forward the `model` field.
5. Emit `model.switched` event from `drainSteering()`.

**Estimated diff:** ~80 lines across runner, server, and types.

### Phase 3: ModelSchedule on RunRequest (optional compile-time scheduling)

1. Add `ModelSchedule []ModelScheduleEntry` to `RunRequest`.
2. At the top of each step in `execute()`, after reading `state.currentModel`, check if any schedule entry's `FromStep` matches `step` and apply the override.
3. Schedule entries fire at-most-once and emit `model.switched` events.

**Estimated diff:** ~50 lines.

### Phase 4: Monitoring enhancements

1. Add model string to `llm.request.snapshot` payload.
2. Add `model.breakdown` sub-object to `usage.delta` and `run.completed` payloads.
3. Persist `ModelBreakdown` to the JSONL rollout record.

---

## 7. Recommendation

**Implement Phase 1 and Phase 2 together as a single PR.** The structural change (mutable `currentModel` on `runState`) is small and non-breaking. Extending `SteerRun` gives immediate utility. The key design invariants to preserve:

- Model switches are always between-turn (never mid-stream).
- Provider resolution is called per-step when `currentModel` changes, but remains cheap (catalog lookup only).
- Cost accounting requires no change — pricing is already per-step.
- Message history is provider-agnostic; cross-provider switches are safe for OpenAI→Anthropic but require validation testing.

The `RoleModels.Primary` mechanism should be kept as-is and treated as an "initial model" convenience. Once Phase 1 lands, `RoleModels.Primary` simply sets the initial value of `state.currentModel`.

**Risk:** The only non-trivial risk is cross-provider mid-run switching (e.g., OpenAI → Anthropic in a session with accumulated tool call history). The message translation layer handles this today, but the tool call ID format difference should be explicitly tested with a fixture that has multiple tool call/result pairs.

---

## Appendix: Relevant Code Locations

| Component | File | Lines |
|-----------|------|-------|
| Model resolution at execute start | `internal/harness/runner.go` | ~1107-1118 |
| primaryModel captured and used in step loop | `internal/harness/runner.go` | ~1115, 1465 |
| RoleModels resolution | `internal/harness/runner.go` | ~4373-4388 |
| SteerRun implementation | `internal/harness/runner.go` | ~820-853 |
| drainSteering step-boundary inject | `internal/harness/runner.go` | ~2607-2629 |
| CompletionRequest.Model | `internal/harness/types.go` | 75 |
| RoleModels struct | `internal/harness/types.go` | 317-320 |
| RunRequest.RoleModels | `internal/harness/types.go` | 302-305 |
| RunnerConfig.DefaultModel | `internal/harness/types.go` | 323 |
| OpenAI model passthrough | `internal/provider/openai/client.go` | ~78-80 |
| Anthropic model passthrough | `internal/provider/anthropic/client.go` | ~77-80 |
| Anthropic message translation | `internal/provider/anthropic/client.go` | ~474-580 |
| recordAccounting (cost per step) | `internal/harness/runner.go` | ~2933-2976 |
| Config.Model (TOML / env layer) | `internal/config/config.go` | 134, 472-474 |
| EventTypes list | `internal/harness/events.go` | 14-220+ |
