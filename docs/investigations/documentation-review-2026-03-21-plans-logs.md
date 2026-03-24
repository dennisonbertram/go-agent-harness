# Documentation Review: Plans, Logs, and Index Files

**Date**: 2026-03-21
**Scope**: `docs/plans/INDEX.md`, `docs/logs/engineering-log.md`, `docs/logs/long-term-thinking-log.md`, and all other non-runbook/non-investigation `.md` files in `docs/` subdirectories. Cross-referenced against current code in `internal/harness/`, `internal/server/`, `internal/config/`, `internal/profiles/`, `internal/skills/`, `internal/workspace/`, and `cmd/`.

---

## 1. docs/plans/INDEX.md

### Accurate
- All 25 plan files referenced in the INDEX exist on disk.
- Descriptions match the actual plan content (spot-checked `2026-03-19-issue-361-golden-path-smoke-plan.md`, `2026-03-20-profile-subagent-backlog-plan.md`, `PLAN_TEMPLATE.md`, `IMPACT_MAP_TEMPLATE.md`).
- `active-plan.md` is correctly referenced.

### Mismatches / Outdated
- **INDEX omits ~30 plan files** that exist in the directory but are not listed. Missing entries include:
  - `issue-11-plan.md`, `issue-181-plan.md`, `issue-184-plan.md`, `issue-185-plan.md`, `issue-187-plan.md`
  - `orchestrator-workspace-plumbing-plan.md`, `demo-cli-model-picker-plan.md`
  - `issue-231-plan.md` through `issue-238-plan.md` (various)
  - `mcp-client-http-transport.md`, `mcp-server-stdio-transport.md`, `mcp-client-per-run-config.md`, `mcp-server-sse-streaming.md`, `mcp-client-config-startup.md`, `mcp-server-richer-tools.md`
  - `conclusion-watcher-plan.md`, `tui-ux-implementation-plan.md`, `tui-issue-map.md`
  - `issue-318-plan.md`, `issue-319-plan.md`, `issue-321-plan.md`, `issue-325-plan.md`
  - `issue-362-plan.md`, `issue-364-plan.md`
  - `2026-03-19-issue-361-impact-map.md`
  - `issue-375-plan.md`, `issue-391-plan.md`, `issue-393-plan.md`
- **`active-plan.md` is stale**: says issue #361 is the active plan and #362 is implemented, but the most recent engineering log entry (2026-03-20) covers the profile backlog planning (#375-#394) with no update to the active plan pointer.

---

## 2. docs/logs/engineering-log.md

