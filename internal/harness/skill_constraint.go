package harness

import "sync"

// SkillConstraint represents an active skill's tool access restriction.
type SkillConstraint struct {
	SkillName    string   // name of the active skill
	AllowedTools []string // nil = no restriction (all tools allowed)
}

// AlwaysAvailableTools are tool names that bypass skill allowed-tools restrictions.
// These are infrastructure tools required for basic agent operation.
var AlwaysAvailableTools = map[string]bool{
	"AskUserQuestion": true,
	"find_tool":       true,
	"skill":           true,
}

// SkillConstraintTracker tracks active skill constraints per run.
// Thread-safe. A run can have at most one active skill constraint.
type SkillConstraintTracker struct {
	mu          sync.RWMutex
	constraints map[string]*SkillConstraint // runID -> constraint (nil entry = no constraint)
}

// NewSkillConstraintTracker creates a new SkillConstraintTracker.
func NewSkillConstraintTracker() *SkillConstraintTracker {
	return &SkillConstraintTracker{
		constraints: make(map[string]*SkillConstraint),
	}
}

// Activate sets the active skill constraint for a run.
// If a constraint already exists for this run, it is replaced.
func (t *SkillConstraintTracker) Activate(runID string, constraint SkillConstraint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.constraints[runID] = &constraint
}

// Deactivate clears the active skill constraint for a run.
func (t *SkillConstraintTracker) Deactivate(runID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.constraints, runID)
}

// Active returns the active skill constraint for a run, if any.
func (t *SkillConstraintTracker) Active(runID string) (*SkillConstraint, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	c, ok := t.constraints[runID]
	return c, ok
}

// IsToolAllowed returns true if the tool is permitted under the current
// constraints for the given run. Returns true if:
// - No constraint is active for the run.
// - The constraint has nil AllowedTools (no restriction).
// - The tool is in the AlwaysAvailableTools set.
// - The tool is in the constraint's AllowedTools list.
func (t *SkillConstraintTracker) IsToolAllowed(runID, toolName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	c, ok := t.constraints[runID]
	if !ok {
		return true // no constraint active
	}
	if c.AllowedTools == nil {
		return true // nil means no restriction
	}
	if AlwaysAvailableTools[toolName] {
		return true
	}
	for _, allowed := range c.AllowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// Cleanup removes all constraint state for a run.
func (t *SkillConstraintTracker) Cleanup(runID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.constraints, runID)
}
