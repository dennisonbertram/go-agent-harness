---
name: ci-debug
description: "Diagnose CI failures: parse error logs, identify flaky tests, compare with passing runs, suggest fixes. Trigger: when CI is failing, debugging a pipeline failure, investigating a flaky test"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# CI Debug

You are now operating in CI debug mode. Follow this systematic diagnostic process for all CI failures.

## Step 1: Get the Failed Run ID

```bash
# Find recent failed runs
gh run list --json databaseId,status,conclusion,name,headBranch \
  | jq '[.[] | select(.conclusion == "failure")]'

# Or find the run for a specific PR
gh pr checks <pr-number>
```

## Step 2: Fetch Failed Logs

```bash
# Get logs for failed jobs only (most efficient)
gh run view <run-id> --log-failed

# Get full logs if needed
gh run view <run-id> --log
```

## Step 3: Identify Failure Pattern

### Test Failures

Look for patterns like:
```
--- FAIL: TestMyFunction (0.00s)
    myfile_test.go:42: expected "foo", got "bar"
FAIL
FAIL    go-agent-harness/internal/mypackage    0.012s
```

**Fix**: Read the failing test, understand what it expects, fix the implementation.

### Build Failures

Look for patterns like:
```
./internal/mypackage/file.go:10:5: undefined: SomeType
./internal/mypackage/file.go:20:1: syntax error: unexpected token
```

**Fix**: Fix compilation errors first. They cascade — one error can cause many reported failures.

### Race Conditions

Look for patterns like:
```
WARNING: DATA RACE
Write at 0x... by goroutine ...:
  go-agent-harness/internal/...
Read at 0x... by goroutine ...:
  go-agent-harness/internal/...
```

**Fix**: Add mutex protection around the shared state. Run `go test ./... -race` locally to reproduce.

### Timeout Issues

Look for patterns like:
```
panic: test timed out after 10m0s
goroutine 1 [running]:
testing.(*M).startAlarm...
```

**Fix**: Identify the hanging goroutine from the stack trace. Common causes: deadlock, blocked channel, infinite loop.

### Environment Issues

Look for patterns like:
```
Error: missing required environment variable: API_KEY
```

**Fix**: Add the secret to repository settings (`gh secret set KEY`). Verify it's referenced correctly in the workflow.

### Dependency / Module Issues

Look for patterns like:
```
go: module go-agent-harness: go.sum file has wrong expected hash for module
```

**Fix**: Run `go mod tidy` and `go mod verify` locally, commit the updated `go.sum`.

## Step 4: Detecting Flaky Tests

A test is flaky if it passes sometimes and fails other times without code changes.

```bash
# Compare the last 10 runs for a specific workflow
gh run list --workflow ci.yml --limit 10 \
  --json databaseId,conclusion,createdAt \
  | jq '.[]'

# Re-run the failed job to see if it's flaky
gh run rerun <run-id> --failed

# If re-running fixes it, the test is likely flaky
```

**Signs of a flaky test**:
- Passes on retry without code changes
- Failure is non-deterministic (different goroutine in race output)
- Failure correlates with high system load or slow CI runners

**Fixes for flaky tests**:
- Race condition: add proper mutex protection
- Timing dependency: use `sync.WaitGroup` or channels instead of `time.Sleep`
- External dependency: mock the external call
- Ordering assumption: don't assume map or goroutine ordering

## Step 5: Re-run Strategy

```bash
# Re-run only failed jobs first (faster feedback)
gh run rerun <run-id> --failed

# If still failing, re-run everything
gh run rerun <run-id>

# Enable debug logging for more verbose output
gh run rerun <run-id> --debug --failed
```

## Step 6: Local Reproduction

Always try to reproduce the failure locally:

```bash
# Run the specific failing package
go test ./internal/mypackage/... -v -run TestMyFunction

# Run with race detector
go test ./internal/mypackage/... -race -run TestMyFunction

# Run multiple times to catch flakiness
go test ./internal/mypackage/... -race -count=10
```

## Common Fixes Reference

| Failure Pattern | Likely Cause | Fix |
|-----------------|--------------|-----|
| `undefined: X` | Missing import or renamed type | Check imports, run `go build ./...` |
| `DATA RACE` | Shared state without sync | Add mutex, use channels |
| `test timed out` | Deadlock or infinite loop | Check for blocked goroutines |
| `wrong expected hash` | `go.sum` out of date | Run `go mod tidy` |
| `connection refused` | External service not mocked | Add mock or skip in CI |
| `permission denied` | File created without write perms | Check `os.WriteFile` mode |
