package observationalmemory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS om_memory_records (
	memory_id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	state_version BIGINT NOT NULL,
	last_observed_message_index BIGINT NOT NULL,
	active_observations_json TEXT NOT NULL,
	active_observation_tokens BIGINT NOT NULL,
	active_reflection TEXT NOT NULL,
	active_reflection_tokens BIGINT NOT NULL,
	last_reflected_observation_seq BIGINT NOT NULL,
	config_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	UNIQUE (tenant_id, conversation_id, agent_id)
);

CREATE TABLE IF NOT EXISTS om_operation_log (
	operation_id TEXT PRIMARY KEY,
	memory_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	scope_sequence BIGINT NOT NULL,
	operation_type TEXT NOT NULL,
	status TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	error_text TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS om_markers (
	marker_id TEXT PRIMARY KEY,
	memory_id TEXT NOT NULL,
	marker_type TEXT NOT NULL,
	cycle_id TEXT NOT NULL,
	message_index_start BIGINT NOT NULL,
	message_index_end BIGINT NOT NULL,
	token_count BIGINT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS om_operation_log_memory_scope_seq_idx ON om_operation_log(memory_id, scope_sequence);
CREATE INDEX IF NOT EXISTS om_operation_log_status_created_idx ON om_operation_log(status, created_at);
CREATE INDEX IF NOT EXISTS om_markers_memory_created_idx ON om_markers(memory_id, created_at);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, sqliteSchema)
	if err != nil {
		return fmt.Errorf("sqlite migrate: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ResetStaleOperations(ctx context.Context, olderThan time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE om_operation_log
SET status = 'queued', error_text = 'requeued after stale processing operation', updated_at = ?
WHERE status = 'processing' AND updated_at < ?
`, nowString(time.Now().UTC()), nowString(olderThan.UTC()))
	if err != nil {
		return fmt.Errorf("reset stale operations: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetOrCreateRecord(ctx context.Context, key ScopeKey, defaultEnabled bool, defaultConfig Config, now time.Time) (Record, error) {
	memoryID := key.MemoryID()
	rec, err := s.getRecord(ctx, memoryID)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Record{}, err
	}

	configJSON, err := json.Marshal(defaultConfig)
	if err != nil {
		return Record{}, fmt.Errorf("marshal default config: %w", err)
	}
	activeJSON := "[]"
	nowText := nowString(now.UTC())
	_, insertErr := s.db.ExecContext(ctx, `
INSERT INTO om_memory_records (
	memory_id, tenant_id, conversation_id, agent_id, enabled, state_version,
	last_observed_message_index, active_observations_json, active_observation_tokens,
	active_reflection, active_reflection_tokens, last_reflected_observation_seq,
	config_json, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, 1, -1, ?, 0, '', 0, 0, ?, ?, ?)
`, memoryID, key.TenantID, key.ConversationID, key.AgentID, defaultEnabled, activeJSON, string(configJSON), nowText, nowText)
	if insertErr != nil {
		// Handle write races by querying again after insert conflict.
		rec, err = s.getRecord(ctx, memoryID)
		if err == nil {
			return rec, nil
		}
		return Record{}, fmt.Errorf("insert memory record: %w", insertErr)
	}
	return s.getRecord(ctx, memoryID)
}

func (s *SQLiteStore) UpdateRecord(ctx context.Context, rec Record) error {
	activeJSON, err := json.Marshal(rec.ActiveObservations)
	if err != nil {
		return fmt.Errorf("marshal active observations: %w", err)
	}
	configJSON, err := json.Marshal(rec.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE om_memory_records
SET
	enabled = ?,
	state_version = ?,
	last_observed_message_index = ?,
	active_observations_json = ?,
	active_observation_tokens = ?,
	active_reflection = ?,
	active_reflection_tokens = ?,
	last_reflected_observation_seq = ?,
	config_json = ?,
	updated_at = ?
WHERE memory_id = ?
`,
		rec.Enabled,
		rec.StateVersion,
		rec.LastObservedMessageIndex,
		string(activeJSON),
		rec.ActiveObservationTokens,
		rec.ActiveReflection,
		rec.ActiveReflectionTokens,
		rec.LastReflectedObservationSeq,
		string(configJSON),
		nowString(rec.UpdatedAt.UTC()),
		rec.MemoryID,
	)
	if err != nil {
		return fmt.Errorf("update memory record: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CreateOperation(ctx context.Context, op Operation) (Operation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Operation{}, fmt.Errorf("begin operation tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var nextSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(scope_sequence), 0) + 1 FROM om_operation_log WHERE memory_id = ?`, op.MemoryID).Scan(&nextSeq); err != nil {
		return Operation{}, fmt.Errorf("read next scope sequence: %w", err)
	}
	op.ScopeSequence = nextSeq
	if op.Status == "" {
		op.Status = "queued"
	}
	if op.PayloadJSON == "" {
		op.PayloadJSON = "{}"
	}
	if op.ErrorText == "" {
		op.ErrorText = ""
	}
	if op.CreatedAt.IsZero() {
		op.CreatedAt = time.Now().UTC()
	}
	if op.UpdatedAt.IsZero() {
		op.UpdatedAt = op.CreatedAt
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO om_operation_log (
	operation_id, memory_id, run_id, tool_call_id, scope_sequence,
	operation_type, status, payload_json, error_text, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		op.OperationID,
		op.MemoryID,
		op.RunID,
		op.ToolCallID,
		op.ScopeSequence,
		op.OperationType,
		op.Status,
		op.PayloadJSON,
		op.ErrorText,
		nowString(op.CreatedAt.UTC()),
		nowString(op.UpdatedAt.UTC()),
	)
	if err != nil {
		return Operation{}, fmt.Errorf("insert operation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Operation{}, fmt.Errorf("commit operation tx: %w", err)
	}
	return op, nil
}

func (s *SQLiteStore) UpdateOperationStatus(ctx context.Context, operationID, status, errorText string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE om_operation_log
SET status = ?, error_text = ?, updated_at = ?
WHERE operation_id = ?
`, status, errorText, nowString(now.UTC()), operationID)
	if err != nil {
		return fmt.Errorf("update operation status: %w", err)
	}
	return nil
}

func (s *SQLiteStore) InsertMarker(ctx context.Context, marker Marker) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO om_markers (
	marker_id, memory_id, marker_type, cycle_id,
	message_index_start, message_index_end, token_count,
	payload_json, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		marker.MarkerID,
		marker.MemoryID,
		marker.MarkerType,
		marker.CycleID,
		marker.MessageIndexStart,
		marker.MessageIndexEnd,
		marker.TokenCount,
		marker.PayloadJSON,
		nowString(marker.CreatedAt.UTC()),
	)
	if err != nil {
		return fmt.Errorf("insert marker: %w", err)
	}
	return nil
}

