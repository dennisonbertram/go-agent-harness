package harness

import (
	"context"
	"path/filepath"
	"testing"
)

func TestConversationStoreMetaMessagePersistence(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Activating skill", ToolCalls: []ToolCall{
			{ID: "call_1", Name: "skill", Arguments: `{"command":"deploy"}`},
		}},
		{Role: "tool", ToolCallID: "call_1", Name: "skill", Content: `{"skill":"deploy","status":"activated"}`},
		{Role: "system", Content: "<skill name=\"deploy\">Deploy instructions here</skill>", IsMeta: true},
		{Role: "assistant", Content: "Skill activated."},
	}

	if err := store.SaveConversation(ctx, "conv-meta", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-meta")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded))
	}

	// Verify IsMeta is correctly persisted and loaded
	for i, msg := range loaded {
		if i == 3 {
			if !msg.IsMeta {
				t.Errorf("message[3] should have IsMeta=true")
			}
			if msg.Role != "system" {
				t.Errorf("message[3] role: got %q, want %q", msg.Role, "system")
			}
			if msg.Content != `<skill name="deploy">Deploy instructions here</skill>` {
				t.Errorf("message[3] content mismatch: got %q", msg.Content)
			}
		} else {
			if msg.IsMeta {
				t.Errorf("message[%d] should have IsMeta=false", i)
			}
		}
	}
}

func TestConversationStoreMetaMessageMigration(t *testing.T) {
	t.Parallel()

	// Create a store, migrate, add data without is_meta column awareness
	// (simulating an existing database before the migration)
	dbPath := filepath.Join(t.TempDir(), "migration-test.db")
	store, err := NewSQLiteConversationStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	// Run the full migration (creates table with is_meta)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Save a message without IsMeta (default false)
	ctx := context.Background()
	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	if err := store.SaveConversation(ctx, "conv-premigrate", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	// Close and reopen
	store.Close()
	store, err = NewSQLiteConversationStore(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close()

	// Run migrate again (should be idempotent)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("re-Migrate: %v", err)
	}

	// Load existing messages -- is_meta should be false (default 0)
	loaded, err := store.LoadMessages(ctx, "conv-premigrate")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	for i, msg := range loaded {
		if msg.IsMeta {
			t.Errorf("message[%d] should have IsMeta=false after migration (default)", i)
		}
	}

	// Now save messages WITH meta
	msgsWithMeta := []Message{
		{Role: "user", Content: "Test"},
		{Role: "system", Content: "meta instruction", IsMeta: true},
		{Role: "assistant", Content: "OK"},
	}
	if err := store.SaveConversation(ctx, "conv-postmigrate", msgsWithMeta); err != nil {
		t.Fatalf("SaveConversation after migration: %v", err)
	}

	loaded2, err := store.LoadMessages(ctx, "conv-postmigrate")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded2) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(loaded2))
	}
	if !loaded2[1].IsMeta {
		t.Error("message[1] should have IsMeta=true")
	}
	if loaded2[0].IsMeta || loaded2[2].IsMeta {
		t.Error("message[0] and message[2] should have IsMeta=false")
	}
}

func TestConversationStoreMultipleMetaMessages(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "system", Content: "meta 1", IsMeta: true},
		{Role: "system", Content: "meta 2", IsMeta: true},
		{Role: "assistant", Content: "Done"},
		{Role: "system", Content: "meta 3", IsMeta: true},
	}

	if err := store.SaveConversation(ctx, "conv-multi-meta", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-multi-meta")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded))
	}

	metaCount := 0
	for _, msg := range loaded {
		if msg.IsMeta {
			metaCount++
		}
	}
	if metaCount != 3 {
		t.Errorf("expected 3 meta-messages, got %d", metaCount)
	}
}

func TestConversationStoreMetaMessageOverwrite(t *testing.T) {
	t.Parallel()
	store := newTestConversationStore(t)
	ctx := context.Background()

	// First save without meta
	msgs1 := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	if err := store.SaveConversation(ctx, "conv-overwrite-meta", msgs1); err != nil {
		t.Fatalf("SaveConversation (1): %v", err)
	}

	// Second save with meta (overwrite)
	msgs2 := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "system", Content: "new meta", IsMeta: true},
		{Role: "assistant", Content: "Done"},
	}
	if err := store.SaveConversation(ctx, "conv-overwrite-meta", msgs2); err != nil {
		t.Fatalf("SaveConversation (2): %v", err)
	}

	loaded, err := store.LoadMessages(ctx, "conv-overwrite-meta")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(loaded))
	}
	if !loaded[2].IsMeta {
		t.Error("message[2] should have IsMeta=true")
	}
}
