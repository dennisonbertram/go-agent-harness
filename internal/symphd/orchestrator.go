package symphd

import (
	"context"
	"sync"
	"time"
)

// Orchestrator coordinates agent dispatch across workspaces.
// This is a stub — real logic is added in issues #188-#190.
type Orchestrator struct {
	config    *Config
	startedAt time.Time
	mu        sync.RWMutex
	agents    int
}

// NewOrchestrator creates a new Orchestrator with the given config.
func NewOrchestrator(cfg *Config) *Orchestrator {
	return &Orchestrator{
		config:    cfg,
		startedAt: time.Now(),
	}
}

// State returns a snapshot of the orchestrator's current state.
func (o *Orchestrator) State() map[string]any {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return map[string]any{
		"version":       "0.1.0",
		"running_since": o.startedAt.UTC().Format(time.RFC3339),
		"agent_count":   o.agents,
		"config": map[string]any{
			"workspace_type":        o.config.WorkspaceType,
			"max_concurrent_agents": o.config.MaxConcurrentAgents,
		},
	}
}

// Start begins orchestration. Currently a no-op stub.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Real logic added in #188 (tracker), #189 (dispatcher), #190 (retry)
	return nil
}

// Shutdown gracefully stops the orchestrator.
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	return nil
}
