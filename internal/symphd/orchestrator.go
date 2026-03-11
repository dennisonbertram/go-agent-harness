package symphd

import (
	"context"
	"sync"
	"time"
)

// Orchestrator coordinates agent dispatch across workspaces.
type Orchestrator struct {
	config     *Config
	startedAt  time.Time
	mu         sync.RWMutex
	agents     int
	tracker    Tracker
	dispatcher *Dispatcher
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

// SetDispatcher replaces the dispatcher (useful for testing).
func (o *Orchestrator) SetDispatcher(d *Dispatcher) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.dispatcher = d
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

// Start begins orchestration. It runs a polling loop that claims unclaimed
// issues from the tracker and dispatches them via the Dispatcher. If no
// dispatcher is configured, Start returns immediately.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.RLock()
	d := o.dispatcher
	tr := o.tracker
	o.mu.RUnlock()

	if d == nil || tr == nil {
		// No dispatcher or tracker configured; nothing to do.
		return nil
	}

	pollInterval := time.Duration(o.config.PollIntervalMs) * time.Millisecond
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Drain results in a background goroutine so the semaphore is never blocked
	// by an unread results channel.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-d.Results():
				if !ok {
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			// Claim all unclaimed issues, then dispatch claimed candidates.
			for _, issue := range tr.Issues() {
				if issue.ClaimState == ClaimStateUnclaimed {
					_ = tr.Claim(issue.Number)
				}
			}
			for _, candidate := range tr.Candidates() {
				if err := d.Dispatch(ctx, candidate); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					// Log dispatch errors but keep looping.
					continue
				}
			}
		}
	}
}

// Shutdown gracefully stops the orchestrator and any in-flight dispatches.
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	o.mu.RLock()
	d := o.dispatcher
	o.mu.RUnlock()

	if d != nil {
		d.Shutdown(ctx)
	}
	return nil
}
