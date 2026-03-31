package scheduling_test

import (
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/scheduling"
)

// TestApplyJitterWithSeed_AddsJitter verifies that jittered time differs from original.
func TestApplyJitterWithSeed_AddsJitter(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)
	jittered := scheduling.ApplyJitterWithSeed(original, cfg, 42)
	if jittered.Equal(original) {
		t.Errorf("expected jittered time to differ from original %v, but got equal time", original)
	}
}

// TestApplyJitterWithSeed_WithinBounds verifies jitter stays within min/max bounds.
func TestApplyJitterWithSeed_WithinBounds(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)

	for seed := int64(0); seed < 100; seed++ {
		jittered := scheduling.ApplyJitterWithSeed(original, cfg, seed)
		diff := jittered.Sub(original)
		diffSeconds := int(diff.Seconds())

		if diffSeconds < cfg.MinSeconds {
			t.Errorf("seed=%d: jitter %d seconds is below minimum %d", seed, diffSeconds, cfg.MinSeconds)
		}
		if diffSeconds > cfg.MaxSeconds {
			t.Errorf("seed=%d: jitter %d seconds is above maximum %d", seed, diffSeconds, cfg.MaxSeconds)
		}
	}
}

// TestApplyJitterWithSeed_Disabled verifies that disabled jitter returns the original time unchanged.
func TestApplyJitterWithSeed_Disabled(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           false,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)
	result := scheduling.ApplyJitterWithSeed(original, cfg, 42)
	if !result.Equal(original) {
		t.Errorf("disabled jitter: expected original time %v, got %v", original, result)
	}
}

// TestApplyJitterWithSeed_Deterministic verifies same seed produces same result.
func TestApplyJitterWithSeed_Deterministic(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)

	first := scheduling.ApplyJitterWithSeed(original, cfg, 99)
	second := scheduling.ApplyJitterWithSeed(original, cfg, 99)
	if !first.Equal(second) {
		t.Errorf("same seed must produce same result: got %v and %v", first, second)
	}
}

// TestApplyJitterWithSeed_DifferentSeeds verifies different seeds produce different results.
func TestApplyJitterWithSeed_DifferentSeeds(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: false,
	}
	original := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)

	a := scheduling.ApplyJitterWithSeed(original, cfg, 1)
	b := scheduling.ApplyJitterWithSeed(original, cfg, 2)
	if a.Equal(b) {
		t.Errorf("different seeds should produce different results, both returned %v", a)
	}
}

// TestIsRoundMinute_OnTheHour verifies that :00 minutes are detected as round.
func TestIsRoundMinute_OnTheHour(t *testing.T) {
	t9 := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	if !scheduling.IsRoundMinute(t9) {
		t.Errorf("expected 9:00 to be a round minute, but IsRoundMinute returned false")
	}
}

// TestIsRoundMinute_HalfHour verifies that :30 minutes are detected as round.
func TestIsRoundMinute_HalfHour(t *testing.T) {
	t9 := time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)
	if !scheduling.IsRoundMinute(t9) {
		t.Errorf("expected 9:30 to be a round minute, but IsRoundMinute returned false")
	}
}

// TestIsRoundMinute_NotRound verifies that arbitrary minutes are not detected as round.
func TestIsRoundMinute_NotRound(t *testing.T) {
	t9 := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)
	if scheduling.IsRoundMinute(t9) {
		t.Errorf("expected 9:17 to not be a round minute, but IsRoundMinute returned true")
	}
}

// TestAvoidRoundMinute_MovesOffHour verifies that a :00 time is adjusted away from round minutes.
func TestAvoidRoundMinute_MovesOffHour(t *testing.T) {
	t9 := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 20; seed++ {
		adjusted := scheduling.AvoidRoundMinute(t9, seed)
		min := adjusted.Minute()
		if min == 0 || min == 30 {
			t.Errorf("seed=%d: expected minute to avoid :00 and :30, got minute=%d", seed, min)
		}
	}
}

// TestAvoidRoundMinute_LeavesNonRound verifies that a non-round time is returned unchanged.
func TestAvoidRoundMinute_LeavesNonRound(t *testing.T) {
	t9 := time.Date(2026, 1, 15, 9, 17, 0, 0, time.UTC)
	adjusted := scheduling.AvoidRoundMinute(t9, 42)
	if !adjusted.Equal(t9) {
		t.Errorf("expected 9:17 to be returned unchanged, got %v", adjusted)
	}
}

// TestFormatJitterLog_ShowsDifference verifies the log string contains both times.
func TestFormatJitterLog_ShowsDifference(t *testing.T) {
	original := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	jittered := time.Date(2026, 1, 15, 9, 3, 42, 0, time.UTC)
	msg := scheduling.FormatJitterLog(original, jittered)
	if !strings.Contains(msg, "09:00") && !strings.Contains(msg, "9:00") {
		t.Errorf("expected log to contain original time, got: %q", msg)
	}
	if !strings.Contains(msg, "09:03") && !strings.Contains(msg, "9:03") {
		t.Errorf("expected log to contain jittered time, got: %q", msg)
	}
}

// TestDefaultJitterConfig verifies the default config has the expected values.
func TestDefaultJitterConfig(t *testing.T) {
	cfg := scheduling.DefaultJitterConfig()
	if !cfg.Enabled {
		t.Errorf("expected Enabled=true, got false")
	}
	if cfg.MinSeconds != 60 {
		t.Errorf("expected MinSeconds=60, got %d", cfg.MinSeconds)
	}
	if cfg.MaxSeconds != 300 {
		t.Errorf("expected MaxSeconds=300, got %d", cfg.MaxSeconds)
	}
	if !cfg.AvoidRoundMinutes {
		t.Errorf("expected AvoidRoundMinutes=true, got false")
	}
}

// TestJitterConfig_FromTOML verifies the [scheduling] TOML section parses correctly.
func TestJitterConfig_FromTOML(t *testing.T) {
	type tomlDoc struct {
		Scheduling scheduling.JitterConfig `toml:"scheduling"`
	}
	raw := `
[scheduling]
jitter_enabled = true
jitter_min_seconds = 60
jitter_max_seconds = 300
avoid_round_minutes = true
`
	var doc tomlDoc
	if _, err := toml.Decode(raw, &doc); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	cfg := doc.Scheduling
	if !cfg.Enabled {
		t.Errorf("expected Enabled=true from TOML, got false")
	}
	if cfg.MinSeconds != 60 {
		t.Errorf("expected MinSeconds=60 from TOML, got %d", cfg.MinSeconds)
	}
	if cfg.MaxSeconds != 300 {
		t.Errorf("expected MaxSeconds=300 from TOML, got %d", cfg.MaxSeconds)
	}
	if !cfg.AvoidRoundMinutes {
		t.Errorf("expected AvoidRoundMinutes=true from TOML, got false")
	}
}

// TestApplyJitterWithSeed_RespectAvoidRoundMinutes verifies that when AvoidRoundMinutes is true,
// the result never lands on :00 or :30.
func TestApplyJitterWithSeed_RespectAvoidRoundMinutes(t *testing.T) {
	cfg := scheduling.JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: true,
	}
	// Use a base time on the hour to maximize chance of hitting round minutes
	original := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

	for seed := int64(0); seed < 100; seed++ {
		jittered := scheduling.ApplyJitterWithSeed(original, cfg, seed)
		min := jittered.Minute()
		if min == 0 || min == 30 {
			t.Errorf("seed=%d: expected result to avoid :00 and :30, got minute=%d (time=%v)", seed, min, jittered)
		}
	}
}
