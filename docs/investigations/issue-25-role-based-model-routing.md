# Issue #25 Research: Role-Based Model Routing for Cost-Optimized Multi-Model Runs

## Executive Summary

Role-based model routing assigns different LLM models to different phases ("roles") of an agent
run. Not every step in a run needs the most capable — and most expensive — model. Summarization
tasks, memory observation, context compaction, and simple tool-dispatch decisions are all tasks
that cheap models handle well. Only planning, final-answer generation, and complex reasoning
benefit from a premium model. Routing correctly across roles can yield 3-5x cost reduction at
scale, consistent with the grooming estimate.

The harness already has one instance of role-based routing: `observationalmemory` uses `gpt-5-nano`
as its Observer/Reflector models rather than the run's primary model. This pattern should be
generalized.

---

## 1. Natural Roles in an Agent Run

Based on examining `runner.go` and the step loop structure, the following distinct phases exist
inside a single harness run:

### Role: `primary` (default)
- The main tool-calling LLM turn in the step loop.
- Receives the full conversation history + tool list.
- Decides which tools to call, in what order.
- Generates the final answer when terminating.
- **Requires**: tool calling, parallel tool calls, high instruction-following quality.
- **Current model**: `gpt-4.1-mini` (default), configurable via `RunRequest.Model`.

### Role: `planner` (hypothetical)
- An optional first step that decomposes a complex task into sub-steps.
- Emits a structured plan before the main step loop begins.
- **Requires**: strong reasoning, structured output.
- **Best model**: o3, claude-opus, gpt-4.1 (more expensive but invoked only once).

### Role: `summarizer`
- Invoked by `SummarizeMessages()` during `CompactRun` / `autoCompactMessages`.
- Receives a window of recent messages; returns a short summary string.
- No tool calling needed. Output is a plain text summary.
- **Current model**: `r.config.DefaultModel` (same as primary — unnecessary).
- **Cheapest viable model**: gpt-4.1-mini, deepseek-chat, claude-haiku — any model with
  reasonable text coherence.

### Role: `memory_observer`
- Used by `observationalmemory.ModelObserver` to produce short observations about each turn.
- Currently uses its own `OpenAIModel` configured at startup (e.g., `gpt-5-nano` in tests).
- **Already decoupled** from the primary model.
- **Best model**: any nano/mini model that can read a transcript and extract facts.

### Role: `memory_reflector`
- Used by `observationalmemory.ModelReflector` to synthesize observations into structured memory.
- Same decoupling as observer; runs infrequently (reflection intervals).
- **Best model**: slightly better than observer (needs coherent synthesis), still cheap.

### Role: `step_classifier` (new, optional)
- If implementing Approach B (dynamic inference), a lightweight classifier decides whether
  the current step is a "simple" tool dispatch vs. a "complex" reasoning step.
- Could be a rule-based heuristic (no model needed) or a tiny LLM call.
- If heuristic: zero cost. If LLM: use nano model.

---

## 2. Model Catalog and Cost/Quality Matrix

All pricing data from `catalog/models.json` (March 2026 state).

### Available Models (with pricing and role suitability)

| Model | Input $/1M | Output $/1M | Context | Tool Calling | Speed | Reasoning | Best Role |
|---|---|---|---|---|---|---|---|
| gpt-4.1-mini | $0.40 | $1.60 | 1M | Yes | Fast | Medium | primary (default), summarizer |
| gpt-4.1 | $2.00 | $8.00 | 1M | Yes | Fast | Strong | planner, final-answer |
| deepseek-chat | $0.28 | $0.42 | 131K | Yes | Fast | Medium | summarizer, primary (budget) |
| qwen-turbo | $0.20 | $0.60 | 1M | Yes | Fast | Medium | summarizer, memory |
| claude-haiku-4.5 | $0.80 | $4.00 | 200K | Yes | Ultra-fast | Light | summarizer, memory |
| grok-3-mini | $0.30 | $0.50 | 131K | Yes | Fast | Reasoning | step_classifier |
| llama-4-maverick | $0.27 | $0.35 | 1M | Yes | Fast | Medium | summarizer, budget-primary |
| claude-sonnet-4.6 | $3.00 | $15.00 | 200K | Yes | Fast | Strong | planner, complex-primary |
| claude-opus-4.6 | $15.00 | $75.00 | 200K | Yes | Medium | Highest | planner (high-stakes) |
| grok-4.1-fast-reasoning | $5.00 | $25.00 | 262K | Yes | Fast | Premium | planner |
| deepseek-reasoner | $0.55 | $2.19 | 131K | No | Medium | Strong | planner (no tools) |
| qwen-qwq-32b | $0.20 | $0.20 | 131K | No | Ultra-fast | Reasoning | step_classifier (no tools) |

