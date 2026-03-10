---
name: github-actions
description: "Manage GitHub Actions workflows: view runs, re-run failed jobs, read logs, and debug CI failures. Trigger: when managing CI, viewing workflow runs, re-running GitHub Actions, checking pipeline status"
version: 1
allowed-tools:
  - bash
  - read
  - glob
  - grep
---
# GitHub Actions

You are now operating in GitHub Actions management mode. Follow these guidelines for all CI/CD operations.

## Viewing Workflow Runs

```bash
# List recent runs (all workflows)
gh run list

# List runs with status details as JSON
gh run list --json status,name,conclusion,databaseId,headBranch

# List runs for a specific workflow
gh run list --workflow ci.yml

# List runs for a specific branch
gh run list --branch feature/42-add-auth

# Limit output
gh run list --limit 10
```

## Viewing a Specific Run

```bash
# View run summary
gh run view <run-id>

# View detailed run logs
gh run view <run-id> --log

# View only failed job logs (most useful for debugging)
gh run view <run-id> --log-failed

# Watch a run in progress
gh run watch <run-id>
```

## Re-running Jobs

```bash
# Re-run the entire workflow
gh run rerun <run-id>

# Re-run only the failed jobs (faster, avoids re-running passing jobs)
gh run rerun <run-id> --failed

# Re-run with debug logging enabled
gh run rerun <run-id> --debug
```

## Managing Workflows

```bash
# List all workflows in the repository
gh workflow list

# View a specific workflow definition
gh workflow view ci.yml

# Manually trigger a workflow
gh workflow run ci.yml

# Trigger with inputs and on a specific branch
gh workflow run deploy.yml --ref main --field environment=production

# Disable a workflow (stops future runs)
gh workflow disable ci.yml

# Re-enable a workflow
gh workflow enable ci.yml
```

## Common Go CI Workflow Pattern

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Run tests
        run: go test ./... -race -count=1

      - name: Check coverage
        run: |
          go test ./... -coverprofile=coverage.out
          go tool cover -func=coverage.out

      - name: Build
        run: go build ./...
```

## Secrets Management

```bash
# List secrets for the repository
gh secret list

# Set a secret
gh secret set MY_API_KEY

# Set a secret from a file
gh secret set MY_CERT < cert.pem

# Remove a secret
gh secret delete MY_API_KEY
```

## Environment Variables in Workflows

Secrets are accessed in workflow YAML as:
```yaml
env:
  API_KEY: ${{ secrets.MY_API_KEY }}
```

Never hardcode secrets in workflow files. Always use `${{ secrets.NAME }}`.

## Artifacts

```bash
# List artifacts for a run
gh run view <run-id> --json artifacts

# Download artifacts from a run
gh run download <run-id>

# Download a specific artifact
gh run download <run-id> --name my-artifact
```
