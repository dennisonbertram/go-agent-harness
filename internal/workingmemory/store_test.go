package workingmemory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	om "go-agent-harness/internal/observationalmemory"
)

func TestMemoryStoreCRUDAndScopeIsolation(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	scopeA := om.ScopeKey{TenantID: "t1", ConversationID: "c1", AgentID: "a1"}
	scopeB := om.ScopeKey{TenantID: "t1", ConversationID: "c1", AgentID: "a2"}

	if err := store.Set(context.Background(), scopeA, "plan", map[string]any{"step": "collect"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Set(context.Background(), scopeA, "constraint", "stay in repo"); err != nil {
		t.Fatalf("Set constraint: %v", err)
	}

	got, ok, err := store.Get(context.Background(), scopeA, "plan")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected stored value")
	}
	if got == "" {
		t.Fatal("expected stored json")
	}

	if _, ok, err := store.Get(context.Background(), scopeB, "plan"); err != nil {
		t.Fatalf("Get scopeB: %v", err)
	} else if ok {
		t.Fatal("expected scope isolation")
	}

	snippet, err := store.Snippet(context.Background(), scopeA)
	if err != nil {
		t.Fatalf("Snippet: %v", err)
	}
	if snippet == "" {
		t.Fatal("expected snippet")
	}

	if err := store.Delete(context.Background(), scopeA, "constraint"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	entries, err := store.List(context.Background(), scopeA)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
}

func TestSQLiteStoreReopenPreservesCanonicalJSONForSnippet(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "working-memory.db")
	scope := om.ScopeKey{TenantID: "tenant", ConversationID: "conversation", AgentID: "agent"}
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for key, value := range map[string]any{
		"text":   "api-memory-value",
		"object": map[string]any{"step": "collect"},
		"list":   []any{"one", 2},
	} {
		if err := store.Set(context.Background(), scope, key, value); err != nil {
			t.Fatalf("Set %q: %v", key, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close first store: %v", err)
	}

	reopened, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen SQLite store: %v", err)
	}
	defer reopened.Close()
	if err := reopened.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate reopened store: %v", err)
	}
	entries, err := reopened.List(context.Background(), scope)
	if err != nil {
		t.Fatalf("List reopened store: %v", err)
	}
	for key, want := range map[string]string{
		"text":   `"api-memory-value"`,
		"object": `{"step":"collect"}`,
		"list":   `["one",2]`,
	} {
		if got := entries[key]; got != want {
			t.Errorf("entries[%q] = %q, want %q", key, got, want)
		}
		if !json.Valid([]byte(entries[key])) {
			t.Errorf("entries[%q] is not canonical JSON: %q", key, entries[key])
		}
	}
	snippet, err := reopened.Snippet(context.Background(), scope)
	if err != nil {
		t.Fatalf("Snippet reopened store: %v", err)
	}
	for _, want := range []string{
		`list: ["one",2]`,
		`object: {"step":"collect"}`,
		`text: "api-memory-value"`,
	} {
		if !strings.Contains(snippet, want) {
			t.Errorf("snippet missing %q: %s", want, snippet)
		}
	}
}

func TestSQLiteStoreDeleteRemovesScopedEntry(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "working-memory.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	defer store.Close()

	scope := om.ScopeKey{TenantID: "tenant", ConversationID: "conv", AgentID: "agent"}
	if err := store.Set(context.Background(), scope, "plan", map[string]any{"step": "collect"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok, err := store.Get(context.Background(), scope, "plan"); err != nil {
		t.Fatalf("Get before delete: %v", err)
	} else if !ok {
		t.Fatal("expected entry before delete")
	}

	if err := store.Delete(context.Background(), scope, "plan"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, err := store.Get(context.Background(), scope, "plan"); err != nil {
		t.Fatalf("Get after delete: %v", err)
	} else if ok {
		t.Fatal("entry still exists after delete")
	}
	entries, err := store.List(context.Background(), scope)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries after delete = %#v", entries)
	}
}
