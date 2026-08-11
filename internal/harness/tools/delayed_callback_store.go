package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// CallbackStore is the durable callback state-machine boundary. Every
// dispatch transition is token-fenced so a stale timer or recovered process
// cannot commit over the current lease owner.
type CallbackStore interface {
	Migrate(context.Context) error
	Create(context.Context, CallbackInfo) error
	Get(context.Context, string) (CallbackInfo, error)
	Update(context.Context, CallbackInfo) error
	ListPending(context.Context) ([]CallbackInfo, error)
	ListAll(context.Context) ([]CallbackInfo, error)
	ClaimDue(context.Context, string, string, time.Time, time.Time) (CallbackInfo, bool, error)
	ReclaimExpired(context.Context, string, string, string, time.Time, time.Time) (CallbackInfo, bool, error)
	ExtendLease(context.Context, string, string, time.Time, time.Time) (bool, error)
	// ReleaseLease is the token-fenced live-owner handoff.  A manager calls it
	// only after its canceled StartCallback has returned, making the next retry
	// claimable without allowing a concurrent expired-lease takeover.
	ReleaseLease(context.Context, string, string, time.Time, string) error
	// RecoverExpiredLease is a startup-only crash-recovery transition.  The
	// caller must have established that the former process is gone; it must not
	// be used by a normally armed competing manager to take over a live owner.
	RecoverExpiredLease(context.Context, string, string, time.Time) (CallbackInfo, bool, error)
	MarkStarted(context.Context, string, string, string) error
	MarkRetry(context.Context, string, string, time.Time, string) error
	MarkFailed(context.Context, string, string, string) error
	CancelPending(context.Context, string) (CallbackInfo, error)
	Close() error
}
type SQLiteCallbackStore struct {
	db *sql.DB

	// recoveryLock is a process-lifetime workspace fence.  A dispatch lease
	// tells us only that a callback heartbeat expired; it cannot prove the
	// daemon that owns that heartbeat died.  The sidecar flock is released by
	// the kernel on process loss, so only its holder may reclaim abandoned
	// dispatching rows during Recover.
	recoveryMu       sync.Mutex
	recoveryLockPath string
	recoveryLock     *os.File
}

func NewSQLiteCallbackStore(path string) (*SQLiteCallbackStore, error) {
	if path == "" {
		return nil, fmt.Errorf("callback sqlite path is required")
	}
	dsn, filesystemPath, err := callbackSQLiteLocation(path)
	if err != nil {
		return nil, err
	}
	if filesystemPath != "" {
		if e := os.MkdirAll(filepath.Dir(filesystemPath), 0755); e != nil {
			return nil, e
		}
	}
	// modernc's _pragma query parameters are applied while every physical
	// connection is opened. Executing PRAGMA once on sql.DB only configured the
	// first pooled connection, which let a competing manager receive immediate
	// SQLITE_BUSY on another connection.
	db, e := sql.Open("sqlite", dsn)
	if e != nil {
		return nil, e
	}
	lockPath := ""
	if filesystemPath != "" {
		lockPath = filesystemPath + ".recovery.lock"
	}
	return &SQLiteCallbackStore{db: db, recoveryLockPath: lockPath}, nil
}

func callbackSQLiteLocation(path string) (dsn, filesystemPath string, err error) {
	if path == ":memory:" {
		// Keep SQLite's literal in-memory sentinel intact. Encoding it as a URL
		// path turns it into a physical `:memory:` filename on modernc/sqlite.
		query := url.Values{}
		query.Add("_pragma", "busy_timeout(5000)")
		query.Add("_pragma", "journal_mode(WAL)")
		return ":memory:?" + query.Encode(), "", nil
	}
	// Treat ordinary input as a filesystem path, not an already-escaped URI, so
	// characters such as '?' name the intended workspace database. File URIs
	// retain their caller-supplied location/query semantics and receive the same
	// per-connection pragma values.
	var uri *url.URL
	if strings.HasPrefix(path, "file:") {
		uri, err = url.Parse(path)
		if err != nil {
			return "", "", fmt.Errorf("callback sqlite URI: %w", err)
		}
		filesystemPath = uri.Path
	}
	if uri == nil {
		filesystemPath, err = filepath.Abs(path)
		if err != nil {
			return "", "", fmt.Errorf("absolute callback sqlite path: %w", err)
		}
		uri = &url.URL{Scheme: "file", Path: filesystemPath}
	}
	return callbackSQLiteDSN(uri), filesystemPath, nil
}

