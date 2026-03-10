---
name: go-vuln-check
description: "Check Go dependencies for known vulnerabilities using govulncheck and the official Go vulnerability database. Trigger: when checking Go dependencies for CVEs, running vulnerability scanning, auditing Go modules"
version: 1
argument-hint: "[path] (default: ./...)"
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# Go Vulnerability Check (govulncheck)

You are now operating in Go vulnerability checking mode.

## Installation

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## Basic Usage

```bash
# Check all packages (default)
govulncheck ./...

# JSON output for programmatic parsing
govulncheck -json ./...

# Verbose output — includes informational findings
govulncheck -show verbose ./...

# Check a compiled binary instead of source
govulncheck -mode=binary ./myapp

# Check a specific package
govulncheck ./internal/server/...
```

## Interpreting Results

### Vulnerability Found

```
Vulnerability #1: GO-2023-1234
    A vulnerability in dependency X allows remote code execution.
  More info: https://pkg.go.dev/vuln/GO-2023-1234
  Module: github.com/example/vulnerable@v1.2.3
    Found in: github.com/example/vulnerable.Function
    Fixed in: github.com/example/vulnerable@v1.2.4
    Example traces found:
      #1: your/package/file.go:42:10
```

### No Vulnerabilities

```
No vulnerabilities found.
```

### Informational (with -show verbose)

```
Informational: GO-2023-5678
    Your code imports the affected package but does not call the
    vulnerable function. No action required.
```

## Fixing Vulnerabilities

### Update the Affected Module

```bash
# Update to the fixed version
go get github.com/example/vulnerable@v1.2.4

# Update all dependencies to latest minor/patch versions
go get -u ./...

# Tidy up go.sum
go mod tidy

# Verify modules
go mod verify

# Re-run govulncheck to confirm fix
govulncheck ./...
```

### When No Fix is Available

If no fixed version exists:
1. Check if the vulnerable function is actually called in your code path
2. If not called, the vulnerability may not apply (govulncheck shows call traces)
3. Consider replacing the dependency with an alternative
4. Add a comment in the code explaining the accepted risk and tracking issue

## Binary Scanning

Use binary mode to scan deployed artifacts:

```bash
# Build the binary first
go build -o myapp ./cmd/myapp

# Scan the compiled binary
govulncheck -mode=binary ./myapp
```

Binary scanning detects vulnerabilities even when source is unavailable, but provides less detailed call trace information.

## JSON Output Structure

```json
{
  "config": { ... },
  "progress": [ ... ],
  "osv": [ {
    "id": "GO-2023-1234",
    "aliases": ["CVE-2023-12345"],
    "summary": "...",
    "affected": [ ... ]
  } ],
  "finding": [ {
    "osv": "GO-2023-1234",
    "fixed_version": "v1.2.4",
    "trace": [ { "module": "...", "version": "...", "path": "..." } ]
  } ]
}
```

## CI Integration

Add to `.github/workflows/security.yml`:

```yaml
- name: Check for Go vulnerabilities
  run: |
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...
```

`govulncheck` exits with a non-zero status code when vulnerabilities are found, making it suitable for CI gates.

## Difference from gosec

- **govulncheck**: Checks dependencies against the Go vulnerability database (known CVEs). Finds vulnerabilities in third-party modules.
- **gosec**: Static analysis of your own code for security anti-patterns (SQL injection, hardcoded secrets, etc.).

Use both for complete security coverage.
