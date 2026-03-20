package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/store"
)

// newProfileRunsDB creates a fresh SQLiteProfileRunStore for testing.
func newProfileRunsDB(t *testing.T) *store.SQLiteProfileRunStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "profile_runs_test.db")
	s, err := store.NewSQLiteProfileRunStore(dbPath)
	require.NoError(t, err, "NewSQLiteProfileRunStore")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProfileRunStore_RecordRun(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	rec := store.ProfileRunRecord{
		ID:          "pr-1",
		ProfileName: "researcher",
		RunID:       "run-abc",
		Status:      "completed",
		StepCount:   10,
		CostUSD:     0.05,
		StartedAt:   now,
		FinishedAt:  now.Add(30 * time.Second),
		ToolCalls:   5,
		TopTools:    []string{"read", "grep", "glob"},
	}

	err := s.RecordProfileRun(ctx, rec)
	require.NoError(t, err, "RecordProfileRun should not return error")

	// Read it back via QueryRecentProfileRuns.
	runs, err := s.QueryRecentProfileRuns(ctx, "researcher", 10)
	require.NoError(t, err, "QueryRecentProfileRuns should not return error")
	require.Len(t, runs, 1, "should have exactly one run")

	got := runs[0]
	assert.Equal(t, "pr-1", got.ID)
	assert.Equal(t, "researcher", got.ProfileName)
	assert.Equal(t, "run-abc", got.RunID)
	assert.Equal(t, "completed", got.Status)
	assert.Equal(t, 10, got.StepCount)
	assert.InDelta(t, 0.05, got.CostUSD, 0.0001)
	assert.Equal(t, now, got.StartedAt)
	assert.Equal(t, now.Add(30*time.Second), got.FinishedAt)
	assert.Equal(t, 5, got.ToolCalls)
	assert.Equal(t, []string{"read", "grep", "glob"}, got.TopTools)
}

func TestProfileRunStore_QueryRecentRuns(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)

	// Insert 5 runs for "researcher".
	for i := 0; i < 5; i++ {
		rec := store.ProfileRunRecord{
			ID:          fmt.Sprintf("pr-%d", i),
			ProfileName: "researcher",
			RunID:       fmt.Sprintf("run-%d", i),
			Status:      "completed",
			StepCount:   i + 1,
			CostUSD:     float64(i) * 0.01,
			StartedAt:   base.Add(time.Duration(i) * time.Minute),
			FinishedAt:  base.Add(time.Duration(i)*time.Minute + 30*time.Second),
			ToolCalls:   i,
			TopTools:    []string{"read"},
		}
		require.NoError(t, s.RecordProfileRun(ctx, rec))
	}

	// Query with limit=3 — should return the 3 most recent.
	runs, err := s.QueryRecentProfileRuns(ctx, "researcher", 3)
	require.NoError(t, err)
	assert.Len(t, runs, 3, "limit=3 should return exactly 3 runs")

	// Verify they are ordered by started_at DESC (most recent first).
	assert.True(t, runs[0].StartedAt.After(runs[1].StartedAt) || runs[0].StartedAt.Equal(runs[1].StartedAt),
		"results should be ordered newest first")
}

