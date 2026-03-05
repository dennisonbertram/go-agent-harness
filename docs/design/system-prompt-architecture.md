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
- `cost_status` (`unavailable_phase1` in this phase)

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

## Phase 2 (Deferred)

- Add provider usage/cost capture to `CompletionResult.Usage` and `CompletionResult.CostUSD`.
- Replace `cost_status: unavailable_phase1` with real token/cost values in runtime context.
