package store

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// CreateAPIKey persists an API key in memory.
func (m *MemoryStore) CreateAPIKey(_ context.Context, key APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.apiKeys[key.ID]; exists {
		return &ErrKeyNotFound{ID: key.ID} // reuse not-found, but with "already exists" semantics
	}
	// Check uniqueness on KeyHash as well (mirrors SQL UNIQUE constraint).
	for _, existing := range m.apiKeys {
		if existing.KeyHash == key.KeyHash {
			return &ErrKeyNotFound{ID: key.ID}
		}
	}
	cp := key
	cp.Scopes = copyStrings(key.Scopes)
	m.apiKeys[key.ID] = &cp
	return nil
}

// ValidateAPIKey checks rawToken against stored key hashes, updates last_used_at, and returns the key.
// bcrypt comparison is done without holding the lock to allow concurrent validations.
func (m *MemoryStore) ValidateAPIKey(_ context.Context, rawToken string) (*APIKey, error) {
	// Phase 1: snapshot all key data under a read lock.
	m.mu.RLock()
	type snapshot struct {
		id        string
		hash      string
		expiresAt *time.Time
	}
	snaps := make([]snapshot, 0, len(m.apiKeys))
	for _, key := range m.apiKeys {
		var exp *time.Time
		if key.ExpiresAt != nil {
			t := *key.ExpiresAt
			exp = &t
		}
		snaps = append(snaps, snapshot{id: key.ID, hash: key.KeyHash, expiresAt: exp})
	}
	m.mu.RUnlock()

	// Phase 2: compare hashes without holding any lock (bcrypt is slow).
	now := time.Now().UTC()
	for _, s := range snaps {
		if err := bcrypt.CompareHashAndPassword([]byte(s.hash), []byte(rawToken)); err != nil {
			continue
		}
		// Matched. Check expiration.
		if s.expiresAt != nil && now.After(*s.expiresAt) {
			return nil, ErrKeyExpired
		}
		// Phase 3: re-acquire write lock to update last_used_at.
		m.mu.Lock()
		key, ok := m.apiKeys[s.id]
		if !ok {
			// Key was deleted between phase 1 and phase 3; treat as not found.
			m.mu.Unlock()
			return nil, &ErrKeyNotFound{}
		}
		key.LastUsedAt = &now
		cp := copyAPIKey(key)
		m.mu.Unlock()
		return cp, nil
	}
	return nil, &ErrKeyNotFound{}
}

// ListAPIKeys returns all API keys for a tenant (key hashes excluded).
func (m *MemoryStore) ListAPIKeys(_ context.Context, tenantID string) ([]APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []APIKey
	for _, key := range m.apiKeys {
		if key.TenantID != tenantID {
			continue
		}
		cp := copyAPIKey(key)
		cp.KeyHash = "" // never return the hash
		result = append(result, *cp)
	}
	if result == nil {
		result = []APIKey{}
	}
	return result, nil
}

// RevokeAPIKey removes an API key by ID.
func (m *MemoryStore) RevokeAPIKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.apiKeys[id]; !exists {
		return &ErrKeyNotFound{ID: id}
	}
	delete(m.apiKeys, id)
	return nil
}

func copyAPIKey(k *APIKey) *APIKey {
	if k == nil {
		return nil
	}
	cp := *k
	cp.Scopes = copyStrings(k.Scopes)
	if k.LastUsedAt != nil {
		t := *k.LastUsedAt
		cp.LastUsedAt = &t
	}
	if k.ExpiresAt != nil {
		t := *k.ExpiresAt
		cp.ExpiresAt = &t
	}
	return &cp
}

func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}
