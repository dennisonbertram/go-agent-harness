package cron

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS cron_jobs (
	job_id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL DEFAULT '',
	conversation_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	schedule TEXT NOT NULL,
	execution_type TEXT NOT NULL,
	execution_config TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL DEFAULT 'active',
	timeout_seconds INTEGER NOT NULL DEFAULT 30,
	tags TEXT NOT NULL DEFAULT '',
	next_run_at TIMESTAMP NOT NULL,
	last_run_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS cron_executions (
	execution_id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	started_at TIMESTAMP NOT NULL,
	finished_at TIMESTAMP,
	status TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	output_summary TEXT NOT NULL DEFAULT '',
	error_text TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (job_id) REFERENCES cron_jobs(job_id)
);

CREATE INDEX IF NOT EXISTS idx_cron_executions_job_id ON cron_executions(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_executions_started_at ON cron_executions(started_at);
`

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed store.
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

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate creates the schema tables.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, sqliteSchema)
	if err != nil {
		return fmt.Errorf("sqlite migrate: %w", err)
	}
	if err := s.ensureCronJobsScopeColumns(ctx); err != nil {
		return err
	}
	if err := s.migrateScopedNameUniqueness(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_cron_jobs_active_scope_name ON cron_jobs(tenant_id, conversation_id, agent_id, name) WHERE status != 'deleted'`); err != nil {
		return fmt.Errorf("index scoped cron names: %w", err)
	}
	return nil
}

