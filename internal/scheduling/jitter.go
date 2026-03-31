// Package scheduling provides utilities for scheduled task execution,
// including jitter to prevent thundering-herd on the API fleet.
package scheduling

import (
	"fmt"
	"math/rand"
	"time"
)

// JitterConfig holds configuration for scheduled task jitter.
type JitterConfig struct {
	// Enabled controls whether random jitter is added to scheduled times.
	Enabled bool `toml:"jitter_enabled"`
	// MinSeconds is the minimum jitter offset in seconds.
	MinSeconds int `toml:"jitter_min_seconds"`
	// MaxSeconds is the maximum jitter offset in seconds.
	MaxSeconds int `toml:"jitter_max_seconds"`
	// AvoidRoundMinutes prevents results from landing on :00 or :30 minute marks.
	AvoidRoundMinutes bool `toml:"avoid_round_minutes"`
}

// DefaultJitterConfig returns sensible defaults for jitter configuration.
func DefaultJitterConfig() JitterConfig {
	return JitterConfig{
		Enabled:           true,
		MinSeconds:        60,
		MaxSeconds:        300,
		AvoidRoundMinutes: true,
	}
}

// ApplyJitter adds random jitter to a scheduled time using a random seed
// derived from the current time in nanoseconds.
func ApplyJitter(t time.Time, cfg JitterConfig) time.Time {
	return ApplyJitterWithSeed(t, cfg, time.Now().UnixNano())
}

// ApplyJitterWithSeed adds jitter to a scheduled time using a specific random
// seed. Using a fixed seed produces deterministic results, which is useful for
// testing. When Enabled is false the original time is returned unchanged.
func ApplyJitterWithSeed(t time.Time, cfg JitterConfig, seed int64) time.Time {
	if !cfg.Enabled {
		return t
	}

	// Use a seeded source for deterministic behaviour.
	//nolint:gosec // math/rand is intentional — jitter does not need crypto randomness
	r := rand.New(rand.NewSource(seed))

	// Compute a random offset in [MinSeconds, MaxSeconds].
	spread := cfg.MaxSeconds - cfg.MinSeconds
	if spread < 0 {
		spread = 0
	}
	offsetSeconds := cfg.MinSeconds + r.Intn(spread+1)
	jittered := t.Add(time.Duration(offsetSeconds) * time.Second)

	if cfg.AvoidRoundMinutes {
		// Use a derived seed so AvoidRoundMinute has independent randomness
		// from the offset selection, yet the overall output remains deterministic.
		jittered = AvoidRoundMinute(jittered, seed^0xdeadbeef)
	}

	return jittered
}

// IsRoundMinute reports whether t falls exactly on a :00 or :30 minute mark
// (regardless of seconds/nanoseconds — only the minute field is checked).
func IsRoundMinute(t time.Time) bool {
	m := t.Minute()
	return m == 0 || m == 30
}

// AvoidRoundMinute adjusts t away from :00 and :30 by adding 1–5 minutes when
// t is on a round minute. If t is not on a round minute it is returned unchanged.
// The seed is used to choose the exact adjustment deterministically.
func AvoidRoundMinute(t time.Time, seed int64) time.Time {
	if !IsRoundMinute(t) {
		return t
	}
	//nolint:gosec
	r := rand.New(rand.NewSource(seed))
	// Add 1–5 minutes (Intn(5) gives [0,4], +1 gives [1,5])
	shift := time.Duration(1+r.Intn(5)) * time.Minute
	adjusted := t.Add(shift)

	// If after adding the shift we still land on :30, add one more minute.
	// (This can happen only if we shifted by exactly 30 minutes — which Intn(5)+1
	// never produces — but we guard defensively.)
	if IsRoundMinute(adjusted) {
		adjusted = adjusted.Add(time.Minute)
	}
	return adjusted
}

// FormatJitterLog returns a human-readable string explaining the jitter that
// was applied, showing both the original and jittered times together with the
// offset in seconds.
func FormatJitterLog(original, jittered time.Time) string {
	delta := jittered.Sub(original)
	return fmt.Sprintf(
		"scheduled jitter applied: original=%s jittered=%s offset=%.0fs",
		original.Format("15:04:05"),
		jittered.Format("15:04:05"),
		delta.Seconds(),
	)
}
