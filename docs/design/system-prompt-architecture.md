# System Prompt Architecture

## Purpose

Create a modular, file-driven system prompt subsystem that keeps prompt logic auditable and model-aware while preserving compatibility with existing `system_prompt` overrides.

## Goals

- Keep a pure base prompt in source-controlled files.
- Allow harness-coordinated startup intent (`general`, `code_review`, `frontend_design`).
- Support model-specific overlays with deterministic fallback.
- Support curated extension IDs (`behaviors`, `talents`) plus optional custom text.
- Inject runtime context ephemerally on every model turn.
- Keep skills reserved for a separate project (no runtime behavior here).

## Folder Layout

- Prompt engine code: `internal/systemprompt/`
  - `types.go`: request/result and engine interface.
  - `catalog.go`: YAML catalog loading + validation + file indexing.
  - `matcher.go`: model/profile matching logic.
  - `engine.go`: static prompt composition and extension resolution.
  - `runtime_context.go`: per-turn runtime context rendering.
  - `errors.go`: validation error types.
- Prompt assets: `prompts/`
  - `catalog.yaml`
  - `base/main.md`
  - `intents/*.md`
  - `models/*.md`
  - `extensions/behaviors/*.md`
  - `extensions/talents/*.md`

## Composition Order

For non-override runs (`system_prompt` empty), static prompt composition order is:

1. Base prompt
2. Intent prompt
3. Model profile prompt
4. Task context (if present)
5. Behavior extensions
6. Talent extensions
7. Custom extension text

Each section is wrapped with explicit `[SECTION ...]` markers for inspection and debugging.

## Runtime Context

Every provider turn receives a fresh runtime system message:

- `run_started_at_utc`
- `current_time_utc`
- `elapsed_seconds`
- `step`
- `prompt_tokens_total`
- `completion_tokens_total`
- `total_tokens`
- `last_turn_tokens`
- `cost_usd_total`
- `last_turn_cost_usd`
- `cost_status` (`pending|available|unpriced_model|provider_unreported`)

Runtime context is ephemeral and is not persisted into run transcript history.

## Request Surface

`RunRequest` now accepts:

- `agent_intent`
- `task_context`
- `prompt_profile`
- `prompt_extensions.behaviors[]`
- `prompt_extensions.talents[]`
- `prompt_extensions.skills[]` (reserved, ignored)
- `prompt_extensions.custom`

Compatibility rule: if `system_prompt` is provided, prompt engine composition is bypassed.

## Runner Integration

- Prompt resolution occurs at `StartRun`.
- Unknown intent/profile/behavior/talent returns `invalid_request` from run creation path.
- Runner stores resolved static prompt in run state and rebuilds turn messages each step:
  - memory snippet (if enabled)
  - static system prompt
  - runtime context system prompt (engine-managed)
  - persisted conversation history (`user|assistant|tool`)

## Events

New events:

- `prompt.resolved`
  - `intent`, `model_profile`, `model_fallback`, applied extension ids.
- `prompt.warning`
  - used for reserved-field warnings such as `skills_reserved_noop`.

## Failure Handling

- Invalid or missing `prompts/catalog.yaml` causes harness startup failure (fail fast).
- Unknown prompt references in request return `400 invalid_request`.
- Runtime context generation is fail-open (always emits a valid block with current fallback data).

## Token Counting and Cost Tracking (OpenAI-First Implemented)

### Objectives

- Capture normalized token usage and USD cost per turn and as run-level totals.
- Prefer provider-reported usage as source of truth; mark estimator-based values as approximate.
- Surface token/cost state in both runtime context and run events so clients can display live burn.
- Current scope: OpenAI provider path is implemented; additional providers are follow-up work.

### Data Model Changes

- Extend `CompletionUsage` beyond `prompt_tokens|completion_tokens|total_tokens` with optional detail fields where providers expose them:
  - `cached_prompt_tokens`
  - `reasoning_tokens`
  - `input_audio_tokens`
  - `output_audio_tokens`
- Add a cost structure (or expand `CompletionResult.CostUSD`) so each turn can carry:
  - `input_usd`
  - `output_usd`
  - `cache_read_usd`
  - `cache_write_usd`
  - `total_usd`
  - `estimated` (boolean)
  - `pricing_version` (for auditability when pricing tables change)

### Provider Normalization Strategy

- OpenAI adapter: map `usage.prompt_tokens`, `usage.completion_tokens`, `usage.total_tokens`, and token detail fields into normalized usage.
- Anthropic adapter: deferred for a follow-up iteration.
- Missing provider usage: set usage/cost values to zero and mark `usage_status=provider_unreported` and `cost_status=provider_unreported`.

### Pricing Source and Resolution

- Add a versioned local pricing catalog keyed by provider + model profile.
- Resolve model aliases (for example profile names or dated model variants) to pricing entries before cost math.
- If pricing is missing for a model, keep token counts but emit `cost_status=unpriced_model` and `total_usd=0`.
- No default in-repo prices are required; pricing is enabled when `HARNESS_PRICING_CATALOG_PATH` is set.

### Runner Integration and Events

- Track cumulative usage and cumulative cost in run state.
- Emit a per-turn `usage.delta` event with:
  - turn token usage + turn USD
  - cumulative token usage + cumulative USD
  - `usage_status` and `cost_status`
- Include final cumulative usage/cost in `run.completed` payload for downstream reporting.

### Runtime Context Update

- Replace phase-1 placeholder `cost_status: unavailable_phase1` with live fields:
  - `prompt_tokens_total`
  - `completion_tokens_total`
  - `total_tokens`
  - `last_turn_tokens`
  - `cost_usd_total`
  - `last_turn_cost_usd`
  - `cost_status` (`pending`, `available`, `unpriced_model`, `provider_unreported`)
- Keep runtime context ephemeral per turn (same as current behavior).

### Test Plan

- Unit tests for OpenAI usage mapping, missing-usage fallback, and cost status transitions.
- Unit tests for pricing resolution (alias mapping, pricing-version propagation, missing-price fallback).
- Runner integration tests for cumulative accounting across multi-step runs and tool turns.
- Runtime-context tests validating `pending -> available` and `unpriced_model` transitions.
