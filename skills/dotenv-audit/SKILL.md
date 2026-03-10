---
name: dotenv-audit
description: "Audit .env files and environment variable usage to prevent secret leaks in commits, logs, and error messages. Trigger: when auditing secrets, checking for exposed credentials, reviewing environment variable handling"
version: 1
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# Environment File Audit (dotenv-audit)

You are now operating in dotenv audit mode. Perform all checks below to prevent secret leaks.

## Check 1: .env Files in .gitignore

```bash
# Check if .env is in .gitignore
grep -n "\.env" .gitignore 2>/dev/null || echo "CRITICAL: .env not in .gitignore"

# Check all .env variants
for pattern in ".env" ".env.*" "*.env"; do
  grep -q "$pattern" .gitignore && echo "OK: $pattern is ignored" \
    || echo "WARNING: $pattern not found in .gitignore"
done
```

**If .env is missing from .gitignore**: Add `.env` to `.gitignore` immediately. A `.env` file with real credentials committed to git is a critical security incident.

## Check 2: .env Files Already Committed

```bash
# Check if any .env files are tracked by git
git ls-files | grep -E "\.env$|\.env\." | head -20

# Check git history for .env files
git log --all --full-history -- "*.env" "**/.env" ".env" 2>/dev/null | head -20
```

**If .env files are tracked**:
```bash
# Remove from tracking without deleting the local file
git rm --cached .env
git rm --cached .env.local

# Add to .gitignore
echo ".env" >> .gitignore
echo ".env.*" >> .gitignore

# Commit the removal
git commit -m "Remove .env from tracking, add to .gitignore"
```

**WARNING**: If the .env file was ever pushed to a remote repository, the secrets are compromised. Rotate all exposed credentials immediately — git history cannot be fully erased from all forks.

## Check 3: Hardcoded Secrets in Source Code

```bash
# Scan for common API key patterns
grep -rn --include="*.go" \
  -E "(api[_-]?key|secret|password|token|credential)\s*=\s*['\"][A-Za-z0-9+/]{16,}" \
  . | grep -v "_test.go" | grep -v "vendor/"

# Scan for specific key prefixes
grep -rn --include="*.go" \
  -E "(sk-[A-Za-z0-9]{32,}|ghp_[A-Za-z0-9]{36}|AKIA[A-Z0-9]{16})" \
  . | grep -v "_test.go"

# Check for Base64-encoded secrets (common obfuscation)
grep -rn --include="*.go" \
  -E "['\"][A-Za-z0-9+/]{40,}={0,2}['\"]" \
  . | grep -v "_test.go" | head -20
```

**If hardcoded secrets found**: Move to environment variables immediately. Rotate the exposed credential — even if it's in an old commit, treat it as compromised.

## Check 4: .env.example Exists

```bash
# Check for .env.example
ls -la .env.example 2>/dev/null || echo "WARNING: .env.example is missing"
```

Every project with environment variables should have a `.env.example` file with placeholder values. This documents required variables without exposing secrets:

```bash
# Example .env.example content
cat > .env.example << 'EOF'
# Required: OpenAI API key
OPENAI_API_KEY=your-api-key-here

# Required: Server address
HARNESS_ADDR=:8080

# Optional: Default model
HARNESS_MODEL=gpt-4.1-mini
EOF
```

## Check 5: os.Getenv Calls Match .env.example

```bash
# Extract all os.Getenv calls from Go code
grep -rn --include="*.go" 'os\.Getenv(' . | \
  grep -oE '"[A-Z_]+"' | sort -u | tr -d '"'
```

```bash
# Extract keys from .env.example
grep -v '^#' .env.example | grep '=' | cut -d= -f1 | sort -u
```

Compare the two lists. Every `os.Getenv("KEY")` call should have a corresponding entry in `.env.example`.

## Check 6: Secrets in Logs or Error Messages

```bash
# Look for potential secret interpolation in log statements
grep -rn --include="*.go" \
  -E '(log\.|fmt\.Print|fmt\.Sprintf).*(key|secret|token|password)' \
  . | grep -v "_test.go"
```

Secrets should never appear in log output. Use redacted representations:
```go
// WRONG
log.Printf("connecting with key=%s", apiKey)

// CORRECT
log.Printf("connecting with key=%s...", apiKey[:8])
```

## Summary Report

After running all checks, produce a summary:

```
DOTENV AUDIT RESULTS
====================
[ ] .env in .gitignore
[ ] No .env files tracked by git
[ ] No hardcoded secrets in source
[ ] .env.example exists
[ ] All os.Getenv vars documented in .env.example
[ ] No secrets in log statements
```

Mark each as PASS, FAIL, or WARNING with details.
