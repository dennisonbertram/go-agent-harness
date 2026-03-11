package symphd

import (
	"context"
	"sync"
	"time"
)

// Orchestrator coordinates agent dispatch across workspaces.
type Orchestrator struct {
	config      *Config
	startedAt   time.Time
	mu          sync.RWMutex
	agents      int
	tracker     Tracker
	retryPolicy RetryPolicy
	deadLetters *DeadLetterQueue
}

// NewOrchestrator creates a new Orchestrator with the given config.
// If the config has GitHubOwner and GitHubRepo set, a GitHubTracker is
// initialised automatically.
func NewOrchestrator(cfg *Config) *Orchestrator {
	o := &Orchestrator{
		config:    cfg,
		startedAt: time.Now(),
		retryPolicy: RetryPolicy{
			MaxAttempts: cfg.RetryMaxAttempts,
			BaseDelayMs: cfg.RetryBaseDelayMs,
			MaxDelayMs:  cfg.RetryMaxDelayMs,
		},
		deadLetters: NewDeadLetterQueue(),
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

// DeadLetters returns the current dead letter queue items.
func (o *Orchestrator) DeadLetters() []*DeadLetter {
	return o.deadLetters.Items()
}

// RetryFailed checks a failed issue and either resets it for another attempt
// or moves it to the dead letter queue. Returns true if the issue was retried,
// false if it was dead-lettered.
func (o *Orchestrator) RetryFailed(issue *TrackedIssue, lastErr string) bool {
	o.mu.RLock()
	tr := o.tracker
	policy := o.retryPolicy
	dlq := o.deadLetters
	o.mu.RUnlock()

	if policy.ShouldRetry(issue.Attempts) {
		if tr != nil {
			_ = tr.Reset(issue.Number)
		}
		return true
	}
	dlq.Add(issue, lastErr)
	return false
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
