# Secrets Scan Report

**Date**: 2026-03-10
**Scope**: Full repository at `/Users/dennisonbertram/Develop/go-agent-harness` (main branch + worktrees)
**Result**: **CLEAN -- No real secrets or credentials found.**

---

## Scan Summary

| Category | Matches | Real Secrets? |
|---|---|---|
| API keys (sk-, ghp_, gho_, github_pat_, AKIA) | 0 | N/A |
| Bearer tokens | 0 | N/A |
| Private keys (PEM, RSA, etc.) | 0 | N/A |
| JWT tokens (eyJ...) | 0 | N/A |
| Slack tokens (xox[bpras]-) | 0 | N/A |
| .env files | 0 | N/A |
| Sensitive file types (.pem, .key, id_rsa, credentials.json) | 0 | N/A |
| Hardcoded passwords | 7 (skill docs) | No -- all example/placeholder values |
| Hardcoded API keys | 10+ (tests + skill docs) | No -- all test mocks or placeholders |
| Connection strings with credentials | 3 (test + skill docs) | No -- all example/placeholder values |
| Database files tracked in git | 1 | No secrets -- empty schema-only DB |
| Webhook/Slack/Discord URLs | 0 | N/A |

---

## Detailed Findings

### 1. Password-like strings in skill documentation (NOT secrets)

**Files** (main branch only; worktree copies omitted):
- `skills/vault-ops/SKILL.md` -- lines 73, 99, 231, 312

These are **documentation examples** for HashiCorp Vault operations. Values like `"supersecret"`, `"vault-password"`, and `"newpassword123"` are clearly illustrative placeholder passwords used in example commands. The file is a SKILL.md instructional document.

- `skills/docker-compose/SKILL.md` -- line 119

Contains `postgres://postgres:dev@postgres:5432/myapp` as a docker-compose example DATABASE_URL. This is a standard dev-only example, not a real credential.

- `skills/helm-deploy/SKILL.md` -- line 267

Contains `--set db.password="${DB_PASSWORD}"` which references an environment variable, not a hardcoded secret.

**Verdict**: All safe. These are documentation examples with obviously fake placeholder values.

### 2. API key test values in Go test files (NOT secrets)

**Files**:
- `internal/provider/openai/client_test.go` -- multiple lines using `APIKey: "test-key"`
- `internal/observationalmemory/openai_model_test.go` -- line 26 using `APIKey: "test-key"`

These use the literal string `"test-key"` as a test mock value. This is standard Go testing practice; they connect to local httptest servers, not real APIs.

**Verdict**: Safe. These are test fixture values connecting to mock servers.

### 3. API key placeholders in skill documentation (NOT secrets)

**Files**:
- `skills/linear-workflow/SKILL.md` -- `LINEAR_API_KEY="lin_api_your_key_here"` (obvious placeholder)
- `skills/fly-deploy/SKILL.md` -- `API_KEY="sk_live_..."` (truncated example)
- `skills/kubectl-ops/SKILL.md` -- `API_KEY="sk_live_..."` (truncated example)
- `skills/gosec/SKILL.md` -- `apiKey = "sk-abc123..."` (gosec example of what NOT to do)
- `skills/sentry-setup/SKILL.md` -- `SENTRY_AUTH_TOKEN="sntrys_your_auth_token_here"` (obvious placeholder)

**Verdict**: Safe. All are clearly placeholder/example values in instructional documentation.

### 4. Connection string in test file (NOT a secret)

**File**: `internal/observationalmemory/store_postgres_test.go` -- line 21

Contains `postgres://user:pass@localhost/db`. This is a generic placeholder connection string (`user:pass`) used in a test that verifies method stubs return "not implemented". No real database connection is made.

**Verdict**: Safe. Generic test placeholder, not a real credential.

### 5. SQLite database tracked in git (LOW concern)

**File**: `cmd/harnessd/.harness/state.db` (tracked by git)

This is an empty SQLite database containing only schema definitions (tables: `om_markers`, `om_memory_records`, `om_operation_log`) with zero rows in all tables. It contains no secrets, credentials, or sensitive data.

**Recommendation**: Consider adding `*.db` or `.harness/*.db` to `.gitignore` and removing this from tracking. While it contains no sensitive data now, tracked database files can accumulate sensitive data in future commits. The schema could be created programmatically at runtime instead.

Note: `cmd/harnessd/.harness/cron.db` exists on disk but is NOT tracked by git (confirmed via `git ls-files`).

---

## .gitignore Review

The current `.gitignore` correctly excludes:
- `.env` and `.env.*` files
- Build artifacts (`bin/`, `*.test`, `*.out`, `coverage.out`)
- Editor files (`.DS_Store`, `.vscode/`, `.idea/`)
- Cache directories (`node_modules/`, `dist/`, `.tmp/`, `tmp/`)

**Missing from .gitignore** (recommended additions):
- `*.db` or `**/.harness/*.db` -- SQLite databases
- `*.pem`, `*.key` -- certificate/key files (defense in depth)
- `*.p12`, `*.pfx`, `*.jks` -- keystores

---

## Conclusion

The codebase contains **no real credentials, API keys, tokens, passwords, private keys, or connection strings**. All password/key-like strings found are either:

1. **Documentation examples** in SKILL.md files with obviously fake placeholder values
2. **Test fixture values** (`"test-key"`) used with mock HTTP servers
3. **Placeholder strings** (e.g., `"your_key_here"`, `"sk-abc123..."`)

The only minor recommendation is to add `*.db` to `.gitignore` and remove the tracked `state.db` (which is empty but could accumulate sensitive data in future).
