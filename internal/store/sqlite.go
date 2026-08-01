package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// schema defines the SQLite tables for run persistence.
const schema = `
CREATE TABLE IF NOT EXISTS runs (
	id               TEXT PRIMARY KEY,
	conversation_id  TEXT NOT NULL DEFAULT '',
	tenant_id        TEXT NOT NULL DEFAULT '',
	agent_id         TEXT NOT NULL DEFAULT '',
	model            TEXT NOT NULL DEFAULT '',
	provider_name    TEXT NOT NULL DEFAULT '',
	prompt           TEXT NOT NULL DEFAULT '',
	status           TEXT NOT NULL,
	output           TEXT NOT NULL DEFAULT '',
	error            TEXT NOT NULL DEFAULT '',
	usage_totals_json TEXT NOT NULL DEFAULT '',
	cost_totals_json  TEXT NOT NULL DEFAULT '',
	recap_json        TEXT NOT NULL DEFAULT '',
	created_at       TEXT NOT NULL,
	updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runs_conversation ON runs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_runs_tenant      ON runs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_runs_status      ON runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_created     ON runs(created_at);

CREATE TABLE IF NOT EXISTS conversation_run_owners (
	conversation_id TEXT PRIMARY KEY,
	tenant_id       TEXT NOT NULL,
	agent_id        TEXT NOT NULL,
	created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cron_run_starts (
	tenant_id           TEXT NOT NULL,
	idempotency_key     TEXT NOT NULL,
	fingerprint         TEXT NOT NULL,
	run_id              TEXT NOT NULL,
	accepted            INTEGER NOT NULL DEFAULT 0,
	dispatch_owner      TEXT NOT NULL DEFAULT '',
	dispatch_lease_until INTEGER NOT NULL DEFAULT 0,
	created_at          TEXT NOT NULL,
	PRIMARY KEY (tenant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_cron_run_starts_run ON cron_run_starts(run_id);

CREATE TABLE IF NOT EXISTS run_messages (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
	seq             INTEGER NOT NULL,
	role            TEXT NOT NULL,
	content         TEXT NOT NULL DEFAULT '',
	tool_calls_json TEXT,
	tool_call_id    TEXT NOT NULL DEFAULT '',
	name            TEXT NOT NULL DEFAULT '',
	is_meta         INTEGER NOT NULL DEFAULT 0,
	is_compact_summary INTEGER NOT NULL DEFAULT 0,
	UNIQUE(run_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_run_messages_run ON run_messages(run_id, seq);

CREATE TABLE IF NOT EXISTS run_events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id     TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
	seq        INTEGER NOT NULL,
	event_id   TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL,
	payload    TEXT NOT NULL DEFAULT '',
	timestamp  TEXT NOT NULL,
	UNIQUE(run_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_run_events_run ON run_events(run_id, seq);
CREATE INDEX IF NOT EXISTS idx_run_events_event_id ON run_events(event_id);
`

// SQLiteStore is a SQLite-backed implementation of Store.
type SQLiteStore struct {
	db *sql.DB
	// Test seams for deterministic interleavings. Production stores leave both nil.
	cronRunAcquireAfterUpdate func()
	migrationBeforeAddColumn  func(table, column string)
}

