# Issue #412 Grooming — feat(github): add GitHub trigger ingestion and source-context hydration

**Date**: 2026-03-23
**Labels**: enhancement, infrastructure, well-specified, medium

## Already Addressed?

No. No GitHub webhook ingestion, signature validation, context hydration, or reply surfaces exist in the codebase.

## Clarity

Clear. GitHub-first trigger adapter ingesting issue/comment/review events, hydrating task context from payloads, routing follow-ups via deterministic thread mapping.

## Acceptance Criteria

Explicit:
- Webhook ingestion for supported GitHub event types
- Signature validation, fail closed on invalid
- Context hydration from title, body, comments, repo metadata
- Deterministic thread routing (repeated mentions → same conversation)
- Follow-up routing: active run → SteerRun, completed run → ContinueRun
- Outbound GitHub replies for acknowledgements, clarifications, completion summaries
- Tests: signature validation, mention/event filtering, context hydration, thread mapping, reply behavior

## Scope

Atomic — GitHub-only. Explicitly excludes Slack/Linear (Phase 5).

## Blockers

Requires #411 (trigger envelope substrate) to be implemented first.

## Labels

Appropriate. No changes needed.

## Effort

Medium — builds on #411 infrastructure, GitHub API/webhook-specific parsing.

## Recommendation

**well-specified** — Ready after #411. Implement third in the epic sequence.
