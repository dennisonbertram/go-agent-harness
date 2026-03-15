# Issue #136: Mid-Run Model Switching — Technical Analysis

Date: 2026-03-14
Status: Research complete
Related: #25 (role-based routing), #11 (multi-provider)

---

## 1. Current State: How Model Selection Works Today

### 1.1 Model Resolution Flow

Model selection is a three-layer cascade at run start:

1. **Environment variable**: `HARNESS_MODEL` sets `RunnerConfig.DefaultModel` via `applyEnvLayer()` in `internal/config/config.go:269`
2. **RunRequest field**: `RunRequest.Model` overrides the default for that specific run (optional)
3. **execute() resolution**: `internal/harness/runner.go:870-873`
   ```go
   model := req.Model
   if model == "" {
       model = r.config.DefaultModel
   }
   ```

### 1.2 Model is Per-Run, Not Per-Step

The resolved model is a **single local variable** inside `execute()` that is captured once before the step loop begins and never updated:

```go
// runner.go:870-879
model := req.Model
if model == "" {
    model = r.config.DefaultModel
}

activeProvider, providerName, err := r.resolveProvider(runID, model, req.ProviderName, req.AllowFallback)
...

for step := 1; effectiveMaxSteps == 0 || step <= effectiveMaxSteps; step++ {
    // model and activeProvider are closed over, never re-evaluated
    completionReq := CompletionRequest{
        Model:    model,        // same model every step
        Messages: turnMessages,
        ...
    }
    result, err := activeProvider.Complete(context.Background(), completionReq)
```

The `model` variable and `activeProvider` are resolved **once** before the loop and used for **all steps** of the run. There is no mechanism to change either mid-loop.

### 1.3 Provider Resolution

`resolveProvider()` (`runner.go:770-820`) handles multi-provider selection:
- If `req.ProviderName` is set, that provider is tried first
- Otherwise auto-detection via `providerRegistry.GetClientForModel(model)` resolves which catalog provider owns that model
- With `AllowFallback=true`, failures fall back to `r.provider` (the default provider)

The `ProviderRegistry` (`internal/provider/catalog/registry.go`) has `GetClientForModel(modelID)` which searches all catalog providers for the model. Once resolved, the client is cached per-provider-name (not per-model). Switching to a different provider mid-run would require calling `resolveProvider()` again.

### 1.4 Token Cost Tracking

`recordAccounting()` (`runner.go:2402`) accumulates usage into `runState.usageTotals` and `runState.costTotals`. The cost values come from `CompletionResult.Cost` which is **populated by the provider client** (not by the runner). The `openai/client.go` populates cost from the pricing catalog keyed on the model name.

Critically: `usageTotalsAccumulator` is a simple sum of tokens across all steps with no per-step model attribution:

```go
// runner.go:2535-2556
func (a *usageTotalsAccumulator) add(turn CompletionUsage) {
    a.promptTokensTotal += turn.PromptTokens
    a.completionTokensTotal += turn.CompletionTokens
    ...
}
```

There is **no per-model breakdown** in the current accounting. `RunCostTotals` holds one `CostUSDTotal` float64 for the entire run.

### 1.5 What Happens If You Change Model Today

There is no API surface to change the model mid-run. The only mechanisms are:
- `SteerRun()`: injects a user message — cannot change model
- `ContinueRun()`: starts a new run, **inherits `existingModel`** from the source run (runner.go:507, 527, 579)

If a caller wanted to use a different model, they would need to start an entirely new run with `StartRun()` passing the new model. Context from the previous run could be threaded through via `ConversationID`, but it would be a new run with a new run ID.

### 1.6 The `list_models` Tool Exists

`internal/harness/tools/list_models.go` exposes a `list_models` tool that lets agents query available models (filter by capability, cost tier, speed tier, etc.). This is scaffolding for agent-aware model selection — the agent can **discover** what models exist but cannot act on that knowledge to switch.

---

## 2. Use Cases for Mid-Run Switching

