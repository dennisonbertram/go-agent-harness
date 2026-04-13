package store_test

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"go-agent-harness/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func generateFastAPIKey(t *testing.T, tenantID, name string, scopes []string) (string, store.APIKey) {
	t.Helper()

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand.Read(raw): %v", err)
	}
	suffix := base64.RawURLEncoding.EncodeToString(raw)
	rawToken := "harness_sk_" + suffix

	hash, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		t.Fatalf("rand.Read(id): %v", err)
	}

	prefix := suffix
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	scopesCopy := append([]string(nil), scopes...)

	return rawToken, store.APIKey{
		ID:        base64.RawURLEncoding.EncodeToString(idBytes),
		KeyHash:   string(hash),
		KeyPrefix: prefix,
		TenantID:  tenantID,
		Name:      name,
		Scopes:    scopesCopy,
		CreatedAt: time.Now().UTC(),
	}
}
