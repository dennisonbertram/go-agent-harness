# Worktree Flow Runbook

## Policy

All implementation work must happen in a dedicated git worktree branch.

## Create Worktree

```bash
git fetch origin
git worktree add ../go-agent-harness-<task-slug> -b <task-branch> main
cd ../go-agent-harness-<task-slug>
```

## Execute Task

1. Create plan in `docs/plans/` from `PLAN_TEMPLATE.md`.
   - If the task touches provider/model flows, create a one-page impact map from `docs/plans/IMPACT_MAP_TEMPLATE.md` before implementation and link it from the plan.
   - Fill all four headings: config, server API, TUI state, regression tests.
   - If a heading is truly unaffected, write `None` with rationale. Blank headings are a warning.
   - If the task touches exported or state-storing types with mutable fields, review `docs/runbooks/ownership-copy-semantics.md` before implementation.
2. Write failing tests first.
3. Implement and keep checklist updated.
4. Run full tests.

## Merge Back to Main (Test-Gated)

```bash
./scripts/verify-and-merge.sh <task-branch> "./scripts/test-regression.sh" main
```

The script runs pre-merge tests, merges to `main`, reruns tests on `main`, and pushes `main` automatically when `origin` is configured.

If merge conflicts occur, resolve and rerun full tests before retrying merge.
