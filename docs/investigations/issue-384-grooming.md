# Grooming: Issue #384 — feat(subagents): add typed parent-to-child context handoff bundles

## Already Addressed?
No — there is no typed handoff bundle concept anywhere in the codebase.

Current state:
- `spawn_agent` (`spawn_agent.go:93`) builds a child system prompt string via `buildSubagentSystemPrompt` which injects only `task` and `max_steps` into plain text. No structured parent context.
- `run_agent` (`run_agent.go:99`) builds a `SubagentRequest` with `Prompt`, `Model`, `SystemPrompt`, `MaxSteps`, `MaxCostUSD`, `AllowedTools`. There is no structured "here is what the parent knows" bundle.
- `ForkConfig` (`tools/types.go:198`) has `Prompt`, `SkillName`, `Agent`, `AllowedTools`, `Metadata map[string]string`. The `Metadata` map is a loose bag of strings — not typed, not bounded, not rendered into child prompts in a structured way.
- `harness.RunRequest` has no handoff bundle field.
- `subagents.Request` has no handoff bundle field.
- No `ContextHandoff` or `HandoffBundle` type exists anywhere.

Searching for `handoff`, `HandoffBundle`, `context.*bundle`, or `parent.*child.*context` returns zero hits.

## Clarity
Moderately clear on the goal (typed, size-bounded parent-to-child context bundle) but vague on specifics:
- What fields does the handoff bundle contain? (Current working directory? Active file paths? Recent findings? Parent run ID? Arbitrary key-value pairs?)
- "Size-bounded" — what is the limit? Character count? Number of fields? Both?
- "Render into child prompts" — injected into the system prompt? Into the user turn? Both?
- "Store with child run" — stored in `harness.RunRequest`/`subagents.Subagent`? In the conversation store? In a new table?
- Which tools accept the bundle: `spawn_agent` only, `run_agent` only, both, all subagent paths?

## Acceptance Criteria
Missing — the issue describes the concept but does not specify:
- The `ContextHandoff` struct fields
- Size limit (bytes/tokens)
- Rendering template (what the injected prompt block looks like)
- Storage location and retrieval API
- Whether the bundle is optional (backward compatible) or required
- Whether the child can inspect the bundle programmatically (via a tool) or only via the rendered prompt

## Scope
Too broad — the issue bundles three distinct concerns:
1. Define the typed `ContextHandoff` struct
2. Render it into child prompts (affects `spawn_agent`, `run_agent`, `buildSubagentSystemPrompt`)
3. Store it with the child run (affects `harness.RunRequest`, `subagents.Subagent`, possibly the conversation store)

Each of these is independently implementable. Scope 1 alone is a small PR. Scopes 2+3 together are medium.

## Blockers
Soft dependency on #383 — if child result convergence (#383) is done first, the handoff bundle can reference the canonical `ChildResult` schema as the "previous findings" payload. Not a hard blocker.

No hard blockers otherwise.

## Recommended Labels
needs-clarification, large, medium (if scoped to struct + render only, skipping storage)

## Effort
Large as written (struct + render + storage). Medium if scoped to struct + render only (no storage layer changes).

## Recommendation
needs-clarification — the handoff bundle schema, size limit, rendering template, and storage contract all need to be defined before implementation. As written the issue is aspirational design, not an implementable spec. Recommend a spike/design doc to answer the open questions, then break into 2-3 atomic issues.

## Notes
- `ForkConfig.Metadata map[string]string` (`tools/types.go:202`) is the closest existing concept. A typed `ContextHandoff` struct could replace or extend it.
- `buildSubagentSystemPrompt` in `spawn_agent.go:133` is the injection point for the rendered bundle. A simple `if handoff != nil { append rendered block }` pattern would work without changing the runner.
- `harness.RunRequest` (`internal/harness/runner.go`) would need a new optional field (e.g., `HandoffBundle *tools.ContextHandoff`) to store and propagate the bundle.
- The "store with child run" requirement is the most expensive part: if it means persisting to the conversation store/SQLite, that requires schema migration.
- Size-bounding is important — without it, a parent could inject an unbounded context that exhausts the child's context window before it starts its task. A byte limit (e.g., 4096 bytes serialized) is a reasonable default.
- Consider whether the bundle should be serialized to the `AgentID` or `Metadata` field of `subagents.Subagent` for visibility in `GET /v1/subagents/{id}` without a schema change.
