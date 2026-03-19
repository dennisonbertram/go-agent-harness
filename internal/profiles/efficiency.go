package profiles

import (
	"time"
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
