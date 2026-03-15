# Git History Secrets Scan Report

**Date**: 2026-03-10
**Scope**: Full git history, all branches
**Repository**: go-agent-harness

---

## Executive Summary

**No real credentials, API keys, or passwords were found in the git history.** All flagged patterns are either placeholder/example values in documentation (skill files), hardcoded test fixtures, or local development artifacts. However, there are two operational hygiene issues that should be addressed before the repo goes public.

---

## Findings

### FINDING 1: SQLite database `state.db` is tracked in git (MEDIUM)

**Status**: Currently tracked in git
**File**: `cmd/harnessd/.harness/state.db` (45 KB)
**Commit introduced**: `1e575c0` ("Implement streaming support")

The file is a SQLite database for the observational memory subsystem. It contains schema only (0 rows in `om_memory_records`), so no user data is currently exposed. However:

- SQLite databases should never be in version control (binary files, merge conflicts, potential for future data leakage).
- The `.harness/` directory also contains `state.db-shm` and `state.db-wal` (WAL journal files) on disk that are not tracked but could be.
- The `.gitignore` does NOT cover `*.db` files.

**Recommendation**: Add `*.db` and `*.db-shm` and `*.db-wal` to `.gitignore`, remove `state.db` from tracking (`git rm --cached`), and use `git filter-repo` or BFG to purge it from history before going public.

### FINDING 2: SQLite database `cron.db` was temporarily committed (LOW)

**Status**: Removed from tracking (but still in git history)
**File**: `cmd/harnessd/.harness/cron.db` (32 KB)
**Commit added**: `d3a6df1` ("Enforce allowed-tools constraint during skill activation")
**Commit removed**: `9c23ad9` ("Remove accidentally committed cron.db")

This binary file was accidentally committed and then removed in a subsequent commit. The binary blob (32 KB) remains in git history. It is a SQLite database for the cron/scheduler subsystem.

**Recommendation**: Purge from history with BFG or `git filter-repo` before going public. Add `*.db` to `.gitignore` to prevent recurrence.

### FINDING 3: Placeholder/example credentials in skill documentation (INFO - No Action Required)

Several skill SKILL.md files contain example/placeholder credentials as part of their instructional documentation. **None of these are real secrets:**

| File | Pattern | Value | Assessment |
|------|---------|-------|------------|
| `skills/fly-deploy/SKILL.md` | `FLY_API_TOKEN` | `"your-token-here"` | Placeholder |
| `skills/fly-deploy/SKILL.md` | `API_KEY` | `"sk_live_..."` | Truncated example |
| `skills/linear-workflow/SKILL.md` | `LINEAR_API_KEY` | `"lin_api_your_key_here"` | Placeholder |
| `skills/linear-workflow/SKILL.md` | `SENTRY_AUTH_TOKEN` | `"sntrys_your_auth_token_here"` | Placeholder |
| `skills/vault-ops/SKILL.md` | `db_password` | `"supersecret"` | Example for Vault docs |
| `skills/vault-ops/SKILL.md` | `password` | `"vault-password"` | Example for Vault docs |
| `skills/vault-ops/SKILL.md` | `VAULT_TOKEN` | `'root'` | Dev mode default token |
| `skills/docker-compose/SKILL.md` | `DATABASE_URL` | `postgres://postgres:dev@postgres:5432/myapp` | Docker dev example |
| `skills/railway-deploy/SKILL.md` | `SECRET_KEY` | `$(openssl rand -hex 32)` | Dynamic generation example |

**Assessment**: These are all clearly documentation examples with placeholder values. No action required.

### FINDING 4: Test fixture credentials (INFO - No Action Required)

Test files contain hardcoded strings used as test fixtures:

| File | Pattern | Value | Assessment |
|------|---------|-------|------------|
| `internal/provider/openai/client_test.go` | `APIKey` | `"test-key"` | Test fixture |
| `internal/observationalmemory/store_postgres_test.go` | DSN | `"postgres://user:pass@localhost/db"` | Test fixture (never connects) |

**Assessment**: Standard test practice. These are not real credentials. No action required.

---

## Checks Performed

| Check | Result |
|-------|--------|
| Real API keys (sk-, ghp_, github_pat_, AKIA...) | None found |
| Private keys (PEM, RSA, EC, DSA) | None found |
| .env files committed | Never committed |
| .pem / .key / id_rsa files committed | Never committed |
| JWT tokens | None found |
| Slack tokens / webhooks | None found |
| Hardcoded OpenAI/Anthropic API keys | None found |
| AWS access keys | None found |
| Real connection strings with passwords | None found |
| Binary databases committed | 2 found (see Findings 1 & 2) |

---

## Recommended Actions Before Going Public

### Priority 1: Fix .gitignore and untrack state.db

```bash
# Add to .gitignore
echo '*.db' >> .gitignore
echo '*.db-shm' >> .gitignore
echo '*.db-wal' >> .gitignore

# Remove from tracking (keeps local file)
git rm --cached cmd/harnessd/.harness/state.db
git commit -m "Stop tracking state.db, update .gitignore for database files"
```

### Priority 2: Purge binary databases from history

```bash
# Using BFG Repo-Cleaner (recommended)
bfg --delete-files '*.db' .

# Or using git filter-repo
git filter-repo --path cmd/harnessd/.harness/state.db --invert-paths
git filter-repo --path cmd/harnessd/.harness/cron.db --invert-paths
```

### Priority 3: Consider adding pre-commit hooks

A pre-commit hook that rejects `*.db`, `*.sqlite`, `*.pem`, `*.key`, and `.env*` files would prevent future accidental commits.

---

## Conclusion

The repository is in good shape from a secrets perspective. No real credentials, tokens, or passwords exist in the git history. The only actionable items are the two SQLite database files that should be purged from history and excluded via `.gitignore` before the repository is made public.
