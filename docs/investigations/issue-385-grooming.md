# Grooming: Issue #385 — feat(subagents): add artifact references and child-result retrieval

## Already Addressed?

No — The current child-result contract is minimal and unstructured. `SubagentResult` in `internal/harness/tools/types.go` carries only `{ID, RunID, Status, Output, Error}`. The `run_agent` tool returns raw `output` as a string blob. `spawn_agent` parses a `TaskCompleteResultPayload` with a `findings` array (type + content pairs), but there is no typed artifact/file-reference layer anywhere in `internal/subagents/manager.go`, `internal/workspace/`, or `internal/store/`. No `ArtifactRef`, `FileRef`, `file_refs`, or equivalent type exists in the codebase.

## Clarity

Unclear — The issue names three broad goals (artifact/reference layer, child artifact metadata, retrieval surfaces) without defining:
- What an "artifact" is (a file path? a blob? a pointer to worktree output? a store entry?).
- Whether "retrieval surfaces" means a new HTTP endpoint, a new tool, or simply a field on existing return types.
- Whether artifacts need to be persisted in the store or are ephemeral per-run.
- Whether the artifact layer applies to both `run_agent` and `spawn_agent`, or only one.
- What "metadata" fields an artifact entry carries (name, path, size, MIME type, content hash?).

## Acceptance Criteria

Missing — No concrete acceptance criteria are stated. The issue gestures at three bullet-level goals but does not specify:
- API shape of `ArtifactRef` (fields, encoding).
- HTTP or tool surface for retrieval.
- How artifacts are linked to a child run.
- What the parent receives — reference only, content-inline, or a URL/path?
- Storage medium (in-memory, SQLite table, file system).

## Scope

Too broad — The issue spans artifact type design, store integration, workspace integration, subagents manager changes, and retrieval surfaces, all in one ticket. Each of those layers has independent design questions. The plan document (`docs/plans/2026-03-20-profile-subagent-backlog-plan.md`) lists #385 as a single step but its internal complexity is large. Splitting into at minimum: (a) define artifact type + store schema, and (b) wire retrieval into subagent result, would be safer.

## Blockers

Based on the delivery order in the plan doc, #385 depends on earlier tickets (specifically #383 and #384 which address structured child results and lifecycle). If those are not merged, the artifact layer has no stable result contract to attach to.

Blockers: #383 (structured child result contract), #384 (subagent lifecycle completion).

## Recommended Labels

needs-clarification, large, blocked

## Effort

Large — Requires: defining new types, adding a SQLite table (or file-system store), wiring artifacts through the subagent manager, updating `SubagentResult`, adding an HTTP retrieval surface or tool, and tests at all layers.

## Recommendation

needs-clarification — The issue needs a concrete definition of "artifact" (type fields, storage medium, retrieval protocol) and explicit acceptance criteria before implementation can begin. Also blocked on the structured child-result contract from earlier tickets.

## Notes

- `internal/harness/tools/types.go`: `SubagentResult` has only `{ID, RunID, Status, Output, Error}` — no file refs.
- `internal/harness/tools/deferred/run_agent.go`: returns a flat map with `run_id`, `status`, `profile`, `output` — no artifacts.
- `internal/harness/tools/deferred/spawn_agent.go`: `parseChildResult` handles `TaskCompleteResultPayload` with a `findings []TaskCompleteFinding` — structured but not file-aware.
- `internal/store/sqlite.go`: schema has `runs`, `run_messages`, `run_events` — no `artifacts` or `run_artifacts` table.
- `internal/subagents/manager.go`: `Subagent` struct has `{ID, RunID, Status, Isolation, CleanupPolicy, WorkspacePath, WorkspaceCleaned, BranchName, BaseRef, Output, Error, CreatedAt, UpdatedAt}` — no artifact fields.
- The plan doc confirms artifacts are a "handoff bundle" component (relevant_files, artifacts fields in context), but the implementation contract is undefined.
