package tools

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

// CallbackStore intentionally has no retry/idempotency policy; #1006 owns it.
type CallbackStore interface {
	Migrate(context.Context) error
	Create(context.Context, CallbackInfo) error
	Get(context.Context, string) (CallbackInfo, error)
	Update(context.Context, CallbackInfo) error
	ListPending(context.Context) ([]CallbackInfo, error)
	Close() error
}
type SQLiteCallbackStore struct{ db *sql.DB }

func NewSQLiteCallbackStore(path string) (*SQLiteCallbackStore, error) {
	if path == "" {
		return nil, fmt.Errorf("callback sqlite path is required")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	if _, e = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); e != nil {
		db.Close()
		return nil, e
	}
	return &SQLiteCallbackStore{db}, nil
}
func (s *SQLiteCallbackStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
func (s *SQLiteCallbackStore) Migrate(c context.Context) error {
	_, e := s.db.ExecContext(c, `CREATE TABLE IF NOT EXISTS delayed_callbacks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT '', agent_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL, prompt TEXT NOT NULL, delay TEXT NOT NULL, fires_at TIMESTAMP NOT NULL, state TEXT NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, attempt INTEGER NOT NULL DEFAULT 0, run_id TEXT NOT NULL DEFAULT ''); CREATE INDEX IF NOT EXISTS idx_delayed_callbacks_pending ON delayed_callbacks(state,fires_at);`)
	return e
}
func (s *SQLiteCallbackStore) Create(c context.Context, i CallbackInfo) error {
	_, e := s.db.ExecContext(c, `INSERT INTO delayed_callbacks(id,tenant_id,agent_id,conversation_id,prompt,delay,fires_at,state,created_at,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?)`, i.ID, i.TenantID, i.AgentID, i.ConversationID, i.Prompt, i.Delay, i.FiresAt, i.State, i.CreatedAt, i.CreatedAt)
	return e
}
func (s *SQLiteCallbackStore) Update(c context.Context, i CallbackInfo) error {
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state=?,fires_at=?,updated_at=? WHERE id=?`, i.State, i.FiresAt, time.Now(), i.ID)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("callback %s not found", i.ID)
	}
	return nil
}
func (s *SQLiteCallbackStore) Get(c context.Context, id string) (CallbackInfo, error) {
	return scanCallback(s.db.QueryRowContext(c, `SELECT id,tenant_id,agent_id,conversation_id,prompt,delay,fires_at,state,created_at FROM delayed_callbacks WHERE id=?`, id))
}
func (s *SQLiteCallbackStore) ListPending(c context.Context) ([]CallbackInfo, error) {
	rs, e := s.db.QueryContext(c, `SELECT id,tenant_id,agent_id,conversation_id,prompt,delay,fires_at,state,created_at FROM delayed_callbacks WHERE state='pending' ORDER BY fires_at,id`)
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	var out []CallbackInfo
	for rs.Next() {
		i, e := scanCallback(rs)
		if e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rs.Err()
}

type callbackScanner interface{ Scan(...any) error }

func scanCallback(r callbackScanner) (CallbackInfo, error) {
	var i CallbackInfo
	e := r.Scan(&i.ID, &i.TenantID, &i.AgentID, &i.ConversationID, &i.Prompt, &i.Delay, &i.FiresAt, &i.State, &i.CreatedAt)
	return i, e
}