### 2.1 Cost Optimization (Strongest Case)
Use a cheap model (gpt-5-mini, deepseek-chat) for tool-heavy steps where the LLM is just orchestrating tool calls. Switch to a capable model (gpt-4.1, o3) only for complex reasoning steps. The `omp` fork (oh-my-pi) implements this with named roles: `default`, `smol`, `slow`, `plan`, `commit`. Research (#25) estimates 3-5x cost reduction at scale.

### 2.2 Capability Routing
Route different subtasks to models optimized for them:
- Code generation: a code-specialist model
- Summarization/memory: a fast cheap model
- Planning: a reasoning model with o-series mode

This aligns with the existing `strengths` and `best_for` fields in the catalog's `Model` struct.

### 2.3 Fallback on Failure
If the primary model returns an error (rate limit, context overflow, safety rejection), fall back to a backup model automatically without ending the run. `AllowFallback=true` already does a one-time fallback at run start, but not per-step.

### 2.4 Agent-Initiated Switching
The agent explicitly calls a `switch_model` tool or a model-selection tool after analyzing the upcoming task requirements. Requires agent cooperation and a tool API.

### 2.5 Context-Window-Triggered Switching
When the context window fills (triggering `EventContextWindowWarning`), automatically switch to a model with a larger context window. Currently compaction is used for this; model switching is an alternative.

---

## 3. Technical Approaches

### Approach A: Per-Step Model in `runState`

**Description**: Add a `currentModel` and `activeProvider` field to `runState`. The step loop reads these fields at the top of each iteration instead of using closed-over local variables.

**Mechanism**:
- Add `currentModel string` and `activeProvider Provider` to `runState`
- Initialize from `req.Model` in `StartRun()`
- `execute()` reads `state.currentModel` at the top of each step (requires lock)
- External callers (e.g., a new `SwitchModel(runID, model)` method) can update these fields while the run is paused between steps

**Code touch points**:
- `internal/harness/runner.go` — execute() loop, runState struct, new `SwitchModel()` method
- `internal/harness/types.go` — new exported API surface if needed
- `internal/server/http.go` — new REST endpoint for mid-run model switch

**Integration with #232 fix**: The recent fix (issue #232) to re-read `state.messages` at the top of each step is the same pattern needed here. A `SwitchModel()` call between steps would be safe under this pattern.

**Pros**:
- Consistent with the message re-read pattern established by #232
- Clean state model — the runner owns model selection
- Works for both agent-initiated and external (API-triggered) switches
- No new tool needed; can be a REST endpoint like `POST /runs/{id}/switch-model`

**Cons**:
- Race window: switch happens between steps, but requires the run to be between steps
- Requires concurrent-safe write to runState under `r.mu`
- ContinueRun() propagation: a continuation needs to pick up the latest model, not the original

**Concurrency**: `r.mu.Lock()` before writing `currentModel`; execute() reads under `r.mu.RLock()` at top of each step. Same pattern as steering. Risk: if switch happens mid-step (during LLM call), next step sees the new model — clean boundary.

**Complexity**: Medium (1-2 days). Pattern is well-established in the codebase.

---

### Approach B: `switch_model` Tool

**Description**: A new tool named `switch_model` (or `select_model`) that the agent can call like any other tool. When the tool result is processed, the runner updates its active model and provider for the next step.

**Mechanism**:
- Add `switch_model` tool to `internal/harness/tools/`
- Tool handler stores the requested model in a context value or a special sentinel in the tool result
- After tool execution, the runner detects the sentinel and calls `resolveProvider()` with the new model
- The tool result message is added to the conversation history as usual

**Tool schema**:
```json
{
  "name": "switch_model",
  "parameters": {
    "model_id": "string",
    "reason": "string"
  }
}
```

**Code touch points**:
- New file `internal/harness/tools/switch_model.go`
- New description `internal/harness/tools/descriptions/switch_model.md`
- `execute()` tool dispatch loop: detect `switch_model` calls, update `model` and `activeProvider` local vars (or runState fields)
- `RunnerConfig` needs a flag to enable/disable this tool

**Pros**:
- Agent has full control — it decides when to switch based on task analysis
- No new API endpoints needed
- Integrates naturally with the existing `list_models` tool (agent can query, then switch)
- Conversation history naturally includes reasoning about the switch

**Cons**:
- Requires `list_models` to be available and the agent to reason about model selection
- Tool calling models that are mid-task may not proactively use it
- "Capability routing" use case (automatic cheap-for-simple) requires agent intelligence
- Tool schema versioning: what if requested model is not in the catalog?
- Security: an adversarial prompt could cause the agent to switch to an unexpected model

**Complexity**: Medium (1-2 days). Tool pattern is well-established.

---

### Approach C: Runner-Side Router / Pre-Message Hook

**Description**: A pre-message hook (`PreMessageHook`) that intercepts each `CompletionRequest` and routes it to a different model+provider based on the step context (current step number, message count, last tool used, etc.).

**Mechanism**:
- `PreMessageHook.Execute()` receives the `CompletionRequest` (which includes `Model`) and can return a `MutatedRequest` with a different model name
- The runner already calls `applyPreHooks()` before every `activeProvider.Complete()` call
- A routing hook would need access to the `ProviderRegistry` to resolve the new provider

**Current hook API** (`types.go:508-532`):
```go
type PreMessageHookInput struct {
    RunID   string
    Step    int
    Request CompletionRequest
}
type PreMessageHookResult struct {
    Action         HookAction
    Reason         string
    MutatedRequest *CompletionRequest
}
```

**Gap**: The hook mutates `CompletionRequest.Model` but does NOT change `activeProvider`. The runner calls `activeProvider.Complete(req)` — if `activeProvider` is still the original OpenAI client but `req.Model` now names a model from a different provider (e.g., Anthropic), the API call will fail with "model not found."

**Fix needed**: Either:
- Pass `activeProvider` through the hook (breaking change to hook interface), or
- Route the mutated request through `resolveProvider()` again after hook mutation

**Pros**:
- Transparent to the agent (no tool calls needed)
- Operator-controlled routing (cost optimization at the platform level)
- Composable with existing hook system
- Low agent surface area: routing logic lives in platform code

**Cons**:
- Requires extending the hook interface to also return a new provider (or the runner must re-resolve)
- Current `applyPreHooks()` does not re-resolve the provider after model mutation — requires runner-side changes
- No per-step cost breakdown by model unless accounting is extended
- Harder to debug: agent sees model switches but can't control them

**Complexity**: Medium-to-High (2-3 days). Requires hook interface extension plus runner integration.

---

### Approach D: Role-Based Routing (Related to #25)

**Description**: Extend the YAML prompt/model profile system with named "roles" (e.g., `default`, `smol`, `slow`). The runner selects which role (and therefore which model) to use based on the current step type, tool calls made, or prompt profile.

**Mechanism**:
- Add a `Roles` map to `RunnerConfig` (or model catalog) mapping role names to model IDs
- The prompt engine already has `AgentIntent` and `PromptProfile` — add a `ModelRole` concept
- `execute()` checks current step context against role selection rules to set `model` at the top of each step
- This is the `omp` architecture: `{default: gpt-4.1, smol: gpt-5-mini, slow: o3}`

**omp role example** (from research in `docs/research/pi-review.md`):
```
default -- primary model (most LLM calls)
smol    -- cost-effective (session titles, memory summarization)
slow    -- high-capability (hard problems, planning)
plan    -- planning
commit  -- commit message generation
```

**Relationship to #25**: Issue #25 is specifically this approach. Implementing it requires multi-provider support (#11) to be solid, which it now is via the `ProviderRegistry`.

**Pros**:
- Operator-controlled, predictable cost profile
- No agent intelligence required for routing
- Directly addresses the 3-5x cost reduction potential documented in #25
- Natural extension of existing prompt profile system
- Works with `observational memory` precedent (already uses gpt-5-nano for reflection)

**Cons**:
- Role selection heuristics are hard to get right: "which steps are 'smol' steps?"
- Requires careful tuning per use case to avoid quality degradation
- Shared context: history sent to all roles, so the context window concern applies to all models in the role roster
- YAML schema design needed; not trivial to make ergonomic

**Complexity**: Large (3-5 days for a complete implementation with config schema, role resolution, and accounting).

---

## 4. Pros/Cons Summary Table

| Dimension | A: Per-Step State | B: switch_model Tool | C: Router Hook | D: Role-Based |
|-----------|------------------|---------------------|----------------|---------------|
| Agent control | None (external API) | Full | None | None |
| Operator control | Yes (via API) | No | Yes | Yes |
| Transparency | Low | High | Low | Medium |
| Runner changes | Medium | Low | Medium | Medium |
| New API surface | Yes (endpoint) | Yes (tool) | No | Config |
| Per-step cost tracking | Required | Required | Required | Required |
| Provider re-resolution needed | Yes | Yes | Yes | Yes |
| Complexity | Medium | Medium | Med-High | Large |
| Addresses #25 | No | No | Partially | Yes |
| Time estimate | 1-2d | 1-2d | 2-3d | 3-5d |
| Risk level | Low | Low-Medium | Medium | Medium |

---

## 5. Compatibility Concerns

### 5.1 Does Switching Mid-Conversation Break Context?

All current providers supported by the harness (OpenAI, OpenAI-compatible, Deepseek) use the same `messages` array format. The `Message` struct in `types.go` is provider-agnostic. Switching from `gpt-4.1` to `deepseek-chat` mid-run preserves context because both use the same message array structure.

**Exception**: Models with different tool calling schemas. The catalog's `Model.ToolCalling` bool and `Model.ParallelToolCalls` bool indicate capability differences. A model that does not support tool calling cannot continue a run that has tool call results in the history.

**Recommendation**: Before switching, validate that the target model supports:
- Tool calling (if any tool calls are in the history)
- The same tool call message format (openai-compatible vs. non-standard)

### 5.2 Multi-Model Cost Tracking

The current `usageTotalsAccumulator` is a simple sum with no per-model breakdown. If model A costs $2/M tokens and model B costs $0.10/M tokens, the aggregated `CostUSDTotal` is still correct (each `CompletionResult.Cost` comes from the pricing catalog keyed on the model used for that step). However, the `RunSummary` has no way to report per-model cost breakdown.

**What needs to change**: `recordAccounting()` should optionally record a `perModelCosts map[string]float64` when mid-run switching is enabled. The `EventUsageDelta` payload should include the active model name for attribution.

### 5.3 Model-Specific Parameters

`CompletionRequest.ReasoningEffort` is currently set once from `req.ReasoningEffort` and applied to every step. When switching to an o-series model mid-run, the reasoning effort should be re-set. Similarly, switching away from an o-series model requires that reasoning effort is not sent.

**What needs to change**: If the target model has `ReasoningMode=false` in the catalog, the next `CompletionRequest` must send `ReasoningEffort=""`.

### 5.4 Context Window Mismatch

Switching models may change the effective context window. The `AutoCompact` logic uses `r.config.ModelContextWindow` which is a static config value. After a model switch, this value would be stale.

The registry has `MaxContextTokens(modelID)` — this should be called on every step when the model is dynamic.

### 5.5 System Prompt Compatibility

`resolveSystemPrompt()` takes the model name and passes it to the `PromptEngine.Resolve()` call. A model switch that changes the `model_profile` used for system prompt resolution would generate a different system prompt mid-run — likely undesirable. The resolved system prompt should remain fixed for the run (as it is today) even when the model changes, unless the operator explicitly opts into prompt re-resolution.

---

## 6. Implementation Risks

### 6.1 Provider Re-Resolution Required for Every Model Switch

Each model switch requires calling `resolveProvider()`. This is a catalog lookup + potential client construction (lazy, cached per provider). The risk: if the new model is not in any catalog provider, `resolveProvider()` fails. The run should continue with the current model (emit a warning event) rather than fail hard.

### 6.2 Race Condition: Switch During Active LLM Call

If using Approach A (per-step state), a `SwitchModel` call arriving during an active `activeProvider.Complete()` call must not take effect until the next step. The `r.mu` lock protects `runState` fields, but `activeProvider.Complete()` is called outside the lock. The pattern established by the steering channel (`steeringCh`) should be used: a pending model switch is queued and applied at the top of the next step iteration, not while an LLM call is in flight.

### 6.3 Conversation History Replay Compatibility

`ContinueRun()` re-uses the conversation history for the next run but inherits the model from the source run. If the source run ended on a switched model, the continuation should use that final model, not the model from `RunRequest`. This is currently handled correctly for the static case but needs care for multi-model runs.

### 6.4 Accounting Accuracy

If model A is switched to model B at step 5, and the run completes at step 10, the `RunSummary.TotalCostUSD` must be the sum of costs from both models. This is automatically correct because `CompletionResult.Cost` is already per-call and already aggregated correctly. The only gap is no per-model breakdown in the summary.

### 6.5 Tool Schema Delta Between Models

Different models may support different tool schemas (e.g., not all models support parallel tool calls). If the target model lacks `ToolCalling=true`, the `CompletionRequest.Tools` list must be empty. The `filteredToolsForRun()` call in execute() would need to also consider the active model's capabilities.

---

## 7. Art-of-State Review: Existing Model Routing Code

### 7.1 `list_models` Tool (Tier: Deferred)

File: `internal/harness/tools/list_models.go` and `internal/harness/tools/deferred/list_models.go`

An agent can call `list_models` with filters (`tool_calling`, `speed_tier`, `cost_tier`, `best_for`, etc.) and get back a catalog of available models. This is scaffolding for agent-directed model selection. The tool exists today; only the switching mechanism is missing.

### 7.2 `ProviderRegistry.GetClientForModel()`

`internal/provider/catalog/registry.go:98` — already supports resolving a provider from a model ID. Multi-provider is fully wired for run start. Mid-run switching needs only to call this at the top of each step instead of once.

### 7.3 Observational Memory Already Uses a Different Model

`internal/observationalmemory/` uses `gpt-5-nano` (or configurable cheap model) for memory reflection, independent of the run's primary model. This is an existing precedent for "use model X for this subtask, model Y for the main loop." The pattern is proven; extending it to the main loop is the next logical step.

### 7.4 Pre-Message Hooks Already Mutate `CompletionRequest`

`applyPreHooks()` in runner.go:1136 shows that mutation of the request before the LLM call is the existing pattern for operator-level intervention. A routing hook fits naturally here. The gap is only that provider re-resolution after model mutation is not yet implemented.

### 7.5 `ModelContextWindow` is a Static Config Value

`RunnerConfig.ModelContextWindow` is used by auto-compaction and context window snapshot. It does not adapt when the model changes. For a complete mid-run switching implementation, this must become dynamic (catalog lookup per step).

---

## 8. Dependency Analysis

| Dependency | Status | Required for Approach |
|-----------|--------|----------------------|
| Multi-provider registry (#11) | Done — `ProviderRegistry` complete | A, B, C, D |
| Model catalog (`catalog/`) | Done | A, B, C, D |
| `list_models` tool | Done | B (agent-initiated) |
| Pre-message hooks | Done | C |
| Per-model cost accounting extension | Missing | All |
| Dynamic context window lookup | Missing | All |
| `ReasoningEffort` per-model routing | Missing | All |
| YAML role config schema | Missing | D only |
| `switch_model` tool + description | Missing | B only |
| `SwitchModel()` runner method | Missing | A only |
| Routing hook interface extension | Missing | C only |

**Conclusion**: The infrastructure is 70% there. The multi-provider registry, catalog, and hook system all exist. What is missing is: (1) the mid-step model re-resolution logic in execute(), (2) per-model cost accounting, and (3) dynamic context window.

---

## 9. Recommendation

**Recommended approach: Approach B (switch_model tool) as the first step, then Approach D (role-based routing) as the follow-up.**

### Rationale

**Approach B first** because:
1. It is the lowest-risk change: tools are an established pattern with clear isolation
2. It gives agents immediate capability to opt into model switching where they want it
3. The `list_models` tool already exists — adding `switch_model` completes the pair
4. No changes to execute()'s main loop structure are required beyond reading the tool result
5. The agent's reasoning about why it switched is preserved in the conversation history, which is valuable for debugging and observability
6. `runState` already needs a writable `currentModel` field for this to work — that one field addition enables all four approaches

**Approach D (role-based routing) second** because:
1. It addresses the highest-value use case (3-5x cost reduction at scale) per issue #25
2. It requires the same `currentModel` field in `runState` as Approach B
3. Once Approach B validates the per-step re-resolution pattern, D is additive configuration
4. The observational memory precedent proves this works in practice

**Skip Approach A** (external API for model switch) until there is a concrete use case that requires operator-level control over mid-run model selection without agent cooperation. It adds API surface area with unclear demand.

**Skip Approach C** (pre-message hook router) because it requires a breaking change to the hook interface (adding provider output). The hook system is already a third-party extension point; breaking it has wider impact than adding a tool.

### Recommended Implementation Sequence

1. Add `currentModel string` and `currentProvider Provider` to `runState`
2. In `execute()`, read `state.currentModel` at the top of each step (under lock), call `resolveProvider()` if changed
3. Add `switch_model` tool that sets `state.currentModel` via a context value or direct runState write
4. Extend `recordAccounting()` to include `model` in the `usage.delta` event payload
5. Extend `emitContextWindowSnapshot()` to use catalog-resolved context window instead of static config
6. After B is working: add role-based routing via `RunnerConfig.ModelRoles map[string]string` and automatic role selection

### Key Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Model switch during LLM call | Queue switch in `pendingModelSwitch` channel (like steering); apply at top of next step |
| Target model not in catalog | Emit `provider.resolved` warning, continue with current model |
| Tool calling incompatibility | Validate `Model.ToolCalling=true` before switch; return error from tool |
| Cost accounting accuracy | Per-step model attribution in `usage.delta` event; summary still correct |
| ReasoningEffort mismatch | `filteredToolsForRun()` + `CompletionRequest` construction check `Model.ReasoningMode` |
| Context window staleness | Replace `r.config.ModelContextWindow` with catalog lookup `providerRegistry.MaxContextTokens(model)` |

---

## 10. Estimated Complexity

| Approach | Lines of Code | Calendar Days | Test Work | Risk |
|----------|--------------|---------------|-----------|------|
| A: Per-step state + API | ~200 | 1-2d | Medium | Low |
| B: switch_model tool | ~150 | 1-2d | Medium | Low |
| C: Router hook | ~300 | 2-3d | High | Medium |
| D: Role-based routing | ~500 | 3-5d | High | Medium |
| B + D (recommended) | ~600 | 4-6d | High | Low-Medium |

Each approach shares a common prerequisite: adding `currentModel` to `runState` and wiring per-step re-resolution in `execute()` (~50 lines). That prerequisite unblocks all four approaches.

---

## 11. Open Questions

1. **Conversation history on context window mismatch**: If model A has 32K context and model B has 128K, is the existing message history always valid to replay to model B? Yes — same JSON format, larger window. Reverse (larger → smaller) requires compaction before switch.

2. **System prompt re-resolution on model switch**: The current system prompt is resolved once at run start with the initial model. A model switch to a different model profile should probably NOT change the system prompt mid-run. Confirm this as the desired behavior.

3. **Tool permission enforcement**: Should `switch_model` be gated by `PermissionConfig.Approval`? A model switch is a significant action (cost impact, capability change). Recommend requiring explicit operator enablement via `RunnerConfig`.

4. **ContinueRun inheritance**: After a run that switched models, `ContinueRun()` currently propagates `existingModel` (the model from `Run.Model`, which is set at run creation). Should it propagate the final model used at run end instead?
