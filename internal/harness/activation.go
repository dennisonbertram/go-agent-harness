package harness

import (
	"sync"

	htools "go-agent-harness/internal/harness/tools"
)

// ActivationTracker tracks which deferred tools have been activated per run.
// It is thread-safe and implements tools.ActivationTrackerInterface.
type ActivationTracker struct {
	mu     sync.RWMutex
	active map[string]map[string]bool // runID -> toolName -> true
}

// NewActivationTracker creates a new ActivationTracker.
func NewActivationTracker() *ActivationTracker {
	return &ActivationTracker{
		active: make(map[string]map[string]bool),
	}
}

// Activate marks the given tool names as active for the specified run.
func (t *ActivationTracker) Activate(runID string, toolNames ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active[runID] == nil {
		t.active[runID] = make(map[string]bool)
	}
	for _, name := range toolNames {
		t.active[runID][name] = true
	}
}

// IsActive returns true if the named tool is active for the specified run.
func (t *ActivationTracker) IsActive(runID string, toolName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active[runID][toolName]
}

// ActiveTools returns all activated tool names for a run.
func (t *ActivationTracker) ActiveTools(runID string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := t.active[runID]
	if len(m) == 0 {
		return nil
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

// Cleanup removes all activation state for a run.
func (t *ActivationTracker) Cleanup(runID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.active, runID)
}

// Verify interface compliance at compile time.
var _ htools.ActivationTrackerInterface = (*ActivationTracker)(nil)
