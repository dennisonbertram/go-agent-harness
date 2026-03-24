# Issue #411 Grooming — feat(server): add a source-agnostic trigger envelope and deterministic external thread routing

**Date**: 2026-03-23
**Labels**: enhancement, infrastructure, well-specified, medium

## Already Addressed?

Partially. `RunRequest` in `internal/harness/types.go` has `TenantID`, `ConversationID`, and `AgentID` for conversation scoping. However, no normalized external trigger envelope type, no GitHub/Slack/Linear source metadata, no deterministic thread-ID derivation, and no routing code that distinguishes external-source payloads from direct API calls.

## Clarity

Clear. Normalized external-trigger envelope + deterministic thread-ID routing for GitHub/Slack/Linear sources feeding into existing SteerRun/ContinueRun primitives.

## Acceptance Criteria

Explicit:
- New `ExternalTriggerEnvelope` type (source, external thread ID, repo coords, task context, reply metadata)
- Deterministic thread-ID utilities (hash source + repo + thread identifier)
- Server routing helpers: active run → SteerRun, completed run → ContinueRun
- Validation: invalid/missing metadata fails closed
- Tests: thread routing, route selection, direct API runs unaffected

## Scope

Well-scoped Phase 2. Explicitly excludes GitHub/Slack/Linear source-specific parsing (Phase 3).

## Blockers

Logically follows #410 (recommended to land first). No hard technical blocker.

## Labels

Appropriate. No changes needed.

## Effort

Medium-Large — new envelope type, routing utilities, server HTTP changes, integration tests.

## Recommendation

**well-specified** — Ready after #410. Implement second in the epic sequence.