// NewSQLiteStore opens (or creates) a SQLite database at path.
// Call Migrate before using the store.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store: sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create sqlite directory: %w", err)
	}
	dsn := path + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}
	// Limit to a single writer connection to avoid SQLITE_BUSY under concurrent writes.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set busy timeout: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: set foreign keys: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Migrate creates the schema tables.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	if err := s.backfillConversationRunOwners(ctx); err != nil {
		return err
	}
	if !s.columnExists(ctx, "runs", "recap_json") {
		if s.migrationBeforeAddColumn != nil {
			s.migrationBeforeAddColumn("runs", "recap_json")
		}
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE runs ADD COLUMN recap_json TEXT NOT NULL DEFAULT ''`); err != nil {
			if !isDuplicateColumnError(err) || !s.columnExists(ctx, "runs", "recap_json") {
				return fmt.Errorf("store: migrate add recap_json: %w", err)
			}
		}
	}
	if !s.columnExists(ctx, "cron_run_starts", "dispatch_owner") {
		if s.migrationBeforeAddColumn != nil {
			s.migrationBeforeAddColumn("cron_run_starts", "dispatch_owner")
		}
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE cron_run_starts ADD COLUMN dispatch_owner TEXT NOT NULL DEFAULT ''`); err != nil {
			if !isDuplicateColumnError(err) || !s.columnExists(ctx, "cron_run_starts", "dispatch_owner") {
				return fmt.Errorf("store: migrate add cron dispatch owner: %w", err)
			}
		}
	}
	if !s.columnExists(ctx, "cron_run_starts", "dispatch_lease_until") {
		if s.migrationBeforeAddColumn != nil {
			s.migrationBeforeAddColumn("cron_run_starts", "dispatch_lease_until")
		}
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE cron_run_starts ADD COLUMN dispatch_lease_until INTEGER NOT NULL DEFAULT 0`); err != nil {
			if !isDuplicateColumnError(err) || !s.columnExists(ctx, "cron_run_starts", "dispatch_lease_until") {
				return fmt.Errorf("store: migrate add cron dispatch lease expiry: %w", err)
			}
		}
	}
	return nil
}

// backfillConversationRunOwners upgrades run databases created before durable
// conversation ownership existed. The immediate transaction serializes the
// historical scan, conflict check, and insert with new CreateRun claims.
func (s *SQLiteStore) backfillConversationRunOwners(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin conversation owner backfill: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const normalizedOwners = `
SELECT DISTINCT
	conversation_id,
	CASE WHEN TRIM(tenant_id) = '' THEN 'default' ELSE TRIM(tenant_id) END AS tenant_id,
	CASE WHEN TRIM(agent_id) = '' THEN 'default' ELSE TRIM(agent_id) END AS agent_id
FROM runs
WHERE conversation_id <> ''`

	var conflictingConversation string
	err = tx.QueryRowContext(ctx, `
WITH normalized AS (`+normalizedOwners+`)
SELECT conversation_id
FROM normalized
GROUP BY conversation_id
HAVING COUNT(*) > 1
ORDER BY conversation_id
LIMIT 1
`).Scan(&conflictingConversation)
	if err == nil {
		return fmt.Errorf("store: conversation %q has conflicting historical owners", conflictingConversation)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: inspect historical conversation owners: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
WITH historical AS (`+normalizedOwners+`)
SELECT historical.conversation_id
FROM historical
JOIN conversation_run_owners AS persisted
	ON persisted.conversation_id = historical.conversation_id
WHERE persisted.tenant_id <> historical.tenant_id
	OR persisted.agent_id <> historical.agent_id
ORDER BY historical.conversation_id
LIMIT 1
`).Scan(&conflictingConversation)
	if err == nil {
		return fmt.Errorf("store: conversation %q owner conflicts with historical runs", conflictingConversation)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: compare historical conversation owners: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO conversation_run_owners (
	conversation_id, tenant_id, agent_id, created_at
)
SELECT
	conversation_id,
	CASE WHEN TRIM(tenant_id) = '' THEN 'default' ELSE TRIM(tenant_id) END,
	CASE WHEN TRIM(agent_id) = '' THEN 'default' ELSE TRIM(agent_id) END,
	MIN(created_at)
FROM runs
WHERE conversation_id <> ''
GROUP BY
	conversation_id,
	CASE WHEN TRIM(tenant_id) = '' THEN 'default' ELSE TRIM(tenant_id) END,
	CASE WHEN TRIM(agent_id) = '' THEN 'default' ELSE TRIM(agent_id) END
`); err != nil {
		return fmt.Errorf("store: backfill conversation owners: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit conversation owner backfill: %w", err)
	}
	return nil
}

func (s *SQLiteStore) columnExists(ctx context.Context, table, column string) bool {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// CreateRun persists a new run record.
func (s *SQLiteStore) CreateRun(ctx context.Context, run *Run) error {
	if run.ID == "" {
		return fmt.Errorf("store: run ID is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin create run: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if run.ConversationID != "" {
		tenantID, agentID := normalizeConversationOwner(run.TenantID, run.AgentID)
		if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO conversation_run_owners (
	conversation_id, tenant_id, agent_id, created_at
) VALUES (?, ?, ?, ?)
`, run.ConversationID, tenantID, agentID, timeString(run.CreatedAt)); err != nil {
			return fmt.Errorf("store: claim conversation owner: %w", err)
		}
		var persistedTenant, persistedAgent string
		if err := tx.QueryRowContext(ctx, `
SELECT tenant_id, agent_id
FROM conversation_run_owners
WHERE conversation_id = ?
`, run.ConversationID).Scan(&persistedTenant, &persistedAgent); err != nil {
			return fmt.Errorf("store: read conversation owner: %w", err)
		}
		if persistedTenant != tenantID || persistedAgent != agentID {
			return fmt.Errorf("store: conversation %q: %w", run.ConversationID, ErrConversationOwnerConflict)
		}
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO runs (id, conversation_id, tenant_id, agent_id, model, provider_name, prompt,
                  status, output, error, usage_totals_json, cost_totals_json, recap_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.ID,
		run.ConversationID,
		run.TenantID,
		run.AgentID,
		run.Model,
		run.ProviderName,
		run.Prompt,
		string(run.Status),
		run.Output,
		run.Error,
		run.UsageTotalsJSON,
		run.CostTotalsJSON,
		workflowRecapJSON(run.Recap),
		timeString(run.CreatedAt),
		timeString(run.UpdatedAt),
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return fmt.Errorf("store: run %q already exists", run.ID)
		}
		return fmt.Errorf("store: create run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit create run: %w", err)
	}
	return nil
}

