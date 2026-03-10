---
name: go-deps
description: "Manage Go module dependencies: tidy, update, vendor, audit, and analyze dependency trees with go mod and govulncheck. Trigger: when managing Go dependencies, go mod tidy, go get update, go module graph, govulncheck, vendor dependencies, go module audit"
version: 1
argument-hint: "[tidy|update|vendor|audit|graph|why <module>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Go Dependency Management

You are now operating in Go module dependency management mode.

## Installation Check

```bash
# Verify Go is installed
go version

# Check go.mod exists
cat go.mod
```

## Core Module Commands

```bash
# Remove unused dependencies, add missing ones
go mod tidy

# Download all dependencies to module cache
go mod download

# Verify dependency checksums match go.sum
go mod verify

# Show the module graph (all transitive dependencies)
go mod graph

# Show the module graph in a readable tree (requires graphviz optional)
go mod graph | head -50
```

## Updating Dependencies

```bash
# List all available updates (minor + patch)
go list -m -u all

# List available updates (JSON output for parsing)
go list -m -u -json all 2>/dev/null | jq -r 'select(.Update != null) | "\(.Path) \(.Version) -> \(.Update.Version)"'

# Update all direct dependencies to latest minor/patch
go get -u ./...

# Update all dependencies including major versions (use carefully)
go get -u=patch ./...

# Update a single module to latest
go get -u github.com/some/module@latest

# Update a single module to a specific version
go get github.com/some/module@v1.2.3

# Downgrade a module to a specific version
go get github.com/some/module@v1.1.0

# Remove a dependency entirely
go get github.com/some/module@none
```

## Conservative vs Aggressive Update Strategy

### Conservative (patch-level only — lowest risk)
```bash
# Only update patch versions (1.2.3 -> 1.2.4)
go get -u=patch ./...
go mod tidy
go test ./...
```

### Moderate (minor + patch — safe for semver-compliant modules)
```bash
# Update minor and patch (1.2.3 -> 1.3.0)
go get -u ./...
go mod tidy
go test ./...
```

### Aggressive (includes major versions — may require code changes)
```bash
# Find available major updates manually
go list -m -u -json all 2>/dev/null | jq -r 'select(.Update != null) | .Path + " " + .Update.Version'

# Update a specific module to a new major version
go get github.com/some/module/v2@latest
# Then update all import paths in .go files
```

## Dependency Analysis

```bash
# Why is a particular module needed?
go mod why github.com/some/module
go mod why -m github.com/some/module

# Show all modules that depend on a specific module
go mod graph | grep github.com/some/module

# List all direct dependencies
go list -m -json all | jq -r 'select(.Indirect == null or .Indirect == false) | .Path + " " + .Version'

# List only indirect dependencies
go list -m -json all | jq -r 'select(.Indirect == true) | .Path + " " + .Version'

# Check if a module is used directly or transitively
go mod why -m golang.org/x/net
```

## Vendoring

```bash
# Vendor all dependencies to ./vendor directory
go mod vendor

# Verify vendor directory matches go.sum
go mod verify

# Build using only the vendor directory (no network)
go build -mod=vendor ./...

# Test using vendor directory
go test -mod=vendor ./...

# Remove vendor directory
rm -rf vendor/
```

## Security Audit with govulncheck

```bash
# Install govulncheck (one-time setup)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan for vulnerabilities in the current module
govulncheck ./...

# Scan a specific package
govulncheck github.com/your/module/pkg/...

# Output as JSON for CI parsing
govulncheck -json ./...

# Check all dependencies (not just used code paths)
govulncheck -mode=module ./...
```

## go.sum Management

```bash
# Regenerate go.sum from scratch (safe to delete and re-generate)
rm go.sum
go mod download
go mod verify

# Add missing hashes to go.sum
go mod download all

# Check for inconsistencies
go mod verify
```

## Replacing Modules (Local Development)

```bash
# Replace a module with a local fork during development
# In go.mod:
# replace github.com/some/module => ../local-fork

# Add replace directive from command line
go mod edit -replace github.com/some/module=../local-fork

# Remove replace directive
go mod edit -dropreplace github.com/some/module

# Replace with a specific fork on GitHub
go mod edit -replace github.com/some/module=github.com/your-fork/module@v0.0.0-timestamp-hash
```

## Workspace Mode (Go 1.18+)

```bash
# Initialize a workspace spanning multiple modules
go work init ./moduleA ./moduleB

# Add a module to an existing workspace
go work use ./moduleC

# Build using workspace mode
go build ./...   # automatically uses go.work

# Sync workspace
go work sync

# Disable workspace mode for a command
GOWORK=off go build ./...
```

## Common Workflows

### Post-Clone Setup
```bash
go mod download
go mod verify
go mod tidy
go build ./...
```

### Weekly Dependency Update
```bash
go list -m -u all          # preview available updates
go get -u=patch ./...       # apply safe patch updates
go mod tidy                 # clean up
go test ./... -race         # run tests with race detector
govulncheck ./...           # security audit
```

### Adding a New Dependency
```bash
# Import in your Go code, then:
go mod tidy                 # auto-adds to go.mod + go.sum

# Or explicitly add at a specific version:
go get github.com/new/module@v1.2.3
go mod tidy
```

## go.mod Directives Reference

```
module github.com/your/project   // module path

go 1.22                          // minimum Go version

require (
    github.com/some/dep v1.2.3
    github.com/other/dep v2.0.0+incompatible
)

replace github.com/some/dep => ../local-fork   // local replacement

exclude github.com/bad/dep v1.0.0              // exclude problematic version
```

## Best Practices

- Run `go mod tidy` before every commit to keep go.mod and go.sum clean.
- Pin major version updates explicitly — do not use `go get -u` blindly across major versions.
- Run `govulncheck ./...` in CI to catch known vulnerabilities early.
- Use `-mod=vendor` in CI builds for reproducibility without network access.
- Store `go.sum` in version control — it provides supply-chain security.
- Prefer patch updates (`-u=patch`) for production dependencies; review minor/major changes manually.