### Cost Comparison: Summarizer Role

A compaction summarization call typically processes ~50K tokens of conversation history and emits
~2K tokens summary.

| Model | Input cost (50K) | Output cost (2K) | Total |
|---|---|---|---|
| gpt-4.1-mini (current default) | $0.020 | $0.0032 | **$0.023** |
| deepseek-chat | $0.014 | $0.00084 | **$0.015** |
| qwen-turbo | $0.010 | $0.0012 | **$0.011** |
| gpt-4.1 (overkill) | $0.100 | $0.016 | **$0.116** |

Switching summarizer from gpt-4.1 to gpt-4.1-mini = 5x saving. Mini to qwen-turbo = 2x more.

### Cost Comparison: Primary Role (typical 8-step run, ~8K tokens/step)

Assume 8 steps, each step: 8K prompt tokens + 1K completion tokens.

| Model | Total input cost (64K) | Total output (8K) | Total run cost |
|---|---|---|---|
| gpt-4.1-mini | $0.026 | $0.013 | **$0.039** |
| gpt-4.1 | $0.128 | $0.064 | **$0.192** |
| claude-sonnet-4.6 | $0.192 | $0.120 | **$0.312** |
| claude-haiku-4.5 | $0.051 | $0.032 | **$0.083** |
| deepseek-chat | $0.018 | $0.003 | **$0.021** |

### Role-Routing Cost Savings Estimate

A typical run with gpt-4.1-mini for primary and gpt-4.1-mini for summarization costs ~$0.039 (run)
+ $0.023 (1 compaction) = $0.062 total.

With role-based routing (primary=gpt-4.1-mini, summarizer=qwen-turbo, memory=qwen-turbo):
- Run: $0.039 (unchanged)
- Compaction: $0.011
- Memory observations (4x, 2K tokens each): $0.004 total
- **Total: $0.054** — about 13% saving for this simple case.

The larger saving comes when the primary model is expensive. If an operator uses gpt-4.1 for
quality but routes summarization to deepseek-chat:
- Run: $0.192
- Compaction: $0.015
- **Total: $0.207** vs $0.308 (all gpt-4.1) — **33% saving**.

At scale (1,000 runs/day), the difference compounds significantly.

---

## 3. The Four Routing Approaches

### Approach A: Static Role Assignment (RECOMMENDED for initial implementation)

Each named role in the harness maps to a configured model. Configuration lives in `RunnerConfig`
(or a YAML role-routing profile).

**Configuration schema (proposed TOML/struct)**:
```toml
[role_models]
primary    = "gpt-4.1-mini"        # main step-loop model
summarizer = "qwen-turbo"          # CompactRun / autoCompact / SummarizeMessages
planner    = ""                    # empty = use primary
memory_observer  = "gpt-4.1-mini" # observationalmemory observer
memory_reflector = "gpt-4.1-mini" # observationalmemory reflector
```

Or as a Go struct added to `RunnerConfig`:
```go
type RoleModelConfig struct {
    Primary         string // default: RunnerConfig.DefaultModel
    Summarizer      string // default: same as Primary
    Planner         string // default: same as Primary
    MemoryObserver  string // default: same as Primary
    MemoryReflector string // default: same as Primary
}
```

**How the runner uses it**:
- `SummarizeMessages()` currently uses `r.config.DefaultModel`. It would use
  `r.config.RoleModels.Summarizer` if set.
- The main step loop uses `r.config.RoleModels.Primary` instead of `r.config.DefaultModel`.
- Observational memory already has its own model config; we'd wire `RoleModels.MemoryObserver`
  as the default when creating the `OpenAIModel` in `NewRunner`.

**Implementation effort**: Small.
- Add `RoleModelConfig` struct to `RunnerConfig`.
- Modify `SummarizeMessages()` to prefer `r.config.RoleModels.Summarizer`.
- Modify `execute()` to use `r.config.RoleModels.Primary` for the `CompletionRequest.Model`.
- Modify `autoCompactMessages()` to pass the summarizer model to `SummarizeMessages`.
- No changes to provider layer; already supports any model per request.

