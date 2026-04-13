package workflows

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS workflow_runs (
	id TEXT PRIMARY KEY,
	workflow_name TEXT NOT NULL,
	status TEXT NOT NULL,
	current_step_id TEXT NOT NULL DEFAULT '',
	current_checkpoint_id TEXT NOT NULL DEFAULT '',
	input_json TEXT NOT NULL DEFAULT '',
	output_json TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_step_states (
	workflow_run_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	status TEXT NOT NULL,
	output_json TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workflow_run_id, step_id)
);

CREATE TABLE IF NOT EXISTS workflow_events (
	workflow_run_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	event_type TEXT NOT NULL,
	payload_json TEXT NOT NULL DEFAULT '',
	timestamp TEXT NOT NULL,
	PRIMARY KEY (workflow_run_id, seq)
);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("workflows: sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("workflows: create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("workflows: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("workflows: set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("workflows: set busy timeout: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, sqliteSchema); err != nil {
		return fmt.Errorf("workflows: migrate: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) CreateRun(ctx context.Context, run *Run) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_runs (id, workflow_name, status, current_step_id, current_checkpoint_id, input_json, output_json, error, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, run.ID, run.WorkflowName, string(run.Status), run.CurrentStepID, run.CurrentCheckpointID, run.InputJSON, run.OutputJSON, run.Error, formatTime(run.CreatedAt), formatTime(run.UpdatedAt))
	return err
}

func (s *SQLiteStore) UpdateRun(ctx context.Context, run *Run) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET workflow_name = ?, status = ?, current_step_id = ?, current_checkpoint_id = ?, input_json = ?, output_json = ?, error = ?, updated_at = ?
WHERE id = ?
`, run.WorkflowName, string(run.Status), run.CurrentStepID, run.CurrentCheckpointID, run.InputJSON, run.OutputJSON, run.Error, formatTime(run.UpdatedAt), run.ID)
	return err
}

func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, workflow_name, status, current_step_id, current_checkpoint_id, input_json, output_json, error, created_at, updated_at
FROM workflow_runs WHERE id = ?
`, id)
	var run Run
	var status, createdAt, updatedAt string
	if err := row.Scan(&run.ID, &run.WorkflowName, &status, &run.CurrentStepID, &run.CurrentCheckpointID, &run.InputJSON, &run.OutputJSON, &run.Error, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	run.Status = RunStatus(status)
	run.CreatedAt = parseTime(createdAt)
	run.UpdatedAt = parseTime(updatedAt)
	return &run, nil
}

func (s *SQLiteStore) UpsertStepState(ctx context.Context, state *StepState) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_step_states (workflow_run_id, step_id, status, output_json, error, started_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(workflow_run_id, step_id) DO UPDATE SET
	status = excluded.status,
	output_json = excluded.output_json,
	error = excluded.error,
	started_at = excluded.started_at,
	updated_at = excluded.updated_at
`, state.WorkflowRunID, state.StepID, string(state.Status), state.OutputJSON, state.Error, formatTime(state.StartedAt), formatTime(state.UpdatedAt))
	return err
}

func (s *SQLiteStore) ListStepStates(ctx context.Context, workflowRunID string) ([]StepState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_run_id, step_id, status, output_json, error, started_at, updated_at
FROM workflow_step_states
WHERE workflow_run_id = ?
ORDER BY started_at ASC
`, workflowRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StepState
	for rows.Next() {
		var state StepState
		var status, startedAt, updatedAt string
		if err := rows.Scan(&state.WorkflowRunID, &state.StepID, &status, &state.OutputJSON, &state.Error, &startedAt, &updatedAt); err != nil {
			return nil, err
		}
		state.Status = StepStatus(status)
		state.StartedAt = parseTime(startedAt)
		state.UpdatedAt = parseTime(updatedAt)
		out = append(out, state)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event *Event) error {
	raw, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO workflow_events (workflow_run_id, seq, event_type, payload_json, timestamp)
VALUES (?, ?, ?, ?, ?)
`, event.WorkflowRunID, event.Seq, event.Type, string(raw), formatTime(event.Timestamp))
	return err
}

func (s *SQLiteStore) GetEvents(ctx context.Context, workflowRunID string, afterSeq int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_run_id, seq, event_type, payload_json, timestamp
FROM workflow_events
WHERE workflow_run_id = ? AND seq > ?
ORDER BY seq ASC
`, workflowRunID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var event Event
		var payloadJSON, timestamp string
		if err := rows.Scan(&event.WorkflowRunID, &event.Seq, &event.Type, &payloadJSON, &timestamp); err != nil {
			return nil, err
		}
		if payloadJSON != "" {
			if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
				return nil, err
			}
		}
		event.Timestamp = parseTime(timestamp)
		out = append(out, event)
	}
	return out, rows.Err()
}

func formatTime(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) time.Time {
	ts, _ := time.Parse(time.RFC3339Nano, raw)
	return ts.UTC()
}
