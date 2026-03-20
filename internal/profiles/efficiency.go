package profiles

import (
	"strings"
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

// ProfileStats holds aggregate statistics across multiple runs of a named profile.
// It is used as input to BuildAggregateReport and GenerateSuggestions.
// This is a read-only view — nothing here auto-applies changes to a profile.
type ProfileStats struct {
	ProfileName string
	RunCount    int
	AvgSteps    float64
	AvgCostUSD  float64
	SuccessRate float64
	TopTools    []string
	// MaxSteps is the currently configured max_steps for the profile (0 = no limit).
	MaxSteps int
}

// AggregateReport is the suggest-only efficiency report computed from aggregate run history.
// Suggestions are never auto-applied — they are read-only guidance for human or automated review.
type AggregateReport struct {
	ProfileName string    `json:"profile_name"`
	GeneratedAt time.Time `json:"generated_at"`
	RunCount    int       `json:"run_count"`
	AvgSteps    float64   `json:"avg_steps"`
	AvgCostUSD  float64   `json:"avg_cost_usd"`
	SuccessRate float64   `json:"success_rate"`
	TopTools    []string  `json:"top_tools"`
	// Suggestions contains suggest-only guidance. Never auto-applied.
	Suggestions []string `json:"suggestions"`
	// HasHistory is false when there is no run history (RunCount < 3).
	HasHistory bool `json:"has_history"`
}

const minRunsForSuggestions = 3

// GenerateSuggestions returns a list of suggest-only refinement hints for a profile.
// Suggestions are NEVER auto-applied — they are guidance only.
//
// Rules:
//   - < 3 runs → single "Not enough history" message
//   - success_rate < 0.5 → suggest reviewing profile prompt or constraints
//   - avg_steps > 20 and no step limit (MaxSteps == 0) → suggest adding max_steps
//   - otherwise → empty (healthy profile, no suggestions needed)
func GenerateSuggestions(stats ProfileStats) []string {
	if stats.RunCount < minRunsForSuggestions {
		return []string{"Not enough history to generate suggestions (need ≥ 3 runs)"}
	}

	var suggestions []string

	if stats.SuccessRate < 0.5 {
		suggestions = append(suggestions,
			"Low success rate ("+formatPct(stats.SuccessRate)+"): consider reviewing the profile's system prompt or adjusting task constraints.")
	}

	if stats.AvgSteps > 20 && stats.MaxSteps == 0 {
		suggestions = append(suggestions,
			"Average step count is high ("+formatFloat(stats.AvgSteps)+" steps): consider adding a max_steps limit to the profile.")
	}

	return suggestions
}

// BuildAggregateReport constructs an AggregateReport from a profile name and its aggregate stats.
// If stats.RunCount < 3, the report will have HasHistory=false and a single not-enough-history suggestion.
func BuildAggregateReport(profileName string, stats ProfileStats) AggregateReport {
	suggestions := GenerateSuggestions(stats)
	hasHistory := stats.RunCount >= minRunsForSuggestions

	topTools := append([]string(nil), stats.TopTools...)

	return AggregateReport{
		ProfileName: profileName,
		GeneratedAt: time.Now().UTC(),
		RunCount:    stats.RunCount,
		AvgSteps:    stats.AvgSteps,
		AvgCostUSD:  stats.AvgCostUSD,
		SuccessRate: stats.SuccessRate,
		TopTools:    topTools,
		Suggestions: suggestions,
		HasHistory:  hasHistory,
	}
}

// formatPct formats a float64 as a percentage string (e.g. 0.42 → "42%").
func formatPct(v float64) string {
	pct := int(v * 100)
	return strings.TrimSpace(string(rune('0'+pct/100)) + string(rune('0'+(pct/10)%10)) + string(rune('0'+pct%10)) + "%")
}

// formatFloat formats a float64 to 1 decimal place without importing fmt.
func formatFloat(v float64) string {
	// Use strconv-style manual formatting to keep the import list clean.
	// For reasonable step counts (0-999) this is sufficient.
	whole := int(v)
	frac := int((v-float64(whole))*10+0.5) % 10
	wholeStr := intToStr(whole)
	return wholeStr + "." + string(rune('0'+frac))
}

// intToStr converts a non-negative integer to its decimal string representation.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