**Tradeoffs**:
- Simple, predictable, fully deterministic.
- No per-step overhead.
- Operator must configure roles explicitly; defaults to current behavior if not set.
- No runtime adaptation: expensive model runs for all steps equally.

### Approach B: Dynamic Role Inference

The runner infers the "type" of each step from context and routes to the appropriate model
dynamically:
- Step 1 (first turn after user message) = potentially planning-heavy → use expensive model.
- Steps with only tool results as context = routing/execution → use cheap model.
- Last step (no tool calls in response) = final answer → use quality model.
- Compaction steps = summarizer model (already distinct in the code).

**Heuristics**:
1. **Tool-call-heavy steps**: If the previous assistant message had N tool calls and this step
   is processing results, use `Summarizer` model (lower reasoning needed).
2. **Final answer step**: If the LLM's last response had zero tool calls, it was a final-answer
   step. Retrospectively, this could have used a better model, but this is hard to predict ahead
   of time.
3. **Step number threshold**: Steps 1-2 are expensive (planning), steps 3+ are cheap.

**Implementation complexity**: Medium.
- Requires classifying each step before calling the LLM.
- Heuristics are fragile: a step processing tool results may still require reasoning.
- Risk: routing a reasoning-heavy step to a cheap model degrades quality silently.

**Recommended only as enhancement on top of Approach A**, not as primary mechanism.

### Approach C: Agent-Declared Roles

The agent explicitly signals its current role, either via:
- A special tool call (`declare_role("planner")` or `declare_role("summarizer")`).
- A meta-message prefix (`[role:summarizer]` in the content).
- A new `CompletionRequest` field set via a steering message.

**Strengths**:
- Most flexible: agent knows better than static heuristics what it is doing.
- Could enable per-subtask model switching in orchestrated multi-agent scenarios.

