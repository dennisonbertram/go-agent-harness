---
name: git-merge-strategy
description: "Squash merge, rebase, and conflict resolution strategies for merging pull requests. Trigger: when merging a PR, resolving merge conflicts, choosing a merge strategy"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# Git Merge Strategy

You are now operating in merge strategy mode. Follow these guidelines for all merge operations.

## Merge Strategy Selection

| Strategy | When to use | Command |
|----------|-------------|---------|
| Squash | Default — keeps main history clean | `gh pr merge --squash --delete-branch` |
| Rebase | Preserve individual commits on main | `gh pr merge --rebase --delete-branch` |
| Merge commit | When commit history must be fully preserved | `gh pr merge --merge` |

**Default: always squash merge** unless there is a specific reason to preserve history.

## Merging via GitHub CLI

### Squash Merge (default)

```bash
gh pr merge <pr-number> --squash --delete-branch
```

### Rebase Merge

```bash
gh pr merge <pr-number> --rebase --delete-branch
```

### Merge Commit

```bash
gh pr merge <pr-number> --merge
```

### Auto-merge (merges when CI passes)

```bash
gh pr merge <pr-number> --squash --auto
```

## Pre-Merge Checklist

Before merging any PR:
1. CI is green: `gh pr checks <pr-number>`
2. At least one approval: `gh pr view <pr-number> --json reviews`
3. No requested changes pending
4. Branch is up to date with base branch

## Conflict Resolution

When a branch has conflicts with the base branch:

### Step 1: Fetch and rebase

```bash
git fetch upstream
git checkout <feature-branch>
git rebase upstream/main
```

### Step 2: Resolve conflicts

When rebase stops at a conflict:

```bash
# See which files have conflicts
git status

# Edit the conflicting files to resolve
# Look for <<<<<<, =======, >>>>>>> markers

# After resolving each file:
git add <resolved-file>

# Continue the rebase
git rebase --continue
```

### Step 3: Push the resolved branch

```bash
# Safe force push (refuses if remote has unexpected commits)
git push --force-with-lease origin <feature-branch>
```

## Force Push Safety

**Never use `git push --force`.**

Always use `git push --force-with-lease` — it refuses to overwrite if the remote has received new commits since your last fetch. This prevents accidentally overwriting a teammate's work.

```bash
# WRONG — can overwrite teammates' work
git push --force origin my-branch

# CORRECT — safe force push
git push --force-with-lease origin my-branch
```

## Merge via Project Scripts

This project uses a verify-and-merge script:

```bash
./scripts/verify-and-merge.sh <branch> "./scripts/test-regression.sh" main
```

This script:
1. Runs the regression test suite on the branch
2. Checks coverage gate (80% minimum)
3. Merges only if all checks pass

## Post-Merge Cleanup

```bash
# Return to main and pull latest
git checkout main
git fetch upstream
git merge upstream/main

# Delete the local branch
git branch -d <feature-branch>
```
