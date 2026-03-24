# Issue #408 Grooming — epic(integrations): add a repo-aware external trigger and context layer

**Date**: 2026-03-23
**Labels**: infrastructure, epic, well-specified

## Already Addressed?

No. The codebase has no trigger envelope, GitHub/Slack/Linear adapters, external thread routing, or repo-aware AGENTS.md loading. This is a tracking umbrella issue.

## Clarity

Clear. Backed by two detailed planning docs:
- `docs/plans/2026-03-22-open-swe-trigger-context-adoption-plan.md`
- `docs/plans/2026-03-22-open-swe-trigger-context-impact-map.md`

Explicitly excludes: copying open-swe wholesale, replacing current active implementation plan.

## Acceptance Criteria

Explicit — six phases with concrete deliverables defined in planning docs:
1. Repo AGENTS.md auto-loading (#410)
2. Source-agnostic trigger envelope + thread routing (#411)
3. GitHub-first trigger adapter (#412)
4. Slack/Linear adapters (#413)
5. Explicit workspace-backend selection (#414)

## Scope

Well-scoped epic. Child issues are correctly decomposed and independently implementable.

## Blockers

None technical. Issue #361 must not be disrupted (per long-term-thinking log).

## Labels

Appropriate: infrastructure, epic, well-specified.

## Effort

Large — 5 child issues, multiple integrations, new HTTP surfaces, comprehensive tests.

## Recommendation

**well-specified** — tracking epic, do not implement directly. Implement via child issues #410–#414 in order.