**Weaknesses**:
- Requires agent cooperation: system-prompt instructions must tell the agent how to signal roles.
- Adds a round-trip cost if the signal requires an LLM call to set.
- Coupling between agent behavior and infrastructure.
- Relies on mid-run model switching (#136) being implemented first for within-run changes.

**Dependency**: Approach C for within-step switching requires #136 (mid-run model switching).
For between-run role assignment, it works without #136.

**Recommended for later phases**, after Approach A is established.

### Approach D: Cost-Ceiling-Triggered Routing

The runner switches to a cheaper model when `maxCostUSD` is approached:
- e.g., when 80% of the cost ceiling is consumed, switch `Primary` to `gpt-4.1-mini`.
- Already has partial infrastructure: `maxCostUSD` in `runState`, cost enforcement in `execute()`.

**How it would work**:
1. After each step, compute `remainingCostUSD = maxCostUSD - costTotals.CostUSDTotal`.
2. If `remainingCostUSD < thresholdFraction * maxCostUSD`, switch to `RoleModels.FallbackModel`.
3. Emit a `EventModelDowngrade` event.

**Strengths**:
- Automatic budget protection.
- No external config: purely reactive to cost signals.

**Weaknesses**:
- Reactive, not proactive: cost is already partially spent before downgrade.
- Quality may suddenly degrade mid-conversation, which is jarring.
- Cannot prevent expensive first steps.

**Recommended as an optional add-on** after Approach A, triggered explicitly by setting both
`maxCostUSD` and `RoleModels.FallbackModel`.

---

## 4. Integration with Existing Codebase

### What Exists Today

1. **`RunnerConfig.DefaultModel`** — single model for all LLM calls in a run.
2. **`run.Model`** — set at `StartRun` time from `RunRequest.Model`, used throughout `execute()`.
3. **`SummarizeMessages()`** — uses `r.config.DefaultModel`; no model override parameter.
4. **Observational memory** — already uses a separate model (`OpenAIModel` with its own config).
   This is the only existing role-based routing in the codebase.
5. **`resolveProvider()`** — already handles per-model provider resolution via `ProviderRegistry`.
   The infrastructure to call a different model (and thus a different provider) per request exists.
6. **`CompletionRequest.Model`** — per-request model field, already wired through to the provider.
   The provider layer is model-agnostic: any model can be passed per call.

### Minimal Changes for Approach A

**1. Add `RoleModelConfig` to `RunnerConfig`** (`internal/harness/types.go`):
```go
type RoleModelConfig struct {
    Primary         string
    Summarizer      string
}
```

**2. Modify `SummarizeMessages()` in `runner.go`** (line ~3288):
- Replace `model := r.config.DefaultModel` with:
  ```go
  model := r.config.RoleModels.Summarizer
  if model == "" {
      model = r.config.DefaultModel
  }
  ```

**3. Modify `execute()` in `runner.go`** (line ~870):
- Replace `model := req.Model; if model == "" { model = r.config.DefaultModel }` with:
  ```go
  model := req.Model
  if model == "" {
      model = r.config.RoleModels.Primary
  }
  if model == "" {
      model = r.config.DefaultModel
  }
  ```

That's it for the minimum viable implementation. The change is backward-compatible: if
`RoleModels` is zero-value (not configured), all models fall back to `DefaultModel`.

### Provider Resolution

`resolveProvider()` already supports per-model provider lookup via `ProviderRegistry`:
```go
client, providerName, err := r.providerRegistry.GetClientForModel(model)
```

This means the summarizer role could be on a completely different provider (e.g., deepseek) from
the primary role (openai), with no additional changes required. The provider layer is already
role-model-aware.

---

## 5. Relationship to Issue #136 (Mid-Run Model Switching)

### Key Distinction

| | #25 (Role Routing) | #136 (Mid-Run Switching) |
|---|---|---|
| **Trigger** | Configuration at run start | User/agent action mid-run |
| **Granularity** | Role level (summarizer, primary) | Arbitrary step |
| **Driver** | Cost optimization | User preference / capability need |
| **Context handoff** | Roles use disjoint context by default | Must replay history to new model |
| **Dependency on #11** | Partial (multi-provider already works) | Full |

### Sequencing

**#25 does NOT depend on #136**. Role-based routing can be implemented today:
- The summarizer role runs a single isolated `CompletionRequest` (no conversation history replay).
- The primary role is set once at run start and stays fixed throughout the step loop.
- No mid-run switching within a single role is needed for Approach A.

**#136 is needed to fully unlock Approach C** (agent-declared roles with mid-run switching)
and Approach D (cost-ceiling-triggered fallback within a run). Both require changing the active
model during an ongoing step loop, which is what #136 is scoped for.

**Recommended execution order**:
1. Implement #25 Approach A (static role config) — works today, no blockers.
2. Research and implement #136 (mid-run model switching) — builds on #25's role taxonomy.
3. Layer Approach C or D on top of #136 as enhancements.

---

## 6. Prior Art and Reference Implementations

### omp Role Model

The grooming doc references "omp" (a related project) with these roles: `default`, `smol`, `slow`,
`plan`, `commit`. This maps directly to what we now call:
- `default` = `primary`
- `smol` = `summarizer` (or memory)
- `slow` = `planner` (high-capability, invoked infrequently)
- `plan` = explicitly-triggered planning step
- `commit` = final-answer or critical-output step

The key insight from omp's design: roles are declared in a **persona YAML** (analogous to our
`systemprompt` profiles). Role-to-model mapping is configured per persona, not globally. This
enables per-intent model routing: a `git_historian` persona uses different models than a
`code_writer` persona.

This is consistent with extending the existing `systemprompt.ResolvedPrompt` / persona system.
`PromptProfile` in `RunRequest` already scopes the system prompt — it could also scope the role
model mapping.

### LiteLLM Router

LiteLLM's router supports:
- Model groups with fallback chains.
- Cost-based routing (cheapest model first, fallback on rate limit).
- Load balancing across providers.

This is more infrastructure-level than what #25 describes. The harness already implements the
key parts via `ProviderRegistry`. LiteLLM's router is a good reference for how to express
fallback chains, but the harness's approach (explicit role config) is simpler and more
predictable for an agentic use case where consistency matters.

### LangChain Router

LangChain's `RouterChain` / `MultiPromptChain` routes inputs to different sub-chains. The concept
maps to: each "role" is a chain that uses a specific model. Applicable but heavy-weight compared
to the simple struct field approach recommended here.

### Claude Code Itself

