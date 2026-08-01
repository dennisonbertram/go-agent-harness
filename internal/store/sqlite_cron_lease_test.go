package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSQLiteCronRunDispatchLeaseAcquireReturnsItsOwnLinearizedOwner(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	first, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	second, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Migrate(ctx); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}

	now := time.Now().UTC()
	start := CronRunStart{
		TenantID:       "tenant-linearizable",
		IdempotencyKey: "cron/job-linearizable/execution-linearizable",
		Fingerprint:    "fingerprint-linearizable",
		RunID:          "run_linearizable",
		CreatedAt:      now,
	}
	if _, claimed, err := first.ClaimCronRunStart(ctx, start); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart: claimed=%t err=%v", claimed, err)
	}

	firstUpdated := make(chan struct{})
	releaseFirstRead := make(chan struct{})
	first.cronRunAcquireAfterUpdate = func() {
		close(firstUpdated)
		<-releaseFirstRead
	}
	type result struct {
		binding  CronRunStart
		acquired bool
		err      error
	}
	firstResult := make(chan result, 1)
	go func() {
		binding, acquired, acquireErr := first.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-a", now, now.Add(time.Minute))
		firstResult <- result{binding: binding, acquired: acquired, err: acquireErr}
	}()
	<-firstUpdated
	if _, err := second.db.ExecContext(ctx, `
UPDATE cron_run_starts
SET dispatch_lease_until = 0
WHERE tenant_id = ? AND idempotency_key = ?
`, start.TenantID, start.IdempotencyKey); err != nil {
		t.Fatalf("expire first lease: %v", err)
	}
	secondBinding, secondAcquired, err := second.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-b", now.Add(time.Second), now.Add(time.Minute+time.Second))
	if err != nil || !secondAcquired || secondBinding.DispatchOwner != "owner-b" {
		t.Fatalf("second acquire = %+v acquired=%t err=%v", secondBinding, secondAcquired, err)
	}
	close(releaseFirstRead)
	got := <-firstResult
	if got.err != nil || !got.acquired {
		t.Fatalf("first acquire: acquired=%t err=%v", got.acquired, got.err)
	}
	if got.binding.DispatchOwner != "owner-a" {
		t.Fatalf("first acquired binding owner = %q, want owner-a", got.binding.DispatchOwner)
	}
}

func TestSQLiteCronRunDispatchLeaseIgnoresCallerClockSkew(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	first, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	second, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Migrate(ctx); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}

	now := time.Now().UTC()
	start := CronRunStart{TenantID: "tenant-skew", IdempotencyKey: "cron/skew", Fingerprint: "fingerprint-skew", RunID: "run_skew", CreatedAt: now}
	if _, claimed, err := first.ClaimCronRunStart(ctx, start); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart: claimed=%t err=%v", claimed, err)
	}
	if _, acquired, err := first.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-a", now, now.Add(time.Minute)); err != nil || !acquired {
		t.Fatalf("first acquire: acquired=%t err=%v", acquired, err)
	}
	skewedNow := now.Add(24 * time.Hour)
	got, acquired, err := second.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-b", skewedNow, skewedNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("skewed acquire: %v", err)
	}
	if acquired || got.DispatchOwner != "owner-a" {
		t.Fatalf("skewed acquire = %+v acquired=%t; want live owner-a lease", got, acquired)
	}
}

func TestSQLiteCronRunDispatchLeaseRenewalRequiresLiveOwner(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	now := time.Now().UTC()
	start := CronRunStart{TenantID: "tenant-renew", IdempotencyKey: "cron/renew", Fingerprint: "fingerprint-renew", RunID: "run_renew", CreatedAt: now}
	if _, claimed, err := s.ClaimCronRunStart(ctx, start); err != nil || !claimed {
		t.Fatalf("ClaimCronRunStart: claimed=%t err=%v", claimed, err)
	}
	if _, acquired, err := s.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-a", now, now.Add(time.Minute)); err != nil || !acquired {
		t.Fatalf("Acquire owner-a: acquired=%t err=%v", acquired, err)
	}
	if got, renewed, err := s.RenewCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-b", now, now.Add(time.Minute)); err != nil || renewed || got.DispatchOwner != "owner-a" {
		t.Fatalf("competing renew = %+v renewed=%t err=%v", got, renewed, err)
	}
	var nearExpiry int64
	if err := s.db.QueryRowContext(ctx, `
UPDATE cron_run_starts
SET dispatch_lease_until = CAST((julianday('now') - 2440587.5) * 86400000000000 AS INTEGER) + 1000000000
WHERE tenant_id = ? AND idempotency_key = ?
RETURNING dispatch_lease_until
`, start.TenantID, start.IdempotencyKey).Scan(&nearExpiry); err != nil {
		t.Fatalf("set near expiry: %v", err)
	}
	got, renewed, err := s.RenewCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-a", now, now.Add(time.Minute))
	if err != nil || !renewed || got.DispatchOwner != "owner-a" || got.DispatchLeaseUntil.UnixNano() <= nearExpiry {
		t.Fatalf("owner renew = %+v renewed=%t err=%v near_expiry=%d", got, renewed, err, nearExpiry)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE cron_run_starts SET dispatch_lease_until = 0 WHERE tenant_id = ? AND idempotency_key = ?`, start.TenantID, start.IdempotencyKey); err != nil {
		t.Fatalf("expire owner-a: %v", err)
	}
	if got, renewed, err := s.RenewCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-a", now, now.Add(time.Minute)); err != nil || renewed || got.DispatchOwner != "owner-a" {
		t.Fatalf("expired owner renew = %+v renewed=%t err=%v", got, renewed, err)
	}
	if got, acquired, err := s.AcquireCronRunStartDispatchLease(ctx, start.TenantID, start.IdempotencyKey, "owner-b", now, now.Add(time.Minute)); err != nil || !acquired || got.DispatchOwner != "owner-b" {
		t.Fatalf("owner-b takeover = %+v acquired=%t err=%v", got, acquired, err)
	}
	if err := s.MarkCronRunStartAccepted(ctx, start.TenantID, start.IdempotencyKey, "owner-a"); !errors.Is(err, ErrCronRunDispatchLeaseLost) {
		t.Fatalf("stale owner mark error = %v, want ErrCronRunDispatchLeaseLost", err)
	}
}

func TestSQLiteConcurrentLegacyCronLeaseMigrationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `
CREATE TABLE cron_run_starts (
	tenant_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	run_id TEXT NOT NULL,
	accepted INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	PRIMARY KEY (tenant_id, idempotency_key)
)`); err != nil {
		_ = legacy.Close()
		t.Fatalf("create legacy table: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy sqlite: %v", err)
	}

	first, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	var onceFirst, onceSecond sync.Once
	first.migrationBeforeAddColumn = func(_, column string) {
		if column == "dispatch_owner" {
			onceFirst.Do(func() { arrived <- struct{}{}; <-release })
		}
	}
	second.migrationBeforeAddColumn = func(_, column string) {
		if column == "dispatch_owner" {
			onceSecond.Do(func() { arrived <- struct{}{}; <-release })
		}
	}
	results := make(chan error, 2)
	go func() { results <- first.Migrate(ctx) }()
	go func() { results <- second.Migrate(ctx) }()
	<-arrived
	<-arrived
	close(release)
	if err := <-results; err != nil {
		t.Fatalf("first concurrent Migrate: %v", err)
	}
	if err := <-results; err != nil {
		t.Fatalf("second concurrent Migrate: %v", err)
	}
}