### Accurate
- **`waitForTerminalResult` refactor (2026-03-18)**: Verified -- `waitForTerminalResult` exists in `internal/harness/runner.go` at line 1076 and is called from both `RunPrompt` and `RunForkedSkill`.
- **`copyMessages` normalization**: Verified -- `copyMessages` in `internal/harness/clone.go` is used 15+ times throughout `runner.go` for defensive cloning.
- **`ToolDefinition.Clone()`, `Message.Clone()`, `ToolCall.Clone()`**: All confirmed in `internal/harness/types.go`.
- **`startRecorderGoroutine` ordering**: Verified -- exists in `runner.go` at line 5327.
- **`checkConversationOwnership` (issue #221)**: Verified at line 562 of `runner.go`.
- **Profile backlog issues #375-#394**: Verified -- the plan doc, INDEX entry, and long-term thinking log entry all align.
- **REST vs GraphQL decision**: Accurate at the time written (2026-03-05). The "6 endpoints" claim was correct when written; the server has since grown to 21+ route registrations but the decision rationale remains valid.

### Mismatches / Outdated
- **"6 endpoints" claim (2026-03-05)**: The server now has 21+ route registrations. This is not wrong (it was accurate at the time), but could mislead readers who don't realize it's a point-in-time observation. Not a code contradiction.
- **Head-tail buffer entry (2026-03-06)**: States "blocked by required full regression gate failure (no commit/merge performed)" but the feature (`head_tail_buffer.go`, `bash_manager.go` integration) is present in the current codebase, indicating it was subsequently merged. The log entry was never updated to reflect the merge.
- **`TestApplyPatchToolAcceptsUnifiedPatchPayload` failure**: Referenced as an existing blocker in both the 2026-03-05 and 2026-03-06 entries. No follow-up entry documents its resolution.

---

## 3. docs/logs/long-term-thinking-log.md

### Accurate
- All entries follow the established template (date, command intent, user intent, success definition, non-goals, guardrails, open questions, next verification step).
- The profile-backed subagent backlog entry (2026-03-20) accurately describes the created GitHub issues (#375-#394) and plan doc location.
- The golden-path deployment entry (2026-03-19) matches the plan at `2026-03-19-issue-361-golden-path-smoke-plan.md`.
- Ownership/copy-semantics entry matches the actual `clone.go` and `registry.go` code.

### Mismatches / Outdated
- **Open question about `spawn_agent` vs profile-backed path (2026-03-20)**: Still listed as open. Both `spawn_agent` and `run_agent` tools exist in the current codebase (`internal/harness/tools/deferred/`), so the question remains architecturally relevant and unresolved.
- **Provider token streaming entry (2026-03-05)**: Lists "whether to expose separate event types for tool-call creation vs argument deltas" as an open question. The code now has `EventToolCallStarted`, `EventToolCallCompleted`, and `EventToolCallDelta` as separate event types, which resolves this question. The log was never updated.
- **Regression enforcement entry (2026-03-05)**: `MIN_TOTAL_COVERAGE` tuning question remains open with no follow-up entry, though coverage gating has been actively worked on (2026-03-18 entries).

---

## 4. docs/INDEX.md (Top-Level)

### Accurate
- All referenced subdirectory INDEX files exist: `research/`, `design/`, `explorations/`, `testing/`, `plans/`, `logs/`, `context/`, `runbooks/`, `operations/`.

### Mismatches / Outdated
- **Missing subdirectories**: Two docs subdirectories are not referenced:
  - `docs/implementation/` -- contains 40 implementation docs but has no INDEX.md and is not in the top-level INDEX.
  - `docs/process/` -- contains 7 swarm process docs (under `process/swarms/`) but has no INDEX.md and is not in the top-level INDEX.

---

## 5. Subdirectory INDEX Files

### docs/design/INDEX.md
- **Missing entries**: `event-catalog.md` and `plugins.md` exist in the directory but are not listed in the INDEX.

### docs/testing/INDEX.md
- **Severely stale**: Lists only 1 file (`terminal-bench-harder-suite-2026-03-06.md`) but the directory contains 20+ test result and usability files. Missing entries include all `usability-*-round2.md` files, `manual-curl-smoke-test*.md`, `harness-smoke-test-*.md`, `compaction-trigger-test-20260313.md`, `test-run-2026-03-12-uuid-verbose.md`, `issue-369-branch-test-results.md`, `post-merge-main-test-results.md`.

### docs/research/INDEX.md
- **Significantly stale**: Lists 5 Claude Code UX research files but omits 20+ other research files in the directory, including: `openai-api-completions-and-uploads-research.md`, `anthropic-api-completions-format-research.md`, `charmbracelet-crush-agentic-loop-research.md`, `go-agent-harness-patterns-and-practices.md`, `crush-tools-inspection-and-harness-spec.md`, `research-log.md`, `deferred-tools-design.md`, `codex-cli-architecture.md`, `opencode-review.md`, `codex-cli-review.md`, `crush-review.md`, `pi-review.md`, `harness-comparison-synthesis.md`, `scaling-sub-agent-architecture.md`, `issue-136-mid-run-model-switching.md`, `issue-225-lightpanda-evaluation.md`, `issue-23-os-sandboxing.md`, `issue-24-session-tree-branching.md`.

### docs/explorations/INDEX.md -- Accurate
### docs/operations/INDEX.md -- Accurate
### docs/logs/INDEX.md -- Accurate

---

## 6. Code-Level Verification Summary

| Claim | Source | Status |
|-------|--------|--------|
| Profile struct has `isolation_mode`, `cleanup_policy`, `base_ref`, `reasoning_effort`, `result_mode`, `permissions` | MEMORY.md | Verified in `internal/profiles/profile.go` |
| `ForkConfig` has `Model` and `MaxSteps` fields | MEMORY.md | Verified in `internal/harness/tools/types.go` |
| Profile tools (list, get, create, update, delete, validate, recommend, efficiency_report) exist | MEMORY.md | Verified -- all in `internal/harness/tools/deferred/` |
| HTTP endpoints: GET/POST/PUT/DELETE for profiles | MEMORY.md | Verified in `internal/server/http_profiles.go` |
| `ChildResult` in `internal/harness/tools/deferred/result.go` | MEMORY.md | Verified |
| `store/profile_run_store.go` exists | MEMORY.md | Verified |
| `deepClonePayload` uses reflection | MEMORY.md | Verified in `internal/harness/clone.go` |
| EventType count "currently 54" | MEMORY.md | **OUTDATED** -- actual count is 76 event types in `AllEventTypes()` |
| Skills P1-P6 implemented | MEMORY.md | Verified -- loader, resolver, preprocess, hooks, constraint tracker, and types all in `internal/skills/` |
| Workspace implementations (local, worktree, container, vm, hetzner, pool) | MEMORY.md | Verified in `internal/workspace/` |

---

## Summary of Action Items

1. **Plans INDEX.md**: Add ~30 missing plan file references.
2. **active-plan.md**: Update to reflect current state (post-profile-backlog planning).
3. **Testing INDEX.md**: Add 20+ missing test result file references.
4. **Research INDEX.md**: Add 20+ missing research file references.
5. **Design INDEX.md**: Add `event-catalog.md` and `plugins.md`.
6. **Top-level docs/INDEX.md**: Add references to `docs/implementation/` and `docs/process/` (and create INDEX files for both).
7. **Engineering log**: Add follow-up entry for head-tail buffer merge status.
8. **Long-term thinking log**: Close the resolved open question about separate tool-call event types (now implemented).
9. **MEMORY.md**: Update EventType count from 54 to 76.
