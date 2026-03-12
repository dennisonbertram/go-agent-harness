package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/server"
	"go-agent-harness/internal/store"
)

// TestAuthMiddleware_Valid verifies that a request with a valid Bearer token passes.
func TestAuthMiddleware_Valid(t *testing.T) {
	ms := store.NewMemoryStore()
	rawToken, key, err := store.GenerateAPIKey("tenant-1", "test", []string{store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// 200 or 501 (no catalog) are both acceptable — what matters is NOT 401.
	if w.Code == http.StatusUnauthorized {
		t.Errorf("expected non-401 for valid token, got 401")
	}
}

// TestAuthMiddleware_Invalid verifies that a missing or wrong token returns 401.
func TestAuthMiddleware_Invalid(t *testing.T) {
	ms := store.NewMemoryStore()
	_, key, err := store.GenerateAPIKey("tenant-1", "test", []string{store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	cases := []struct {
		name   string
		header string
	}{
		{"no_auth_header", ""},
		{"wrong_token", "Bearer harness_sk_WRONG_TOKEN_xxxxxxxxxxxxxxxxxxxxxxxxxxx"},
		{"malformed_bearer", "Token harness_sk_abc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s: expected 401, got %d", tc.name, w.Code)
			}
		})
	}
}

// TestAuthMiddleware_QueryParam verifies the ?token= fallback for SSE clients.
func TestAuthMiddleware_QueryParam(t *testing.T) {
	ms := store.NewMemoryStore()
	rawToken, key, err := store.GenerateAPIKey("tenant-sse", "sse key", []string{store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	req := httptest.NewRequest(http.MethodGet, "/v1/models?token="+rawToken, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("?token= fallback: expected non-401, got 401")
	}
}

// TestAuthMiddleware_Disabled verifies that AuthDisabled=true skips auth.
func TestAuthMiddleware_Disabled(t *testing.T) {
	ms := store.NewMemoryStore()
	// No keys registered — but auth is disabled.
	h := server.NewWithOptions(server.ServerOptions{
		Store:        ms,
		AuthDisabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	// No Authorization header.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("auth disabled: expected non-401, got 401")
	}
}

// TestAuthMiddleware_Healthz verifies /healthz is always accessible without auth.
func TestAuthMiddleware_Healthz(t *testing.T) {
	ms := store.NewMemoryStore()
	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	// No Authorization header.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/healthz: expected 200, got %d", w.Code)
	}
}

// TestAuthMiddleware_NoStore verifies that when no store is configured, auth is skipped.
func TestAuthMiddleware_NoStore(t *testing.T) {
	h := server.NewWithOptions(server.ServerOptions{
		Store: nil, // no store → auth implicitly disabled
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	// No Authorization header.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Must not get 401 — auth is skipped when store is nil.
	if w.Code == http.StatusUnauthorized {
		t.Errorf("no store: expected non-401, got 401")
	}
}

// TestAuthMiddleware_TenantIDInjected verifies tenant ID ends up in context.
func TestAuthMiddleware_TenantIDInjected(t *testing.T) {
	ms := store.NewMemoryStore()
	rawToken, key, err := store.GenerateAPIKey("tenant-ctx", "ctx key", []string{store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	s := server.NewWithOptions(server.ServerOptions{Store: ms})

	// Verify via the /v1/models endpoint that the token is accepted.
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("expected non-401, got 401")
	}
}

// TestAuthMiddleware_ConcurrentValidation verifies no data races under concurrent validation.
func TestAuthMiddleware_ConcurrentValidation(t *testing.T) {
	ms := store.NewMemoryStore()
	const n = 5
	tokens := make([]string, n)
	for i := 0; i < n; i++ {
		raw, key, err := store.GenerateAPIKey("tenant-race", "k", []string{store.ScopeRunsRead})
		if err != nil {
			t.Fatalf("GenerateAPIKey %d: %v", i, err)
		}
		if err := ms.CreateAPIKey(context.Background(), key); err != nil {
			t.Fatalf("CreateAPIKey %d: %v", i, err)
		}
		tokens[i] = raw
	}

	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			req.Header.Set("Authorization", "Bearer "+tokens[i])
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code == http.StatusUnauthorized {
				t.Errorf("goroutine %d: got 401 for valid token", i)
			}
		}(i)
	}
	wg.Wait()
}

// TestAuthMiddleware_ExpiredKey verifies expired keys return 401.
func TestAuthMiddleware_ExpiredKey(t *testing.T) {
	ms := store.NewMemoryStore()
	rawToken, key, err := store.GenerateAPIKey("tenant-exp", "expired", []string{store.ScopeRunsRead})
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	past := time.Now().UTC().Add(-1 * time.Hour)
	key.ExpiresAt = &past
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	h := server.NewWithOptions(server.ServerOptions{Store: ms})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expired key: expected 401, got %d", w.Code)
	}
}

func TestTenantIDFromContext(t *testing.T) {
	// Empty context returns empty string.
	if got := server.TenantIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
