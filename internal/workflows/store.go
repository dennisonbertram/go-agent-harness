package workflows

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, id string) (*Run, error)
	UpsertStepState(ctx context.Context, state *StepState) error
	ListStepStates(ctx context.Context, workflowRunID string) ([]StepState, error)
	AppendEvent(ctx context.Context, event *Event) error
	GetEvents(ctx context.Context, workflowRunID string, afterSeq int64) ([]Event, error)
}

type MemoryStore struct {
	mu         sync.RWMutex
	runs       map[string]*Run
	stepStates map[string]map[string]*StepState
	events     map[string][]Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs:       make(map[string]*Run),
		stepStates: make(map[string]map[string]*StepState),
		events:     make(map[string][]Event),
	}
}

func (m *MemoryStore) CreateRun(_ context.Context, run *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *run
	m.runs[run.ID] = &cp
	return nil
}

func (m *MemoryStore) UpdateRun(_ context.Context, run *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *run
	m.runs[run.ID] = &cp
	return nil
}

func (m *MemoryStore) GetRun(_ context.Context, id string) (*Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("workflow run not found: %s", id)
	}
	cp := *run
	return &cp, nil
}

func (m *MemoryStore) UpsertStepState(_ context.Context, state *StepState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stepStates[state.WorkflowRunID]; !ok {
		m.stepStates[state.WorkflowRunID] = make(map[string]*StepState)
	}
	cp := *state
	m.stepStates[state.WorkflowRunID][state.StepID] = &cp
	return nil
}

func (m *MemoryStore) ListStepStates(_ context.Context, workflowRunID string) ([]StepState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	states := m.stepStates[workflowRunID]
	out := make([]StepState, 0, len(states))
	for _, state := range states {
		out = append(out, *state)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out, nil
}

func (m *MemoryStore) AppendEvent(_ context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[event.WorkflowRunID] = append(m.events[event.WorkflowRunID], *event)
	return nil
}

func (m *MemoryStore) GetEvents(_ context.Context, workflowRunID string, afterSeq int64) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	source := m.events[workflowRunID]
	out := make([]Event, 0, len(source))
	for _, event := range source {
		if event.Seq > afterSeq {
			out = append(out, event)
		}
	}
	return out, nil
}
