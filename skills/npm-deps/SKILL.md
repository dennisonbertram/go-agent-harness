---
name: npm-deps
description: "Manage npm dependencies: audit, update, dedupe, outdated, lockfile, workspaces, and package tree analysis. Trigger: when managing npm packages, npm outdated, npm update, npm audit, npm install, package.json dependencies, node_modules, npm ci, npm dedupe"
version: 1
argument-hint: "[outdated|update|audit|dedupe|ci|ls <package>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# NPM Dependency Management

You are now operating in npm dependency management mode.

## Installation Check

```bash
# Verify Node.js and npm are installed
node --version
npm --version

# Check package.json exists
cat package.json
```

## Listing and Auditing

```bash
# List all outdated packages (JSON output)
npm outdated --json

# List all outdated packages (human-readable table)
npm outdated

# Show top-level installed packages only
npm ls --depth=0

# Show where a specific package is used in the dependency tree
npm ls lodash

# Show all installed packages as a flat list
npm ls --all --depth=0 2>/dev/null | grep -v UNMET

# Security audit
npm audit

# Audit with JSON output (for CI parsing)
npm audit --json

# Audit and automatically fix safe patches
npm audit fix

# Audit and fix including breaking changes (review carefully)
npm audit fix --force
```

## Installing Dependencies

```bash
# Install all dependencies from package.json
npm install

# Clean install from package-lock.json (CI-safe, reproducible)
npm ci

# Install a new runtime dependency
npm install <package>

# Install a development-only dependency
npm install --save-dev <package>

# Install a specific version
npm install <package>@1.2.3

# Install to latest version (may break semver range)
npm install <package>@latest

# Install globally (for CLI tools)
npm install -g <package>

# Install without running scripts (safer for untrusted packages)
npm install --ignore-scripts
```

## Updating Dependencies

```bash
# Update all packages within their semver range (safe)
npm update

# Update a specific package within semver range
npm update <package>

# Update a package to its latest version (may go outside current semver range)
npm install <package>@latest

# Update all packages to latest (aggressive — review changelogs)
npx npm-check-updates -u && npm install

# Preview what npm-check-updates would change without applying
npx npm-check-updates
```

## Conservative vs Aggressive Update Strategy

### Conservative (within declared semver ranges — lowest risk)
```bash
npm update                  # respects ^ and ~ ranges in package.json
npm audit fix               # only applies safe patches
npm test                    # verify nothing broke
```

### Moderate (latest within major version)
```bash
npx npm-check-updates --target minor  # only minor + patch updates
npm install
npm test
```

### Aggressive (includes major version bumps)
```bash
npx npm-check-updates -u    # update all to absolute latest
npm install
npm test                    # expect possible breaking changes
```

## Cleaning Up

```bash
# Remove duplicate packages (flatten the tree)
npm dedupe

# Remove packages not listed in package.json
npm prune

# Remove packages not in package.json including dev deps
npm prune --production

# Clean and reinstall completely
rm -rf node_modules package-lock.json
npm install
```

## Lockfile Management

```bash
# Generate/update package-lock.json
npm install

# Validate lockfile is consistent with package.json (CI check)
npm ci --dry-run 2>&1 | head -5

# View lockfile version
cat package-lock.json | head -5

# Check for lockfile conflicts after merge
git diff package-lock.json | head -50
# Resolve by: rm package-lock.json && npm install

# Install without updating lockfile (read-only mode)
npm ci
```

## Workspace Management (npm 7+)

```bash
# List all workspaces
npm ls --workspaces --depth=0

# Install dependencies for all workspaces
npm install --workspaces

# Run a command in a specific workspace
npm run build --workspace=packages/ui

# Run a command in all workspaces
npm run test --workspaces

# Add a dependency to a specific workspace
npm install lodash --workspace=packages/api

# Execute a script across all workspaces
npm run build -ws --if-present
```

## Dependency Analysis

```bash
# List packages that depend on a specific module
npm ls <package> --all

# Check the size of installed packages (approximate)
du -sh node_modules/*/ 2>/dev/null | sort -h | tail -20

# Find packages with known vulnerabilities
npm audit --json | jq '.vulnerabilities | keys[]'

# Find packages with high/critical vulnerabilities
npm audit --json | jq '.vulnerabilities | to_entries[] | select(.value.severity == "high" or .value.severity == "critical") | .key'

# Count total installed packages
npm ls --all --parseable 2>/dev/null | wc -l
```

## Publishing and Pack

```bash
# Preview what files would be included in the published package
npm pack --dry-run

# Create a tarball (for inspection or local install)
npm pack

# Install from a local tarball
npm install ./mypackage-1.0.0.tgz
```

## package.json Scripts Reference

```json
{
  "scripts": {
    "prepare": "npm run build",
    "pretest": "npm run lint",
    "test": "jest",
    "posttest": "npm run coverage",
    "build": "tsc",
    "clean": "rm -rf dist node_modules"
  }
}
```

```bash
# Run a lifecycle script
npm run build

# Run with extra arguments (after --)
npm test -- --watch

# Run pre/post hooks explicitly
npm run prebuild
```

## Common CI Workflow

```bash
# In CI: always use npm ci (not npm install)
npm ci                      # reproducible from lockfile
npm run build
npm test
npm audit --audit-level=high  # fail on high+ vulnerabilities
```

## Best Practices

- Always commit `package-lock.json` to version control for reproducible builds.
- Use `npm ci` in CI environments — it never modifies `package-lock.json`.
- Run `npm audit` in CI and fail builds on `high` or `critical` vulnerabilities.
- Pin exact versions in production with `--save-exact` for critical dependencies.
- Use `--save-dev` for tools that are not needed at runtime (linters, test runners, TypeScript).
- Prefer `npm dedupe` after major updates to reduce bundle size.
- Run `npm prune` to remove packages left over from removed dependencies.
- Review `npm outdated` weekly; apply patch updates promptly; schedule major updates.
