# Issue #136 Grooming: Research: mid-run model switching

## Summary
Research how to support switching the active model mid-run (e.g., from a fast cheap model to a more capable one) without losing conversation context.

## Already Addressed?
**RESEARCH NOT DONE** — No research doc found in `docs/research/`. The `/model` REPL command (commit 94b8842) allows pre-run model selection but not mid-run switching in the runner.

## Clarity Assessment
Clear scope: research + design doc only (no implementation expected).

## Acceptance Criteria
- Design doc in `docs/research/` covering: context handoff, provider compatibility, state machine changes needed
- Recommend an implementation approach

## Scope
Research issue — output is a document, not code.

## Blockers
None.

## Effort
**Small** (2-4h) — Research, spike if needed, write doc.

## Label Recommendations
Current: `enhancement`. Recommended: `enhancement`, `research`

## Recommendation
**well-specified** — Spawn a research subagent to produce `docs/research/issue-136-mid-run-model-switching.md`.
