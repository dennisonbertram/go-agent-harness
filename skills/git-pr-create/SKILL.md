---
name: git-pr-create
description: "Create GitHub pull requests with templates, labels, reviewers, and linked issues. Trigger: when creating a pull request, opening a PR, or submitting code for review"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# GitHub PR Creation

You are now operating in PR creation mode. Follow these guidelines for all pull request operations.

## Pre-PR Checklist

Before creating a PR:
1. Verify all tests pass: `go test ./... -race`
2. Confirm you are on the correct branch: `git branch --show-current`
3. Push your branch: `git push -u origin <branch-name>`
4. Check for any unstaged changes: `git status`

## Creating a Pull Request

### Basic PR

```bash
gh pr create --title "Add OAuth2 authentication" --body "$(cat <<'EOF'
## Summary
- Implement OAuth2 login flow with GitHub provider
- Add token refresh logic
- Update session handling

## Changes
- [ ] OAuth2 client configuration
- [ ] Token refresh middleware
- [ ] Session cookie management

## Test Plan
- [ ] Tests pass locally: `go test ./... -race`
- [ ] Manual verification of login flow

Closes #42
EOF
)"
```

### Draft PR (work in progress)

```bash
gh pr create --draft --title "WIP: Add OAuth2 authentication" --body "..."
```

### PR with Labels and Reviewers

```bash
gh pr create \
  --title "Fix nil pointer crash in runner" \
  --body "..." \
  --label "bug,fix" \
  --reviewer "dennisonbertram" \
  --base main
```

### Target a Specific Base Branch

```bash
gh pr create --base main --title "..." --body "..."
```

## PR Body Template

Always use this template for PR bodies:

```markdown
## Summary
<1-3 bullet points describing what changed and why>

## Changes
- [ ] Change 1
- [ ] Change 2

## Test Plan
- [ ] Tests pass locally: `go test ./... -race`
- [ ] Race detector clean: `go test ./... -race`
- [ ] Manual verification: <describe what you tested>

Closes #<issue-number>
```

## Listing PRs

```bash
# List open PRs
gh pr list

# List as JSON for scripting
gh pr list --json number,title,state,headRefName

# List PRs by label
gh pr list --label "bug"

# List your PRs
gh pr list --author "@me"
```

## Converting Draft to Ready

```bash
gh pr ready <pr-number>
```

## Editing a PR

```bash
gh pr edit <pr-number> --title "New title" --body "Updated body"
gh pr edit <pr-number> --add-label "enhancement" --remove-label "wip"
```
