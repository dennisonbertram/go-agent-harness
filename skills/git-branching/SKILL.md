---
name: git-branching
description: "Create and manage git branches with consistent naming conventions. Trigger: when creating a branch, working with feature branches, bug fix branches, or release branches"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# Git Branching

You are now operating in git branching mode. Follow these conventions for all branch operations.

## Branch Naming Conventions

All branches must use kebab-case with a type prefix:

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/<issue-number>-<short-description>` | `feature/42-add-auth` |
| Bug fix | `fix/<issue-number>-<short-description>` | `fix/17-nil-pointer-crash` |
| Release | `release/v<major>.<minor>.<patch>` | `release/v1.2.0` |
| Hotfix | `hotfix/<issue-number>-<description>` | `hotfix/99-production-outage` |
| Worktree | `worktree-agent-<hash>` | `worktree-agent-a1b2c3d` |

## Creating Branches

```bash
# Create and switch to a new branch
git checkout -b feature/42-add-oauth

# Create from a specific base
git checkout -b fix/17-crash upstream/main

# Verify branch was created
git branch --show-current
```

## Listing Branches

```bash
# List all local branches
git branch

# List branches matching a pattern
git branch --list 'feature/*'

# List remote branches
git branch -r

# List all (local + remote)
git branch -a
```

## Deleting Branches

```bash
# Delete a merged branch (safe — refuses to delete if unmerged)
git branch -d feature/42-add-oauth

# Delete the remote tracking branch
git push origin --delete feature/42-add-oauth

# Prune stale remote refs
git fetch --prune
```

## Keeping Branches Up to Date

```bash
# Fetch latest from upstream
git fetch upstream

# Rebase current branch onto upstream/main
git rebase upstream/main

# Alternative: merge upstream into current branch
git merge upstream/main
```

## Safety Rules

- Never use `--force` when pushing. Use `--force-with-lease` if you must force push.
- Never push directly to `main` or `master`. Always use a branch + PR workflow.
- Always verify the current branch before making commits: `git branch --show-current`
- Delete merged branches promptly to keep the branch list clean.
