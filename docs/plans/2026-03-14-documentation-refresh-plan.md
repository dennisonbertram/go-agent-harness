# Plan: Documentation Refresh

## Context

- Problem: the repository's indexes and overview docs no longer describe the full current docs tree, binary surface, API surface, and tmux-first operating workflow.
- User impact: contributors can follow stale docs, miss important subsystems, or get the wrong picture of what the application currently ships.
- Constraints:
  - Preserve historical implementation notes as records rather than rewriting history.
  - Update every documentation folder index that is missing or incomplete.
  - Keep long-running-process guidance aligned with the tmux requirement in `AGENTS.md`.

## Scope

- In scope:
  - Refresh `README.md` to match the current binaries, API surface, and runtime workflow.
  - Add missing `INDEX.md` files and update stale folder indexes across `docs/`.
  - Update runbooks that conflict with current tmux/process guidance.
  - Record the documentation refresh in planning and engineering logs.
- Out of scope:
  - Rewriting historical issue implementation notes that intentionally capture past point-in-time work.
  - Adding new product features or changing runtime behavior.

## Test Plan (TDD)

- New failing tests to add first:
  - None. This task updates documentation only.
- Existing tests to update:
  - None.
- Regression tests required:
  - Verify every top-level `docs/` folder has an `INDEX.md`.
  - Cross-check README and runbooks against `cmd/`, `internal/server/http.go`, and the repo's tmux policy.

## Implementation Checklist

- [x] Verify the current binaries, routes, and docs tree against source files.
- [x] Refresh `README.md` to describe the current system surface.
- [x] Add missing `INDEX.md` files and update stale folder indexes.
- [x] Update current-facing runbooks to match the tmux-first process guidance.
- [x] Update planning and engineering logs for the documentation refresh.
- [x] Run documentation sanity checks (`git diff --check`, docs-index presence, tmux guidance grep).

## Risks and Mitigations

- Risk: README or indexes may still drift from the live command/API surface.
- Mitigation: verify against `cmd/`, `internal/server/http.go`, and current runbook/process rules before finishing.
