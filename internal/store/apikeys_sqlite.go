package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// apiKeysSchema defines the SQLite table for API key storage.
// This is applied via MigrateAPIKeys in addition to the base Migrate.
const apiKeysSchema = `
CREATE TABLE IF NOT EXISTS api_keys (
	id           TEXT PRIMARY KEY,
	key_hash     TEXT NOT NULL UNIQUE,
	key_prefix   TEXT NOT NULL,
	tenant_id    TEXT NOT NULL,
	name         TEXT NOT NULL DEFAULT '',
	scopes_json  TEXT NOT NULL DEFAULT '[]',
	created_at   TEXT NOT NULL,
	last_used_at TEXT,
	expires_at   TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id);
`

// MigrateAPIKeys creates the api_keys table if it does not exist.
// It is separate from Migrate so that callers can opt-in to key management.
func (s *SQLiteStore) MigrateAPIKeys(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, apiKeysSchema); err != nil {
		return fmt.Errorf("store: migrate api_keys: %w", err)
	}
	return nil
}

// CreateAPIKey persists a new API key record.
func (s *SQLiteStore) CreateAPIKey(ctx context.Context, key APIKey) error {
	scopesJSON, err := json.Marshal(key.Scopes)
	if err != nil {
		return fmt.Errorf("store: marshal scopes: %w", err)
	}

	var lastUsedAt, expiresAt *string
	if key.LastUsedAt != nil {
		ts := timeString(*key.LastUsedAt)
		lastUsedAt = &ts
	}
	if key.ExpiresAt != nil {
		ts := timeString(*key.ExpiresAt)
		expiresAt = &ts
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO api_keys (id, key_hash, key_prefix, tenant_id, name, scopes_json, created_at, last_used_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		key.ID,
		key.KeyHash,
		key.KeyPrefix,
		key.TenantID,
		key.Name,
		string(scopesJSON),
		timeString(key.CreatedAt),
		lastUsedAt,
		expiresAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("store: api key %q already exists", key.ID)
		}
		return fmt.Errorf("store: create api key: %w", err)
	}
	return nil
}

// ValidateAPIKey checks rawToken against all stored key hashes, updates
// last_used_at on success, and returns the matching APIKey.
// Uses bcrypt.CompareHashAndPassword for timing-safe comparison.
func (s *SQLiteStore) ValidateAPIKey(ctx context.Context, rawToken string) (*APIKey, error) {
	// Load all keys first (closing the rows before bcrypt runs), then compare.
	// This avoids holding an open query cursor while bcrypt blocks for ~300ms,
	// which would deadlock on a single-connection SQLite pool when the subsequent
	// UPDATE also needs the connection.
	rows, err := s.db.QueryContext(ctx, `
SELECT id, key_hash, key_prefix, tenant_id, name, scopes_json, created_at, last_used_at, expires_at
FROM api_keys
`)
	if err != nil {
		return nil, fmt.Errorf("store: validate api key query: %w", err)
	}

	var allKeys []*APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("store: scan api key: %w", err)
		}
		allKeys = append(allKeys, key)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("store: validate api key rows: %w", err)
	}
	// Close the rows (release the connection) BEFORE running bcrypt.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: validate api key rows close: %w", err)
	}

	now := time.Now().UTC()
	var matched *APIKey

	for _, key := range allKeys {
		// Timing-safe comparison via bcrypt.
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawToken)); err != nil {
			continue
		}

		// Check expiration.
		if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
			return nil, ErrKeyExpired
		}

		matched = key
		break
	}

	if matched == nil {
		return nil, &ErrKeyNotFound{}
	}

	// Update last_used_at (best-effort; ignore errors).
	ts := timeString(now)
	_, _ = s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, ts, matched.ID)
	matched.LastUsedAt = &now

	return matched, nil
}

// ListAPIKeys returns all API keys for a tenant (key hashes are excluded from the return).
func (s *SQLiteStore) ListAPIKeys(ctx context.Context, tenantID string) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, key_hash, key_prefix, tenant_id, name, scopes_json, created_at, last_used_at, expires_at
FROM api_keys
WHERE tenant_id = ?
ORDER BY created_at DESC
`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan api key: %w", err)
		}
		// Zero out the hash before returning for safety.
		key.KeyHash = ""
		keys = append(keys, *key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list api keys rows: %w", err)
	}
	if keys == nil {
		keys = []APIKey{}
	}
	return keys, nil
}

// RevokeAPIKey removes an API key by ID.
func (s *SQLiteStore) RevokeAPIKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: revoke api key: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: revoke api key rows affected: %w", err)
	}
	if n == 0 {
		return &ErrKeyNotFound{ID: id}
	}
	return nil
}

// scanAPIKey scans a row from the api_keys table.
func scanAPIKey(row rowScanner) (*APIKey, error) {
	key := &APIKey{}
	var scopesJSON string
	var createdText string
	var lastUsedText sql.NullString
	var expiresText sql.NullString

	if err := row.Scan(
		&key.ID,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.TenantID,
		&key.Name,
		&scopesJSON,
		&createdText,
		&lastUsedText,
		&expiresText,
	); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(scopesJSON), &key.Scopes); err != nil {
		key.Scopes = []string{}
	}
	if key.Scopes == nil {
		key.Scopes = []string{}
	}

	if t, err := time.Parse(time.RFC3339Nano, createdText); err == nil {
		key.CreatedAt = t
	}
	if lastUsedText.Valid {
		if t, err := time.Parse(time.RFC3339Nano, lastUsedText.String); err == nil {
			key.LastUsedAt = &t
		}
	}
	if expiresText.Valid {
		if t, err := time.Parse(time.RFC3339Nano, expiresText.String); err == nil {
			key.ExpiresAt = &t
		}
	}

	return key, nil
}
