// Command forensics provides CLI tools for analyzing and comparing rollout files.
//
// Usage:
//
//	forensics diff <rollout_a.jsonl> <rollout_b.jsonl>
package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"go-agent-harness/internal/forensics/differ"
	"go-agent-harness/internal/forensics/rollout"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "forensics: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: forensics <command> [args]\n\nCommands:\n  diff <rollout_a.jsonl> <rollout_b.jsonl>  Compare two rollout files")
	}

	switch args[0] {
	case "diff":
		return runDiff(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runDiff(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: forensics diff <rollout_a.jsonl> <rollout_b.jsonl>")
	}

	eventsA, err := rollout.LoadFile(args[0])
	if err != nil {
		return fmt.Errorf("loading run A: %w", err)
	}

	eventsB, err := rollout.LoadFile(args[1])
	if err != nil {
		return fmt.Errorf("loading run B: %w", err)
	}

	// Canonicalize before diffing.
	canonA := rollout.Canonicalize(eventsA, rollout.DefaultOptions)
	canonB := rollout.Canonicalize(eventsB, rollout.DefaultOptions)

	result := differ.Diff(canonA, canonB)

	printDiffResult(eventsA, eventsB, result)
	return nil
}

func printDiffResult(a, b []rollout.RolloutEvent, result differ.DiffResult) {
	stepsA := countMaxStep(a)
	stepsB := countMaxStep(b)
	costA := extractCost(a)
	costB := extractCost(b)

	fmt.Printf("Run A: %d steps, $%.5f\n", stepsA, costA)
	fmt.Printf("Run B: %d steps, $%.5f\n", stepsB, costB)

	// Count step statuses.
	identical, diverged, onlyA, onlyB := 0, 0, 0, 0
	for _, sd := range result.StepDiffs {
		switch sd.Status {
		case "identical":
			identical++
		case "diverged":
			diverged++
		case "only_in_a":
			onlyA++
		case "only_in_b":
			onlyB++
		}
	}

	parts := []string{}
	if identical > 0 {
		parts = append(parts, fmt.Sprintf("%d identical", identical))
	}
	if diverged > 0 {
		parts = append(parts, fmt.Sprintf("%d diverged", diverged))
	}
	if onlyA > 0 {
		parts = append(parts, fmt.Sprintf("%d only in A", onlyA))
	}
	if onlyB > 0 {
		parts = append(parts, fmt.Sprintf("%d only in B", onlyB))
	}

	fmt.Printf("Steps: ")
	for i, p := range parts {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s", p)
	}
	fmt.Println()

	// Winner summary.
	winnerLabel := "Tie"
	if result.Score.Winner == "a" {
		winnerLabel = "A"
	} else if result.Score.Winner == "b" {
		winnerLabel = "B"
	}

	reasons := ""
	for i, r := range result.Score.Reasons {
		if i > 0 {
			reasons += ", "
		}
		reasons += sanitize(r)
	}
	fmt.Printf("Winner: %s (%s)\n", sanitize(winnerLabel), reasons)
}

// sanitize removes ASCII control characters (including ANSI escape sequences)
// from untrusted strings before printing to the terminal.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1 // drop the rune
		}
		return r
	}, s)
}

func countMaxStep(events []rollout.RolloutEvent) int {
	max := 0
	for _, ev := range events {
		if ev.Step > max {
			max = ev.Step
		}
	}
	return max
}

func extractCost(events []rollout.RolloutEvent) float64 {
	var maxCost float64
	for _, ev := range events {
		if ev.Type == "usage.delta" && ev.Payload != nil {
			if c, ok := ev.Payload["cumulative_cost_usd"].(float64); ok && c > maxCost {
				maxCost = c
			}
		}
	}
	return maxCost
}
