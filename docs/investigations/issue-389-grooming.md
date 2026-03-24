# Grooming: Issue #389 — feat(profiles): implement the async profile efficiency reviewer loop

## Already Addressed?

Partial — A minimal stub of this concept exists: `maybeEmitProfileEfficiencySuggestion()` in `internal/harness/runner.go` fires a `profile.efficiency_suggestion` SSE event when a run's efficiency score falls below 0.6. `BuildEfficiencyReport()` in `internal/profiles/efficiency.go` generates a deterministic (non-LLM) `EfficiencyReport`. However, this is event-only — nothing is persisted, no async reviewer agent is launched, no `EfficiencyReport` is written to the store, and there is no trigger-condition logic that invokes an LLM-backed reviewer. The `ReviewerRunID` field on `EfficiencyReport` (in `profile.go`) exists but is never populated.

## Clarity

Unclear — Several key design questions are unanswered:
- What does "reviewer-backed" mean? Is the reviewer an LLM subagent run (using the `reviewer` built-in profile), a deterministic post-processor, or both?
- How is the async reviewer triggered? Goroutine at run completion? A cron job? An event subscriber?
- What does the reviewer write? A persisted `EfficiencyReport`? A new `profile_suggestions` record? An updated profile TOML?
- How are reviewer failures surfaced (errors, timeouts)?
- What is the shutdown/cancellation story for the async goroutine?
- Is the reviewer per-run or per-profile (batched across recent runs)?

## Acceptance Criteria

Missing — The issue says "analyzes completed profile runs and writes persisted suggestions" but does not specify:
- The trigger condition (every completed profiled run? only when score < threshold? minimum N runs before triggering?).
- The reviewer's input (a single `EfficiencyReport`? a batch of recent run stats?).
- The reviewer's output type and storage location.
- The assertion that it is "suggest-only, not auto-apply" (implied but not stated as an AC).
- Test coverage expectations for async trigger paths.

## Scope

Too broad — This ticket combines: async trigger design, reviewer-agent integration, persisted output writing, and error/shutdown handling. Each piece has non-trivial design decisions. The plan doc acknowledges this is one of the later, higher-dependency tickets (#15 of 20 in the delivery order). It should be split into: (a) async trigger plumbing with noop reviewer, (b) reviewer agent invocation and result parsing, (c) persisted suggestion writing.

## Blockers

Hard blockers: #387 (persistence layer — the reviewer needs history to analyze) and #388 (report retrieval surface — confirms reports are readable before auto-trigger writes them). Also depends on #383 (subagent lifecycle) for launching the reviewer as a child run.

Per the plan doc: "Dependencies: Tickets 9 (subagent lifecycle), 13 (#387), and 14 (#388)."

Blockers: #387, #388, and #383 (subagent lifecycle).

## Recommended Labels

needs-clarification, large, blocked

## Effort

Large — The async plumbing (goroutine, context propagation, cancellation), reviewer-agent integration (launching a subagent, parsing its structured result), persistence writes, error handling, and tests all constitute substantial work. The lack of a defined reviewer output schema and trigger contract makes estimation uncertain.

## Recommendation

needs-clarification — The issue needs a concrete definition of: (1) the trigger condition, (2) whether the "reviewer" is an LLM subagent or a rule-based post-processor, (3) what it writes and where. Also hard-blocked on #387 and #388. Recommend deferring detailed grooming until #387 and #388 are in progress.

## Notes

- `internal/harness/runner.go` line 3192-3221: `maybeEmitProfileEfficiencySuggestion` is the current event-only stub — the reviewer loop would extend or replace this.
- `internal/profiles/profile.go`: `EfficiencyReport.ReviewerRunID string` field exists but is never set — this confirms the reviewer integration was anticipated but not implemented.
- `internal/profiles/builtins/reviewer.toml`: a `reviewer` built-in profile already exists (read-only tools: read, grep, glob, ls, git_diff; 25 steps; code review system prompt). This is a natural candidate for the LLM reviewer agent.
- `internal/profiles/efficiency.go`: `BuildEfficiencyReport()` provides the deterministic phase — the async reviewer would add an LLM-based second phase on top.
- Referenced doc `docs/implementation/issue-237-profile-system.md` confirms the suggest-only constraint: "no profile changes are applied automatically."
- The plan doc warns: "Land profile correctness, CRUD, context, and lifecycle before reviewer automation" — this is explicitly a later-stage ticket.
- No async reviewer goroutine, worker, or trigger exists anywhere in `internal/profiles/` or `internal/harness/`.
