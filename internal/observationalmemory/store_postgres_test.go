package observationalmemory

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresStoreRequiresDSN(t *testing.T) {
	t.Parallel()

	if _, err := NewPostgresStore(""); err == nil {
		t.Fatalf("expected error for empty dsn")
	}
}

func TestPostgresStoreMethodsReturnNotImplemented(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresStore("postgres://user:pass@localhost/db")
	if err != nil {
		t.Fatalf("new postgres store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ctx := context.Background()
	key := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	cfg := DefaultConfig()
	now := time.Now().UTC()

	check := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		if !strings.Contains(err.Error(), "not implemented") {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
	}

	check("migrate", store.Migrate(ctx))
	check("reset_stale_operations", store.ResetStaleOperations(ctx, now))
	_, err = store.GetOrCreateRecord(ctx, key, true, cfg, now)
	check("get_or_create_record", err)
	check("update_record", store.UpdateRecord(ctx, Record{MemoryID: key.MemoryID(), Scope: key, Config: cfg, UpdatedAt: now}))
	_, err = store.CreateOperation(ctx, Operation{OperationID: "op_1", MemoryID: key.MemoryID(), UpdatedAt: now})
	check("create_operation", err)
	check("update_operation_status", store.UpdateOperationStatus(ctx, "op_1", "queued", "", now))
	check("insert_marker", store.InsertMarker(ctx, Marker{MarkerID: "mk_1", MemoryID: key.MemoryID(), CreatedAt: now}))
}
