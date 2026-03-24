# Open SWE Adoption Case

## Recommendation

`go-agent-harness` should adopt the useful product-layer ideas from `langchain-ai/open-swe`, but only in the narrow areas where they strengthen the current architecture:

1. repo `AGENTS.md` auto-loading
2. external trigger adapters
3. deterministic follow-up routing
4. source-context hydration
5. explicit workspace-backend selection

It should not copy `open-swe` wholesale.

## What To Copy

### 1. Repo `AGENTS.md` Auto-Loading

Copy the behavior where the target repo's root `AGENTS.md` is read automatically and injected into the system prompt with explicit provenance.

Why this fits:

- This repo already has a stronger prompt-composition system than `open-swe`.
- The missing piece is not prompt structure; it is automatic repo-local instruction loading.
- This is a high-leverage, low-risk slice because it is read-only and can be tested cleanly.

### 2. External Trigger Adapters

Copy the product pattern where GitHub, Slack, or Linear can be the natural entry point for a task.

Why this fits:

- The repo already exposes a clean HTTP/event-driven harness core.
- Trigger adapters belong around the server boundary, not inside the runner.
- This turns the harness from "an API you call" into "an async coding agent people can use where they already work."

### 3. Deterministic Follow-Up Routing

Copy the idea of stable external thread routing, but map it onto the harness's existing primitives instead of copying `open-swe`'s queue design.

Recommended mapping:

- active run -> `SteerRun`
- completed run -> `ContinueRun`

Why this fits:

- The harness already has the correct run-level semantics.
- Reusing those semantics avoids creating two competing follow-up models.

### 4. Source-Context Hydration

Copy the pattern of hydrating the initial task with issue/thread context:

- title
- body
- recent comments
- repo hint
- attachments or image references when relevant

Why this fits:

- It reduces first-turn exploration churn.
- It gives the prompt engine richer task context without changing the runner loop.
- It composes naturally with the existing prompt sections.

### 5. Explicit Workspace-Backend Selection

Copy the pattern of choosing an execution backend per task, but route it through this repo's existing workspace abstraction instead of a separate sandbox layer.

Why this fits:

- `go-agent-harness` already has `worktree`, `container`, and other workspace backends.
- The missing piece is policy and request-level wiring, not a new abstraction.

## What Not To Copy

### Do Not Copy the Prompt Style

`open-swe` uses a large assembled prompt with strong workflow instructions embedded directly in prompt text.

Why not:

- `go-agent-harness` already has a better prompt system: intent, model profile, behavior, talent, skill, and runtime context are cleanly composed.
- Replacing that with a monolithic prompt would be a regression.

### Do Not Copy the PR Safety Net As-Is

`open-swe` has middleware that opens a PR if the agent forgot to do it.

Why not:

- This repo has stricter expectations around TDD and regression validation.
- A PR backstop is only useful if it respects the repo's stronger verification rules.

### Do Not Copy the Looser Validation Posture

`open-swe` intentionally limits local verification to related tests and relies heavily on CI.

Why not:

- That conflicts with the operating model of this repo.
- The feature should be integrated without weakening local confidence checks.

## Strong Case For Adoption

### It Reuses Existing Strengths Instead of Replacing Them

This proposal is compelling because it builds on what `go-agent-harness` already does well:

- strong runner semantics
- explicit continuation and steering
- modular prompt composition
- isolated workspace abstractions
- rich tool surface

The missing layer is product orchestration around those primitives.

### It Improves the Product More Than the Core

The biggest value is not a smarter model loop. It is making the harness easier to invoke, easier to continue, and easier to operate in real team workflows.

That is the same reason `open-swe` feels compelling: the integration layer makes the system usable.

### It Is a Good Risk/Reward Trade

The proposal can be landed in phases:

1. repo `AGENTS.md` loading
2. external trigger envelope
3. GitHub-first adapter
4. Slack/Linear follow-on
5. workspace selection policy

That sequencing gives a clear early win before any auth-heavy multi-integration rollout.

### It Sharpens the Repo's Positioning

Without this layer, `go-agent-harness` risks feeling like an excellent internal engine that still needs product glue.

With this layer, it becomes easier to argue that the repo is not just a runner but a complete async coding-agent platform with:

- API-native runs
- source-native invocation
- deterministic follow-up routing
- repo-aware prompts
- pluggable execution backends

## Documentation Plan

### Before Implementation

- Plan document in `docs/plans/`
- Impact map in `docs/plans/`
- Proposal memo in `docs/research/`

### During Implementation

- Engineering-log entry for each landed slice
- System-log update describing trigger flow and component boundaries
- Test docs only if new live/smoke procedures are introduced

### At Ship Time

- README section for trigger surfaces and setup
- Runbook for GitHub / Slack / Linear credentials and webhooks
- Prompt-architecture doc update for repo `AGENTS.md` loading
- Endpoint documentation for any new webhook routes

## Recommended First Slice

If this moves forward, the first implementation slice should be:

1. repo `AGENTS.md` auto-loading
2. normalized external trigger envelope
3. GitHub-first adapter using `SteerRun` and `ContinueRun`

That is the smallest slice that proves the core thesis without committing the repo to a wider integration surface too early.