// ClaimCronRunStart atomically reserves a tenant-scoped idempotency key.
func (s *SQLiteStore) ClaimCronRunStart(ctx context.Context, start CronRunStart) (CronRunStart, bool, error) {
	if start.TenantID == "" || start.IdempotencyKey == "" || start.Fingerprint == "" || start.RunID == "" {
		return CronRunStart{}, false, fmt.Errorf("store: complete cron run start binding is required")
	}
	if start.CreatedAt.IsZero() {
		start.CreatedAt = time.Now().UTC()
	}
	accepted := 0
	if start.Accepted {
		accepted = 1
	}
	result, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO cron_run_starts (
	tenant_id, idempotency_key, fingerprint, run_id, accepted, created_at
) VALUES (?, ?, ?, ?, ?, ?)
`, start.TenantID, start.IdempotencyKey, start.Fingerprint, start.RunID, accepted, timeString(start.CreatedAt))
	if err != nil {
		return CronRunStart{}, false, fmt.Errorf("store: claim cron run start: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return CronRunStart{}, false, fmt.Errorf("store: claim cron run start rows affected: %w", err)
	}
	persisted, err := s.getCronRunStart(ctx, start.TenantID, start.IdempotencyKey)
	if err != nil {
		return CronRunStart{}, false, err
	}
	return persisted, rows == 1, nil
}

func (s *SQLiteStore) getCronRunStart(ctx context.Context, tenantID, idempotencyKey string) (CronRunStart, error) {
	return scanCronRunStart(s.db.QueryRowContext(ctx, `
SELECT tenant_id, idempotency_key, fingerprint, run_id, accepted, dispatch_owner, dispatch_lease_until, created_at
FROM cron_run_starts
WHERE tenant_id = ? AND idempotency_key = ?
`, tenantID, idempotencyKey))
}

type cronRunStartScanner interface {
	Scan(dest ...any) error
}

func scanCronRunStart(scanner cronRunStartScanner) (CronRunStart, error) {
	var start CronRunStart
	var accepted int
	var dispatchLeaseUntil int64
	var createdAt string
	err := scanner.Scan(
		&start.TenantID,
		&start.IdempotencyKey,
		&start.Fingerprint,
		&start.RunID,
		&accepted,
		&start.DispatchOwner,
		&dispatchLeaseUntil,
		&createdAt,
	)
	if err != nil {
		return CronRunStart{}, fmt.Errorf("store: get cron run start: %w", err)
	}
	start.Accepted = accepted != 0
	if dispatchLeaseUntil > 0 {
		start.DispatchLeaseUntil = time.Unix(0, dispatchLeaseUntil).UTC()
	}
	if parsed, parseErr := time.Parse(time.RFC3339Nano, createdAt); parseErr == nil {
		start.CreatedAt = parsed
	}
	return start, nil
}

// AcquireCronRunStartDispatchLease atomically claims or renews a dispatch lease.
func (s *SQLiteStore) AcquireCronRunStartDispatchLease(ctx context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (CronRunStart, bool, error) {
	if tenantID == "" || idempotencyKey == "" || owner == "" || !leaseUntil.After(now) {
		return CronRunStart{}, false, fmt.Errorf("store: valid cron run dispatch lease is required")
	}
	leaseDuration := leaseUntil.Sub(now)
	persisted, err := scanCronRunStart(s.db.QueryRowContext(ctx, `
WITH lease_clock(now_ns) AS (
	SELECT CAST((julianday('now') - 2440587.5) * 86400000000000 AS INTEGER)
)
UPDATE cron_run_starts
SET dispatch_owner = ?,
	dispatch_lease_until = (SELECT now_ns FROM lease_clock) + ?
WHERE tenant_id = ? AND idempotency_key = ?
  AND (dispatch_owner = '' OR dispatch_owner = ? OR dispatch_lease_until <= (SELECT now_ns FROM lease_clock))
RETURNING tenant_id, idempotency_key, fingerprint, run_id, accepted, dispatch_owner, dispatch_lease_until, created_at
`, owner, leaseDuration.Nanoseconds(), tenantID, idempotencyKey, owner))
	if err == nil {
		if s.cronRunAcquireAfterUpdate != nil {
			s.cronRunAcquireAfterUpdate()
		}
		return persisted, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CronRunStart{}, false, fmt.Errorf("store: acquire cron run dispatch lease: %w", err)
	}
	persisted, err = s.getCronRunStart(ctx, tenantID, idempotencyKey)
	if err != nil {
		return CronRunStart{}, false, err
	}
	return persisted, false, nil
}

// RenewCronRunStartDispatchLease extends only a live lease held by owner.
func (s *SQLiteStore) RenewCronRunStartDispatchLease(ctx context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (CronRunStart, bool, error) {
	if tenantID == "" || idempotencyKey == "" || owner == "" || !leaseUntil.After(now) {
		return CronRunStart{}, false, fmt.Errorf("store: valid cron run dispatch lease is required")
	}
	leaseDuration := leaseUntil.Sub(now)
	persisted, err := scanCronRunStart(s.db.QueryRowContext(ctx, `
WITH lease_clock(now_ns) AS (
	SELECT CAST((julianday('now') - 2440587.5) * 86400000000000 AS INTEGER)
)
UPDATE cron_run_starts
SET dispatch_lease_until = (SELECT now_ns FROM lease_clock) + ?
WHERE tenant_id = ? AND idempotency_key = ?
  AND dispatch_owner = ?
  AND dispatch_lease_until > (SELECT now_ns FROM lease_clock)
RETURNING tenant_id, idempotency_key, fingerprint, run_id, accepted, dispatch_owner, dispatch_lease_until, created_at
`, leaseDuration.Nanoseconds(), tenantID, idempotencyKey, owner))
	if err == nil {
		return persisted, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CronRunStart{}, false, fmt.Errorf("store: renew cron run dispatch lease: %w", err)
	}
	persisted, err = s.getCronRunStart(ctx, tenantID, idempotencyKey)
	if err != nil {
		return CronRunStart{}, false, err
	}
	return persisted, false, nil
}

// MarkCronRunStartAccepted records that the current lease owner dispatched the reserved run.
func (s *SQLiteStore) MarkCronRunStartAccepted(ctx context.Context, tenantID, idempotencyKey, owner string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE cron_run_starts
SET accepted = 1
WHERE tenant_id = ? AND idempotency_key = ? AND dispatch_owner = ?
`, tenantID, idempotencyKey, owner)
	if err != nil {
		return fmt.Errorf("store: mark cron run start accepted: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: mark cron run start accepted rows affected: %w", err)
	}
	if rows != 1 {
		return ErrCronRunDispatchLeaseLost
	}
	return nil
}

