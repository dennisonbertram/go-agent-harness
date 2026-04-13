package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/store"
)

// apiKeyStoreFactory creates a fresh Store that supports APIKey operations.
type apiKeyStoreFactory func(t *testing.T) store.Store

// sqliteAPIKeyFactory creates a SQLiteStore with both Migrate and MigrateAPIKeys applied.
func sqliteAPIKeyFactory(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := s.MigrateAPIKeys(ctx); err != nil {
		t.Fatalf("MigrateAPIKeys: %v", err)
	}
	return s
}

// memoryAPIKeyFactory creates a MemoryStore.
func memoryAPIKeyFactory(t *testing.T) store.Store {
	t.Helper()
	return store.NewMemoryStore()
}

// TestGenerateAPIKey validates the format and properties of a generated key.
func TestGenerateAPIKey(t *testing.T) {
	rawToken, key, err := store.GenerateAPIKey("tenant-1", "test key", []string{store.ScopeRunsRead, store.ScopeRunsWrite})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}

	// Raw token must start with "harness_sk_"
	if !strings.HasPrefix(rawToken, "harness_sk_") {
		t.Errorf("rawToken %q does not start with 'harness_sk_'", rawToken)
	}

	// Total length: "harness_sk_" (11) + 43 base64 chars = 54
	if len(rawToken) != 54 {
		t.Errorf("rawToken length: got %d, want 54", len(rawToken))
	}

	// Key ID must be non-empty.
	if key.ID == "" {
		t.Error("key.ID is empty")
	}

	// KeyHash must be non-empty and different from the raw token.
	if key.KeyHash == "" {
		t.Error("key.KeyHash is empty")
	}
	if key.KeyHash == rawToken {
		t.Error("key.KeyHash equals rawToken (hash not applied)")
	}

	// KeyPrefix must be non-empty and at most 8 chars.
	if key.KeyPrefix == "" {
		t.Error("key.KeyPrefix is empty")
	}
	if len(key.KeyPrefix) > 8 {
		t.Errorf("key.KeyPrefix length %d > 8", len(key.KeyPrefix))
	}

	// TenantID, Name, Scopes must match.
	if key.TenantID != "tenant-1" {
		t.Errorf("TenantID: got %q, want tenant-1", key.TenantID)
	}
	if key.Name != "test key" {
		t.Errorf("Name: got %q, want 'test key'", key.Name)
	}
	if len(key.Scopes) != 2 {
		t.Errorf("Scopes: got %v, want 2 items", key.Scopes)
	}

	// CreatedAt must be set and recent.
	if key.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if time.Since(key.CreatedAt) > 5*time.Second {
		t.Errorf("CreatedAt %v is too old", key.CreatedAt)
	}

	// Consecutive calls must produce different keys.
	rawToken2, key2, err := store.GenerateAPIKey("tenant-1", "test key 2", nil)
	if err != nil {
		t.Fatalf("second GenerateAPIKey: %v", err)
	}
	if rawToken == rawToken2 {
		t.Error("two calls to GenerateAPIKey produced identical raw tokens")
	}
	if key.ID == key2.ID {
		t.Error("two calls to GenerateAPIKey produced identical IDs")
	}
	if key.KeyHash == key2.KeyHash {
		t.Error("two calls to GenerateAPIKey produced identical hashes")
	}
}

