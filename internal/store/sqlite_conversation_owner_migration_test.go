package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const legacyRunsSchema = `
CREATE TABLE runs (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL DEFAULT '',
	tenant_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	provider_name TEXT NOT NULL DEFAULT '',
	prompt TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	output TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	usage_totals_json TEXT NOT NULL DEFAULT '',
	cost_totals_json TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`

func TestSQLiteMigrateBackfillsLegacyConversationOwnerBeforeNewClaims(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-runs.db")
	seedLegacyConversationRuns(t, path,
		legacyOwnerRun{id: "run-1", conversationID: "conversation-owned", tenantID: "tenant-a", agentID: "agent-a"},
		legacyOwnerRun{id: "run-2", conversationID: "conversation-owned", tenantID: "tenant-a", agentID: "agent-a"},
		legacyOwnerRun{id: "run-default", conversationID: "conversation-default", tenantID: "", agentID: ""},
	)

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	now := time.Now().UTC()
	conflicting := &Run{ID: "run-conflict", ConversationID: "conversation-owned", TenantID: "tenant-b", AgentID: "agent-b", Status: RunStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateRun(ctx, conflicting); !errors.Is(err, ErrConversationOwnerConflict) {
		t.Fatalf("conflicting CreateRun error = %v, want ErrConversationOwnerConflict", err)
	}
	defaultConflict := &Run{ID: "run-default-conflict", ConversationID: "conversation-default", TenantID: "tenant-b", AgentID: "agent-b", Status: RunStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateRun(ctx, defaultConflict); !errors.Is(err, ErrConversationOwnerConflict) {
		t.Fatalf("normalized default conflict error = %v, want ErrConversationOwnerConflict", err)
	}
	sameOwner := &Run{ID: "run-same-owner", ConversationID: "conversation-owned", TenantID: "tenant-a", AgentID: "agent-a", Status: RunStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateRun(ctx, sameOwner); err != nil {
		t.Fatalf("same-owner CreateRun: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if err := reopened.Migrate(ctx); err != nil {
		t.Fatalf("idempotent restart Migrate: %v", err)
	}
	var owners int
	if err := reopened.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_run_owners`).Scan(&owners); err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if owners != 2 {
		t.Fatalf("owner count = %d, want 2", owners)
	}
}

func TestSQLiteMigrateRejectsHistoricallyConflictingConversationOwnersWithoutMutation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "conflicting-runs.db")
	seedLegacyConversationRuns(t, path,
		legacyOwnerRun{id: "run-a", conversationID: "conversation-conflict", tenantID: "tenant-a", agentID: "agent-a"},
		legacyOwnerRun{id: "run-b", conversationID: "conversation-conflict", tenantID: "tenant-b", agentID: "agent-b"},
	)

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	err = store.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "conversation-conflict") {
		t.Fatalf("Migrate error = %v, want named historical owner conflict", err)
	}
	var runs, owners int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs`).Scan(&runs); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_run_owners`).Scan(&owners); err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if runs != 2 || owners != 0 {
		t.Fatalf("post-failure rows: runs=%d owners=%d, want 2 and 0", runs, owners)
	}
	if err := store.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "conversation-conflict") {
		t.Fatalf("repeat Migrate error = %v", err)
	}
}

func TestSQLiteMigrateRejectsPersistedOwnerThatConflictsWithHistoricalRun(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persisted-conflict.db")
	seedLegacyConversationRuns(t, path,
		legacyOwnerRun{id: "run-a", conversationID: "conversation-persisted-conflict", tenantID: "tenant-a", agentID: "agent-a"},
	)
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.db.ExecContext(ctx, `
CREATE TABLE conversation_run_owners (
	conversation_id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	created_at TEXT NOT NULL
);
INSERT INTO conversation_run_owners (conversation_id, tenant_id, agent_id, created_at)
VALUES ('conversation-persisted-conflict', 'tenant-b', 'agent-b', '2026-08-01T00:00:00Z');
`); err != nil {
		t.Fatalf("seed conflicting owner: %v", err)
	}
	err = store.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "conversation-persisted-conflict") {
		t.Fatalf("Migrate error = %v, want persisted historical conflict", err)
	}
	var tenantID, agentID string
	if err := store.db.QueryRowContext(ctx, `SELECT tenant_id, agent_id FROM conversation_run_owners WHERE conversation_id = ?`, "conversation-persisted-conflict").Scan(&tenantID, &agentID); err != nil {
		t.Fatalf("read preserved owner: %v", err)
	}
	if tenantID != "tenant-b" || agentID != "agent-b" {
		t.Fatalf("persisted owner mutated to %q/%q", tenantID, agentID)
	}
}

func TestSQLiteConcurrentMigrationBackfillsOneConversationOwnerIdempotently(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent-runs.db")
	seedLegacyConversationRuns(t, path,
		legacyOwnerRun{id: "run-a", conversationID: "conversation-concurrent", tenantID: "tenant-a", agentID: "agent-a"},
		legacyOwnerRun{id: "run-b", conversationID: "conversation-concurrent", tenantID: "tenant-a", agentID: "agent-a"},
	)
	first, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, current := range []*SQLiteStore{first, second} {
		wg.Add(1)
		go func(store *SQLiteStore) {
			defer wg.Done()
			<-start
			results <- store.Migrate(ctx)
		}(current)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent Migrate: %v", err)
		}
	}
	var owners int
	if err := first.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversation_run_owners WHERE conversation_id = ?`, "conversation-concurrent").Scan(&owners); err != nil {
		t.Fatalf("count owners: %v", err)
	}
	if owners != 1 {
		t.Fatalf("owner count = %d, want 1", owners)
	}
}

type legacyOwnerRun struct {
	id             string
	conversationID string
	tenantID       string
	agentID        string
}

func seedLegacyConversationRuns(t *testing.T, path string, runs ...legacyOwnerRun) {
	t.Helper()
	ctx := context.Background()
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, legacyRunsSchema); err != nil {
		_ = store.Close()
		t.Fatalf("create legacy runs: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, run := range runs {
		if _, err := store.db.ExecContext(ctx, `
INSERT INTO runs (
	id, conversation_id, tenant_id, agent_id, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
`, run.id, run.conversationID, run.tenantID, run.agentID, RunStatusCompleted, now, now); err != nil {
			_ = store.Close()
			t.Fatalf("insert legacy run %q: %v", run.id, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}
}