Claude Code (this project's operator) uses a two-model approach: a fast model for sub-tasks and
a slower model for reasoning. This is Approach A in practice.

---

## 7. Proposed YAML/TOML Schema

If roles are to be expressed in a persona YAML (extending the `prompts/` system):

```yaml
# prompts/personas/code_writer.yaml
name: code_writer
description: "Writes and reviews code"
role_models:
  primary: gpt-4.1
  summarizer: gpt-4.1-mini
  planner: ""        # empty = use primary
  fallback: gpt-4.1-mini  # used when cost ceiling approached
```

Or as environment variables for simpler deployments:
```
HARNESS_ROLE_MODEL_PRIMARY=gpt-4.1
HARNESS_ROLE_MODEL_SUMMARIZER=deepseek-chat
HARNESS_ROLE_MODEL_PLANNER=claude-opus-4-6
```

The `RunnerConfig.RoleModels` struct approach is the right in-code representation; env vars and
YAML are the configuration surface.

---

## 8. Cost Savings Estimate

### Conservative Estimate (Approach A, Summarizer Role Only)

Assumption: operator currently uses gpt-4.1 for everything, runs ~100 runs/day, each run
has ~2 compaction calls (50K tokens in, 2K out each).

Without role routing:
- Compaction per run: $0.116 (gpt-4.1)
- Daily: 100 runs × 2 compactions × $0.116 = **$23.20/day**

With role routing (summarizer = deepseek-chat):
- Compaction per run: $0.015 (deepseek-chat)
- Daily: 100 × 2 × $0.015 = **$3.00/day**

**Saving: $20.20/day = $606/month from summarizer routing alone.**

### Moderate Estimate (Approach A, Primary + Summarizer)

If the primary role is switched from claude-opus-4.6 to claude-sonnet-4.6 for routine steps:
- Per-run primary cost: $0.312 (sonnet) vs $1.560 (opus, 8 steps, 8K+1K tokens)
- Daily (100 runs): $31.20 vs $156.00
- **Saving from primary routing: $124.80/day = $3,744/month**

---

## 9. Recommended Phased Approach

### Phase 1 (Issue #25 core deliverable): Static Role Config
- Add `RoleModelConfig` to `RunnerConfig` with `Primary` and `Summarizer` fields.
- Modify `SummarizeMessages()` to use `RoleModels.Summarizer`.
- Update `execute()` to use `RoleModels.Primary`.
- Wire env vars: `HARNESS_ROLE_MODEL_PRIMARY`, `HARNESS_ROLE_MODEL_SUMMARIZER`.
- Estimated effort: 2-3 hours of implementation + tests.
- **No dependency on #136 or #11.**

### Phase 2: Planner Role (after #136)
- Add `Planner` to `RoleModelConfig`.
- On the first step of a run, optionally call the planner model for task decomposition.
- Requires #136 (mid-run model switching) or a pre-loop planning call.
- Estimated effort: 4-6 hours.

### Phase 3: Agent-Declared Roles (after #136 and Phase 2)
- Add a `declare_role` tool (or `[role:X]` message prefix convention).
- Runner respects role declarations and routes LLM calls accordingly.
- Emits `EventModelRoleSwitch` events.
- Estimated effort: 6-8 hours.

### Phase 4: Cost-Ceiling Fallback (enhancement)
- When cost approaches `maxCostUSD`, switch to `RoleModels.Fallback`.
- Emit `EventModelDowngrade`.
- Estimated effort: 2-3 hours (builds on Phase 1 infrastructure).

---

## 10. Key Decisions Deferred

1. **Should role models be per-intent (persona) or global?** The grooming doc implies per-persona
   (e.g., `git_historian` vs `code_writer`). The `PromptProfile` mechanism in `RunRequest`
   already provides the scoping hook. Phase 1 can start global; per-persona can come in Phase 2.

2. **How to handle cross-provider context replay?** When the summarizer is on a different
   provider (deepseek vs openai), the summarization call gets its own isolated
   `CompletionRequest` with no conversation history — the context window is not shared.
   This is safe for summarization. For the `planner` role, the planner receives the original
   user task, not the accumulated tool history (which doesn't exist yet at planning time).
   Full cross-provider history replay is a #136 concern, not #25.

3. **Model validation at startup**: Should `RunnerConfig` validation check that all configured
   role models exist in the catalog? Recommended: yes, emit a warning but don't fail.
