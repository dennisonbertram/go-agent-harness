# Plan: Open SWE-Inspired Trigger and Context Adoption

## Context

- Problem:
  - `go-agent-harness` already has strong runner primitives for mid-run steering, completed-run continuation, prompt composition, subagents, and isolated workspaces.
  - What it does not yet have is a first-class trigger layer that lets GitHub, Slack, or Linear act as natural entry points, nor automatic loading of repo-local `AGENTS.md` instructions into the runtime prompt.
  - `langchain-ai/open-swe` demonstrates a coherent product pattern around external triggers, deterministic thread routing, and source-context hydration that maps well onto this repo's existing architecture.
- User impact:
  - Users currently need to know the harness API directly.
  - Repo-specific operating rules are present in the repo but not automatically loaded into every run.
  - External follow-up messages do not yet have a native adapter that routes them onto the existing `steer` / `continue` primitives.
- Constraints:
  - Preserve strict TDD and the repo's stronger test/merge discipline.
  - Reuse existing runner semantics instead of cloning `open-swe` middleware behavior.
  - Keep the first slice small enough to ship behind a clear phase boundary.
  - Do not disrupt the current active implementation plan for issue `#361`.

## Scope

- In scope:
  - Auto-load root-level repo `AGENTS.md` content into the resolved system prompt.
  - Add a normalized external trigger envelope for GitHub / Slack / Linear source context.
  - Add deterministic external thread routing so follow-up messages target the same conversation.
  - Route active follow-ups to `SteerRun` and completed follow-ups to `ContinueRun`.
  - Start with a GitHub-first adapter slice, then extend to Slack / Linear on the same abstraction.
  - Define how per-run workspace backend selection can be driven by trigger/profile metadata.
  - Document the architecture, rollout order, and operator setup.
- Out of scope:
  - Copying `open-swe`'s monolithic prompt or LangGraph runtime.
  - Copying `open-swe`'s auto-PR safety net or looser test posture.
  - Replacing the current prompt engine, runner loop, or workspace abstraction.
  - Shipping all three trigger adapters in one undifferentiated implementation pass.

## Test Plan (TDD)

- New failing tests to add first:
  - Prompt-resolution test proving repo `AGENTS.md` is injected as a distinct prompt section when present and omitted cleanly when absent.
  - GitHub trigger tests covering signature validation, deterministic thread ID derivation, and source-context hydration from issue/comment payloads.
  - Routing tests proving follow-up input chooses `SteerRun` for active runs and `ContinueRun` for completed runs.
  - Reply-sink tests proving source-specific outbound messages are formatted and dispatched to the correct integration.
  - Workspace-selection tests proving request/profile metadata resolves the correct workspace backend with documented fallback behavior.
- Existing tests to update:
  - Prompt-engine tests in `internal/systemprompt`.
  - Server routing/auth tests in `internal/server`.
  - Runner continuation/steering tests in `internal/harness`.
  - Workspace option/provisioning tests in `internal/workspace`.
- Regression tests required:
  - Ensure existing direct API clients keep working unchanged.
  - Ensure runs without external trigger metadata still resolve prompts and workspaces exactly as before.
  - Ensure invalid signatures and missing repo mappings fail closed.

## Cross-Surface Impact Map

- See `docs/plans/2026-03-22-open-swe-trigger-context-impact-map.md`.

## Implementation Checklist

- [ ] Keep this work as a phased proposal until the current active plan no longer blocks implementation.
- [ ] Land the repo `AGENTS.md` prompt-loading slice first because it has the best leverage-to-risk ratio.
- [ ] Introduce a small external trigger envelope type instead of source-specific branching throughout the runner.
- [ ] Add deterministic external thread identity utilities before adding webhook handlers.
- [ ] Implement GitHub-trigger ingestion first.
- [ ] Route GitHub follow-ups onto existing `SteerRun` / `ContinueRun` behavior instead of inventing a second queueing path.
- [ ] Add a reply-sink abstraction so GitHub / Slack / Linear can share the same run lifecycle hooks.
- [ ] Add workspace backend selection as an explicit request/profile choice rather than hidden trigger-specific behavior.
- [ ] Update docs and indexes.
- [ ] Update engineering/system/observational logs as needed.
- [ ] Run targeted tests for each slice, then the full regression gate before merge.

## Rollout Phases

### Phase 0: Decision and Documentation

- Produce a concise proposal memo that explains what to copy, what not to copy, and why this fits the current harness architecture.
- Keep the change framed as "adopt the trigger/context layer" rather than "copy Open SWE."
- Record the impact map before implementation starts.

### Phase 1: Repo Instruction Loading

- Detect and read the target repo's root `AGENTS.md`.
- Inject it into the prompt as a clearly delimited prompt section after the static system prompt is resolved.
- Keep this feature read-only and low-risk.

### Phase 2: External Trigger Envelope

- Introduce a normalized structure for:
  - source system
  - external thread identifier
  - repo coordinates
  - hydrated task context
  - reply target metadata
- Keep this separate from raw webhook payloads so the runner remains integration-agnostic.

### Phase 3: GitHub-First Trigger Adapter

- Add GitHub webhook ingestion for issue/comment/PR-review events.
- Hydrate prompt context from issue title, body, recent comments, and repo metadata.
- Use deterministic thread IDs so repeated mentions map to the same conversation.
- On follow-up:
  - active run -> `SteerRun`
  - completed run -> `ContinueRun`

### Phase 4: Shared Reply Sink

- Add a source-aware reply abstraction that can post updates back to GitHub first, then Slack/Linear later.
- Keep outbound messaging outside the runner core.

### Phase 5: Slack and Linear Adapters

- Reuse the same envelope and reply sink.
- Reuse the same thread routing strategy.
- Reuse the same hydrated-context pattern.

### Phase 6: Workspace Backend Selection

- Allow trigger/profile metadata to request `worktree`, `container`, `vm`, or future backends.
- Keep backend defaults explicit and documented.
- Avoid burying execution-mode policy inside a specific trigger adapter.

## Documentation Deliverables

- Before implementation:
  - proposal memo in `docs/research/`
  - phased plan in `docs/plans/`
  - impact map in `docs/plans/`
- During implementation:
  - engineering-log entry per landed slice
  - system-log update for trigger flow and component ownership
- At ship time:
  - README update for new trigger surfaces and setup
  - operator runbook for webhook/auth configuration
  - prompt-architecture doc update for repo `AGENTS.md` loading

## Risks and Mitigations

- Risk:
  - Scope creep into a full `open-swe` clone.
- Mitigation:
  - Keep the proposal anchored to repo `AGENTS.md` loading, trigger adapters, and context hydration only.

- Risk:
  - Duplicating runner semantics with a second message queue.
- Mitigation:
  - Reuse `SteerRun` and `ContinueRun` as the only follow-up paths.

- Risk:
  - Integration code leaking GitHub/Slack/Linear specifics into the runner core.
- Mitigation:
  - Normalize external payloads into a source-agnostic envelope before run creation.

- Risk:
  - Prompt bloat or instruction ambiguity from raw repo docs.
- Mitigation:
  - Limit auto-loading to the repo root `AGENTS.md` and inject it as its own prompt section with explicit provenance.
