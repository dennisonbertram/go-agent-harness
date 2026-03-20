package profiles

import (
	"time"

	"go-agent-harness/internal/store"
)

// EfficiencyThreshold is the minimum score below which an efficiency suggestion is emitted.
const EfficiencyThreshold = 0.6

// RunStats holds the minimal run statistics needed to compute an efficiency score.
type RunStats struct {
	RunID       string
	ProfileName string
	Steps       int
	CostUSD     float64
	AllowedTools []string
	UsedTools   []string // tool names that were actually called
}

// ScoreEfficiency computes a deterministic efficiency score for a run.
// Uses the same formula as internal/training/scorer.go:
//
//	efficiency = 1.0 / (1.0 + steps*0.1 + costUSD*10.0)
func ScoreEfficiency(steps int, costUSD float64) float64 {
	s := float64(steps)
	if s <= 0 {
		s = 1
	}
	e := 1.0 / (1.0 + s*0.1 + costUSD*10.0)
	if e > 1.0 {
		e = 1.0
	}
	if e < 0 {
		e = 0
	}
	return e
}

// BuildEfficiencyReport constructs an EfficiencyReport from run statistics.
// This is the deterministic (non-LLM) phase of the efficiency review.
func BuildEfficiencyReport(stats RunStats) EfficiencyReport {
	score := ScoreEfficiency(stats.Steps, stats.CostUSD)

	// Find unused tools: tools in AllowedTools that were never called.
	usedSet := make(map[string]bool, len(stats.UsedTools))
	for _, t := range stats.UsedTools {
		usedSet[t] = true
	}

	var unused []string
	for _, t := range stats.AllowedTools {
		if !usedSet[t] {
			unused = append(unused, t)
		}
	}

	// Suggest removing unused tools when the profile has explicit allow list.
	var refinements ProfileRefinements
	if len(unused) > 0 {
		refinements.RemoveTools = unused
	}
	// Suggest lower max_steps if actual usage was well below limit.
	// (We don't know the limit here — caller can set MaxStepsSuggestion if desired.)

	return EfficiencyReport{
		RunID:                stats.RunID,
		ProfileName:          stats.ProfileName,
		EfficiencyScore:      score,
		UnusedTools:          unused,
		SuggestedRefinements: refinements,
		CreatedAt:            time.Now().UTC(),
	}
}

// ShouldEmitSuggestion reports whether an efficiency suggestion event should be
// emitted for a run based on its score.
func ShouldEmitSuggestion(score float64) bool {
	return score < EfficiencyThreshold
}

// RunCompletionData holds the additional fields needed beyond RunStats to
// build a complete ProfileRunRecord for persistence.
type RunCompletionData struct {
	// RecordID is a unique identifier for this profile run record (e.g. a UUID).
	// When empty, callers should generate one before persisting.
	RecordID   string
	Status     string // "completed" | "failed" | "partial"
	StartedAt  time.Time
	FinishedAt time.Time
}

// BuildProfileRunRecord converts run statistics and completion data into a
// store.ProfileRunRecord suitable for persistence via store.SQLiteProfileRunStore.
//
// TopTools is populated from the top-3 most frequently used tools in
// stats.UsedTools (preserving first-occurrence order when counts tie).
// ToolCalls is set to len(stats.UsedTools).
//
// This function does NOT call the store; callers are responsible for persisting
// the returned record via store.SQLiteProfileRunStore.RecordProfileRun.
func BuildProfileRunRecord(stats RunStats, completion RunCompletionData) store.ProfileRunRecord {
	// Count tool usage frequency.
	counts := make(map[string]int, len(stats.UsedTools))
	order := make([]string, 0, len(stats.UsedTools))
	for _, t := range stats.UsedTools {
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}

	// Sort by frequency descending, preserving first-occurrence as tiebreaker.
	topN := 3
	if len(order) < topN {
		topN = len(order)
	}
	// Simple selection sort for the top-N (small N, O(N²) is fine).
	top := make([]string, 0, topN)
	remaining := append([]string(nil), order...)
	for i := 0; i < topN; i++ {
		best := 0
		for j := 1; j < len(remaining); j++ {
			if counts[remaining[j]] > counts[remaining[best]] {
				best = j
			}
		}
		top = append(top, remaining[best])
		remaining = append(remaining[:best], remaining[best+1:]...)
	}

	status := completion.Status
	if status == "" {
		status = "completed"
	}

	return store.ProfileRunRecord{
		ID:          completion.RecordID,
		ProfileName: stats.ProfileName,
		RunID:       stats.RunID,
		Status:      status,
		StepCount:   stats.Steps,
		CostUSD:     stats.CostUSD,
		StartedAt:   completion.StartedAt,
		FinishedAt:  completion.FinishedAt,
		ToolCalls:   len(stats.UsedTools),
		TopTools:    top,
	}
}
