---
name: git-pr-review
description: "Review GitHub pull requests: checkout, run tests, post review comments, approve or request changes. Trigger: when reviewing a PR, doing code review, checking a pull request"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# GitHub PR Review

You are now operating in PR review mode. Follow this workflow for all code reviews.

## Review Workflow

### Step 1: Examine the PR

```bash
# View PR summary and metadata
gh pr view <pr-number>

# View the full diff
gh pr diff <pr-number>

# Check CI status
gh pr checks <pr-number>
```

### Step 2: Checkout Locally

```bash
# Check out the PR branch for local testing
gh pr checkout <pr-number>
```

### Step 3: Run Tests

```bash
# Run full test suite
go test ./...

# Run with race detector (mandatory)
go test ./... -race

# Check coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
```

### Step 4: Review Code Quality

Examine the diff for:
- **Correctness**: Does the logic match the stated intent?
- **Tests**: Are new code paths covered? Are edge cases tested?
- **Error handling**: Are all errors checked and propagated correctly?
- **Concurrency**: Any shared state that needs a mutex? Race conditions?
- **Security**: Any SQL injection, command injection, or hardcoded secrets?
- **Performance**: Any unnecessary allocations or blocking operations?

### Step 5: Post Your Review

```bash
# Approve the PR
gh pr review <pr-number> --approve

# Approve with a comment
gh pr review <pr-number> --approve --body "LGTM — tests pass, code looks clean."

# Request changes
gh pr review <pr-number> --request-changes --body "$(cat <<'EOF'
Please address the following before merge:

1. **Missing error check**: `os.Remove()` return value is unchecked on line 42
2. **Race condition**: `counter` field accessed from multiple goroutines without sync

Run `go test ./... -race` to confirm the race is detected.
EOF
)"

# Add a comment without a formal review decision
gh pr review <pr-number> --comment --body "Nit: consider extracting the retry logic into a helper."
```

### Step 6: Merge (if you have merge rights)

See the `git-merge-strategy` skill for merge workflow.

## Viewing PR Comments and History

```bash
# View all reviews and comments
gh pr view <pr-number> --comments

# View the PR timeline
gh pr view <pr-number> --json reviews,comments
```

## Checking Out Multiple PRs

If you need to review multiple PRs, return to main between checkouts:

```bash
git checkout main
gh pr checkout <next-pr-number>
```

## Review Checklist

- [ ] Read the PR description and linked issue
- [ ] Review the diff: `gh pr diff <pr-number>`
- [ ] Check CI status: `gh pr checks <pr-number>`
- [ ] Checkout and run tests locally: `go test ./... -race`
- [ ] Check for missing error handling
- [ ] Check for concurrency issues
- [ ] Post review with clear, actionable feedback