// UpdateRun overwrites an existing run record.
func (s *SQLiteStore) UpdateRun(ctx context.Context, run *Run) error {
	if run.ID == "" {
		return fmt.Errorf("store: run ID is required")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE runs
SET conversation_id  = ?,
    tenant_id        = ?,
    agent_id         = ?,
    model            = ?,
    provider_name    = ?,
    prompt           = ?,
    status           = ?,
    output           = ?,
    error            = ?,
    usage_totals_json = ?,
    cost_totals_json  = ?,
    recap_json       = ?,
    updated_at       = ?
WHERE id = ?
`,
		run.ConversationID,
		run.TenantID,
		run.AgentID,
		run.Model,
		run.ProviderName,
		run.Prompt,
		string(run.Status),
		run.Output,
		run.Error,
		run.UsageTotalsJSON,
		run.CostTotalsJSON,
		workflowRecapJSON(run.Recap),
		timeString(run.UpdatedAt),
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update run: %w", err)
	}
	return nil
}

// GetRun retrieves a run by ID.
func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, conversation_id, tenant_id, agent_id, model, provider_name, prompt,
       status, output, error, usage_totals_json, cost_totals_json, recap_json, created_at, updated_at
FROM runs
WHERE id = ?
`, id)
	run, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, &NotFoundError{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("store: get run %q: %w", id, err)
	}
	return run, nil
}

