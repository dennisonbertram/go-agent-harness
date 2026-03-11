package symphd

import (
	"context"
	"sync"
	"time"
)

// Orchestrator coordinates agent dispatch across workspaces.
type Orchestrator struct {
	config    *Config
	startedAt time.Time
	mu        sync.RWMutex
	agents    int
	tracker   Tracker
}

// NewOrchestrator creates a new Orchestrator with the given config.
// If the config has GitHubOwner and GitHubRepo set, a GitHubTracker is
// initialised automatically.
func NewOrchestrator(cfg *Config) *Orchestrator {
	o := &Orchestrator{
		config:    cfg,
		startedAt: time.Now(),
	}
	if cfg.GitHubOwner != "" && cfg.GitHubRepo != "" {
		o.tracker = NewGitHubTracker(cfg.GitHubOwner, cfg.GitHubRepo, cfg.TrackLabel, cfg.GitHubToken)
	}
	return o
}

// SetTracker replaces the tracker (useful for testing).
func (o *Orchestrator) SetTracker(t Tracker) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tracker = t
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

// Issues returns all tracked issues, or an empty slice if no tracker is set.
func (o *Orchestrator) Issues() []*TrackedIssue {
	o.mu.RLock()
	tr := o.tracker
	o.mu.RUnlock()

	if tr == nil {
		return []*TrackedIssue{}
	}
	return tr.Issues()
}

// Refresh polls the tracker for new issues. It is a no-op when no tracker is
// configured.
func (o *Orchestrator) Refresh(ctx context.Context) error {
	o.mu.RLock()
	tr := o.tracker
	o.mu.RUnlock()

	if tr == nil {
		return nil
	}
	return tr.Poll(ctx)
}

// Start begins orchestration. Currently a no-op stub.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Real logic added in #189 (dispatcher), #190 (retry)
	return nil
}

// Shutdown gracefully stops the orchestrator.
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	return nil
}
