# Worktree Flow Runbook

## Policy

All implementation work must happen in a dedicated git worktree branch.

## Create Worktree

Preferred path for new agent worktrees:

```bash
./scripts/init.sh <task-slug>
```

That script creates a dedicated worktree under `.codex-worktrees/`, downloads Go dependencies, builds local binaries into the worktree, writes a sourceable env file, and can optionally start `harnessd` in tmux. `scripts/bootstrap-worktree.sh` remains as a compatibility wrapper.

```bash
git fetch origin
git worktree add ../go-agent-harness-<task-slug> -b <task-branch> main
cd ../go-agent-harness-<task-slug>
```

## Execute Task

1. Create or verify a current structured GitHub issue before branching or
   implementation. Follow `issue-driven-development.md`; minor changes still
   require an issue and PR.
2. Create plan in `docs/plans/` from `PLAN_TEMPLATE.md` and link the issue.
   - Reconcile the issue's impact analysis across current ownership/callers,
     config/API/CLI, persistence, lifecycle, security, clients,
     provider/model/tool catalogs, deployment, compatibility, tests, and docs.
   - For complex changes, create a one-page impact map from
     `docs/plans/IMPACT_MAP_TEMPLATE.md` before implementation and link it from
     the plan.
   - If a heading is truly unaffected, write `None` with rationale. Blank headings are a warning.
   - If the task touches exported or state-storing types with mutable fields, review `docs/runbooks/ownership-copy-semantics.md` before implementation.
3. Write failing tests first and record the expected red result.
4. Implement and keep the issue and checklist updated.
5. Run full tests and reconcile the PR template against the issue.

## Merge Back to Main (Test-Gated)

```bash
./scripts/verify-and-merge.sh <task-branch> "./scripts/test-regression.sh" main
```

The script runs pre-merge tests, merges to `main`, reruns tests on `main`, and pushes `main` automatically when `origin` is configured.

If merge conflicts occur, resolve and rerun full tests before retrying merge.
