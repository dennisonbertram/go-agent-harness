package cron

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"time"

	cronparser "github.com/robfig/cron/v3"
)

// JitterConfig controls the per-job jitter applied to scheduled task execution times.
type JitterConfig struct {
	// Enabled controls whether jitter is applied. Default: true.
	Enabled bool

	// MinSec is the minimum jitter offset in seconds. Default: 60.
	MinSec int

	// MaxSec is the maximum jitter offset in seconds. Default: 300.
	MaxSec int

	// AvoidMarks lists the minute marks (0-59) to avoid landing on.
	// Default: [0, 30].
	AvoidMarks []int

	// LogJitteredTimes controls whether jittered execution times are logged.
	// Default: true.
	LogJitteredTimes bool
}

// DefaultJitterConfig returns the default jitter configuration.
func DefaultJitterConfig() JitterConfig {
	return JitterConfig{
		Enabled:          true,
		MinSec:           60,
		MaxSec:           300,
		AvoidMarks:       []int{0, 30},
		LogJitteredTimes: true,
	}
}

// computeJitter returns a deterministic base jitter duration for a job. The
// jitter is computed from the job ID and schedule using a hash-based seed so
// the same job always gets the same base jitter offset across scheduler
// restarts.
//
// The returned duration is in [cfg.MinSec, cfg.MaxSec]. Minute-mark avoidance
// is applied separately at fire time via avoidMinuteMarks.
func computeJitter(cfg JitterConfig, jobID, schedule string) time.Duration {
	// Sanity: if disabled or the range is invalid (min greater than max, or negative min), return zero.
	if !cfg.Enabled || cfg.MinSec > cfg.MaxSec || cfg.MinSec < 0 {
		return 0
	}

	// Deterministic seed from job ID + schedule.
	h := fnv.New64a()
	h.Write([]byte(jobID))
	h.Write([]byte{0})
	h.Write([]byte(schedule))

	// Use the hash as a seed for a new random source.
	src := rand.NewPCG(h.Sum64(), h.Sum64()^0x5bd1e995)
	rng := rand.New(src)

	// Produce a raw jitter in [MinSec, MaxSec].
	span := cfg.MaxSec - cfg.MinSec
	raw := rng.IntN(span+1) + cfg.MinSec

	return clampJitterToInterval(time.Duration(raw)*time.Second, schedule)
}

// clampJitterToInterval bounds jitter so it cannot exceed the schedule it is
// perturbing.
//
// Jitter exists to keep many jobs off the same popular marks (:00, :30), which
// matters for hourly and daily schedules. Applied unclamped it breaks short
// ones: an every-minute job drew a 3m56s offset, so each evaluation pushed the
// fire time past its own next several slots and last_run_at stayed zero — the
// job was created, active, scheduled, and never ran.
//
// Half the interval is the most that can be added while still landing inside
// the current period, so the job keeps its cadence and only loses alignment.
func clampJitterToInterval(jitter time.Duration, schedule string) time.Duration {
	interval, ok := scheduleInterval(schedule)
	if !ok || interval <= 0 {
		return jitter
	}
	max := interval / 2
	if jitter > max {
		return max
	}
	return jitter
}

// scheduleInterval estimates the gap between consecutive fires by asking the
// parsed schedule for its next two times. Returns false when the expression
// cannot be parsed, in which case the caller leaves jitter untouched rather
// than guessing.
func scheduleInterval(schedule string) (time.Duration, bool) {
	parser := cronparser.NewParser(
		cronparser.Minute | cronparser.Hour | cronparser.Dom |
			cronparser.Month | cronparser.Dow | cronparser.Descriptor)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return 0, false
	}
	// A fixed base keeps this deterministic; the gap between consecutive
	// fires is what matters, not when they are.
	base := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	first := sched.Next(base)
	second := sched.Next(first)
	if second.Before(first) || second.Equal(first) {
		return 0, false
	}
	return second.Sub(first), true
}

// avoidMinuteMarks adjusts the jitter offset so that (fireTime + offset) does
// not land on any minute listed in avoidMarks. It walks forward in 1-second
// increments until a clean minute is found.
func avoidMinuteMarks(offset time.Duration, fireTime time.Time, avoidMarks []int) time.Duration {
	if len(avoidMarks) == 0 {
		return offset
	}

	// Build a quick-lookup set.
	bad := make(map[int]bool, len(avoidMarks))
	for _, m := range avoidMarks {
		bad[m%60] = true
	}

	const maxWalk = 120 // max seconds to walk forward; safety cap

	base := offset
	for i := 0; i < maxWalk; i++ {
		landing := fireTime.Add(offset)
		if !bad[landing.Minute()] {
			return offset
		}
		offset += time.Second
	}
	// Safety fallback: return original offset even if it lands on a bad minute.
	return base
}

// jitterCacheKey returns the cache key for a job's jitter computation.
func jitterCacheKey(jobID, schedule string) string {
	return fmt.Sprintf("%s|%s", jobID, schedule)
}
