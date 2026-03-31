package harness

import (
	"testing"

	"github.com/BurntSushi/toml"

	htools "go-agent-harness/internal/harness/tools"
)

// TestNewTurnCounter_ZeroIsUnlimited verifies that MaxTurns=0 creates an unlimited counter.
func TestNewTurnCounter_ZeroIsUnlimited(t *testing.T) {
	tc := NewTurnCounter(0)
	if !tc.IsUnlimited() {
		t.Error("NewTurnCounter(0).IsUnlimited() = false, want true")
	}
}

// TestNewTurnCounter_PositiveLimit verifies that MaxTurns=5 creates a limited counter.
func TestNewTurnCounter_PositiveLimit(t *testing.T) {
	tc := NewTurnCounter(5)
	if tc.IsUnlimited() {
		t.Error("NewTurnCounter(5).IsUnlimited() = true, want false")
	}
}

// TestIncrement_UnderBudget verifies that increments below the limit return exhausted=false.
func TestIncrement_UnderBudget(t *testing.T) {
	tc := NewTurnCounter(5)
	for i := 0; i < 3; i++ {
		if exhausted := tc.Increment(); exhausted {
			t.Errorf("increment %d: exhausted = true, want false (limit 5)", i+1)
		}
	}
}

// TestIncrement_ExactBudget verifies that the Nth increment (where N == limit) returns exhausted=true.
func TestIncrement_ExactBudget(t *testing.T) {
	tc := NewTurnCounter(5)
	var exhausted bool
	for i := 0; i < 5; i++ {
		exhausted = tc.Increment()
	}
	if !exhausted {
		t.Error("5th increment with limit 5: exhausted = false, want true")
	}
}

// TestIncrement_OverBudget verifies that increments beyond the limit also return exhausted=true.
func TestIncrement_OverBudget(t *testing.T) {
	tc := NewTurnCounter(5)
	for i := 0; i < 5; i++ {
		tc.Increment()
	}
	if exhausted := tc.Increment(); !exhausted {
		t.Error("6th increment with limit 5: exhausted = false, want true")
	}
}

// TestIncrement_Unlimited verifies that 100 increments with limit 0 never exhaust.
func TestIncrement_Unlimited(t *testing.T) {
	tc := NewTurnCounter(0)
	for i := 0; i < 100; i++ {
		if exhausted := tc.Increment(); exhausted {
			t.Errorf("increment %d: exhausted = true for unlimited counter", i+1)
		}
	}
}

// TestCount_TracksCorrectly verifies that Count() returns the number of increments so far.
func TestCount_TracksCorrectly(t *testing.T) {
	tc := NewTurnCounter(10)
	for i := 0; i < 3; i++ {
		tc.Increment()
	}
	if got := tc.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3 after 3 increments", got)
	}
}

// TestRemaining_UnderBudget verifies Remaining() returns budget - used when under budget.
func TestRemaining_UnderBudget(t *testing.T) {
	tc := NewTurnCounter(5)
	tc.Increment()
	tc.Increment()
	if got := tc.Remaining(); got != 3 {
		t.Errorf("Remaining() = %d, want 3 (limit 5, used 2)", got)
	}
}

// TestRemaining_Unlimited verifies Remaining() returns -1 for unlimited counters.
func TestRemaining_Unlimited(t *testing.T) {
	tc := NewTurnCounter(0)
	if got := tc.Remaining(); got != -1 {
		t.Errorf("Remaining() = %d, want -1 for unlimited counter", got)
	}
}

// TestRemaining_Exhausted verifies Remaining() returns 0 when budget is exactly used.
func TestRemaining_Exhausted(t *testing.T) {
	tc := NewTurnCounter(5)
	for i := 0; i < 5; i++ {
		tc.Increment()
	}
	if got := tc.Remaining(); got != 0 {
		t.Errorf("Remaining() = %d, want 0 when budget is exhausted (limit 5, used 5)", got)
	}
}

// TestAgentLimitsConfig_Defaults verifies that default AgentLimits values are correct.
func TestAgentLimitsConfig_Defaults(t *testing.T) {
	cfg := DefaultAgentLimits()
	if cfg.DefaultMaxTurns != 0 {
		t.Errorf("DefaultMaxTurns = %d, want 0 (unlimited)", cfg.DefaultMaxTurns)
	}
	if cfg.ForkedAgentMaxTurns != 5 {
		t.Errorf("ForkedAgentMaxTurns = %d, want 5", cfg.ForkedAgentMaxTurns)
	}
	if cfg.BackgroundAgentMaxTurns != 2 {
		t.Errorf("BackgroundAgentMaxTurns = %d, want 2", cfg.BackgroundAgentMaxTurns)
	}
}

// TestAgentLimitsConfig_FromTOML verifies that [agent_limits] parses correctly from TOML.
func TestAgentLimitsConfig_FromTOML(t *testing.T) {
	input := `
[agent_limits]
default_max_turns = 10
forked_agent_max_turns = 7
background_agent_max_turns = 3
`
	type tomlRoot struct {
		AgentLimits AgentLimitsConfig `toml:"agent_limits"`
	}
	var root tomlRoot
	if _, err := toml.Decode(input, &root); err != nil {
		t.Fatalf("toml.Decode failed: %v", err)
	}
	if root.AgentLimits.DefaultMaxTurns != 10 {
		t.Errorf("DefaultMaxTurns = %d, want 10", root.AgentLimits.DefaultMaxTurns)
	}
	if root.AgentLimits.ForkedAgentMaxTurns != 7 {
		t.Errorf("ForkedAgentMaxTurns = %d, want 7", root.AgentLimits.ForkedAgentMaxTurns)
	}
	if root.AgentLimits.BackgroundAgentMaxTurns != 3 {
		t.Errorf("BackgroundAgentMaxTurns = %d, want 3", root.AgentLimits.BackgroundAgentMaxTurns)
	}
}

// TestForkConfig_MaxTurns verifies that the MaxTurns field exists on ForkConfig
// and correctly holds the configured value.
// This test fails to compile if MaxTurns is not defined on htools.ForkConfig.
func TestForkConfig_MaxTurns(t *testing.T) {
	cfg := htools.ForkConfig{
		SkillName: "test-skill",
		MaxTurns:  8,
	}
	if cfg.MaxTurns != 8 {
		t.Errorf("ForkConfig.MaxTurns = %d, want 8", cfg.MaxTurns)
	}
}

// TestTurnBudgetExhausted_EventEmitted is a regression test that verifies
// the EventRunTurnBudgetExhausted event constant exists and is distinct from
// other event types (catches regressions where the event is accidentally removed).
func TestTurnBudgetExhausted_EventEmitted(t *testing.T) {
	// The event must exist as a named constant.
	evt := EventRunTurnBudgetExhausted
	if evt == "" {
		t.Error("EventRunTurnBudgetExhausted is empty string")
	}
	// It must be included in AllEventTypes().
	found := false
	for _, et := range AllEventTypes() {
		if et == EventRunTurnBudgetExhausted {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("EventRunTurnBudgetExhausted (%q) not found in AllEventTypes()", evt)
	}
}
