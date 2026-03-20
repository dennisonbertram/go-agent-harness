package profiles

import (
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
