package tooluse

import (
	"fmt"
	"time"
)

// Timer tracks duration for a tool call.
// Timer is an immutable value type — all mutating methods return a new Timer.
type Timer struct {
	startTime time.Time
	endTime   time.Time
	running   bool
}

// NewTimer creates a new, unstarted Timer.
func NewTimer() Timer {
	return Timer{}
}

// Start returns a new Timer that has been started at the current time.
func (t Timer) Start() Timer {
	return t.start(time.Now())
}

// start is the internal version that accepts an explicit time (for testing).
func (t Timer) start(at time.Time) Timer {
	return Timer{
		startTime: at,
		running:   true,
	}
}

// Stop returns a new Timer that has been stopped at the current time.
func (t Timer) Stop() Timer {
	return t.stop(time.Now())
}

// stop is the internal version that accepts an explicit time (for testing).
func (t Timer) stop(at time.Time) Timer {
	return Timer{
		startTime: t.startTime,
		endTime:   at,
		running:   false,
	}
}

// IsRunning returns true if the timer has been started but not stopped.
func (t Timer) IsRunning() bool {
	return t.running
}

// Elapsed returns the duration recorded by the timer.
// Returns 0 if the timer was never started.
// If the timer is still running, returns elapsed time since start.
// If stopped, returns the duration between start and stop.
func (t Timer) Elapsed() time.Duration {
	if t.startTime.IsZero() {
		return 0
	}
	if t.running {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

// FormatDuration formats the elapsed duration as a human-readable string.
//
// Format rules:
//   - < 1s:  "NNNms"  (e.g. "500ms")
//   - 1s–59s: "N.Ns"  (e.g. "2.3s")
//   - ≥ 60s: "Nm Ns"  (e.g. "2m 30s")
//
// Returns "0ms" if the timer was never started.
func (t Timer) FormatDuration() string {
	d := t.Elapsed()
	return FormatDuration(d)
}

// FormatDuration formats a time.Duration as a human-readable string using
// the same rules as Timer.FormatDuration.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < 60*time.Second {
		seconds := d.Seconds()
		return fmt.Sprintf("%.1fs", seconds)
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
