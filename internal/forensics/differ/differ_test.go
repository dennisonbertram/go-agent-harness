package differ

import (
	"testing"

	"go-agent-harness/internal/forensics/rollout"
)

func TestDiff_IdenticalRuns(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0, Payload: map[string]any{"step": float64(0)}},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{"step": float64(1), "tool": "bash"}},
		{Type: "run.completed", Step: 2, Payload: map[string]any{"step": float64(2)}},
	}

	result := Diff(events, events)

	for _, sd := range result.StepDiffs {
		if sd.Status != "identical" {
			t.Errorf("step %d: expected identical, got %s", sd.Step, sd.Status)
		}
	}
	if result.OutcomeDiff != "identical" {
		t.Errorf("expected identical outcome, got %s", result.OutcomeDiff)
	}
	if result.CostDelta != 0 {
		t.Errorf("expected zero cost delta, got %f", result.CostDelta)
	}
}

func TestDiff_DifferentStepCounts(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "tool.call.started", Step: 1},
		{Type: "tool.call.completed", Step: 2},
		{Type: "run.completed", Step: 3},
	}

	result := Diff(a, b)

	// Step 0 should be identical (same type).
	foundOnlyInB := 0
	for _, sd := range result.StepDiffs {
		if sd.Status == "only_in_b" {
			foundOnlyInB++
		}
	}
	if foundOnlyInB == 0 {
		t.Error("expected at least one only_in_b step diff")
	}
}

func TestDiff_DivergedTypes(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "tool.call.started", Step: 1, Payload: map[string]any{"tool": "bash"}},
		{Type: "run.completed", Step: 2},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "llm.turn.requested", Step: 1, Payload: map[string]any{"model": "gpt-4"}},
		{Type: "run.completed", Step: 2},
	}

	result := Diff(a, b)

	found := false
	for _, sd := range result.StepDiffs {
		if sd.Step == 1 && sd.Status == "diverged" {
			found = true
		}
	}
	if !found {
		t.Error("expected step 1 to be diverged")
	}
}

func TestDiff_CostDelta(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "usage.delta", Step: 1, Payload: map[string]any{"cumulative_cost_usd": 0.00123}},
		{Type: "run.completed", Step: 2},
	}
	b := []rollout.RolloutEvent{
		{Type: "usage.delta", Step: 1, Payload: map[string]any{"cumulative_cost_usd": 0.00456}},
		{Type: "run.completed", Step: 2},
	}

	result := Diff(a, b)

	expectedDelta := 0.00456 - 0.00123
	if result.CostDelta < expectedDelta-0.0001 || result.CostDelta > expectedDelta+0.0001 {
		t.Errorf("expected cost delta ~%f, got %f", expectedDelta, result.CostDelta)
	}
}

func TestDiff_BFailedRegression(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.failed", Step: 1, Payload: map[string]any{"error": "timeout"}},
	}

	result := Diff(a, b)

	if result.OutcomeDiff != "b_failed" {
		t.Errorf("expected b_failed outcome, got %s", result.OutcomeDiff)
	}
}

func TestDiff_AFailedImprovement(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.failed", Step: 1, Payload: map[string]any{"error": "timeout"}},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}

	result := Diff(a, b)

	if result.OutcomeDiff != "a_failed" {
		t.Errorf("expected a_failed outcome, got %s", result.OutcomeDiff)
	}
}

func TestDiff_EmptyRuns(t *testing.T) {
	result := Diff(nil, nil)

	if len(result.StepDiffs) != 0 {
		t.Errorf("expected 0 step diffs, got %d", len(result.StepDiffs))
	}
	if result.OutcomeDiff != "diverged" {
		// Both unknown → diverged.
		t.Errorf("expected diverged for unknown outcomes, got %s", result.OutcomeDiff)
	}
}

func TestDiff_OnlyInA(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "tool.call.started", Step: 1},
		{Type: "run.completed", Step: 2},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 2},
	}

	result := Diff(a, b)

	found := false
	for _, sd := range result.StepDiffs {
		if sd.Step == 1 && sd.Status == "only_in_a" {
			found = true
		}
	}
	if !found {
		t.Error("expected step 1 to be only_in_a")
	}
}

func TestRunOutcome(t *testing.T) {
	tests := []struct {
		name     string
		events   []rollout.RolloutEvent
		expected string
	}{
		{
			name:     "completed",
			events:   []rollout.RolloutEvent{{Type: "run.completed"}},
			expected: "completed",
		},
		{
			name:     "failed",
			events:   []rollout.RolloutEvent{{Type: "run.failed"}},
			expected: "failed",
		},
		{
			name:     "unknown_empty",
			events:   nil,
			expected: "unknown",
		},
		{
			name:     "unknown_no_terminal",
			events:   []rollout.RolloutEvent{{Type: "run.started"}},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runOutcome(tt.events)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestExtractTotalCost(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "usage.delta", Payload: map[string]any{"cumulative_cost_usd": 0.001}},
		{Type: "usage.delta", Payload: map[string]any{"cumulative_cost_usd": 0.003}},
		{Type: "usage.delta", Payload: map[string]any{"cumulative_cost_usd": 0.002}},
	}

	cost := extractTotalCost(events)
	if cost != 0.003 {
		t.Errorf("expected 0.003, got %f", cost)
	}
}

func TestExtractTotalCost_Empty(t *testing.T) {
	cost := extractTotalCost(nil)
	if cost != 0 {
		t.Errorf("expected 0 for empty events, got %f", cost)
	}
}

func TestGroupByStep(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "a", Step: 0},
		{Type: "b", Step: 1},
		{Type: "c", Step: 1},
		{Type: "d", Step: 2},
	}

	groups := groupByStep(events)
	if len(groups[0]) != 1 {
		t.Errorf("expected 1 event at step 0, got %d", len(groups[0]))
	}
	if len(groups[1]) != 2 {
		t.Errorf("expected 2 events at step 1, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("expected 1 event at step 2, got %d", len(groups[2]))
	}
}

func TestMergeStepKeys(t *testing.T) {
	a := map[int][]rollout.RolloutEvent{0: {}, 1: {}, 3: {}}
	b := map[int][]rollout.RolloutEvent{0: {}, 2: {}, 3: {}}

	keys := mergeStepKeys(a, b)
	expected := []int{0, 1, 2, 3}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("key %d: expected %d, got %d", i, expected[i], k)
		}
	}
}
