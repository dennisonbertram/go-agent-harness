package workingmemory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	om "go-agent-harness/internal/observationalmemory"
	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS working_memory_entries (
	memory_id TEXT NOT NULL,
	entry_key TEXT NOT NULL,
	entry_json TEXT NOT NULL,
	PRIMARY KEY (memory_id, entry_key)
);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("working memory: sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("working memory: create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("working memory: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("working memory: set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("working memory: set busy timeout: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, sqliteSchema)
	return err
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) Set(ctx context.Context, scope om.ScopeKey, key string, value any) error {
	mem := NewMemoryStore()
	if err := mem.Set(ctx, scope, key, value); err != nil {
		return err
	}
	entry, _, err := mem.Get(ctx, scope, key)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO working_memory_entries (memory_id, entry_key, entry_json)
VALUES (?, ?, ?)
ON CONFLICT(memory_id, entry_key) DO UPDATE SET entry_json = excluded.entry_json
`, scope.MemoryID(), key, entry)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, scope om.ScopeKey, key string) (string, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT entry_json FROM working_memory_entries
WHERE memory_id = ? AND entry_key = ?
`, scope.MemoryID(), key)
	var value string
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (s *SQLiteStore) Delete(ctx context.Context, scope om.ScopeKey, key string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM working_memory_entries
WHERE memory_id = ? AND entry_key = ?
`, scope.MemoryID(), key)
	return err
}

func (s *SQLiteStore) List(ctx context.Context, scope om.ScopeKey) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT entry_key, entry_json
FROM working_memory_entries
WHERE memory_id = ?
`, scope.MemoryID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Snippet(ctx context.Context, scope om.ScopeKey) (string, error) {
	mem := &MemoryStore{entries: map[string]map[string]string{}}
	entries, err := s.List(ctx, scope)
	if err != nil {
		return "", err
	}
	mem.entries[scope.MemoryID()] = entries
	return mem.Snippet(ctx, scope)
}
