package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/profiles"
)

// minimalRunner creates a Runner with no provider — only suitable for unit
// testing internal state management and event emission.
func minimalRunner() *Runner {
	return NewRunner(nil, NewRegistry(), RunnerConfig{})
}

// TestMaybeEmitProfileEfficiencySuggestion_EmittedWhenLowScore verifies that
// the efficiency suggestion event is emitted when score < threshold.
func TestMaybeEmitProfileEfficiencySuggestion_EmittedWhenLowScore(t *testing.T) {
	t.Parallel()

	r := minimalRunner()
	runID := "run-efficiency-test-001"

	// Create run state with a profile name and high step count (low efficiency).
	r.mu.Lock()
	r.runs[runID] = &runState{
		run: Run{
			ID:     runID,
			Status: RunStatusCompleted,
		},
		profileName: "researcher",
		currentStep: 50, // High step count → low efficiency score
		subscribers: make(map[chan Event]struct{}),
	}
	r.mu.Unlock()

	// Score = 1/(1 + 50*0.1 + 0.0*10) = 1/6 ≈ 0.167 < 0.6 threshold
	score := profiles.ScoreEfficiency(50, 0.0)
	require.True(t, profiles.ShouldEmitSuggestion(score), "score should be below threshold for this test to be meaningful")

	// Subscribe to the run to capture events.
	_, ch, cancel, err := r.Subscribe(runID)
	require.NoError(t, err)
	defer cancel()

	// Trigger the efficiency suggestion.
	r.maybeEmitProfileEfficiencySuggestion(runID, 0.0)

	// Collect the emitted event.
	var ev Event
	select {
	case ev = <-ch:
	default:
		t.Fatal("expected efficiency suggestion event but channel was empty")
	}

	assert.Equal(t, EventProfileEfficiencySuggestion, ev.Type)
	assert.Equal(t, "researcher", ev.Payload["profile_name"])
	assert.Equal(t, runID, ev.Payload["run_id"])
	effScore, ok := ev.Payload["efficiency_score"].(float64)
	require.True(t, ok)
	assert.Less(t, effScore, 0.6)
}

// TestMaybeEmitProfileEfficiencySuggestion_NotEmittedWhenHighScore verifies
// that the efficiency suggestion event is NOT emitted when score >= threshold.
func TestMaybeEmitProfileEfficiencySuggestion_NotEmittedWhenHighScore(t *testing.T) {
	t.Parallel()

	r := minimalRunner()
	runID := "run-efficiency-test-002"

	// Create run state with a profile name and low step count (high efficiency).
	r.mu.Lock()
	r.runs[runID] = &runState{
		run: Run{
			ID:     runID,
			Status: RunStatusCompleted,
		},
		profileName: "bash-runner",
		currentStep: 1, // Low step count → high efficiency score
		subscribers: make(map[chan Event]struct{}),
	}
	r.mu.Unlock()

	// Score = 1/(1 + 1*0.1 + 0.0*10) = 1/1.1 ≈ 0.91 >= 0.6 threshold
	score := profiles.ScoreEfficiency(1, 0.0)
	require.False(t, profiles.ShouldEmitSuggestion(score), "score should be above threshold for this test to be meaningful")

	// Subscribe to capture events.
	_, ch, cancel, err := r.Subscribe(runID)
	require.NoError(t, err)
	defer cancel()

	// Trigger efficiency check — should emit nothing.
	r.maybeEmitProfileEfficiencySuggestion(runID, 0.0)

	// Channel should be empty.
	select {
	case ev := <-ch:
		t.Errorf("unexpected event emitted: %+v", ev)
	default:
		// Expected: no event.
	}
}

// TestMaybeEmitProfileEfficiencySuggestion_NotEmittedWithoutProfile verifies
// that no event is emitted when the run has no profile name.
func TestMaybeEmitProfileEfficiencySuggestion_NotEmittedWithoutProfile(t *testing.T) {
	t.Parallel()

	r := minimalRunner()
	runID := "run-efficiency-test-003"

	r.mu.Lock()
	r.runs[runID] = &runState{
		run: Run{
			ID:     runID,
			Status: RunStatusCompleted,
		},
		profileName: "", // No profile.
		currentStep: 100,
		subscribers: make(map[chan Event]struct{}),
	}
	r.mu.Unlock()

	_, ch, cancel, err := r.Subscribe(runID)
	require.NoError(t, err)
	defer cancel()

	r.maybeEmitProfileEfficiencySuggestion(runID, 5.0)

	select {
	case ev := <-ch:
		t.Errorf("unexpected event emitted for run without profile: %+v", ev)
	default:
		// Expected: no event.
	}
}