func callbackSQLiteDSN(uri *url.URL) string {
	query := uri.Query()
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "journal_mode(WAL)")
	uri.RawQuery = query.Encode()
	return uri.String()
}
func (s *SQLiteCallbackStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.releaseCallbackRecoveryAuthority()
	return s.db.Close()
}

// AcquireCallbackRecoveryAuthority obtains the process-loss fence required
// before Recover can reclaim an expired dispatching row.  TCP listener
// ownership is not enough: another harnessd can use a different port while
// sharing this workspace.  flock is automatically released if this process
// dies, which is the authority wall-clock callback leases lack.
func (s *SQLiteCallbackStore) AcquireCallbackRecoveryAuthority(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.recoveryLockPath == "" {
		return nil, fmt.Errorf("%w: callback SQLite location is not filesystem-backed", ErrCallbackRecoveryAuthorityRequired)
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if s.recoveryLock != nil {
		return nil, fmt.Errorf("callback recovery authority is already held")
	}
	file, err := os.OpenFile(s.recoveryLockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open callback recovery lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("callback workspace is already owned: %w", err)
	}
	s.recoveryLock = file
	var once sync.Once
	return func() {
		once.Do(func() { s.releaseCallbackRecoveryAuthority() })
	}, nil
}

func (s *SQLiteCallbackStore) releaseCallbackRecoveryAuthority() {
	if s == nil {
		return
	}
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if s.recoveryLock == nil {
		return
	}
	_ = syscall.Flock(int(s.recoveryLock.Fd()), syscall.LOCK_UN)
	_ = s.recoveryLock.Close()
	s.recoveryLock = nil
}
func (s *SQLiteCallbackStore) Migrate(c context.Context) error {
	_, e := s.db.ExecContext(c, `CREATE TABLE IF NOT EXISTS delayed_callbacks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT '', agent_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL, prompt TEXT NOT NULL, model TEXT NOT NULL DEFAULT '', provider_name TEXT NOT NULL DEFAULT '', allow_fallback INTEGER NOT NULL DEFAULT 0, fallback_providers TEXT NOT NULL DEFAULT '[]', delay TEXT NOT NULL, fires_at TIMESTAMP NOT NULL, state TEXT NOT NULL, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, attempt INTEGER NOT NULL DEFAULT 0, run_id TEXT NOT NULL DEFAULT '', next_attempt_at TIMESTAMP, last_error TEXT NOT NULL DEFAULT '', dispatch_token TEXT NOT NULL DEFAULT '', dispatch_lease_until TIMESTAMP); CREATE INDEX IF NOT EXISTS idx_delayed_callbacks_pending ON delayed_callbacks(state,fires_at);`)
	if e != nil {
		return e
	}
	for _, q := range []string{"ALTER TABLE delayed_callbacks ADD COLUMN next_attempt_at TIMESTAMP", "ALTER TABLE delayed_callbacks ADD COLUMN last_error TEXT NOT NULL DEFAULT ''", "ALTER TABLE delayed_callbacks ADD COLUMN dispatch_token TEXT NOT NULL DEFAULT ''", "ALTER TABLE delayed_callbacks ADD COLUMN dispatch_lease_until TIMESTAMP", "ALTER TABLE delayed_callbacks ADD COLUMN model TEXT NOT NULL DEFAULT ''", "ALTER TABLE delayed_callbacks ADD COLUMN provider_name TEXT NOT NULL DEFAULT ''", "ALTER TABLE delayed_callbacks ADD COLUMN allow_fallback INTEGER NOT NULL DEFAULT 0", "ALTER TABLE delayed_callbacks ADD COLUMN fallback_providers TEXT NOT NULL DEFAULT '[]'"} {
		_, err := s.db.ExecContext(c, q)
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	if _, e = s.db.ExecContext(c, `UPDATE delayed_callbacks SET run_id='run_callback_' || id WHERE run_id=''`); e != nil {
		return e
	}
	return s.normalizeTimes(c)
}

func (s *SQLiteCallbackStore) normalizeTimes(c context.Context) error {
	rows, err := s.db.QueryContext(c, `SELECT id,fires_at,created_at,updated_at,next_attempt_at,dispatch_lease_until FROM delayed_callbacks`)
	if err != nil {
		return err
	}
	type storedTimes struct {
		id                            string
		firesAt, createdAt, updatedAt time.Time
		nextAttempt, lease            *time.Time
	}
	var all []storedTimes
	for rows.Next() {
		var item storedTimes
		var firesAt, createdAt, updatedAt, nextAttempt, lease any
		if err := rows.Scan(&item.id, &firesAt, &createdAt, &updatedAt, &nextAttempt, &lease); err != nil {
			rows.Close()
			return err
		}
		if item.firesAt, err = parseStoredCallbackTime(firesAt); err != nil {
			rows.Close()
			return fmt.Errorf("callback %s fires_at: %w", item.id, err)
		}
		if item.createdAt, err = parseStoredCallbackTime(createdAt); err != nil {
			rows.Close()
			return fmt.Errorf("callback %s created_at: %w", item.id, err)
		}
		if item.updatedAt, err = parseStoredCallbackTime(updatedAt); err != nil {
			rows.Close()
			return fmt.Errorf("callback %s updated_at: %w", item.id, err)
		}
		if item.nextAttempt, err = parseOptionalStoredCallbackTime(nextAttempt); err != nil {
			rows.Close()
			return fmt.Errorf("callback %s next_attempt_at: %w", item.id, err)
		}
		if item.lease, err = parseOptionalStoredCallbackTime(lease); err != nil {
			rows.Close()
			return fmt.Errorf("callback %s dispatch_lease_until: %w", item.id, err)
		}
		all = append(all, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range all {
		var next, lease any
		if item.nextAttempt != nil {
			next = item.nextAttempt.UTC()
		}
		if item.lease != nil {
			lease = item.lease.UTC()
		}
		if _, err := s.db.ExecContext(c, `UPDATE delayed_callbacks SET fires_at=?,created_at=?,updated_at=?,next_attempt_at=?,dispatch_lease_until=? WHERE id=?`, item.firesAt.UTC(), item.createdAt.UTC(), item.updatedAt.UTC(), next, lease, item.id); err != nil {
			return err
		}
	}
	return nil
}

func parseOptionalStoredCallbackTime(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := parseStoredCallbackTime(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseStoredCallbackTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed, nil
	case string:
		return parseStoredCallbackTimeText(typed)
	case []byte:
		return parseStoredCallbackTimeText(string(typed))
	case nil:
		return time.Time{}, fmt.Errorf("timestamp is NULL")
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

func parseStoredCallbackTimeText(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if monotonic := strings.Index(value, " m="); monotonic >= 0 {
		value = strings.TrimSpace(value[:monotonic])
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, strings.TrimSuffix(value, "Z")); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}
func (s *SQLiteCallbackStore) Create(c context.Context, i CallbackInfo) error {
	i.LastError = SafeCallbackErrorSummary(i.LastError)
	if i.UpdatedAt.IsZero() {
		i.UpdatedAt = i.CreatedAt
	}
	fallbackProviders, err := json.Marshal(i.FallbackProviders)
	if err != nil {
		return fmt.Errorf("marshal callback fallback providers: %w", err)
	}
	_, e := s.db.ExecContext(c, `INSERT INTO delayed_callbacks(id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,attempt,run_id,next_attempt_at,last_error)VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, i.ID, i.TenantID, i.AgentID, i.ConversationID, i.Prompt, i.Model, i.ProviderName, i.AllowFallback, string(fallbackProviders), i.Delay, i.FiresAt.UTC(), i.State, i.CreatedAt.UTC(), i.UpdatedAt.UTC(), i.Attempt, i.RunID, nullableCallbackTime(i.NextAttemptAt), i.LastError)
	return e
}

func nullableCallbackTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

// ClaimDue atomically fences one due pending/retry row for one dispatcher.
func (s *SQLiteCallbackStore) ClaimDue(c context.Context, id, token string, now, until time.Time) (CallbackInfo, bool, error) {
	return s.claimReturning(c, id, token, `UPDATE delayed_callbacks SET state='dispatching_fenced',next_attempt_at=NULL,dispatch_token=?,dispatch_lease_until=?,attempt=attempt+1,updated_at=? WHERE id=? AND ((state='pending' AND fires_at<=?) OR (state='retry_wait' AND next_attempt_at<=?)) RETURNING id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,run_id,attempt,next_attempt_at,last_error,dispatch_token,dispatch_lease_until`, token, until.UTC(), now.UTC(), id, now.UTC(), now.UTC())
}
func (s *SQLiteCallbackStore) ReclaimExpired(c context.Context, id, expectedToken, token string, now, until time.Time) (CallbackInfo, bool, error) {
	return s.claimReturning(c, id, token, `UPDATE delayed_callbacks SET dispatch_token=?,dispatch_lease_until=?,attempt=attempt+1,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=? AND (dispatch_lease_until IS NULL OR dispatch_lease_until<=?) RETURNING id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,run_id,attempt,next_attempt_at,last_error,dispatch_token,dispatch_lease_until`, token, until.UTC(), now.UTC(), id, expectedToken, now.UTC())
}

// claimReturning makes claiming and reading the owner one SQLite statement.
// A caller only owns a dispatch when the row returned by its UPDATE carries
// its exact private token; a later manager cannot race a separate Get between
// those two observations.
func (s *SQLiteCallbackStore) claimReturning(c context.Context, id, token, query string, args ...any) (CallbackInfo, bool, error) {
	got, err := scanCallback(s.db.QueryRowContext(c, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		current, getErr := s.Get(c, id)
		return current, false, getErr
	}
	if err != nil {
		return CallbackInfo{}, false, err
	}
	if got.DispatchToken != token {
		return CallbackInfo{}, false, fmt.Errorf("callback claim returned an unverified owner")
	}
	return got, true, nil
}

func (s *SQLiteCallbackStore) ExtendLease(c context.Context, id, token string, now, until time.Time) (bool, error) {
	r, err := s.db.ExecContext(c, `UPDATE delayed_callbacks SET dispatch_lease_until=?,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=? AND dispatch_lease_until>?`, until.UTC(), now.UTC(), id, token, now.UTC())
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *SQLiteCallbackStore) ReleaseLease(c context.Context, id, token string, next time.Time, summary string) error {
	summary = SafeCallbackErrorSummary(summary)
	r, err := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state='retry_wait',next_attempt_at=?,last_error=?,dispatch_token='',dispatch_lease_until=NULL,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=?`, next.UTC(), summary, time.Now().UTC(), id, token)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("callback %s dispatch lease lost", id)
	}
	return nil
}

func (s *SQLiteCallbackStore) RecoverExpiredLease(c context.Context, id, expectedToken string, now time.Time) (CallbackInfo, bool, error) {
	got, err := scanCallback(s.db.QueryRowContext(c, `UPDATE delayed_callbacks SET state='retry_wait',next_attempt_at=?,dispatch_token='',dispatch_lease_until=NULL,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=? AND (dispatch_lease_until IS NULL OR dispatch_lease_until<=?) RETURNING id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,run_id,attempt,next_attempt_at,last_error,dispatch_token,dispatch_lease_until`, now.UTC(), now.UTC(), id, expectedToken, now.UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		current, getErr := s.Get(c, id)
		return current, false, getErr
	}
	if err != nil {
		return CallbackInfo{}, false, err
	}
	return got, true, nil
}
func (s *SQLiteCallbackStore) MarkStarted(c context.Context, id, token, runID string) error {
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state='started',run_id=?,next_attempt_at=NULL,dispatch_token='',dispatch_lease_until=NULL,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=?`, runID, time.Now().UTC(), id, token)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("callback %s dispatch lease lost", id)
	}
	return nil
}
func (s *SQLiteCallbackStore) MarkRetry(c context.Context, id, token string, next time.Time, summary string) error {
	summary = SafeCallbackErrorSummary(summary)
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state='retry_wait',next_attempt_at=?,last_error=?,dispatch_token='',dispatch_lease_until=NULL,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=?`, next.UTC(), summary, time.Now().UTC(), id, token)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("callback %s dispatch lease lost", id)
	}
	return nil
}
func (s *SQLiteCallbackStore) MarkFailed(c context.Context, id, token, summary string) error {
	summary = SafeCallbackErrorSummary(summary)
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state='failed',next_attempt_at=NULL,last_error=?,dispatch_token='',dispatch_lease_until=NULL,updated_at=? WHERE id=? AND state='dispatching_fenced' AND dispatch_token=?`, summary, time.Now().UTC(), id, token)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return fmt.Errorf("callback %s dispatch lease lost", id)
	}
	return nil
}
func (s *SQLiteCallbackStore) CancelPending(c context.Context, id string) (CallbackInfo, error) {
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state='canceled',updated_at=? WHERE id=? AND state IN ('pending','retry_wait')`, time.Now().UTC(), id)
	if e != nil {
		return CallbackInfo{}, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return CallbackInfo{}, fmt.Errorf("callback %s cannot be canceled", id)
	}
	return s.Get(c, id)
}
func (s *SQLiteCallbackStore) Update(c context.Context, i CallbackInfo) error {
	r, e := s.db.ExecContext(c, `UPDATE delayed_callbacks SET state=?,fires_at=?,updated_at=? WHERE id=?`, i.State, i.FiresAt.UTC(), time.Now().UTC(), i.ID)
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
	return scanCallback(s.db.QueryRowContext(c, `SELECT id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,run_id,attempt,next_attempt_at,last_error,dispatch_token,dispatch_lease_until FROM delayed_callbacks WHERE id=?`, id))
}
func (s *SQLiteCallbackStore) ListPending(c context.Context) ([]CallbackInfo, error) {
	return s.list(c, `WHERE state='pending' ORDER BY fires_at,id`)
}

func (s *SQLiteCallbackStore) list(c context.Context, clause string) ([]CallbackInfo, error) {
	rs, e := s.db.QueryContext(c, `SELECT id,tenant_id,agent_id,conversation_id,prompt,model,provider_name,allow_fallback,fallback_providers,delay,fires_at,state,created_at,updated_at,run_id,attempt,next_attempt_at,last_error,dispatch_token,dispatch_lease_until FROM delayed_callbacks `+clause)
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
func (s *SQLiteCallbackStore) ListAll(c context.Context) ([]CallbackInfo, error) {
	out, e := s.list(c, `ORDER BY created_at,id`)
	for n := range out {
		i := &out[n]
		i.DispatchToken = ""
		i.DispatchLeaseUntil = time.Time{}
	}
	return out, e
}

type callbackScanner interface{ Scan(...any) error }

func scanCallback(r callbackScanner) (CallbackInfo, error) {
	var i CallbackInfo
	var next, lease sql.NullTime
	var fallbackProviders string
	e := r.Scan(&i.ID, &i.TenantID, &i.AgentID, &i.ConversationID, &i.Prompt, &i.Model, &i.ProviderName, &i.AllowFallback, &fallbackProviders, &i.Delay, &i.FiresAt, &i.State, &i.CreatedAt, &i.UpdatedAt, &i.RunID, &i.Attempt, &next, &i.LastError, &i.DispatchToken, &lease)
	if e != nil {
		return CallbackInfo{}, e
	}
	if err := json.Unmarshal([]byte(fallbackProviders), &i.FallbackProviders); err != nil {
		return CallbackInfo{}, fmt.Errorf("decode callback fallback providers: %w", err)
	}
	i.LastError = SafeCallbackErrorSummary(i.LastError)
	if next.Valid {
		i.NextAttemptAt = next.Time
	}
	if lease.Valid {
		i.DispatchLeaseUntil = lease.Time
	}
	return i, nil
}
