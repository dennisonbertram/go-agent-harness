package harness

// TurnCounter tracks assistant turns in a conversation and enforces a hard
// budget. When maxTurns is 0 the counter is unlimited (Increment never
// returns exhausted=true).
type TurnCounter struct {
	count    int
	maxTurns int
}

// NewTurnCounter creates a counter with the given limit. 0 means unlimited.
func NewTurnCounter(maxTurns int) *TurnCounter {
	return &TurnCounter{maxTurns: maxTurns}
}

// Increment records one assistant turn and reports whether the budget is now
// exhausted. For an unlimited counter it always returns false.
func (tc *TurnCounter) Increment() (exhausted bool) {
	tc.count++
	if tc.maxTurns <= 0 {
		return false
	}
	return tc.count >= tc.maxTurns
}

// Count returns the total number of increments recorded so far.
func (tc *TurnCounter) Count() int {
	return tc.count
}

// Remaining returns the number of turns still available.
// Returns -1 if the counter is unlimited (maxTurns == 0).
// Returns 0 when the budget is exactly exhausted or over.
func (tc *TurnCounter) Remaining() int {
	if tc.maxTurns <= 0 {
		return -1
	}
	r := tc.maxTurns - tc.count
	if r < 0 {
		return 0
	}
	return r
}

// IsUnlimited returns true when maxTurns is 0 (no limit applies).
func (tc *TurnCounter) IsUnlimited() bool {
	return tc.maxTurns <= 0
}

// AgentLimitsConfig holds turn-budget configuration for background and forked agents.
// These values are read from the [agent_limits] section of a TOML config file.
type AgentLimitsConfig struct {
	// DefaultMaxTurns is the default turn budget for top-level agents. 0 = unlimited.
	DefaultMaxTurns int `toml:"default_max_turns"`
	// ForkedAgentMaxTurns is the default turn budget for forked/skill subagents. 0 = unlimited.
	ForkedAgentMaxTurns int `toml:"forked_agent_max_turns"`
	// BackgroundAgentMaxTurns is the default turn budget for background extraction agents. 0 = unlimited.
	BackgroundAgentMaxTurns int `toml:"background_agent_max_turns"`
}

// DefaultAgentLimits returns the built-in default agent limit configuration.
// DefaultMaxTurns is 0 (unlimited) to preserve backward compatibility.
// ForkedAgentMaxTurns is 5 to provide a reasonable default for subagents.
// BackgroundAgentMaxTurns is 2 because background extraction agents should be brief.
func DefaultAgentLimits() AgentLimitsConfig {
	return AgentLimitsConfig{
		DefaultMaxTurns:         0,
		ForkedAgentMaxTurns:     5,
		BackgroundAgentMaxTurns: 2,
	}
}