// ListRuns returns runs matching filter, ordered by created_at DESC.
func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]*Run, error) {
	query := `SELECT id, conversation_id, tenant_id, agent_id, model, provider_name, prompt,
	                  status, output, error, usage_totals_json, cost_totals_json, recap_json, created_at, updated_at
	           FROM runs`
	args := make([]any, 0, 3)
	conditions := make([]string, 0, 3)

	if filter.ConversationID != "" {
		conditions = append(conditions, "conversation_id = ?")
		args = append(args, filter.ConversationID)
	}
	if filter.TenantID != "" {
		conditions = append(conditions, "tenant_id = ?")
		args = append(args, filter.TenantID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list runs rows: %w", err)
	}
	if runs == nil {
		runs = []*Run{}
	}
	return runs, nil
}

// AppendMessage appends a message to a run's message log.
func (s *SQLiteStore) AppendMessage(ctx context.Context, msg *Message) error {
	isMeta := 0
	if msg.IsMeta {
		isMeta = 1
	}
	isCompact := 0
	if msg.IsCompactSummary {
		isCompact = 1
	}
	var toolCallsJSON *string
	if msg.ToolCallsJSON != "" {
		toolCallsJSON = &msg.ToolCallsJSON
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO run_messages (run_id, seq, role, content, tool_calls_json, tool_call_id, name, is_meta, is_compact_summary)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		msg.RunID, msg.Seq, msg.Role, msg.Content,
		toolCallsJSON, msg.ToolCallID, msg.Name,
		isMeta, isCompact,
	)
	if err != nil {
		return fmt.Errorf("store: append message: %w", err)
	}
	return nil
}

// GetMessages returns all messages for a run, ordered by seq ASC.
func (s *SQLiteStore) GetMessages(ctx context.Context, runID string) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT seq, run_id, role, content, tool_calls_json, tool_call_id, name, is_meta, is_compact_summary
FROM run_messages
WHERE run_id = ?
ORDER BY seq ASC
`, runID)
	if err != nil {
		return nil, fmt.Errorf("store: get messages: %w", err)
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		msg := &Message{}
		var toolCallsJSON sql.NullString
		var isMeta, isCompact int
		if err := rows.Scan(
			&msg.Seq, &msg.RunID, &msg.Role, &msg.Content,
			&toolCallsJSON, &msg.ToolCallID, &msg.Name,
			&isMeta, &isCompact,
		); err != nil {
			return nil, fmt.Errorf("store: scan message: %w", err)
		}
		msg.IsMeta = isMeta == 1
		msg.IsCompactSummary = isCompact == 1
		if toolCallsJSON.Valid {
			msg.ToolCallsJSON = toolCallsJSON.String
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: messages rows: %w", err)
	}
	if msgs == nil {
		msgs = []*Message{}
	}
	return msgs, nil
}

// AppendEvent appends an event to a run's event log.
func (s *SQLiteStore) AppendEvent(ctx context.Context, event *Event) error {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO run_events (run_id, seq, event_id, event_type, payload, timestamp)
VALUES (?, ?, ?, ?, ?, ?)
`,
		event.RunID, event.Seq, event.EventID, event.EventType, event.Payload,
		timeString(event.Timestamp),
	)
	if err != nil {
		return fmt.Errorf("store: append event: %w", err)
	}
	if cursor, cursorErr := result.LastInsertId(); cursorErr == nil {
		event.Cursor = cursor
	}
	return nil
}

