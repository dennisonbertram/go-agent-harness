package differ

import (
	"fmt"

	"go-agent-harness/internal/forensics/rollout"
)

// RegressionScore summarizes which run is better and why.
type RegressionScore struct {
	Winner  string   `json:"winner"` // "a" | "b" | "tie"
	Reasons []string `json:"reasons"`
}

// Score evaluates two rollout event sequences and a diff result to determine
// which run is better. Scoring criteria:
//   - fewer errors → better
//   - lower cost → better
//   - fewer steps → better
//   - b fails where a succeeded → regression
func Score(a, b []rollout.RolloutEvent, diff DiffResult) RegressionScore {
	var aPoints, bPoints int
	var reasons []string

	// 1. Outcome comparison — heaviest weight.
	outcomeA := runOutcome(a)
	outcomeB := runOutcome(b)
	if outcomeA == "completed" && outcomeB == "failed" {
		aPoints += 3
		reasons = append(reasons, "B failed where A succeeded (regression)")
	} else if outcomeA == "failed" && outcomeB == "completed" {
		bPoints += 3
		reasons = append(reasons, "B succeeded where A failed (improvement)")
	}

	// 2. Error count comparison.
	errA := countErrors(a)
	errB := countErrors(b)
	if errA < errB {
		aPoints++
		reasons = append(reasons, fmt.Sprintf("fewer errors (A:%d vs B:%d)", errA, errB))
	} else if errB < errA {
		bPoints++
		reasons = append(reasons, fmt.Sprintf("fewer errors (A:%d vs B:%d)", errA, errB))
	}

	// 3. Cost comparison.
	costA := extractTotalCost(a)
	costB := extractTotalCost(b)
	if costA > 0 || costB > 0 {
		if costA < costB {
			aPoints++
			reasons = append(reasons, fmt.Sprintf("lower cost (A:$%.5f vs B:$%.5f)", costA, costB))
		} else if costB < costA {
			bPoints++
			reasons = append(reasons, fmt.Sprintf("lower cost (A:$%.5f vs B:$%.5f)", costA, costB))
		}
	}

	// 4. Step count comparison.
	stepsA := maxStep(a)
	stepsB := maxStep(b)
	if stepsA < stepsB {
		aPoints++
		reasons = append(reasons, fmt.Sprintf("fewer steps (A:%d vs B:%d)", stepsA, stepsB))
	} else if stepsB < stepsA {
		bPoints++
		reasons = append(reasons, fmt.Sprintf("fewer steps (A:%d vs B:%d)", stepsA, stepsB))
	}

	var winner string
	switch {
	case aPoints > bPoints:
		winner = "a"
	case bPoints > aPoints:
		winner = "b"
	default:
		winner = "tie"
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "runs are equivalent")
	}

	return RegressionScore{
		Winner:  winner,
		Reasons: reasons,
	}
}

// countErrors counts error-related events in a rollout.
func countErrors(events []rollout.RolloutEvent) int {
	count := 0
	for _, ev := range events {
		switch ev.Type {
		case "run.failed", "hook.failed", "tool_hook.failed",
			"memory.observe.failed", "skill.fork.failed", "error.context":
			count++
		}
	}
	return count
}

// maxStep returns the highest step number in the event sequence.
func maxStep(events []rollout.RolloutEvent) int {
	max := 0
	for _, ev := range events {
		if ev.Step > max {
			max = ev.Step
		}
	}
	return max
}
