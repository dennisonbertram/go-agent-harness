package scheduling_test

// Regression tests for the scheduling jitter package.
// These tests cover angles that differ from the behavioral tests:
// edge cases, boundary conditions, integration concerns, and
// invariants that would catch silent regressions.

import (
	"testing"
	"time"

	"go-agent-harness/internal/scheduling"
)

// type aliases for brevity within this file
type JitterConfig = scheduling.JitterConfig

func DefaultJitterConfig() scheduling.JitterConfig  { return scheduling.DefaultJitterConfig() }
func ApplyJitter(t time.Time, cfg scheduling.JitterConfig) time.Time {
	return scheduling.ApplyJitter(t, cfg)
}
func ApplyJitterWithSeed(t time.Time, cfg scheduling.JitterConfig, seed int64) time.Time {
	return scheduling.ApplyJitterWithSeed(t, cfg, seed)
}
func IsRoundMinute(t time.Time) bool                              { return scheduling.IsRoundMinute(t) }
func AvoidRoundMinute(t time.Time, seed int64) time.Time         { return scheduling.AvoidRoundMinute(t, seed) }
func FormatJitterLog(orig, jittered time.Time) string            { return scheduling.FormatJitterLog(orig, jittered) }

// TestApplyJitterWithSeed_MinEqualsMax verifies behaviour when MinSeconds == MaxSeconds.
// Every result should be exactly MinSeconds after original with no spread.
func TestApplyJitterWithSeed_MinEqualsMax(t *testing.T) {
	cfg := JitterConfig{
		Enabled:           true,
		MinSeconds:        120,
		MaxSeconds:        120,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)
	for seed := int64(0); seed < 10; seed++ {
		jittered := ApplyJitterWithSeed(original, cfg, seed)
		diff := int(jittered.Sub(original).Seconds())
		if diff != 120 {
			t.Errorf("seed=%d: expected exactly 120s offset with min==max, got %d", seed, diff)
		}
	}
}

// TestApplyJitterWithSeed_PreservesDate verifies jitter never changes the date
// for offsets within [60, 300] seconds — a regression guard against accidentally
// adding days or hours instead of seconds.
func TestApplyJitterWithSeed_PreservesDate(t *testing.T) {
	cfg := DefaultJitterConfig()
	cfg.AvoidRoundMinutes = false
	// Use a time well away from midnight so the small jitter won't cross a date boundary.
	original := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 50; seed++ {
		jittered := ApplyJitterWithSeed(original, cfg, seed)
		if jittered.Year() != original.Year() || jittered.Month() != original.Month() || jittered.Day() != original.Day() {
			t.Errorf("seed=%d: jitter crossed a date boundary: original=%v jittered=%v",
				seed, original, jittered)
		}
	}
}

// TestIsRoundMinute_AllHours is a comprehensive regression check that every
// :00 minute across a full day (24 hours) is detected as a round minute.
func TestIsRoundMinute_AllHours(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for h := 0; h < 24; h++ {
		atHour := base.Add(time.Duration(h) * time.Hour)
		if !IsRoundMinute(atHour) {
			t.Errorf("hour=%d:00 should be a round minute", h)
		}
		atHalf := atHour.Add(30 * time.Minute)
		if !IsRoundMinute(atHalf) {
			t.Errorf("hour=%d:30 should be a round minute", h)
		}
	}
}

// TestAvoidRoundMinute_HalfHour verifies :30 times are also moved off the round mark.
func TestAvoidRoundMinute_HalfHour(t *testing.T) {
	t9h30 := time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)
	for seed := int64(0); seed < 20; seed++ {
		adjusted := AvoidRoundMinute(t9h30, seed)
		min := adjusted.Minute()
		if min == 0 || min == 30 {
			t.Errorf("seed=%d: expected 9:30 to be adjusted away from round minutes, got minute=%d", seed, min)
		}
	}
}

// TestFormatJitterLog_NegativeDelta ensures FormatJitterLog works correctly when
// jittered is before original (edge case, not expected in normal use).
func TestFormatJitterLog_NegativeDelta(t *testing.T) {
	original := time.Date(2026, 1, 15, 9, 5, 0, 0, time.UTC)
	jittered := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	msg := FormatJitterLog(original, jittered)
	if msg == "" {
		t.Errorf("FormatJitterLog returned empty string for negative delta")
	}
}

// TestApplyJitter_EnabledNonDeterministic verifies that calling ApplyJitter
// (the non-seeded public variant) when enabled returns a different time than original.
// This test could theoretically be flaky if the PRNG returns the same value as
// the minimum offset and that happens to produce the original time, but with a
// 60-second minimum offset that is impossible.
func TestApplyJitter_EnabledNonDeterministic(t *testing.T) {
	cfg := DefaultJitterConfig()
	cfg.AvoidRoundMinutes = false
	original := time.Date(2026, 1, 15, 10, 17, 0, 0, time.UTC)
	result := ApplyJitter(original, cfg)
	if result.Equal(original) {
		t.Errorf("ApplyJitter with enabled config returned original time unchanged")
	}
}

// TestApplyJitter_DisabledNonDeterministic verifies the non-seeded variant also
// returns original when disabled.
func TestApplyJitter_DisabledNonDeterministic(t *testing.T) {
	cfg := DefaultJitterConfig()
	cfg.Enabled = false
	original := time.Date(2026, 1, 15, 10, 17, 0, 0, time.UTC)
	result := ApplyJitter(original, cfg)
	if !result.Equal(original) {
		t.Errorf("ApplyJitter with disabled config should return original, got %v", result)
	}
}
