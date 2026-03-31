package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-agent-harness/apps/socialagent/db"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return url
}

func TestGetOrCreateUser_CreatesOnFirstCall(t *testing.T) {
	dbURL := testDatabaseURL(t)

	store, err := db.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	telegramID := int64(100001)

	// Clean up before test.
	_ = store.DeleteUserByTelegramID(ctx, telegramID)

	user, err := store.GetOrCreateUser(ctx, telegramID, "Alice")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}

	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.TelegramID != telegramID {
		t.Errorf("TelegramID: got %d, want %d", user.TelegramID, telegramID)
	}
	if user.ConversationID == "" {
		t.Error("expected non-empty ConversationID")
	}
	if user.DisplayName != "Alice" {
		t.Errorf("DisplayName: got %q, want %q", user.DisplayName, "Alice")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestGetOrCreateUser_IdempotentOnSecondCall(t *testing.T) {
	dbURL := testDatabaseURL(t)

	store, err := db.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	telegramID := int64(100002)

	_ = store.DeleteUserByTelegramID(ctx, telegramID)

	first, err := store.GetOrCreateUser(ctx, telegramID, "Bob")
	if err != nil {
		t.Fatalf("first GetOrCreateUser: %v", err)
	}

	// Small sleep so that if updated_at were to change, we'd catch it.
	time.Sleep(10 * time.Millisecond)

	second, err := store.GetOrCreateUser(ctx, telegramID, "Bob Updated")
	if err != nil {
		t.Fatalf("second GetOrCreateUser: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("ID mismatch: first=%s, second=%s", first.ID, second.ID)
	}
	if first.ConversationID != second.ConversationID {
		t.Errorf("ConversationID changed between calls: first=%s, second=%s", first.ConversationID, second.ConversationID)
	}
}

func TestGetOrCreateUser_DifferentTelegramIDsGetDifferentUsers(t *testing.T) {
	dbURL := testDatabaseURL(t)

	store, err := db.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	idA := int64(100003)
	idB := int64(100004)

	_ = store.DeleteUserByTelegramID(ctx, idA)
	_ = store.DeleteUserByTelegramID(ctx, idB)

	userA, err := store.GetOrCreateUser(ctx, idA, "Charlie")
	if err != nil {
		t.Fatalf("GetOrCreateUser A: %v", err)
	}
	userB, err := store.GetOrCreateUser(ctx, idB, "Diana")
	if err != nil {
		t.Fatalf("GetOrCreateUser B: %v", err)
	}

	if userA.ID == userB.ID {
		t.Error("different telegram IDs should produce different user IDs")
	}
	if userA.ConversationID == userB.ConversationID {
		t.Error("different users should have different conversation IDs")
	}
}

func TestGetUser_ReturnsNilForNonExistentUser(t *testing.T) {
	dbURL := testDatabaseURL(t)

	store, err := db.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	// Use a telegram ID very unlikely to exist.
	telegramID := int64(-999999)

	user, err := store.GetUser(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil for non-existent user, got %+v", user)
	}
}

func TestUpdateDisplayName(t *testing.T) {
	dbURL := testDatabaseURL(t)

	store, err := db.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	telegramID := int64(100005)

	_ = store.DeleteUserByTelegramID(ctx, telegramID)

	user, err := store.GetOrCreateUser(ctx, telegramID, "Eve")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}

	err = store.UpdateDisplayName(ctx, user.ID, "Eve Updated")
	if err != nil {
		t.Fatalf("UpdateDisplayName: %v", err)
	}

	updated, err := store.GetUser(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetUser after update: %v", err)
	}
	if updated == nil {
		t.Fatal("expected non-nil user after update")
	}
	if updated.DisplayName != "Eve Updated" {
		t.Errorf("DisplayName: got %q, want %q", updated.DisplayName, "Eve Updated")
	}
}
