# Issue #414 Grooming — feat(workspace): add explicit workspace-backend selection for runs, profiles, and trigger adapters

**Date**: 2026-03-23
**Labels**: enhancement, infrastructure, well-specified, medium

## Already Addressed?

Partially. `internal/profiles/profile.go` already has `IsolationMode`, `CleanupPolicy`, and `BaseRef` fields. `RunRequest` has a `WorkspaceType` field. However, the explicit policy wiring from trigger adapters → profiles → backend selection is not implemented.

## Clarity

Mostly clear. The issue is about adding request/profile-level backend selector and threading selection through trigger adapters. The partial overlap with existing fields (profile.IsolationMode, RunRequest.WorkspaceType) means the scope needs to be clarified:
- Is this about adding new fields, or wiring existing ones?
- Does this include profile override precedence (default vs per-run)?
- How does fallback work when backend is unavailable?

## Acceptance Criteria

Explicit from issue body:
- Request/profile-level backend selector (worktree/container/vm/future)
- Default backend and fallback behavior defined
- Thread through trigger adapters without making them the source of truth
- Tests: request-level selection, profile default+override precedence, fallback on unavailable backend, trigger-adapter propagation

## Scope

Mostly atomic — but overlaps with existing RunRequest.WorkspaceType. May need clarification on whether this extends existing fields vs adds new ones.

## Blockers

Can be implemented independently from trigger adapters (pure backend/workspace layer). No hard blockers.

## Labels

well-specified is arguably slightly optimistic given partial implementation overlap. Labels are otherwise appropriate.

## Effort

Medium — glue work given profiles already support isolation fields; need wiring through runner and trigger adapters.

## Recommendation

**well-specified** — Accept as-is given existing planning docs. Can be implemented in parallel with #412. Implementer should check existing WorkspaceType field in RunRequest and IsolationMode in Profile to avoid duplication.