func (s *SQLiteStore) getRecord(ctx context.Context, memoryID string) (Record, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	memory_id, tenant_id, conversation_id, agent_id,
	enabled, state_version, last_observed_message_index,
	active_observations_json, active_observation_tokens,
	active_reflection, active_reflection_tokens,
	last_reflected_observation_seq,
	config_json, created_at, updated_at
FROM om_memory_records
WHERE memory_id = ?
`, memoryID)

	var rec Record
	var activeJSON string
	var configJSON string
	var createdText string
	var updatedText string
	if err := row.Scan(
		&rec.MemoryID,
		&rec.Scope.TenantID,
		&rec.Scope.ConversationID,
		&rec.Scope.AgentID,
		&rec.Enabled,
		&rec.StateVersion,
		&rec.LastObservedMessageIndex,
		&activeJSON,
		&rec.ActiveObservationTokens,
		&rec.ActiveReflection,
		&rec.ActiveReflectionTokens,
		&rec.LastReflectedObservationSeq,
		&configJSON,
		&createdText,
		&updatedText,
	); err != nil {
		return Record{}, err
	}
	if err := json.Unmarshal([]byte(activeJSON), &rec.ActiveObservations); err != nil {
		return Record{}, fmt.Errorf("unmarshal active observations: %w", err)
	}
	if err := json.Unmarshal([]byte(configJSON), &rec.Config); err != nil {
		return Record{}, fmt.Errorf("unmarshal config: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdText)
	if err != nil {
		return Record{}, fmt.Errorf("parse created_at: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedText)
	if err != nil {
		return Record{}, fmt.Errorf("parse updated_at: %w", err)
	}
	rec.CreatedAt = createdAt
	rec.UpdatedAt = updatedAt
	return rec, nil
}

func nowString(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
