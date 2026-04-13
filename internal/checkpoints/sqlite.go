package checkpoints

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS checkpoints (
	id              TEXT PRIMARY KEY,
	kind            TEXT NOT NULL,
	status          TEXT NOT NULL,
	run_id          TEXT NOT NULL DEFAULT '',
	workflow_run_id TEXT NOT NULL DEFAULT '',
	call_id         TEXT NOT NULL DEFAULT '',
	tool            TEXT NOT NULL DEFAULT '',
	args            TEXT NOT NULL DEFAULT '',
	questions       TEXT NOT NULL DEFAULT '',
	suspend_payload TEXT NOT NULL DEFAULT '',
	resume_schema   TEXT NOT NULL DEFAULT '',
	resume_payload  TEXT NOT NULL DEFAULT '',
	deadline_at     TEXT NOT NULL DEFAULT '',
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_run_pending
	ON checkpoints(run_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_checkpoints_workflow_pending
	ON checkpoints(workflow_run_id, status, updated_at);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("checkpoints: sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("checkpoints: create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("checkpoints: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("checkpoints: set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("checkpoints: set busy timeout: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, sqliteSchema); err != nil {
		return fmt.Errorf("checkpoints: migrate: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Create(ctx context.Context, record *Record) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO checkpoints (
	id, kind, status, run_id, workflow_run_id, call_id, tool, args, questions,
	suspend_payload, resume_schema, resume_payload, deadline_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		record.ID,
		string(record.Kind),
		string(record.Status),
		record.RunID,
		record.WorkflowRunID,
		record.CallID,
		record.Tool,
		record.Args,
		record.Questions,
		record.SuspendPayload,
		record.ResumeSchema,
		record.ResumePayload,
		timeString(record.DeadlineAt),
		timeString(record.CreatedAt),
		timeString(record.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("checkpoints: create: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Update(ctx context.Context, record *Record) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE checkpoints
SET kind = ?,
	status = ?,
	run_id = ?,
	workflow_run_id = ?,
	call_id = ?,
	tool = ?,
	args = ?,
	questions = ?,
	suspend_payload = ?,
	resume_schema = ?,
	resume_payload = ?,
	deadline_at = ?,
	updated_at = ?
WHERE id = ?
`,
		string(record.Kind),
		string(record.Status),
		record.RunID,
		record.WorkflowRunID,
		record.CallID,
		record.Tool,
		record.Args,
		record.Questions,
		record.SuspendPayload,
		record.ResumeSchema,
		record.ResumePayload,
		timeString(record.DeadlineAt),
		timeString(record.UpdatedAt),
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("checkpoints: update: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*Record, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, kind, status, run_id, workflow_run_id, call_id, tool, args, questions,
       suspend_payload, resume_schema, resume_payload, deadline_at, created_at, updated_at
FROM checkpoints
WHERE id = ?
`, id)
	record, err := scanRecord(row)
	if err == sql.ErrNoRows {
		return nil, &NotFoundError{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("checkpoints: get: %w", err)
	}
	return record, nil
}

func (s *SQLiteStore) PendingByRun(ctx context.Context, runID string) (*Record, error) {
	return s.pendingBy(ctx, "run_id", runID)
}

func (s *SQLiteStore) PendingByWorkflowRun(ctx context.Context, workflowRunID string) (*Record, error) {
	return s.pendingBy(ctx, "workflow_run_id", workflowRunID)
}

func (s *SQLiteStore) pendingBy(ctx context.Context, column, value string) (*Record, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, kind, status, run_id, workflow_run_id, call_id, tool, args, questions,
       suspend_payload, resume_schema, resume_payload, deadline_at, created_at, updated_at
FROM checkpoints
WHERE `+column+` = ? AND status = ?
ORDER BY updated_at DESC
LIMIT 1
`, value, string(StatusPending))
	record, err := scanRecord(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checkpoints: pending query: %w", err)
	}
	return record, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (*Record, error) {
	var (
		record                           Record
		kind, status                     string
		deadlineAt, createdAt, updatedAt string
	)
	if err := row.Scan(
		&record.ID,
		&kind,
		&status,
		&record.RunID,
		&record.WorkflowRunID,
		&record.CallID,
		&record.Tool,
		&record.Args,
		&record.Questions,
		&record.SuspendPayload,
		&record.ResumeSchema,
		&record.ResumePayload,
		&deadlineAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	record.Kind = Kind(kind)
	record.Status = Status(status)
	record.DeadlineAt = parseTime(deadlineAt)
	record.CreatedAt = parseTime(createdAt)
	record.UpdatedAt = parseTime(updatedAt)
	return &record, nil
}

func timeString(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return ts.UTC()
}
