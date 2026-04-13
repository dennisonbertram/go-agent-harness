package workingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	om "go-agent-harness/internal/observationalmemory"
)

type Store interface {
	Set(ctx context.Context, scope om.ScopeKey, key string, value any) error
	Get(ctx context.Context, scope om.ScopeKey, key string) (string, bool, error)
	Delete(ctx context.Context, scope om.ScopeKey, key string) error
	List(ctx context.Context, scope om.ScopeKey) (map[string]string, error)
	Snippet(ctx context.Context, scope om.ScopeKey) (string, error)
}

type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: make(map[string]map[string]string)}
}

func (s *MemoryStore) Set(_ context.Context, scope om.ScopeKey, key string, value any) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("working memory key is required")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal working memory value: %w", err)
	}
	scopeKey := scope.MemoryID()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[scopeKey]; !ok {
		s.entries[scopeKey] = make(map[string]string)
	}
	s.entries[scopeKey][key] = string(raw)
	return nil
}

func (s *MemoryStore) Get(_ context.Context, scope om.ScopeKey, key string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.entries[scope.MemoryID()][strings.TrimSpace(key)]
	return value, ok, nil
}

func (s *MemoryStore) Delete(_ context.Context, scope om.ScopeKey, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entries, ok := s.entries[scope.MemoryID()]; ok {
		delete(entries, strings.TrimSpace(key))
	}
	return nil
}

func (s *MemoryStore) List(_ context.Context, scope om.ScopeKey) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	source := s.entries[scope.MemoryID()]
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out, nil
}

func (s *MemoryStore) Snippet(ctx context.Context, scope om.ScopeKey) (string, error) {
	entries, err := s.List(ctx, scope)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys)+2)
	lines = append(lines, "<working-memory>")
	for _, key := range keys {
		lines = append(lines, key+": "+entries[key])
	}
	lines = append(lines, "</working-memory>")
	return strings.Join(lines, "\n"), nil
}
