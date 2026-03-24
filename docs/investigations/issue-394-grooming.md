# Grooming: Issue #394 — docs(profiles): add runbooks for authoring and operating profiles

## Already Addressed?

No — No profile-specific runbooks exist anywhere in the codebase.

Evidence from `docs/runbooks/`:
- Existing runbooks: `issue-triage.md`, `documentation-maintenance.md`, `testing.md`, `observational-memory.md`, `terminal-bench-periodic-suite.md`, `tool-usability-testing.md`, `mcp.md`, `deployment.md`, `harnesscli-live-testing.md`, `INDEX.md`, `ownership-copy-semantics.md`, `provider-model-impact-mapping.md`, `worktree-flow.md`, `golden-path-deployment.md`
- None of these are profile-focused. `golden-path-deployment.md` mentions `--profile full` as a startup argument but does not explain how profiles work, how to author them, or how to debug profile-backed runs.

Evidence from `README.md` (not read directly but inferred from CLAUDE.md and plan context): The plan document lists `README.md` as a related file, implying profiles are not documented there either.

Evidence from `docs/plans/2026-03-20-profile-subagent-backlog-plan.md`:
- Section "Where To Find Additional Context" points to `docs/implementation/issue-237-profile-system.md` and `docs/investigations/issue-237-profile-system-research.md` — these are implementation notes, not user-facing runbooks.
- No user/operator docs for "how to choose a profile" or "how to author one" exist.

## Clarity

Clear — The issue body and plan document are explicit about what should be documented:
- How to choose a profile
- How to author a profile (TOML schema)
- How context handoff works between parent and child
- How to inspect and debug a child run

The plan document adds: "Make profiles usable without reading code." This is a well-stated goal.

## Acceptance Criteria

Present — The plan document lists the four runbook topics. Formalized:
1. `docs/runbooks/profile-authoring.md` — covers: TOML schema reference, built-in profiles catalog, how to create a user/project-level profile, inheritance/extends usage, validation
2. `docs/runbooks/profile-operations.md` (or incorporated into `golden-path-deployment.md`) — covers: how to choose a profile for a given task type, profile recommendation/auto-routing, how to start harnessd with a specific profile
3. `docs/runbooks/subagent-context-handoff.md` — covers: what gets passed from parent to child, handoff bundle fields, size limits and truncation behavior, how to inspect the handoff
4. `docs/runbooks/subagent-debugging.md` — covers: how to inspect a child run's status, how to read structured completion results, how to find child run logs/events
5. `docs/runbooks/INDEX.md` updated to include new runbooks

## Scope

Potentially too broad — Four new runbooks is a significant documentation effort. However, the issue explicitly scopes it to user/operator docs (not API reference), and the plan document's "Out of scope" for the overall backlog includes no documentation restrictions. The four topics map to distinct user workflows and could be split:
- Option A: One consolidated "profiles runbook" covering all four topics
- Option B: Four separate runbooks

The issue is marked as the last content ticket in the delivery order (position 19), correctly placed after the implementation it documents is complete.

## Blockers

Blocked on multiple upstream tickets — Documentation cannot be written accurately until the features it describes are implemented:

- **#377** (profile discovery HTTP surfaces) — required to document "how to inspect available profiles"
- **#378** (mutating profile management) — required to document "how to author and manage profiles"
- **#379** (expanded profile schema) — required to document the TOML schema reference accurately
- **#384** (context handoff bundles) — required to document "how context handoff works"

Without these, any runbook written now would describe planned behavior, not actual behavior, and would need to be rewritten after implementation. The plan document correctly places #394 at position 19 (second to last).

Writing documentation stubs now (with TBD sections) is possible but not the recommended approach — it creates maintenance debt.

## Recommended Labels

blocked, medium

## Effort

Medium — Estimated 2-3 days (once unblocked):
- Four runbook files at approximately 200-400 lines each
- INDEX.md update
- Potential README.md update
- No code changes required — pure documentation
- The main effort is reading the implementation and accurately describing it

If written as one consolidated runbook, effort drops to Small.

## Recommendation

blocked — This is a well-specified documentation ticket with the correct scope and clear deliverables. However, it cannot be executed until the implementation tickets it documents are merged (#377, #378, #379, #384 at minimum). Attempting to write these docs before the implementation lands will produce stale content. Recommend marking as `blocked` and scheduling after the core implementation wave completes.

The plan document's delivery order (position 19) is correct.

## Notes

- `docs/runbooks/INDEX.md` exists and will need updating when runbooks are added
- `docs/runbooks/golden-path-deployment.md` is the closest existing analog — it documents one operational path end-to-end. The new profile runbooks should follow its structure.
- `docs/plans/2026-03-20-profile-subagent-backlog-plan.md` (Section "Context Implementation Approach") contains a good description of the handoff bundle fields (`task`, `goal`, `definition_of_done`, `constraints`, `relevant_files`, etc.) — this section is a natural input to the context-handoff runbook.
- `internal/profiles/builtins/` likely contains the built-in profile TOML files (referenced in golden-path-deployment.md as `internal/profiles/builtins/full.toml`) — the schema in those files is the ground truth for the authoring runbook.
- The "TDD first: Not applicable beyond doc/link validation" note in the plan is accurate — docs tickets don't have failing tests, but link/reference validation (checking that docs reference real endpoints and files) is a reasonable lightweight acceptance check.