// migrateScopedNameUniqueness replaces the legacy global UNIQUE(name)
// constraint without dropping jobs or their execution history. SQLite cannot
// drop an inline uniqueness constraint, so rebuild both related tables in one
// transaction. The schema check makes repeat startups a no-op.
func (s *SQLiteStore) migrateScopedNameUniqueness(ctx context.Context) error {
	legacyGlobalNameUnique, err := s.hasLegacyGlobalNameUnique(ctx)
	if err != nil {
		return err
	}
	if !legacyGlobalNameUnique {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin scoped-name migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmts := []string{
		`ALTER TABLE cron_executions RENAME TO cron_executions_legacy`,
		`ALTER TABLE cron_jobs RENAME TO cron_jobs_legacy`,
		`CREATE TABLE cron_jobs (job_id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', agent_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, schedule TEXT NOT NULL, execution_type TEXT NOT NULL, execution_config TEXT NOT NULL DEFAULT '{}', status TEXT NOT NULL DEFAULT 'active', timeout_seconds INTEGER NOT NULL DEFAULT 30, tags TEXT NOT NULL DEFAULT '', next_run_at TIMESTAMP NOT NULL, last_run_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`INSERT INTO cron_jobs SELECT job_id, tenant_id, conversation_id, agent_id, name, schedule, execution_type, execution_config, status, timeout_seconds, tags, next_run_at, last_run_at, created_at, updated_at FROM cron_jobs_legacy`,
		`CREATE TABLE cron_executions (execution_id TEXT PRIMARY KEY, job_id TEXT NOT NULL, started_at TIMESTAMP NOT NULL, finished_at TIMESTAMP, status TEXT NOT NULL, run_id TEXT NOT NULL DEFAULT '', output_summary TEXT NOT NULL DEFAULT '', error_text TEXT NOT NULL DEFAULT '', duration_ms INTEGER NOT NULL DEFAULT 0, FOREIGN KEY (job_id) REFERENCES cron_jobs(job_id))`,
		`INSERT INTO cron_executions SELECT execution_id, job_id, started_at, finished_at, status, run_id, output_summary, error_text, duration_ms FROM cron_executions_legacy`,
		`DROP TABLE cron_executions_legacy`, `DROP TABLE cron_jobs_legacy`,
		`CREATE INDEX idx_cron_executions_job_id ON cron_executions(job_id)`, `CREATE INDEX idx_cron_executions_started_at ON cron_executions(started_at)`,
		`CREATE INDEX idx_cron_jobs_tenant_id ON cron_jobs(tenant_id)`,
		`CREATE UNIQUE INDEX idx_cron_jobs_active_scope_name ON cron_jobs(tenant_id, conversation_id, agent_id, name) WHERE status != 'deleted'`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("scoped-name migration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scoped-name migration: %w", err)
	}
	return nil
}

// hasLegacyGlobalNameUnique asks SQLite which indexes enforce cron_jobs
// uniqueness instead of parsing CREATE TABLE text. Inline, named, quoted, and
// collated UNIQUE(name) constraints all surface as a non-partial unique index
// with exactly one key column named "name". Composite scoped constraints and
// partial active-name indexes must not trigger a table rebuild.
func (s *SQLiteStore) hasLegacyGlobalNameUnique(ctx context.Context) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, "unique", origin, partial FROM pragma_index_list('cron_jobs')`)
	if err != nil {
		return false, fmt.Errorf("inspect cron_jobs indexes: %w", err)
	}
	type indexMetadata struct {
		name    string
		unique  bool
		origin  string
		partial bool
	}
	var indexes []indexMetadata
	for rows.Next() {
		var index indexMetadata
		if err := rows.Scan(&index.name, &index.unique, &index.origin, &index.partial); err != nil {
			_ = rows.Close()
			return false, fmt.Errorf("scan cron_jobs index: %w", err)
		}
		indexes = append(indexes, index)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return false, fmt.Errorf("inspect cron_jobs index rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("close cron_jobs index rows: %w", err)
	}

	for _, index := range indexes {
		if !index.unique || index.partial || index.origin == "pk" {
			continue
		}
		columns, err := s.db.QueryContext(ctx, `SELECT name FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, index.name)
		if err != nil {
			return false, fmt.Errorf("inspect cron_jobs index %q: %w", index.name, err)
		}
		var keyColumns []sql.NullString
		for columns.Next() {
			var name sql.NullString
			if err := columns.Scan(&name); err != nil {
				_ = columns.Close()
				return false, fmt.Errorf("scan cron_jobs index %q: %w", index.name, err)
			}
			keyColumns = append(keyColumns, name)
		}
		if err := columns.Err(); err != nil {
			_ = columns.Close()
			return false, fmt.Errorf("inspect cron_jobs index %q rows: %w", index.name, err)
		}
		if err := columns.Close(); err != nil {
			return false, fmt.Errorf("close cron_jobs index %q rows: %w", index.name, err)
		}
		if len(keyColumns) == 1 && keyColumns[0].Valid && strings.EqualFold(keyColumns[0].String, "name") {
			return true, nil
		}
	}
	return false, nil
}

func (s *SQLiteStore) ensureCronJobsScopeColumns(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(cron_jobs)`)
	if err != nil {
		return fmt.Errorf("inspect cron_jobs schema: %w", err)
	}

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan cron_jobs schema: %w", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("inspect cron_jobs schema rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close cron_jobs schema rows: %w", err)
	}
	for _, column := range []string{"tenant_id", "conversation_id", "agent_id"} {
		if !columns[column] {
			if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE cron_jobs ADD COLUMN %s TEXT NOT NULL DEFAULT ''", column)); err != nil {
				return fmt.Errorf("add cron_jobs %s: %w", column, err)
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_cron_jobs_tenant_id ON cron_jobs(tenant_id)`); err != nil {
		return fmt.Errorf("index cron_jobs tenant_id: %w", err)
	}
	return nil
}

// CreateJob inserts a new job.
func (s *SQLiteStore) CreateJob(ctx context.Context, job Job) (Job, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO cron_jobs (
	job_id, tenant_id, conversation_id, agent_id, name, schedule, execution_type, execution_config,
	status, timeout_seconds, tags, next_run_at, last_run_at,
	created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		job.ID,
		job.TenantID,
		job.ConversationID,
		job.AgentID,
		job.Name,
		job.Schedule,
		job.ExecType,
		job.ExecConfig,
		job.Status,
		job.TimeoutSec,
		job.Tags,
		nowString(job.NextRunAt),
		nullableTimeString(job.LastRunAt),
		nowString(job.CreatedAt),
		nowString(job.UpdatedAt),
	)
	if err != nil {
		return Job{}, fmt.Errorf("insert job: %w", err)
	}
	return job, nil
}

// GetJob retrieves a job by ID.
func (s *SQLiteStore) GetJob(ctx context.Context, id string) (Job, error) {
	return s.scanJob(s.db.QueryRowContext(ctx, `
SELECT job_id, tenant_id, name, schedule, execution_type, execution_config,
	conversation_id, agent_id,
	status, timeout_seconds, tags, next_run_at, last_run_at,
	created_at, updated_at
FROM cron_jobs
WHERE job_id = ? AND status != ?
`, id, StatusDeleted))
}

// GetJobByName retrieves a job by name.
func (s *SQLiteStore) GetJobByName(ctx context.Context, name string) (Job, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, tenant_id, name, schedule, execution_type, execution_config,
	conversation_id, agent_id,
	status, timeout_seconds, tags, next_run_at, last_run_at,
	created_at, updated_at
FROM cron_jobs
WHERE name = ? AND status != ?
ORDER BY created_at, job_id
LIMIT 2
`, name, StatusDeleted)
	if err != nil {
		return Job{}, fmt.Errorf("get job by name: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return Job{}, sql.ErrNoRows
	}
	job, err := s.scanJobRow(rows)
	if err != nil {
		return Job{}, err
	}
	if rows.Next() {
		return Job{}, ErrJobAmbiguous
	}
	return job, rows.Err()
}

func (s *SQLiteStore) GetJobInScope(ctx context.Context, id string, scope Scope) (Job, error) {
	return s.scanJob(s.db.QueryRowContext(ctx, `SELECT job_id, tenant_id, name, schedule, execution_type, execution_config, conversation_id, agent_id, status, timeout_seconds, tags, next_run_at, last_run_at, created_at, updated_at FROM cron_jobs WHERE job_id = ? AND tenant_id = ? AND conversation_id = ? AND agent_id = ? AND status != ?`, id, scope.TenantID, scope.ConversationID, scope.AgentID, StatusDeleted))
}

func (s *SQLiteStore) GetJobByNameInScope(ctx context.Context, name string, scope Scope) (Job, error) {
	return s.scanJob(s.db.QueryRowContext(ctx, `SELECT job_id, tenant_id, name, schedule, execution_type, execution_config, conversation_id, agent_id, status, timeout_seconds, tags, next_run_at, last_run_at, created_at, updated_at FROM cron_jobs WHERE name = ? AND tenant_id = ? AND conversation_id = ? AND agent_id = ? AND status != ?`, name, scope.TenantID, scope.ConversationID, scope.AgentID, StatusDeleted))
}

// ListJobs returns all non-deleted jobs.
func (s *SQLiteStore) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT job_id, tenant_id, name, schedule, execution_type, execution_config,
	conversation_id, agent_id,
	status, timeout_seconds, tags, next_run_at, last_run_at,
	created_at, updated_at
FROM cron_jobs
WHERE status != ?
ORDER BY created_at DESC
`, StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) ListJobsInScope(ctx context.Context, scope Scope) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT job_id, tenant_id, name, schedule, execution_type, execution_config, conversation_id, agent_id, status, timeout_seconds, tags, next_run_at, last_run_at, created_at, updated_at FROM cron_jobs WHERE tenant_id = ? AND conversation_id = ? AND agent_id = ? AND status != ? ORDER BY created_at DESC`, scope.TenantID, scope.ConversationID, scope.AgentID, StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("list scoped jobs: %w", err)
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		job, err := s.scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// UpdateJob updates a job record.
func (s *SQLiteStore) UpdateJob(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE cron_jobs
SET tenant_id = ?, name = ?, schedule = ?, execution_type = ?, execution_config = ?,
	conversation_id = ?, agent_id = ?,
	status = ?, timeout_seconds = ?, tags = ?, next_run_at = ?,
	last_run_at = ?, updated_at = ?
WHERE job_id = ?
`,
		job.TenantID,
		job.Name,
		job.Schedule,
		job.ExecType,
		job.ExecConfig,
		job.ConversationID,
		job.AgentID,
		job.Status,
		job.TimeoutSec,
		job.Tags,
		nowString(job.NextRunAt),
		nullableTimeString(job.LastRunAt),
		nowString(job.UpdatedAt),
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

// UpdateJobCAS updates a job only when its persisted version still matches
// expectedUpdatedAt. The database row count is the authority for conflict
// detection; callers never perform a read/check/write sequence in memory.
func (s *SQLiteStore) UpdateJobCAS(ctx context.Context, job Job, expectedUpdatedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE cron_jobs
SET tenant_id = ?, name = ?, schedule = ?, execution_type = ?, execution_config = ?,
	conversation_id = ?, agent_id = ?,
	status = ?, timeout_seconds = ?, tags = ?, next_run_at = ?,
	last_run_at = ?, updated_at = ?
WHERE job_id = ? AND status != ? AND updated_at = ?
`,
		job.TenantID,
		job.Name,
		job.Schedule,
		job.ExecType,
		job.ExecConfig,
		job.ConversationID,
		job.AgentID,
		job.Status,
		job.TimeoutSec,
		job.Tags,
		nowString(job.NextRunAt),
		nullableTimeString(job.LastRunAt),
		nowString(job.UpdatedAt),
		job.ID,
		StatusDeleted,
		nowString(expectedUpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("compare-and-swap job: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("compare-and-swap job rows affected: %w", err)
	}
	if rows == 0 {
		return ErrJobConflict
	}
	return nil
}

// ClaimJobTenant atomically assigns a tenantless legacy shell row. UPDATE
// ... RETURNING is the linearization point across cronsd processes sharing the
// database; a losing tenant observes the persisted winner and cannot expose
// the row.
func (s *SQLiteStore) ClaimJobTenant(ctx context.Context, jobID, tenantID string) (Job, bool, error) {
	if jobID == "" || tenantID == "" {
		return Job{}, false, fmt.Errorf("claim job tenant requires job and tenant IDs")
	}
	now := time.Now().UTC()
	job, err := s.scanJob(s.db.QueryRowContext(ctx, `
UPDATE cron_jobs
SET tenant_id = ?, updated_at = ?
WHERE job_id = ? AND tenant_id = '' AND execution_type = ? AND status != ?
RETURNING job_id, tenant_id, name, schedule, execution_type, execution_config,
	conversation_id, agent_id,
	status, timeout_seconds, tags, next_run_at, last_run_at,
	created_at, updated_at
`, tenantID, nowString(now), jobID, ExecTypeShell, StatusDeleted))
	if err == nil {
		return job, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, fmt.Errorf("claim job tenant: %w", err)
	}
	job, err = s.GetJob(ctx, jobID)
	if err != nil {
		if IsJobNotFound(err) {
			return Job{}, false, nil
		}
		return Job{}, false, fmt.Errorf("read claimed job tenant: %w", err)
	}
	return job, job.TenantID == tenantID, nil
}

// TouchJobRun updates only the run-tracking columns for a job
// (last_run_at, next_run_at, updated_at), leaving schedule, execution
// config, status, timeout, and tags untouched. This is used by the
// scheduler after firing a job so a concurrent user edit (or pause) is
// never silently reverted by a full-row overwrite.
func (s *SQLiteStore) TouchJobRun(ctx context.Context, jobID string, lastRun, nextRun, updatedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE cron_jobs
SET last_run_at = ?, next_run_at = ?, updated_at = ?
WHERE job_id = ?
  AND (last_run_at IS NULL OR last_run_at <= ?)
`,
		nullableTimeString(lastRun),
		nowString(nextRun),
		nowString(updatedAt),
		jobID,
		nullableTimeString(lastRun),
	)
	if err != nil {
		return fmt.Errorf("touch job run: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("touch job run rows affected: %w", err)
	}
	if rows == 0 {
		// A late completion is an expected stale write, not a not-found error.
		// Check existence so callers still receive the accurate error for a
		// deleted/nonexistent job without permitting any timestamp regression.
		if _, err := s.GetJob(ctx, jobID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteJob performs a soft delete by setting status to deleted.
// Scoped active-name uniqueness releases the name on delete without mutating
// the historical row's display name.
func (s *SQLiteStore) DeleteJob(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
UPDATE cron_jobs SET status = ?, updated_at = ? WHERE job_id = ?
`, StatusDeleted, nowString(now), id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete job rows affected: %w", err)
	}
	if rows == 0 {
		return ErrJobNotFound
	}
	return nil
}

// DeleteJobCAS soft-deletes only the version the caller most recently read.
func (s *SQLiteStore) DeleteJobCAS(ctx context.Context, id string, expectedUpdatedAt time.Time) error {
	now := time.Now().UTC()
	if !now.After(expectedUpdatedAt) {
		now = expectedUpdatedAt.Add(time.Nanosecond)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE cron_jobs SET status = ?, updated_at = ?
WHERE job_id = ? AND status != ? AND updated_at = ?
`, StatusDeleted, nowString(now), id, StatusDeleted, nowString(expectedUpdatedAt))
	if err != nil {
		return fmt.Errorf("compare-and-swap delete job: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("compare-and-swap delete job rows affected: %w", err)
	}
	if rows == 0 {
		return ErrJobConflict
	}
	return nil
}

// CreateExecution inserts a new execution record.
func (s *SQLiteStore) CreateExecution(ctx context.Context, exec Execution) (Execution, error) {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO cron_executions (
	execution_id, job_id, started_at, finished_at, status,
	run_id, output_summary, error_text, duration_ms
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		exec.ID,
		exec.JobID,
		nowString(exec.StartedAt),
		nullableTimeString(exec.FinishedAt),
		exec.Status,
		exec.RunID,
		exec.OutputSummary,
		exec.Error,
		exec.DurationMs,
	)
	if err != nil {
		return Execution{}, fmt.Errorf("insert execution: %w", err)
	}
	return exec, nil
}

// UpdateExecution updates an execution record.
func (s *SQLiteStore) UpdateExecution(ctx context.Context, exec Execution) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE cron_executions
SET finished_at = ?, status = ?, run_id = ?, output_summary = ?,
	error_text = ?, duration_ms = ?
WHERE execution_id = ?
`,
		nullableTimeString(exec.FinishedAt),
		exec.Status,
		exec.RunID,
		exec.OutputSummary,
		exec.Error,
		exec.DurationMs,
		exec.ID,
	)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}
	return nil
}

// ListExecutions returns executions for a job, ordered by started_at desc.
func (s *SQLiteStore) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT execution_id, job_id, started_at, finished_at, status,
	run_id, output_summary, error_text, duration_ms
FROM cron_executions
WHERE job_id = ?
ORDER BY started_at DESC
LIMIT ? OFFSET ?
`, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var execs []Execution
	for rows.Next() {
		var e Execution
		var startedText string
		var finishedText sql.NullString
		if err := rows.Scan(
			&e.ID, &e.JobID, &startedText, &finishedText,
			&e.Status, &e.RunID, &e.OutputSummary, &e.Error, &e.DurationMs,
		); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		e.StartedAt, _ = time.Parse(time.RFC3339Nano, startedText)
		if finishedText.Valid {
			e.FinishedAt, _ = time.Parse(time.RFC3339Nano, finishedText.String)
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

// ListActiveExecutions returns the nonterminal execution rows that need
// restart reconciliation before Scheduler can safely admit another scoped
// conversation run.
func (s *SQLiteStore) ListActiveExecutions(ctx context.Context) ([]Execution, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT execution_id, job_id, started_at, finished_at, status,
	run_id, output_summary, error_text, duration_ms
FROM cron_executions
WHERE status IN (?, ?, ?, ?)
ORDER BY started_at ASC, execution_id ASC
`, ExecStatusQueued, "pending", ExecStatusStarting, ExecStatusRunning)
	if err != nil {
		return nil, fmt.Errorf("list active executions: %w", err)
	}
	defer rows.Close()
	var execs []Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

func scanExecution(rows *sql.Rows) (Execution, error) {
	var e Execution
	var startedText string
	var finishedText sql.NullString
	if err := rows.Scan(
		&e.ID, &e.JobID, &startedText, &finishedText,
		&e.Status, &e.RunID, &e.OutputSummary, &e.Error, &e.DurationMs,
	); err != nil {
		return Execution{}, fmt.Errorf("scan execution: %w", err)
	}
	e.StartedAt, _ = time.Parse(time.RFC3339Nano, startedText)
	if finishedText.Valid {
		e.FinishedAt, _ = time.Parse(time.RFC3339Nano, finishedText.String)
	}
	return e, nil
}

// scanJob scans a single job row from QueryRow.
type jobScanner interface {
	Scan(dest ...any) error
}

func (s *SQLiteStore) scanJob(row jobScanner) (Job, error) {
	var job Job
	var nextRunText, createdText, updatedText string
	var lastRunText sql.NullString
	if err := row.Scan(
		&job.ID, &job.TenantID, &job.Name, &job.Schedule, &job.ExecType, &job.ExecConfig,
		&job.ConversationID, &job.AgentID,
		&job.Status, &job.TimeoutSec, &job.Tags, &nextRunText, &lastRunText,
		&createdText, &updatedText,
	); err != nil {
		return Job{}, err
	}
	job.NextRunAt, _ = time.Parse(time.RFC3339Nano, nextRunText)
	if lastRunText.Valid {
		job.LastRunAt, _ = time.Parse(time.RFC3339Nano, lastRunText.String)
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdText)
	job.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedText)
	return job, nil
}

// scanJobRow scans a single job from sql.Rows.
func (s *SQLiteStore) scanJobRow(rows *sql.Rows) (Job, error) {
	var job Job
	var nextRunText, createdText, updatedText string
	var lastRunText sql.NullString
	if err := rows.Scan(
		&job.ID, &job.TenantID, &job.Name, &job.Schedule, &job.ExecType, &job.ExecConfig,
		&job.ConversationID, &job.AgentID,
		&job.Status, &job.TimeoutSec, &job.Tags, &nextRunText, &lastRunText,
		&createdText, &updatedText,
	); err != nil {
		return Job{}, fmt.Errorf("scan job: %w", err)
	}
	job.NextRunAt, _ = time.Parse(time.RFC3339Nano, nextRunText)
	if lastRunText.Valid {
		job.LastRunAt, _ = time.Parse(time.RFC3339Nano, lastRunText.String)
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdText)
	job.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedText)
	return job, nil
}

func nowString(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func nullableTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return nowString(t)
}
