package workingmemory

import (
	"context"
	"strings"
	"testing"

	om "go-agent-harness/internal/observationalmemory"
)

func TestSQLiteStoreCRUDAndScopeIsolation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir() + "/working-memory.db")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	scopeA := om.ScopeKey{TenantID: "t1", ConversationID: "c1", AgentID: "a1"}
	scopeB := om.ScopeKey{TenantID: "t1", ConversationID: "c1", AgentID: "a2"}
	if err := store.Set(ctx, scopeA, "plan", map[string]any{"step": "collect"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Set(ctx, scopeA, "constraint", "stay in repo"); err != nil {
		t.Fatalf("Set constraint: %v", err)
	}

	got, ok, err := store.Get(ctx, scopeA, "plan")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected stored value")
	}
	if !strings.Contains(got, "collect") {
		t.Fatalf("stored json = %q, want collect", got)
	}
	if _, ok, err := store.Get(ctx, scopeB, "plan"); err != nil {
		t.Fatalf("Get scopeB: %v", err)
	} else if ok {
		t.Fatal("expected scope isolation")
	}

	if err := store.Delete(ctx, scopeA, "constraint"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	entries, err := store.List(ctx, scopeA)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, ok := entries["constraint"]; ok {
		t.Fatal("deleted key should not be listed")
	}
	if _, ok := entries["plan"]; !ok {
		t.Fatal("remaining key should be listed")
	}
}
