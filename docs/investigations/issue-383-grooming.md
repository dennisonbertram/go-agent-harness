# Grooming: Issue #383 — feat(subagents): converge child completion on a structured result contract

## Already Addressed?
Partial — `task_complete` has a structured schema, but `spawn_agent` and `run_agent` do not normalize to the same schema, and there are concrete divergences in the result shape.

Current state:
- `task_complete` (`internal/harness/tools/deferred/task_complete.go`) emits `{status, summary, findings[]}` with a `_task_complete: true` marker.
- `spawn_agent` (`internal/harness/tools/deferred/spawn_agent.go:146`) tries to parse `task_complete` output via `parseChildResult()`, but falls back to a `{status, summary, jsonl[]}` shape where the findings array is renamed to `jsonl`. This is an inconsistency — `task_complete` uses `findings`, `spawn_agent` returns `jsonl`.
- `run_agent` (`internal/harness/tools/deferred/run_agent.go:114`) returns `{run_id, status, profile, output}` — a completely different shape. No `findings`/`jsonl` array. No `summary` field.
- The inline subagent path (`InlineManager.CreateAndWait`, `internal/subagents/inline_manager.go:24`) uses `tools.SubagentResult{ID, RunID, Status, Output, Error}` — another distinct shape.
- There is no shared `ChildResult` or `AgentResult` type that all paths normalize to.

So there are at least three distinct result shapes: `task_complete`, `spawn_agent` (with `jsonl`), and `run_agent` (with `output`).

## Clarity
Clear on intent — "unify child-run completion around one structured result schema." The issue names the four tools to converge (`task_complete`, `spawn_agent`, `run_agent`, async completion) and names the relevant files. The goal is a single schema that all paths produce and consume.

One ambiguity: "async completion" is mentioned but not yet implemented (see #382), so this issue may need to be sequenced after #382.

## Acceptance Criteria
Missing — the issue does not specify:
- The canonical schema fields (is `findings` or `jsonl` the correct array field name?)
- Whether the schema is a Go struct (exported type in tools package?) or just a documented JSON shape
- Whether backward compatibility with existing callers is required (breaking change for `run_agent` consumers)
- Whether `SubagentResult` in `tools/types.go` is extended or replaced
- Whether this blocks on #382 (async completion normalization)

## Scope
Atomic in intent but medium in surface area — touches four files across two packages. The normalizing type can live in `internal/harness/tools/types.go` (already has `SubagentResult`, `SubagentRequest`, `ForkResult`). The actual changes are surgical find-and-replace of divergent result construction.

## Blockers
Soft block on #382 — if #382 adds async lifecycle tools, their completion path should also normalize to this contract. Implementing #383 before #382 means the async tools will need to adopt the new schema, which is fine as long as the implementer of #382 knows. Not a hard blocker.

## Recommended Labels
well-specified, medium, needs-clarification (schema fields)

## Effort
Medium — the main effort is:
1. Define the canonical `ChildResult` type (or rename/extend `SubagentResult`) in `tools/types.go`
2. Update `task_complete` output to use the canonical type
3. Update `parseChildResult` in `spawn_agent.go` to produce it (rename `jsonl` → `findings`)
4. Update `run_agent` result construction to include `summary` and `findings`
5. Update `InlineManager.CreateAndWait` to populate the extended fields
6. Update tests for all four tools

## Recommendation
well-specified — the inconsistency is real and clearly scoped. Two clarifications needed (canonical schema, `SubagentResult` vs new type) but can be resolved during implementation. This is a healthy cleanup issue.

## Notes
- The `jsonl` field name in `spawn_agent` appears to be a holdover from an earlier design. `findings` (the `task_complete` terminology) is more semantically correct.
- `tools/types.go:243` has `SubagentResult{ID, RunID, Status, Output, Error}` — extending this with `Summary string` and `Findings []ChildFinding` would give a minimal unified shape.
- `spawn_agent.go:204` defines `TaskCompleteResultPayload` and `TaskCompleteFinding` inline in the deferred package. These should move to `tools/types.go` as the canonical types.
- `run_agent.go` currently ignores the structured `task_complete` signal from the child run — it only captures the raw `Output` string. Fixing this requires the `InlineManager`/`SubagentResult` to propagate structured findings from the underlying run events.
- The `_task_complete: true` marker in `task_complete`'s output (line 112) is the detection hook the runner uses to terminate the run; this mechanism does not need to change.
