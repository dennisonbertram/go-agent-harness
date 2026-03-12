package server

import (
	"context"
	"net/http"
	"os"
	"strings"

	"go-agent-harness/internal/store"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	// contextKeyTenantID is the context key for the authenticated tenant ID.
	contextKeyTenantID contextKey = iota
)

// TenantIDFromContext returns the tenant ID injected by authMiddleware.
// Returns "" if no tenant ID is in the context.
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyTenantID).(string)
	return v
}

// authMiddleware enforces Bearer token authentication for all requests.
//
// Token extraction order:
//  1. Authorization: Bearer <token> header
//  2. ?token= query parameter (fallback for SSE EventSource connections that
//     cannot set custom headers in all browsers)
//
// Auth can be disabled at startup via:
//   - ServerOptions.AuthDisabled = true
//   - HARNESS_AUTH_DISABLED=true environment variable
//
// When disabled, all requests are allowed through and tenant ID is set to "".
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth when explicitly disabled.
		if s.authDisabled {
			next.ServeHTTP(w, r)
			return
		}
		// If no auth store is configured, auth is implicitly disabled.
		if s.runStore == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Extract the raw token.
		rawToken := extractToken(r)
		if rawToken == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authorization required")
			return
		}

		// Validate against the key store.
		key, err := s.runStore.ValidateAPIKey(r.Context(), rawToken)
		if err != nil {
			if err == store.ErrKeyExpired {
				writeError(w, http.StatusUnauthorized, "unauthorized", "api key expired")
				return
			}
			if store.IsKeyNotFound(err) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid api key")
				return
			}
			// Unexpected store error — log-safe message without leaking details.
			writeError(w, http.StatusInternalServerError, "internal_error", "auth check failed")
			return
		}

		// Inject tenant_id into context.
		ctx := context.WithValue(r.Context(), contextKeyTenantID, key.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken pulls the Bearer token from the Authorization header or the
// ?token= query parameter.
func extractToken(r *http.Request) string {
	// Authorization: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
		return ""
	}
	// Fallback for SSE EventSource.
	return r.URL.Query().Get("token")
}

// authDisabledFromEnv returns true when HARNESS_AUTH_DISABLED=true is set.
func authDisabledFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("HARNESS_AUTH_DISABLED")), "true")
}
