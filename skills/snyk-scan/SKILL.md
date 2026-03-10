---
name: snyk-scan
description: "Scan code and dependencies for vulnerabilities with Snyk: snyk test, snyk monitor, snyk code test, fix guidance, severity thresholds, CI integration. Trigger: when using Snyk, snyk test, snyk monitor, vulnerability scanning, dependency security, snyk code, SAST scanning, CVE scan"
version: 1
argument-hint: "[test|monitor|code|fix|ignore] [--severity-threshold=high]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Snyk Security Scanning

You are now operating in Snyk vulnerability scanning mode.

## Installation and Authentication

```bash
# Install Snyk CLI
npm install -g snyk

# macOS via Homebrew
brew install snyk-cli

# Authenticate with Snyk (opens browser)
snyk auth

# Authenticate with a token (CI/CD)
snyk auth $SNYK_TOKEN
export SNYK_TOKEN=your-api-token  # alternative method

# Verify authentication
snyk whoami
```

## Dependency Vulnerability Scanning

```bash
# Scan current project dependencies for vulnerabilities
snyk test

# Scan with JSON output (machine-readable)
snyk test --json

# Scan with SARIF output (for GitHub Code Scanning)
snyk test --sarif

# Save SARIF output to file
snyk test --sarif-file-output=snyk-results.sarif

# Scan a specific package.json or pom.xml
snyk test --file=package.json
snyk test --file=pom.xml

# Scan all projects in a monorepo
snyk test --all-projects

# Scan with a severity threshold (only fail on high or critical)
snyk test --severity-threshold=high

# Severity thresholds: low, medium, high, critical
snyk test --severity-threshold=critical

# Show all vulnerabilities including low severity
snyk test --severity-threshold=low

# Skip dev dependencies (Node.js)
snyk test --dev=false

# Scan a Docker image for OS vulnerabilities
snyk container test myapp:latest

# Scan with a specific Docker file for more context
snyk container test myapp:latest --file=Dockerfile
```

## Code Security Analysis (SAST)

```bash
# Scan source code for security issues (static analysis)
snyk code test

# Scan with JSON output
snyk code test --json

# Scan with SARIF output
snyk code test --sarif

# Save SARIF output to file
snyk code test --sarif-file-output=snyk-code-results.sarif

# Scan a specific directory
snyk code test ./src

# Filter by severity
snyk code test --severity-threshold=high
```

## Infrastructure as Code Scanning

```bash
# Scan Terraform files for misconfigurations
snyk iac test ./terraform/

# Scan Kubernetes manifests
snyk iac test ./k8s/

# Scan CloudFormation templates
snyk iac test ./cloudformation/

# Scan with severity threshold
snyk iac test --severity-threshold=high ./terraform/

# Scan with SARIF output
snyk iac test --sarif ./terraform/
```

## Continuous Monitoring

```bash
# Monitor a project (sends results to Snyk dashboard)
snyk monitor

# Monitor with a project name
snyk monitor --project-name=myapp-production

# Monitor all projects in a monorepo
snyk monitor --all-projects

# Monitor a Docker image
snyk container monitor myapp:latest

# Monitor with org specification
snyk monitor --org=my-org-slug
```

## Fix and Remediation

```bash
# Automatically fix vulnerabilities by upgrading packages
snyk fix

# Fix in dry-run mode (show what would be fixed)
snyk fix --dry-run

# Fix with specific package manager
snyk fix --unmanaged

# View fix options for a vulnerability
snyk test --json | jq '.vulnerabilities[] | {id, title, fixedIn}'

# Upgrade a specific package to fix vulnerabilities
# (Node.js example — Snyk will suggest the version)
npm install lodash@4.17.21

# Open Snyk web UI for a project's issues
snyk open
```

## Ignore and Exceptions

```bash
# Ignore a specific vulnerability for 30 days with a reason
snyk ignore --id=SNYK-JS-LODASH-567746 \
  --reason="No current fix available" \
  --expiry=2024-12-31

# Ignore a vulnerability permanently
snyk ignore --id=SNYK-JS-LODASH-567746 --reason="False positive"

# Ignore vulnerabilities in a specific path
snyk ignore --id=SNYK-JS-LODASH-567746 --path="lodash@4.17.15"

# List ignored vulnerabilities
cat .snyk  # Snyk stores ignores in .snyk file

# Example .snyk file format
cat > .snyk <<'EOF'
version: v1.25.0
ignore:
  SNYK-JS-LODASH-567746:
    - lodash@4.17.15:
        reason: No fix available yet
        expires: '2024-12-31T00:00:00.000Z'
EOF
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/snyk.yml
name: Snyk Security Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  snyk:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run Snyk dependency scan
        uses: snyk/actions/node@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
        with:
          args: --severity-threshold=high

      - name: Run Snyk code scan
        uses: snyk/actions/node@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
        with:
          command: code test

      - name: Upload SARIF to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: snyk.sarif
```

### GitLab CI

```yaml
# .gitlab-ci.yml snippet
snyk-scan:
  image: node:18
  script:
    - npm install -g snyk
    - snyk auth $SNYK_TOKEN
    - snyk test --severity-threshold=high
  variables:
    SNYK_TOKEN: $SNYK_TOKEN
```

### Shell Script for CI

```bash
#!/bin/bash
set -euo pipefail

# Authenticate
snyk auth "$SNYK_TOKEN"

# Run dependency scan, fail on high+ severity
if ! snyk test --severity-threshold=high --json > snyk-results.json 2>&1; then
  echo "Snyk found high severity vulnerabilities:"
  cat snyk-results.json | jq '.vulnerabilities[] | select(.severity == "high" or .severity == "critical") | {id, title, severity}'
  exit 1
fi

# Run SAST scan
if ! snyk code test --severity-threshold=high; then
  echo "Snyk Code found high severity issues"
  exit 1
fi

echo "All Snyk scans passed"
```

## Interpreting Results

```bash
# Parse JSON output to find critical vulnerabilities
snyk test --json | jq '.vulnerabilities[] | select(.severity == "critical") | {
  id: .id,
  title: .title,
  packageName: .packageName,
  version: .version,
  fixedIn: .fixedIn
}'

# Count vulnerabilities by severity
snyk test --json | jq '
  .vulnerabilities |
  group_by(.severity) |
  map({severity: .[0].severity, count: length}) |
  .[]
'

# Get CVE IDs for all vulnerabilities
snyk test --json | jq '.vulnerabilities[].identifiers.CVE[]?' 2>/dev/null

# Check if any fixable vulnerabilities exist
snyk test --json | jq '[.vulnerabilities[] | select(.isUpgradable == true)] | length'
```

## Troubleshooting

```bash
# Run with verbose output for debugging
snyk test -d

# Check Snyk CLI version
snyk --version

# Clear Snyk cache
snyk config unset api

# Test without internet (use cached data)
snyk test --offline

# Common issues:
# "Authentication required" — run: snyk auth
# "Could not detect package manager" — specify with: --file=package.json
# "No supported target files detected" — check working directory
# High false positive rate — use: snyk ignore or configure in .snyk file
```
