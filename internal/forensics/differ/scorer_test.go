package differ

import (
	"strings"
	"testing"

	"go-agent-harness/internal/forensics/rollout"
)

func TestScore_AWinsFewerStepsAndCost(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "usage.delta", Step: 1, Payload: map[string]any{"cumulative_cost_usd": 0.001}},
		{Type: "run.completed", Step: 2},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "usage.delta", Step: 1, Payload: map[string]any{"cumulative_cost_usd": 0.005}},
		{Type: "tool.call.started", Step: 3},
		{Type: "tool.call.completed", Step: 4},
		{Type: "run.completed", Step: 5},
	}

	diff := DiffResult{OutcomeDiff: "identical"}
	score := Score(a, b, diff)

	if score.Winner != "a" {
		t.Errorf("expected winner=a, got %s", score.Winner)
	}
	if len(score.Reasons) == 0 {
		t.Error("expected reasons to be non-empty")
	}
}

func TestScore_BWinsImprovement(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.failed", Step: 1, Payload: map[string]any{"error": "timeout"}},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}

	diff := DiffResult{OutcomeDiff: "a_failed"}
	score := Score(a, b, diff)

	if score.Winner != "b" {
		t.Errorf("expected winner=b, got %s", score.Winner)
	}
	hasImprovement := false
	for _, r := range score.Reasons {
		if strings.Contains(r, "improvement") {
			hasImprovement = true
		}
	}
	if !hasImprovement {
		t.Error("expected improvement reason")
	}
}

func TestScore_BFailsRegression(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.failed", Step: 1, Payload: map[string]any{"error": "crash"}},
	}

	diff := DiffResult{OutcomeDiff: "b_failed"}
	score := Score(a, b, diff)

	if score.Winner != "a" {
		t.Errorf("expected winner=a, got %s", score.Winner)
	}
	hasRegression := false
	for _, r := range score.Reasons {
		if strings.Contains(r, "regression") {
			hasRegression = true
		}
	}
	if !hasRegression {
		t.Error("expected regression reason")
	}
}

func TestScore_Tie(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}

	diff := DiffResult{OutcomeDiff: "identical"}
	score := Score(events, events, diff)

	if score.Winner != "tie" {
		t.Errorf("expected winner=tie, got %s", score.Winner)
	}
}

func TestScore_FewerErrors(t *testing.T) {
	a := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "hook.failed", Step: 1},
		{Type: "hook.failed", Step: 2},
		{Type: "run.completed", Step: 3},
	}
	b := []rollout.RolloutEvent{
		{Type: "run.started", Step: 0},
		{Type: "run.completed", Step: 1},
	}

	diff := DiffResult{OutcomeDiff: "identical"}
	score := Score(a, b, diff)

	if score.Winner != "b" {
		t.Errorf("expected winner=b (fewer errors), got %s", score.Winner)
	}
}

func TestCountErrors(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Type: "run.started"},
		{Type: "run.failed"},
		{Type: "hook.failed"},
		{Type: "tool_hook.failed"},
		{Type: "memory.observe.failed"},
		{Type: "skill.fork.failed"},
		{Type: "error.context"},
		{Type: "run.completed"},
	}

	count := countErrors(events)
	if count != 6 {
		t.Errorf("expected 6 errors, got %d", count)
	}
}

func TestMaxStep(t *testing.T) {
	events := []rollout.RolloutEvent{
		{Step: 0},
		{Step: 3},
		{Step: 1},
		{Step: 5},
		{Step: 2},
	}

	max := maxStep(events)
	if max != 5 {
		t.Errorf("expected max step 5, got %d", max)
	}
}

func TestMaxStep_Empty(t *testing.T) {
	max := maxStep(nil)
	if max != 0 {
		t.Errorf("expected 0 for empty, got %d", max)
	}
}

func TestScore_EmptyEvents(t *testing.T) {
	diff := DiffResult{OutcomeDiff: "diverged"}
	score := Score(nil, nil, diff)

	if score.Winner != "tie" {
		t.Errorf("expected tie for empty events, got %s", score.Winner)
	}
}
