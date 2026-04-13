package checkpoints

import (
	"context"
	"sort"
	"sync"
)

type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]*Record)}
}

func (m *MemoryStore) Create(_ context.Context, record *Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[record.ID] = cloneRecord(record)
	return nil
}

func (m *MemoryStore) Update(_ context.Context, record *Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[record.ID] = cloneRecord(record)
	return nil
}

func (m *MemoryStore) Get(_ context.Context, id string) (*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.records[id]
	if !ok {
		return nil, &NotFoundError{ID: id}
	}
	return cloneRecord(record), nil
}

func (m *MemoryStore) PendingByRun(_ context.Context, runID string) (*Record, error) {
	return m.pending(func(record *Record) bool {
		return record.RunID == runID
	})
}

func (m *MemoryStore) PendingByWorkflowRun(_ context.Context, workflowRunID string) (*Record, error) {
	return m.pending(func(record *Record) bool {
		return record.WorkflowRunID == workflowRunID
	})
}

func (m *MemoryStore) pending(match func(*Record) bool) (*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	matches := make([]*Record, 0, len(m.records))
	for _, record := range m.records {
		if record.Status == StatusPending && match(record) {
			matches = append(matches, cloneRecord(record))
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].UpdatedAt.After(matches[j].UpdatedAt)
	})
	if len(matches) == 0 {
		return nil, nil
	}
	return matches[0], nil
}

func (m *MemoryStore) Close() error { return nil }

func cloneRecord(record *Record) *Record {
	if record == nil {
		return nil
	}
	cp := *record
	return &cp
}
