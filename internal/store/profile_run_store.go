package store

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

// profileRunsSchema defines the profile_runs table.
const profileRunsSchema = `
CREATE TABLE IF NOT EXISTS profile_runs (
	id           TEXT PRIMARY KEY,
	profile_name TEXT NOT NULL,
	run_id       TEXT NOT NULL,
	status       TEXT NOT NULL,
	step_count   INTEGER NOT NULL DEFAULT 0,
	cost_usd     REAL NOT NULL DEFAULT 0.0,
	started_at   TEXT NOT NULL,
	finished_at  TEXT NOT NULL,
	tool_calls   INTEGER NOT NULL DEFAULT 0,
	top_tools    TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_profile_runs_profile    ON profile_runs(profile_name);
CREATE INDEX IF NOT EXISTS idx_profile_runs_started    ON profile_runs(profile_name, started_at DESC);
`

// ProfileRunStoreIface is the interface used by the runner for profile run
// persistence. SQLiteProfileRunStore implements this interface.
type ProfileRunStoreIface interface {
	RecordProfileRun(ctx context.Context, r ProfileRunRecord) error
	QueryRecentProfileRuns(ctx context.Context, profileName string, limit int) ([]ProfileRunRecord, error)
	AggregateProfileStats(ctx context.Context, profileName string) (ProfileStats, error)
}

// ProfileRunRecord holds one persisted profile run entry.
type ProfileRunRecord struct {
	ID          string
	ProfileName string
	RunID       string
	// Status is "completed", "failed", or "partial".
	Status     string
	StepCount  int
	CostUSD    float64
	StartedAt  time.Time
	FinishedAt time.Time
	ToolCalls  int
	TopTools   []string // top-3 tool names (JSON-encoded in the database)
}

// ProfileStats holds aggregate statistics for a profile.
type ProfileStats struct {
	ProfileName string
	RunCount    int
	AvgSteps    float64
	AvgCostUSD  float64
	SuccessRate float64
	LastRunAt   time.Time
}

// SQLiteProfileRunStore persists per-profile run history in a SQLite database.
// It is a separate database from the main run store to avoid coupling.
type SQLiteProfileRunStore struct {
	db *sql.DB
}

// NewSQLiteProfileRunStore opens (or creates) a SQLite database at path and
// applies the profile_runs schema. The database is ready to use immediately.
func NewSQLiteProfileRunStore(path string) (*SQLiteProfileRunStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store: profile run store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create profile run store directory: %w", err)
	}
	dsn := path + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open profile run store: %w", err)
	}
	// Single writer to avoid SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: profile run store set WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: profile run store busy timeout: %w", err)
	}
	if _, err := db.Exec(profileRunsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: profile run store migrate: %w", err)
	}
	return &SQLiteProfileRunStore{db: db}, nil
}

// Close releases the database connection.
func (s *SQLiteProfileRunStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// RecordProfileRun persists a completed profile run record.
// If a record with the same ID already exists the call is a no-op (idempotent).
func (s *SQLiteProfileRunStore) RecordProfileRun(ctx context.Context, r ProfileRunRecord) error {
	topToolsJSON, err := json.Marshal(r.TopTools)
	if err != nil {
		return fmt.Errorf("store: marshal top_tools: %w", err)
	}
	_, execErr := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO profile_runs
	(id, profile_name, run_id, status, step_count, cost_usd, started_at, finished_at, tool_calls, top_tools)
VALUES
	(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		r.ID,
		r.ProfileName,
		r.RunID,
		r.Status,
		r.StepCount,
		r.CostUSD,
		timeString(r.StartedAt),
		timeString(r.FinishedAt),
		r.ToolCalls,
		string(topToolsJSON),
	)
	if execErr != nil {
		return fmt.Errorf("store: record profile run: %w", execErr)
	}
	return nil
}

// QueryRecentProfileRuns returns the most recent `limit` runs for a profile,
// ordered by started_at DESC. Returns an empty slice (not an error) when there
// is no history for the given profile name.
func (s *SQLiteProfileRunStore) QueryRecentProfileRuns(ctx context.Context, profileName string, limit int) ([]ProfileRunRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, profile_name, run_id, status, step_count, cost_usd,
       started_at, finished_at, tool_calls, top_tools
FROM profile_runs
WHERE profile_name = ?
ORDER BY started_at DESC
LIMIT ?
`, profileName, limit)
	if err != nil {
		return nil, fmt.Errorf("store: query recent profile runs: %w", err)
	}
	defer rows.Close()

	var records []ProfileRunRecord
	for rows.Next() {
		var rec ProfileRunRecord
		var startedText, finishedText, topToolsJSON string
		if err := rows.Scan(
			&rec.ID,
			&rec.ProfileName,
			&rec.RunID,
			&rec.Status,
			&rec.StepCount,
			&rec.CostUSD,
			&startedText,
			&finishedText,
			&rec.ToolCalls,
			&topToolsJSON,
		); err != nil {
			return nil, fmt.Errorf("store: scan profile run: %w", err)
		}
		if t, err := time.Parse(time.RFC3339Nano, startedText); err == nil {
			rec.StartedAt = t
		}
		if t, err := time.Parse(time.RFC3339Nano, finishedText); err == nil {
			rec.FinishedAt = t
		}
		if topToolsJSON != "" && topToolsJSON != "null" {
			_ = json.Unmarshal([]byte(topToolsJSON), &rec.TopTools)
		}
		if rec.TopTools == nil {
			rec.TopTools = []string{}
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: profile runs rows: %w", err)
	}
	if records == nil {
		records = []ProfileRunRecord{}
	}
	return records, nil
}

// AggregateProfileStats returns aggregate statistics for a profile.
// When there is no history for the profile, it returns a zero-value
// ProfileStats (RunCount=0) without an error.
func (s *SQLiteProfileRunStore) AggregateProfileStats(ctx context.Context, profileName string) (ProfileStats, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	COUNT(*)                                              AS run_count,
	COALESCE(AVG(step_count), 0.0)                        AS avg_steps,
	COALESCE(AVG(cost_usd), 0.0)                          AS avg_cost_usd,
	COALESCE(
		CAST(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS REAL)
		/ NULLIF(COUNT(*), 0),
		0.0
	)                                                     AS success_rate,
	COALESCE(MAX(started_at), '')                         AS last_run_at
FROM profile_runs
WHERE profile_name = ?
`, profileName)

	var stats ProfileStats
	stats.ProfileName = profileName

	var lastRunText string
	if err := row.Scan(
		&stats.RunCount,
		&stats.AvgSteps,
		&stats.AvgCostUSD,
		&stats.SuccessRate,
		&lastRunText,
	); err != nil {
		return ProfileStats{}, fmt.Errorf("store: aggregate profile stats: %w", err)
	}
	if lastRunText != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastRunText); err == nil {
			stats.LastRunAt = t
		}
	}
	return stats, nil
}
