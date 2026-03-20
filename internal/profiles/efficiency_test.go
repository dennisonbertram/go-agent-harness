package profiles

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScoreEfficiency(t *testing.T) {
	tests := []struct {
		name    string
		steps   int
		costUSD float64
		wantMin float64
		wantMax float64
	}{
		{"zero steps uses 1", 0, 0.0, 0.90, 1.0},
		{"1 step zero cost", 1, 0.0, 0.90, 1.0},
		{"10 steps zero cost", 10, 0.0, 0.45, 0.55},
		{"1 step 1.0 cost", 1, 1.0, 0.07, 0.10},
		{"100 steps 5.0 cost", 100, 5.0, 0.005, 0.02},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ScoreEfficiency(tt.steps, tt.costUSD)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
			assert.GreaterOrEqual(t, score, tt.wantMin, "score too low")
			assert.LessOrEqual(t, score, tt.wantMax, "score too high")
		})
	}
}

func TestShouldEmitSuggestion(t *testing.T) {
	assert.True(t, ShouldEmitSuggestion(0.0), "score 0 should suggest")
	assert.True(t, ShouldEmitSuggestion(0.5), "score 0.5 < threshold should suggest")
	assert.True(t, ShouldEmitSuggestion(0.59), "score just below threshold should suggest")
	assert.False(t, ShouldEmitSuggestion(0.6), "score at threshold should NOT suggest")
	assert.False(t, ShouldEmitSuggestion(0.9), "high score should NOT suggest")
	assert.False(t, ShouldEmitSuggestion(1.0), "perfect score should NOT suggest")
}

func TestBuildEfficiencyReport(t *testing.T) {
	stats := RunStats{
		RunID:        "run-123",
		ProfileName:  "researcher",
		Steps:        5,
		CostUSD:      0.01,
		AllowedTools: []string{"read", "grep", "glob", "web_search"},
		UsedTools:    []string{"read", "grep"},
	}

	report := BuildEfficiencyReport(stats)

	assert.Equal(t, "run-123", report.RunID)
	assert.Equal(t, "researcher", report.ProfileName)
	assert.Greater(t, report.EfficiencyScore, 0.0)
	assert.LessOrEqual(t, report.EfficiencyScore, 1.0)
	// glob and web_search were unused
	assert.ElementsMatch(t, []string{"glob", "web_search"}, report.UnusedTools)
	// Unused tools should be suggested for removal
	assert.ElementsMatch(t, []string{"glob", "web_search"}, report.SuggestedRefinements.RemoveTools)
	assert.False(t, report.CreatedAt.IsZero())
}

func TestBuildEfficiencyReportNoUnusedTools(t *testing.T) {
	stats := RunStats{
		RunID:        "run-456",
		ProfileName:  "bash-runner",
		Steps:        3,
		CostUSD:      0.005,
		AllowedTools: []string{"bash"},
		UsedTools:    []string{"bash"},
	}

	report := BuildEfficiencyReport(stats)
	assert.Empty(t, report.UnusedTools)
	assert.Empty(t, report.SuggestedRefinements.RemoveTools)
}

func TestBuildEfficiencyReportEmptyAllowedTools(t *testing.T) {
	// When AllowedTools is empty (all tools allowed), no "unused" tools can be identified.
	stats := RunStats{
		RunID:        "run-789",
		ProfileName:  "full",
		Steps:        10,
		CostUSD:      0.10,
		AllowedTools: nil,
		UsedTools:    []string{"bash", "read"},
	}

	report := BuildEfficiencyReport(stats)
	assert.Empty(t, report.UnusedTools)
}

func TestBuildProfileRunRecord(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	stats := RunStats{
		RunID:       "run-xyz",
		ProfileName: "researcher",
		Steps:       7,
		CostUSD:     0.03,
		UsedTools:   []string{"read", "grep", "read", "glob", "read", "grep", "bash"},
	}
	completion := RunCompletionData{
		RecordID:   "rec-1",
		Status:     "completed",
		StartedAt:  now,
		FinishedAt: now.Add(20 * time.Second),
	}

	rec := BuildProfileRunRecord(stats, completion)

	assert.Equal(t, "rec-1", rec.ID)
	assert.Equal(t, "researcher", rec.ProfileName)
	assert.Equal(t, "run-xyz", rec.RunID)
	assert.Equal(t, "completed", rec.Status)
	assert.Equal(t, 7, rec.StepCount)
	assert.InDelta(t, 0.03, rec.CostUSD, 0.0001)
	assert.Equal(t, now, rec.StartedAt)
	assert.Equal(t, now.Add(20*time.Second), rec.FinishedAt)
	// ToolCalls = len(UsedTools) = 7
	assert.Equal(t, 7, rec.ToolCalls)
	// Top 3 by frequency: read(3), grep(2), then glob or bash(1) — order of first occurrence
	assert.Len(t, rec.TopTools, 3)
	assert.Equal(t, "read", rec.TopTools[0])
	assert.Equal(t, "grep", rec.TopTools[1])
}

func TestBuildProfileRunRecordEmptyUsedTools(t *testing.T) {
	now := time.Now().UTC()
	stats := RunStats{
		RunID:       "run-empty",
		ProfileName: "coder",
		Steps:       2,
		CostUSD:     0.001,
		UsedTools:   nil,
	}
	completion := RunCompletionData{
		RecordID:   "rec-empty",
		Status:     "failed",
		StartedAt:  now,
		FinishedAt: now.Add(5 * time.Second),
	}

	rec := BuildProfileRunRecord(stats, completion)
	assert.Equal(t, 0, rec.ToolCalls)
	assert.Empty(t, rec.TopTools)
	assert.Equal(t, "failed", rec.Status)
}

