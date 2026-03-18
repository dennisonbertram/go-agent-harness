# Agent Bootstrap Guide

This file is a quick-start map. Most working rules and context live in the docs below.

## 1) Start Here (Bootstrap Order)

1. `docs/context/critical-context.md` (project intent and operating model)
2. `docs/logs/long-term-thinking-log.md` (command intent, user intent, success definition)
3. `docs/plans/active-plan.md` and `docs/plans/PLAN_TEMPLATE.md` (plan before implementation)
4. `docs/runbooks/worktree-flow.md` and `docs/runbooks/testing.md` (execution constraints)
5. `docs/logs/engineering-log.md`, `docs/logs/observational-log.md`, `docs/logs/system-log.md` (current state)
6. `docs/design/ux-requirements.md` (user-facing requirements)
7. `docs/operations/nightly-tasks.md` and `docs/operations/agent-completion-template.md` (automation and reporting)

## 2) Intent Precedence (When Unsure)

- Always resolve uncertainty using:
1. Command intent (what the request explicitly asks to accomplish)
2. User intent (what outcome matters most to the user)
- If details are ambiguous, default to the action that best satisfies command intent and user intent together.
- Success criteria must be written or updated in `docs/logs/long-term-thinking-log.md`.

## 3) Non-Negotiables (Detailed Rules in Runbooks)

- Strict TDD and no trivial/underspecified tests: `docs/runbooks/testing.md`
- Tests must pass before commit: `docs/runbooks/testing.md`
- Zero tolerance for broken tests — pre-existing test failures must be fixed before merging new work; broken tests mask regressions
- Worktree-only implementation and test-gated merge to `main`: `docs/runbooks/worktree-flow.md`
- Use `scripts/verify-and-merge.sh` for auto-merge and auto-push to `main` after tests pass.
- Every bug requires engineering-log entry + regression test + GitHub issue: `docs/runbooks/issue-triage.md`
- Maintain folder indexes on every doc change: `docs/runbooks/documentation-maintenance.md`
- Provider/model flow changes require a one-page impact map across config, server API, TUI state, and regression tests before implementation; blank sections are a warning, not an acceptable omission.
- Long-running processes must run in tmux.
- Enforcement mode for now: process-guided (documentation and agent discipline), not hard-blocked by local hooks/CI gates.
- Do not suggest follow-up work unless it is directly required to complete the current task.
- Commit policy: commit the files you changed for the current task by default. Only commit all dirty/unrelated files when the user explicitly asks to commit everything.
- Every final response must explicitly state completion status with a clear line:
  - `Task status: DONE` when the requested work is fully complete.
  - `Task status: NOT DONE` when anything remains, followed by the exact blocker or missing item.
- Never leave task completion implicit.

## 4) Documentation Navigation

- Master docs index: `docs/INDEX.md`
- Research: `docs/research/INDEX.md`
- Design: `docs/design/INDEX.md`
- Explorations: `docs/explorations/INDEX.md`
- Plans: `docs/plans/INDEX.md`
- Logs: `docs/logs/INDEX.md`
- Context: `docs/context/INDEX.md`
- Runbooks: `docs/runbooks/INDEX.md`
- Operations: `docs/operations/INDEX.md`
