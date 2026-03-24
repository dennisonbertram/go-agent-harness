# Issue #413 Grooming — feat(integrations): add Slack and Linear trigger adapters on the shared trigger layer

**Date**: 2026-03-23
**Labels**: enhancement, infrastructure, well-specified, medium

## Already Addressed?

No. No Slack or Linear webhook ingestion, auth, or context hydration exists.

## Clarity

Clear. Slack and Linear adapters on top of shared trigger envelope from #411, following GitHub patterns from #412.

## Acceptance Criteria

Explicit:
- Slack: signature/payload validation, thread mapping, context hydration, follow-up routing
- Linear: signature/payload validation, thread mapping, context hydration, follow-up routing
- Reuse shared envelope, deterministic routing, reply-sink
- Tests: signature validation for each source, thread mapping, context hydration, fallback behavior
- Pin: direct HTTP API flows remain unaffected

## Scope

Atomic — batches Slack and Linear together since they share identical infrastructure from #411.

## Blockers

Requires #411 and #412 (GitHub) to be implemented first. Logically also benefits from #414 (workspace backend selection) for backend routing.

## Labels

Appropriate. No changes needed.

## Effort

Medium — leverages proven GitHub patterns from #412.

## Recommendation

**well-specified** — Ready after #412. Implement last (or concurrently with #414).