func TestBuildProfileRunRecordDefaultStatus(t *testing.T) {
	// When Status is empty, it defaults to "completed".
	now := time.Now().UTC()
	stats := RunStats{RunID: "r1", ProfileName: "p", Steps: 1, CostUSD: 0}
	completion := RunCompletionData{RecordID: "id", StartedAt: now, FinishedAt: now}
	rec := BuildProfileRunRecord(stats, completion)
	assert.Equal(t, "completed", rec.Status)
}

// --- Tests for aggregate ProfileStats, GenerateSuggestions, BuildAggregateReport ---

func TestGenerateSuggestions_NotEnoughHistory(t *testing.T) {
	stats := ProfileStats{
		ProfileName: "my-profile",
		RunCount:    2,
		AvgSteps:    25.0,
		AvgCostUSD:  0.10,
		SuccessRate: 0.4,
		TopTools:    []string{"bash"},
		MaxSteps:    0,
	}
	suggestions := GenerateSuggestions(stats)
	assert.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0], "Not enough history")
}

func TestGenerateSuggestions_LowSuccessRate(t *testing.T) {
	stats := ProfileStats{
		ProfileName: "flaky-profile",
		RunCount:    5,
		AvgSteps:    10.0,
		AvgCostUSD:  0.05,
		SuccessRate: 0.4, // below 0.5
		TopTools:    []string{"bash"},
		MaxSteps:    0,
	}
	suggestions := GenerateSuggestions(stats)
	found := false
	for _, s := range suggestions {
		if containsAny(s, "success rate", "prompt", "constraint") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a suggestion about low success rate or prompt review, got: %v", suggestions)
}

func TestGenerateSuggestions_HighStepCount(t *testing.T) {
	stats := ProfileStats{
		ProfileName: "verbose-profile",
		RunCount:    5,
		AvgSteps:    25.0, // > 20
		AvgCostUSD:  0.05,
		SuccessRate: 0.9,
		TopTools:    []string{"bash"},
		MaxSteps:    0, // no step limit configured
	}
	suggestions := GenerateSuggestions(stats)
	found := false
	for _, s := range suggestions {
		if containsAny(s, "max_steps", "step limit") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a suggestion about max_steps, got: %v", suggestions)
}

func TestGenerateSuggestions_HighStepCountWithStepLimit(t *testing.T) {
	// If profile already has a step limit set, don't suggest adding one.
	stats := ProfileStats{
		ProfileName: "already-limited",
		RunCount:    5,
		AvgSteps:    25.0,
		AvgCostUSD:  0.05,
		SuccessRate: 0.9,
		TopTools:    []string{"bash"},
		MaxSteps:    30, // step limit already set
	}
	suggestions := GenerateSuggestions(stats)
	for _, s := range suggestions {
		if containsAny(s, "max_steps", "step limit") {
			t.Errorf("should not suggest max_steps when limit is already set, but got: %q", s)
		}
	}
}

func TestGenerateSuggestions_HealthyProfile(t *testing.T) {
	stats := ProfileStats{
		ProfileName: "healthy",
		RunCount:    10,
		AvgSteps:    8.0,
		AvgCostUSD:  0.02,
		SuccessRate: 0.95,
		TopTools:    []string{"bash", "read"},
		MaxSteps:    0,
	}
	suggestions := GenerateSuggestions(stats)
	// Healthy profiles should have no critical suggestions (empty or positive).
	// We do not mandate empty — just that no negative suggestions are returned.
	for _, s := range suggestions {
		if containsAny(s, "Not enough history") {
			t.Errorf("unexpected not-enough-history suggestion for healthy profile: %q", s)
		}
	}
}

func TestBuildAggregateReport_NoHistory(t *testing.T) {
	report := BuildAggregateReport("empty-profile", ProfileStats{
		ProfileName: "empty-profile",
		RunCount:    0,
	})
	assert.False(t, report.HasHistory, "expected has_history=false when RunCount=0")
	assert.Equal(t, "empty-profile", report.ProfileName)
	assert.False(t, report.GeneratedAt.IsZero())
	assert.NotEmpty(t, report.Suggestions, "expected not-enough-history suggestion")
}

func TestBuildAggregateReport_WithHistory(t *testing.T) {
	stats := ProfileStats{
		ProfileName: "researcher",
		RunCount:    5,
		AvgSteps:    8.0,
		AvgCostUSD:  0.02,
		SuccessRate: 0.9,
		TopTools:    []string{"read", "grep"},
		MaxSteps:    0,
	}
	report := BuildAggregateReport("researcher", stats)
	assert.True(t, report.HasHistory)
	assert.Equal(t, "researcher", report.ProfileName)
	assert.Equal(t, 5, report.RunCount)
	assert.InDelta(t, 8.0, report.AvgSteps, 0.001)
	assert.InDelta(t, 0.02, report.AvgCostUSD, 0.0001)
	assert.InDelta(t, 0.9, report.SuccessRate, 0.001)
	assert.Equal(t, []string{"read", "grep"}, report.TopTools)
	assert.False(t, report.GeneratedAt.IsZero())
}

// containsAny reports whether s contains any of the substrings (case-insensitive).
func containsAny(s string, substrings ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrings {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