func TestProfileRunStore_AggregateStats(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)

	runs := []store.ProfileRunRecord{
		{ID: "a1", ProfileName: "coder", RunID: "r1", Status: "completed", StepCount: 4, CostUSD: 0.04, StartedAt: base, FinishedAt: base.Add(time.Second), ToolCalls: 2},
		{ID: "a2", ProfileName: "coder", RunID: "r2", Status: "completed", StepCount: 6, CostUSD: 0.06, StartedAt: base.Add(time.Minute), FinishedAt: base.Add(time.Minute + time.Second), ToolCalls: 3},
		{ID: "a3", ProfileName: "coder", RunID: "r3", Status: "failed", StepCount: 2, CostUSD: 0.02, StartedAt: base.Add(2 * time.Minute), FinishedAt: base.Add(2*time.Minute + time.Second), ToolCalls: 1},
	}
	for _, rec := range runs {
		require.NoError(t, s.RecordProfileRun(ctx, rec))
	}

	stats, err := s.AggregateProfileStats(ctx, "coder")
	require.NoError(t, err)

	assert.Equal(t, "coder", stats.ProfileName)
	assert.Equal(t, 3, stats.RunCount)
	assert.InDelta(t, 4.0, stats.AvgSteps, 0.01, "avg steps: (4+6+2)/3=4")
	assert.InDelta(t, 0.04, stats.AvgCostUSD, 0.001, "avg cost: (0.04+0.06+0.02)/3=0.04")
	// 2 of 3 runs completed => success rate = 2/3 ≈ 0.667
	assert.InDelta(t, 2.0/3.0, stats.SuccessRate, 0.01)
	assert.False(t, stats.LastRunAt.IsZero(), "LastRunAt should be set")
}

func TestProfileRunStore_EmptyHistory(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	// No runs recorded yet — should return empty slice, not error.
	runs, err := s.QueryRecentProfileRuns(ctx, "no-such-profile", 10)
	require.NoError(t, err, "empty history should not error")
	assert.Empty(t, runs, "empty history should return empty slice")
}

func TestProfileRunStore_EmptyHistoryAggregateStats(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	// AggregateProfileStats for a profile with no history.
	stats, err := s.AggregateProfileStats(ctx, "no-such-profile")
	require.NoError(t, err, "aggregate with no history should not error")
	assert.Equal(t, "no-such-profile", stats.ProfileName)
	assert.Equal(t, 0, stats.RunCount)
	assert.InDelta(t, 0.0, stats.AvgSteps, 0.001)
	assert.InDelta(t, 0.0, stats.AvgCostUSD, 0.001)
	assert.InDelta(t, 0.0, stats.SuccessRate, 0.001)
}

func TestProfileRunStore_MultipleProfiles(t *testing.T) {
	s := newProfileRunsDB(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)

	// Insert runs for two different profiles.
	for i := 0; i < 3; i++ {
		require.NoError(t, s.RecordProfileRun(ctx, store.ProfileRunRecord{
			ID:          fmt.Sprintf("alpha-%d", i),
			ProfileName: "alpha",
			RunID:       fmt.Sprintf("run-alpha-%d", i),
			Status:      "completed",
			StepCount:   5,
			CostUSD:     0.05,
			StartedAt:   base,
			FinishedAt:  base.Add(time.Second),
		}))
		require.NoError(t, s.RecordProfileRun(ctx, store.ProfileRunRecord{
			ID:          fmt.Sprintf("beta-%d", i),
			ProfileName: "beta",
			RunID:       fmt.Sprintf("run-beta-%d", i),
			Status:      "completed",
			StepCount:   3,
			CostUSD:     0.03,
			StartedAt:   base,
			FinishedAt:  base.Add(time.Second),
		}))
	}

	// Querying "alpha" should only return alpha's runs.
	alphaRuns, err := s.QueryRecentProfileRuns(ctx, "alpha", 100)
	require.NoError(t, err)
	assert.Len(t, alphaRuns, 3, "alpha should have 3 runs")
	for _, r := range alphaRuns {
		assert.Equal(t, "alpha", r.ProfileName, "all runs should belong to alpha")
	}

	// Querying "beta" should only return beta's runs.
	betaRuns, err := s.QueryRecentProfileRuns(ctx, "beta", 100)
	require.NoError(t, err)
	assert.Len(t, betaRuns, 3, "beta should have 3 runs")
	for _, r := range betaRuns {
		assert.Equal(t, "beta", r.ProfileName, "all runs should belong to beta")
	}

	// Stats for each profile should be independent.
	alphaStats, err := s.AggregateProfileStats(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 3, alphaStats.RunCount)

	betaStats, err := s.AggregateProfileStats(ctx, "beta")
	require.NoError(t, err)
	assert.Equal(t, 3, betaStats.RunCount)
}
