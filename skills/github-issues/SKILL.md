---
name: github-issues
description: "Create, update, label, assign, and close GitHub issues with templates and linked PRs. Trigger: when creating GitHub issues, filing bugs, managing issue lifecycle, labeling issues, assigning issues, closing issues"
version: 1
argument-hint: "[create|list|view|close <number>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# GitHub Issues

You are now operating in GitHub issues management mode.

## Prerequisites

```bash
# Verify auth (always unset GITHUB_TOKEN first)
unset GITHUB_TOKEN && gh auth switch --user dennisonbertram
gh auth status
```

## Creating Issues

```bash
# Basic issue
gh issue create --title "Fix nil pointer in handler" --body "Description of the bug."

# With labels and assignee
gh issue create \
  --title "Add rate limiting to API" \
  --body "## Summary\nImplement rate limiting..." \
  --label "enhancement,api" \
  --assignee "@me"

# From a template (uses .github/ISSUE_TEMPLATE/)
gh issue create --template bug_report.md

# With milestone
gh issue create \
  --title "Update dependencies" \
  --body "Run go get -u ./... and verify tests pass." \
  --milestone "v1.2.0"

# Multi-line body using heredoc
gh issue create \
  --title "Implement health check endpoint" \
  --body "$(cat <<'EOF'
## Summary
Add a /health HTTP endpoint that returns 200 OK with a JSON body.

## Acceptance Criteria
- [ ] Returns 200 on success
- [ ] Returns 503 when dependencies are unhealthy
- [ ] Includes version and uptime in response

## Notes
See health-check skill for implementation patterns.
EOF
)"
```

## Listing Issues

```bash
# List open issues
gh issue list

# List with JSON output
gh issue list --json number,title,state,labels,assignees

# Filter by label
gh issue list --label "bug"
gh issue list --label "enhancement,skills"

# Filter by assignee
gh issue list --assignee "@me"

# Search by text
gh issue list --search "skills"

# All states (open + closed)
gh issue list --state all --limit 50
```

## Viewing Issues

```bash
# View in terminal
gh issue view 65

# Open in browser
gh issue view 65 --web

# View as JSON
gh issue view 65 --json number,title,body,state,labels,assignees
```

## Editing Issues

```bash
# Add labels
gh issue edit 65 --add-label "in-progress"

# Remove labels
gh issue edit 65 --remove-label "needs-triage"

# Add assignee
gh issue edit 65 --add-assignee "@me"

# Change title
gh issue edit 65 --title "Updated: Implement Database Management Skills"

# Set milestone
gh issue edit 65 --milestone "v1.2.0"
```

## Closing and Reopening

```bash
# Close as completed
gh issue close 65 --reason completed

# Close as not planned
gh issue close 65 --reason "not planned"

# Close with a comment
gh issue close 65 --comment "Completed in PR #72."

# Reopen
gh issue reopen 65
```

## Commenting

```bash
# Add a comment
gh issue comment 65 --body "Working on this — see branch issue-65-skill-files."

# Update with progress
gh issue comment 65 --body "$(cat <<'EOF'
## Progress Update

- [x] postgres-ops SKILL.md
- [x] sqlite-ops SKILL.md
- [ ] db-migrations SKILL.md

ETA: end of day.
EOF
)"
```

## Linking Issues to PRs

```bash
# Reference issue in PR body (GitHub auto-links)
# Use "Closes #65" in PR body to auto-close on merge
gh pr create \
  --title "Add database management skills" \
  --body "$(cat <<'EOF'
Closes #65

## Changes
- Added postgres-ops skill
- Added sqlite-ops skill
- Added db-migrations skill
EOF
)"

# Link from commit message
git commit -m "Add postgres-ops skill

Closes #65"
```

## Bulk Operations

```bash
# Close all issues with a specific label
gh issue list --label "stale" --json number --jq '.[].number' | \
  xargs -I{} gh issue close {} --reason "not planned"

# Export issues to CSV
gh issue list --json number,title,state,labels --jq \
  '.[] | [.number, .title, .state] | @csv' > issues.csv
```
