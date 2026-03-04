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
2. Write failing tests first.
3. Implement and keep checklist updated.
4. Run full tests.

## Merge Back to Main (Test-Gated)

```bash
./scripts/verify-and-merge.sh <task-branch> "go test ./..." main
```

The script runs pre-merge tests, merges to `main`, reruns tests on `main`, and pushes `main` automatically when `origin` is configured.

If merge conflicts occur, resolve and rerun full tests before retrying merge.
