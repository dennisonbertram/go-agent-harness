// Package differ provides run comparison and regression detection for
// JSONL rollout files. It compares two canonicalized event sequences
// step-by-step and produces a structured diff result.
package differ

import (
	"encoding/json"
	"fmt"

	"go-agent-harness/internal/forensics/rollout"
)

// StepDiff describes how a single step compares between two runs.
type StepDiff struct {
	Step    int    `json:"step"`
	Status  string `json:"status"` // "identical" | "diverged" | "only_in_a" | "only_in_b"
	TypeA   string `json:"type_a,omitempty"`
	TypeB   string `json:"type_b,omitempty"`
	Details string `json:"details,omitempty"`
}

// DiffResult is the outcome of comparing two rollout event sequences.
type DiffResult struct {
	StepDiffs   []StepDiff      `json:"step_diffs"`
	CostDelta   float64         `json:"cost_delta"` // b.cost - a.cost
	OutcomeDiff string          `json:"outcome_diff"`
	Score       RegressionScore `json:"score"`
}

// Diff compares two canonicalized rollout event sequences and returns
// a structured diff. Events should be canonicalized before calling Diff.
func Diff(a, b []rollout.RolloutEvent) DiffResult {
	stepsA := groupByStep(a)
	stepsB := groupByStep(b)

	// Collect all step numbers.
	allSteps := mergeStepKeys(stepsA, stepsB)

	var diffs []StepDiff
	for _, step := range allSteps {
		evA, inA := stepsA[step]
		evB, inB := stepsB[step]

		switch {
		case inA && !inB:
			diffs = append(diffs, StepDiff{
				Step:   step,
				Status: "only_in_a",
				TypeA:  summarizeTypes(evA),
			})
		case !inA && inB:
			diffs = append(diffs, StepDiff{
				Step:   step,
				Status: "only_in_b",
				TypeB:  summarizeTypes(evB),
			})
		default:
			if eventsEqual(evA, evB) {
				diffs = append(diffs, StepDiff{
					Step:   step,
					Status: "identical",
					TypeA:  summarizeTypes(evA),
					TypeB:  summarizeTypes(evB),
				})
			} else {
				diffs = append(diffs, StepDiff{
					Step:    step,
					Status:  "diverged",
					TypeA:   summarizeTypes(evA),
					TypeB:   summarizeTypes(evB),
					Details: describeDivergence(evA, evB),
				})
			}
		}
	}

	costA := extractTotalCost(a)
	costB := extractTotalCost(b)

	result := DiffResult{
		StepDiffs:   diffs,
		CostDelta:   costB - costA,
		OutcomeDiff: computeOutcomeDiff(a, b),
	}
	result.Score = Score(a, b, result)

	return result
}

// groupByStep groups events by their Step field.
func groupByStep(events []rollout.RolloutEvent) map[int][]rollout.RolloutEvent {
	m := make(map[int][]rollout.RolloutEvent)
	for _, ev := range events {
		m[ev.Step] = append(m[ev.Step], ev)
	}
	return m
}

// mergeStepKeys returns a sorted slice of all unique step numbers from both maps.
func mergeStepKeys(a, b map[int][]rollout.RolloutEvent) []int {
	seen := make(map[int]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	steps := make([]int, 0, len(seen))
	for k := range seen {
		steps = append(steps, k)
	}
	// Sort.
	for i := 0; i < len(steps); i++ {
		for j := i + 1; j < len(steps); j++ {
			if steps[i] > steps[j] {
				steps[i], steps[j] = steps[j], steps[i]
			}
		}
	}
	return steps
}

// summarizeTypes returns a comma-separated list of event types in the step.
func summarizeTypes(events []rollout.RolloutEvent) string {
	if len(events) == 0 {
		return ""
	}
	if len(events) == 1 {
		return events[0].Type
	}
	result := events[0].Type
	for _, ev := range events[1:] {
		result += "," + ev.Type
	}
	return result
}

// eventsEqual checks if two event slices have identical types and payloads.
func eventsEqual(a, b []rollout.RolloutEvent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return false
		}
		// Compare payloads via JSON serialization.
		pA, _ := json.Marshal(a[i].Payload)
		pB, _ := json.Marshal(b[i].Payload)
		if string(pA) != string(pB) {
			return false
		}
	}
	return true
}

// describeDivergence returns a human-readable description of how the events differ.
func describeDivergence(a, b []rollout.RolloutEvent) string {
	if len(a) != len(b) {
		return fmt.Sprintf("event count: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return fmt.Sprintf("event %d type: %s vs %s", i, a[i].Type, b[i].Type)
		}
	}
	return "payload differences"
}

// extractTotalCost extracts the cumulative cost from usage.delta or
// run.completed events.
func extractTotalCost(events []rollout.RolloutEvent) float64 {
	var maxCost float64
	for _, ev := range events {
		if ev.Type == "usage.delta" && ev.Payload != nil {
			if c, ok := ev.Payload["cumulative_cost_usd"].(float64); ok && c > maxCost {
				maxCost = c
			}
		}
		if ev.Type == "run.completed" && ev.Payload != nil {
			if ct, ok := ev.Payload["cost_totals"].(map[string]any); ok {
				if c, ok := ct["total_cost_usd"].(float64); ok && c > maxCost {
					maxCost = c
				}
			}
		}
	}
	return maxCost
}

// computeOutcomeDiff determines the overall outcome comparison.
func computeOutcomeDiff(a, b []rollout.RolloutEvent) string {
	outcomeA := runOutcome(a)
	outcomeB := runOutcome(b)

	switch {
	case outcomeA == "failed" && outcomeB == "failed":
		return "identical"
	case outcomeA == "completed" && outcomeB == "completed":
		return "identical"
	case outcomeA == "completed" && outcomeB == "failed":
		return "b_failed"
	case outcomeA == "failed" && outcomeB == "completed":
		return "a_failed"
	default:
		return "diverged"
	}
}

// runOutcome returns "completed", "failed", or "unknown" based on terminal events.
func runOutcome(events []rollout.RolloutEvent) string {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Type {
		case "run.completed":
			return "completed"
		case "run.failed":
			return "failed"
		}
	}
	return "unknown"
}
