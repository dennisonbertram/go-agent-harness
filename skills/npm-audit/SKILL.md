---
name: npm-audit
description: "Audit npm dependencies for known vulnerabilities and suggest fixes. Trigger: when scanning npm packages for security issues, auditing JavaScript dependencies, checking Node.js vulnerabilities"
version: 1
argument-hint: "[--fix] [--severity high|critical]"
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# NPM Security Audit

You are now operating in npm audit mode.

## Basic Usage

```bash
# Run full audit
npm audit

# JSON output for programmatic parsing
npm audit --json

# Only report high and critical severity
npm audit --audit-level=high

# Only report critical severity
npm audit --audit-level=critical
```

## Interpreting Results

```
found 3 vulnerabilities (1 moderate, 2 high)

# npm audit report

lodash  <4.17.21
Severity: high
Prototype Pollution - https://npmjs.com/advisories/1523
fix available via `npm audit fix`
node_modules/lodash
  package-using-lodash  *
  Depends on vulnerable versions of lodash
  node_modules/package-using-lodash
```

### Severity Levels

- **critical**: Immediate action required — remote code execution, privilege escalation
- **high**: Fix before merge — significant security risk
- **moderate**: Fix soon — lower risk, but should not accumulate
- **low**: Informational — may not require action

## Fixing Vulnerabilities

### Automatic Fix (safe updates)

```bash
# Fix vulnerabilities that don't require breaking changes
npm audit fix

# Verify fix
npm audit
```

### Force Fix (may include breaking changes)

```bash
# Fix including major version bumps (review carefully)
npm audit fix --force

# Always test after force fix
npm test
```

### Manual Fix

When automatic fix is not available:

```bash
# Update a specific package to a safe version
npm install lodash@4.17.21

# Update to latest
npm install lodash@latest

# Verify fix
npm audit
```

## JSON Output Structure

```json
{
  "auditReportVersion": 2,
  "vulnerabilities": {
    "lodash": {
      "name": "lodash",
      "severity": "high",
      "isDirect": false,
      "via": [...],
      "effects": [...],
      "range": "<4.17.21",
      "nodes": ["node_modules/lodash"],
      "fixAvailable": {
        "name": "lodash",
        "version": "4.17.21",
        "isSemVerMajor": false
      }
    }
  },
  "metadata": {
    "vulnerabilities": {
      "info": 0,
      "low": 0,
      "moderate": 1,
      "high": 2,
      "critical": 0,
      "total": 3
    }
  }
}
```

Parse total count:
```bash
npm audit --json | jq '.metadata.vulnerabilities.total'
npm audit --json | jq '.metadata.vulnerabilities.high + .metadata.vulnerabilities.critical'
```

## CI Integration

```yaml
- name: NPM Security Audit
  run: |
    npm ci
    # Fail on high or critical vulnerabilities
    npm audit --audit-level=high
```

`npm audit` exits non-zero when vulnerabilities at or above the specified level are found.

## Checking if npm is Available

```bash
which npm && npm --version || echo "npm not installed"
```

If npm is not installed and the project has a `package.json`, install Node.js from https://nodejs.org or via a version manager:

```bash
# Using nvm (Node Version Manager)
nvm install --lts
nvm use --lts
```

## Scoped Auditing

```bash
# Audit only production dependencies (exclude devDependencies)
npm audit --omit=dev

# Audit only a specific workspace (monorepo)
npm audit --workspace=packages/my-app
```

## When No Fix is Available

If `npm audit fix` reports no fix available:

1. Check if there is a newer version: `npm show <package> versions`
2. Look for a fork or replacement package
3. If the vulnerable code path is not used in your application, document the accepted risk
4. File an issue with the upstream package maintainer
5. Consider using `npm audit --audit-level=critical` in CI to avoid blocking on unfixable moderate/low issues