// runAPIKeyContractTests runs the full APIKey contract test suite.
func runAPIKeyContractTests(t *testing.T, factory apiKeyStoreFactory) {
	t.Helper()

	t.Run("CreateAPIKey_HappyPath", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		rawToken, key := generateFastAPIKey(t, "tenant-1", "my key", []string{store.ScopeRunsWrite})
		_ = rawToken

		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
	})

	t.Run("CreateAPIKey_DuplicateID", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		_, key := generateFastAPIKey(t, "tenant-1", "key1", []string{store.ScopeRunsRead})
		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("first CreateAPIKey: %v", err)
		}
		// Attempt to insert same ID again — must fail.
		if err := s.CreateAPIKey(ctx, key); err == nil {
			t.Fatal("expected error on duplicate CreateAPIKey, got nil")
		}
	})

	t.Run("ValidateAPIKey_Valid", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		rawToken, key := generateFastAPIKey(t, "tenant-2", "valid key", []string{store.ScopeRunsRead})
		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		got, err := s.ValidateAPIKey(ctx, rawToken)
		if err != nil {
			t.Fatalf("ValidateAPIKey: %v", err)
		}
		if got.TenantID != "tenant-2" {
			t.Errorf("TenantID: got %q, want tenant-2", got.TenantID)
		}
		if got.ID != key.ID {
			t.Errorf("ID: got %q, want %q", got.ID, key.ID)
		}
		// last_used_at must be populated after validation.
		if got.LastUsedAt == nil {
			t.Error("LastUsedAt is nil after successful validation")
		}
	})

	t.Run("ValidateAPIKey_WrongKey", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		_, key := generateFastAPIKey(t, "tenant-3", "real key", []string{store.ScopeRunsRead})
		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		_, err := s.ValidateAPIKey(ctx, "harness_sk_wrongtoken_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
		if err == nil {
			t.Fatal("expected error for wrong token, got nil")
		}
		if !store.IsKeyNotFound(err) {
			t.Errorf("expected IsKeyNotFound, got: %v", err)
		}
	})

	t.Run("ValidateAPIKey_Expired", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		rawToken, key := generateFastAPIKey(t, "tenant-4", "expired key", []string{store.ScopeRunsRead})
		past := time.Now().UTC().Add(-1 * time.Hour)
		key.ExpiresAt = &past

		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		_, err := s.ValidateAPIKey(ctx, rawToken)
		if err == nil {
			t.Fatal("expected error for expired key, got nil")
		}
		if err != store.ErrKeyExpired {
			t.Errorf("expected ErrKeyExpired, got: %v", err)
		}
	})

	t.Run("ValidateAPIKey_UpdatesLastUsedAt", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		rawToken, key := generateFastAPIKey(t, "tenant-5", "usage key", []string{store.ScopeRunsRead})
		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		before := time.Now().UTC()
		got, err := s.ValidateAPIKey(ctx, rawToken)
		if err != nil {
			t.Fatalf("ValidateAPIKey: %v", err)
		}
		after := time.Now().UTC()

		if got.LastUsedAt == nil {
			t.Fatal("LastUsedAt is nil")
		}
		if got.LastUsedAt.Before(before) || got.LastUsedAt.After(after.Add(time.Second)) {
			t.Errorf("LastUsedAt %v not in expected range [%v, %v]", got.LastUsedAt, before, after)
		}
	})

	t.Run("RevokeAPIKey_HappyPath", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		rawToken, key := generateFastAPIKey(t, "tenant-6", "to revoke", []string{store.ScopeAdmin})
		if err := s.CreateAPIKey(ctx, key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		// Revoke.
		if err := s.RevokeAPIKey(ctx, key.ID); err != nil {
			t.Fatalf("RevokeAPIKey: %v", err)
		}

		// Validate after revocation must fail.
		_, err := s.ValidateAPIKey(ctx, rawToken)
		if err == nil {
			t.Fatal("expected error after revocation, got nil")
		}
	})

	t.Run("RevokeAPIKey_NotFound", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		err := s.RevokeAPIKey(ctx, "nonexistent-id")
		if err == nil {
			t.Fatal("expected error for nonexistent key, got nil")
		}
		if !store.IsKeyNotFound(err) {
			t.Errorf("expected IsKeyNotFound, got: %v", err)
		}
	})

	t.Run("ListAPIKeys_FilterByTenant", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		// Create keys for two tenants.
		for i := 0; i < 3; i++ {
			_, key := generateFastAPIKey(t, "tenant-A", "key", []string{store.ScopeRunsRead})
			if err := s.CreateAPIKey(ctx, key); err != nil {
				t.Fatalf("CreateAPIKey: %v", err)
			}
		}
		_, keyB := generateFastAPIKey(t, "tenant-B", "other", []string{store.ScopeRunsWrite})
		if err := s.CreateAPIKey(ctx, keyB); err != nil {
			t.Fatalf("CreateAPIKey for tenant-B: %v", err)
		}

		keys, err := s.ListAPIKeys(ctx, "tenant-A")
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		if len(keys) != 3 {
			t.Errorf("expected 3 keys for tenant-A, got %d", len(keys))
		}
		// Key hashes must NOT be returned.
		for _, k := range keys {
			if k.KeyHash != "" {
				t.Errorf("KeyHash should be empty in ListAPIKeys response, got non-empty")
			}
		}
	})

	t.Run("ListAPIKeys_Empty", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		keys, err := s.ListAPIKeys(ctx, "nobody")
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("expected 0 keys, got %d", len(keys))
		}
	})

	t.Run("Concurrent_ValidateAPIKey", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()

		// Create N keys and validate them concurrently.
		const n = 5
		rawTokens := make([]string, n)
		for i := 0; i < n; i++ {
			raw, key := generateFastAPIKey(t, "tenant-conc", "key", []string{store.ScopeRunsRead})
			if err := s.CreateAPIKey(ctx, key); err != nil {
				t.Fatalf("CreateAPIKey %d: %v", i, err)
			}
			rawTokens[i] = raw
		}

		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = s.ValidateAPIKey(ctx, rawTokens[i])
			}(i)
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: ValidateAPIKey: %v", i, err)
			}
		}
	})
}

// TestAPIKey_SQLite runs contract tests against the SQLite implementation.
func TestAPIKey_SQLite(t *testing.T) {
	runAPIKeyContractTests(t, sqliteAPIKeyFactory)
}

// TestAPIKey_Memory runs contract tests against the in-memory implementation.
func TestAPIKey_Memory(t *testing.T) {
	runAPIKeyContractTests(t, memoryAPIKeyFactory)
}

func TestErrKeyNotFoundError(t *testing.T) {
	// With ID
	e1 := &store.ErrKeyNotFound{ID: "key-123"}
	if e1.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
	if !strings.Contains(e1.Error(), "key-123") {
		t.Fatalf("expected ID in error message, got: %s", e1.Error())
	}
	// Without ID
	e2 := &store.ErrKeyNotFound{}
	if e2.Error() == "" {
		t.Fatal("expected non-empty error message for empty ID")
	}
}