// GetEvents returns events for a run with seq > afterSeq, ordered by seq ASC.
// Pass afterSeq=-1 to get all events.
func (s *SQLiteStore) GetEvents(ctx context.Context, runID string, afterSeq int) ([]*Event, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, seq, run_id, event_id, event_type, payload, timestamp
FROM run_events
WHERE run_id = ? AND seq > ?
ORDER BY seq ASC
`, runID, afterSeq)
	if err != nil {
		return nil, fmt.Errorf("store: get events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		e := &Event{}
		var tsText string
		if err := rows.Scan(&e.Cursor, &e.Seq, &e.RunID, &e.EventID, &e.EventType, &e.Payload, &tsText); err != nil {
			return nil, fmt.Errorf("store: scan event: %w", err)
		}
		if t, err := time.Parse(time.RFC3339Nano, tsText); err == nil {
			e.Timestamp = t
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: events rows: %w", err)
	}
	if events == nil {
		events = []*Event{}
	}
	return events, nil
}

// GetConversationEvents returns events across every run on one conversation,
// ordered by the SQLite row id that was assigned at append time. The public
// EventID remains unchanged and is resolved exactly to that durable cursor.
func (s *SQLiteStore) GetConversationEvents(
	ctx context.Context, filter ConversationEventFilter,
) (ConversationEventPage, error) {
	page := ConversationEventPage{CursorFound: filter.AfterEventID == ""}
	scope := "r.conversation_id = ?"
	scopeArgs := []any{filter.ConversationID}
	if filter.TenantID != "" {
		scope += " AND r.tenant_id = ?"
		scopeArgs = append(scopeArgs, filter.TenantID)
	}

	var afterCursor int64
	if filter.AfterEventID != "" {
		cursorQuery := `
SELECT re.id
FROM run_events re
JOIN runs r ON r.id = re.run_id
WHERE ` + scope + ` AND re.event_id = ?
ORDER BY re.id DESC
LIMIT 1`
		args := append(append([]any(nil), scopeArgs...), filter.AfterEventID)
		err := s.db.QueryRowContext(ctx, cursorQuery, args...).Scan(&afterCursor)
		switch {
		case err == nil:
			page.CursorFound = true
		case errors.Is(err, sql.ErrNoRows):
			page.CursorFound = false
		case err != nil:
			return ConversationEventPage{}, fmt.Errorf("store: resolve conversation event cursor: %w", err)
		}
	}

	query := `
SELECT re.id, re.seq, re.run_id, re.event_id, re.event_type, re.payload, re.timestamp
FROM run_events re
JOIN runs r ON r.id = re.run_id
WHERE ` + scope
	args := append([]any(nil), scopeArgs...)
	if filter.AfterEventID != "" && page.CursorFound {
		query += " AND re.id > ?"
		args = append(args, afterCursor)
	}
	query += " ORDER BY re.id ASC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit+1)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ConversationEventPage{}, fmt.Errorf("store: get conversation events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		event := &Event{}
		var tsText string
		if err := rows.Scan(
			&event.Cursor,
			&event.Seq,
			&event.RunID,
			&event.EventID,
			&event.EventType,
			&event.Payload,
			&tsText,
		); err != nil {
			return ConversationEventPage{}, fmt.Errorf("store: scan conversation event: %w", err)
		}
		if timestamp, parseErr := time.Parse(time.RFC3339Nano, tsText); parseErr == nil {
			event.Timestamp = timestamp
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return ConversationEventPage{}, fmt.Errorf("store: conversation events rows: %w", err)
	}
	if filter.Limit > 0 && len(page.Events) > filter.Limit {
		page.Truncated = true
		page.Events = page.Events[:filter.Limit]
	}
	if page.Events == nil {
		page.Events = []*Event{}
	}
	return page, nil
}

// rowScanner abstracts *sql.Row and *sql.Rows for shared scanning.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (*Run, error) {
	run := &Run{}
	var createdText, updatedText, recapJSON string
	err := row.Scan(
		&run.ID,
		&run.ConversationID,
		&run.TenantID,
		&run.AgentID,
		&run.Model,
		&run.ProviderName,
		&run.Prompt,
		&run.Status,
		&run.Output,
		&run.Error,
		&run.UsageTotalsJSON,
		&run.CostTotalsJSON,
		&recapJSON,
		&createdText,
		&updatedText,
	)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339Nano, createdText); err == nil {
		run.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedText); err == nil {
		run.UpdatedAt = t
	}
	run.Recap = workflowRecapFromJSON(recapJSON)
	return run, nil
}

func workflowRecapJSON(recap *WorkflowRecap) string {
	if recap == nil {
		return ""
	}
	data, err := json.Marshal(recap)
	if err != nil {
		return ""
	}
	return string(data)
}

func workflowRecapFromJSON(raw string) *WorkflowRecap {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var recap WorkflowRecap
	if err := json.Unmarshal([]byte(raw), &recap); err != nil {
		return nil
	}
	return &recap
}

func timeString(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// isDuplicateKeyError returns true if err is a SQLite UNIQUE constraint violation.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed")
}

func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
