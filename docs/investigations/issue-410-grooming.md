# Issue #410 Grooming — feat(prompt): auto-load repo AGENTS.md into resolved system prompts

**Date**: 2026-03-23
**Labels**: enhancement, infrastructure, well-specified, medium

## Already Addressed?

No. The `systemprompt/engine.go` Resolve method chains BASE → INTENT → MODEL_PROFILE → TASK_CONTEXT → BEHAVIORS → TALENTS → SKILLS with no repo-local file loading.

## Clarity

Clear. Auto-load repo root `AGENTS.md` into resolved system prompt as a distinct, provenance-labeled section when workspace/repo root is known.

## Acceptance Criteria

Explicit:
- Detect repo root from workspace context
- Read only the root `AGENTS.md` (no recursion)
- Inject as a distinct prompt section with provenance label after static resolution
- Fail soft when file is absent
- Fail closed on path-escape scenarios
- Tests: present/absent/unreadable/path-containment

## Scope

Atomic. Touches only `internal/systemprompt/engine.go` and related tests. Clear single responsibility.

## Blockers

None. Can be implemented independently.

## Labels

Appropriate. No changes needed.

## Effort

Medium — detect repo root, read file, add prompt section, write regression tests.

## Recommendation

**well-specified** — Ready for implementation. Implement first in the epic sequence.
