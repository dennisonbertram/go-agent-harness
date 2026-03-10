---
name: gosec
description: "Run gosec static analysis to find security vulnerabilities in Go code. Trigger: when scanning Go code for security issues, running SAST, checking for SQL injection or command injection"
version: 1
argument-hint: "[path] (default: ./...)"
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# Go Security Scanner (gosec)

You are now operating in gosec security scanning mode.

## Installation

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

## Basic Usage

```bash
# Scan all packages (default)
gosec ./...

# Scan a specific package
gosec ./internal/server/...

# JSON output for programmatic parsing
gosec -fmt json ./... 2>&1

# Show severity threshold (only HIGH and CRITICAL)
gosec -severity high ./...

# Exclude specific rules
gosec -exclude=G104 ./...

# Include only specific rules
gosec -include=G201,G202,G204 ./...
```

## Interpreting Results

Results are reported with severity and confidence:

```
[/path/to/file.go:42] - G201 (CWE-89): SQL string formatting
  Severity: HIGH   Confidence: MEDIUM
  > fmt.Sprintf("SELECT * FROM users WHERE id = %d", userID)
```

### Severity Levels

- **HIGH**: Must fix before merge — injection risks, credential exposure, unsafe deserialization
- **MEDIUM**: Should fix — weak crypto, permission issues, potential misuse
- **LOW**: Review and decide — informational, may be false positives

## Key Rules Reference

| Rule | CWE | Description | Fix |
|------|-----|-------------|-----|
| G101 | CWE-798 | Hardcoded credentials | Move to environment variables |
| G104 | CWE-703 | Unhandled errors | Always check returned errors |
| G201 | CWE-89 | SQL string formatting (injection) | Use parameterized queries |
| G202 | CWE-89 | SQL query with string concat | Use parameterized queries |
| G204 | CWE-78 | Subprocess launched with variable | Validate/sanitize exec.Command args |
| G301 | CWE-276 | Poor file permissions | Use 0o600 for sensitive files |
| G304 | CWE-22 | File path from user input | Validate/sanitize path traversal |
| G401 | CWE-326 | Weak cryptographic primitive (MD5) | Use SHA-256 or stronger |
| G402 | CWE-295 | TLS with InsecureSkipVerify | Remove InsecureSkipVerify |
| G403 | CWE-780 | RSA key too small | Use 2048-bit minimum |
| G501 | CWE-327 | Import of crypto/md5 | Use crypto/sha256 |

## Common Fixes

### G201/G202: SQL Injection

```go
// WRONG
query := fmt.Sprintf("SELECT * FROM users WHERE id = %d", userID)
db.Query(query)

// CORRECT
db.Query("SELECT * FROM users WHERE id = ?", userID)
```

### G204: Command Injection

```go
// WRONG — user input directly in command
cmd := exec.Command("git", "clone", userProvidedURL)

// CORRECT — validate input before use
if !isValidURL(userProvidedURL) {
    return fmt.Errorf("invalid URL: %q", userProvidedURL)
}
cmd := exec.Command("git", "clone", userProvidedURL)
```

### G101: Hardcoded Credentials

```go
// WRONG
const apiKey = "sk-abc123..."

// CORRECT
apiKey := os.Getenv("API_KEY")
if apiKey == "" {
    return fmt.Errorf("API_KEY environment variable is required")
}
```

### G104: Unhandled Errors

```go
// WRONG
os.Remove(tmpFile)

// CORRECT
if err := os.Remove(tmpFile); err != nil {
    log.Printf("warning: failed to remove temp file %s: %v", tmpFile, err)
}
```

## Suppressing False Positives

When a finding is a confirmed false positive, suppress it with a comment:

```go
// #nosec G104 -- Read-only file, error is intentionally ignored
f, _ := os.Open("/etc/hostname")
```

Document why it is a false positive. Do not suppress without justification.

## CI Integration

Add to `.github/workflows/security.yml`:

```yaml
- name: Run gosec
  run: |
    go install github.com/securego/gosec/v2/cmd/gosec@latest
    gosec -fmt json ./... 2>&1 | tee gosec-report.json
    # Fail on HIGH severity findings
    gosec -severity high ./...
```
