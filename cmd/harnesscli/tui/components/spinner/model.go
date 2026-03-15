// Package spinner implements the TUI-024 thinking spinner with rotating verbs.
// It provides an immutable BubbleTea-style Model that advances frame-by-frame
// and rotates through a pool of whimsical verbs (e.g. "Thinking", "Reasoning").
package spinner

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// frames are the 6 animation frames for the thinking spinner.
// These are star/asterisk glyphs, not the braille frames in theme.go.
var frames = []string{"✶", "·", "✻", "✽", "✳", "✢"}

// verbRotateEvery controls how many Tick() calls trigger a verb rotation.
const verbRotateEvery = 8

// durationThreshold is the elapsed time after which the spinner shows a duration.
const durationThreshold = 2 * time.Second

// SpinnerTickMsg triggers a spinner frame advance.
// This is the local equivalent of tui.SpinnerTickMsg; spinner-specific
// so the package has no import cycle with the parent tui package.
type SpinnerTickMsg struct{ T time.Time }

// Model is the immutable thinking spinner state.
// All mutation methods return a new Model value — never modify in place.
// This keeps it safe for use in BubbleTea's single-goroutine Update().
type Model struct {
	frame     int        // current frame index [0, len(frames))
	verb      string     // current displayed verb
	startTime time.Time  // when spinner started (for duration)
	tokens    int        // token count stored on Stop()
	active    bool       // true while spinner is running
	done      bool       // true after Stop()
	tickCount int        // total ticks received (used for verb rotation)
	rng       *rand.Rand // seeded rng for deterministic testing

	// Seed is the seed used to create rng. Exposed so tests can inspect it.
	Seed int64

	// testVerbs overrides DefaultVerbs when non-nil. For testing only.
	testVerbs []string
}

// New creates a new Model with the given seed. The seed makes verb selection
// deterministic which is essential for snapshot and regression tests.
func New(seed int64) Model {
	return Model{
		Seed: seed,
		rng:  rand.New(rand.NewSource(seed)), //nolint:gosec // not for crypto
	}
}

// verbPool returns the verb pool in effect: testVerbs override if set,
// otherwise DefaultVerbs.
func (m Model) verbPool() []string {
	if m.testVerbs != nil {
		return m.testVerbs
	}
	return DefaultVerbs
}

// Start activates the spinner, records the start time, and picks an initial verb.
// Returns a new Model; the receiver is unchanged.
func (m Model) Start() Model {
	m.active = true
	m.done = false
	m.startTime = time.Now()
	m.frame = 0
	m.tickCount = 0
	m.verb = pickVerb(m.verbPool(), m.rng)
	return m
}

// Tick advances the animation by one frame and potentially rotates the verb.
// Has no effect if the spinner is not active.
// Returns a new Model; the receiver is unchanged.
func (m Model) Tick() Model {
	if !m.active {
		return m
	}
	m.tickCount++
	m.frame = (m.frame + 1) % len(frames)
	// Rotate verb every verbRotateEvery ticks.
	if m.tickCount%verbRotateEvery == 0 {
		m.verb = pickVerb(m.verbPool(), m.rng)
	}
	return m
}

// Stop deactivates the spinner and records the final token count.
// Returns a new Model; the receiver is unchanged.
func (m Model) Stop(tokens int) Model {
	m.active = false
	m.done = true
	m.tokens = tokens
	return m
}

// IsActive returns true while the spinner is running (between Start and Stop).
func (m Model) IsActive() bool { return m.active }

// IsDone returns true after Stop() has been called.
func (m Model) IsDone() bool { return m.done }

// View renders the spinner as a single line. The width parameter controls the
// maximum character width; the view degrades gracefully at narrow widths.
//
// Format: "✻ Thinking..." or "✻ Thinking... (2.3s)" once durationThreshold passes.
func (m Model) View(width int) string {
	if width <= 0 {
		width = 80
	}

	currentFrame := frames[m.frame]

	// Build the base text.
	base := currentFrame + " " + m.verb + "..."

	// Append duration if we've exceeded the threshold.
	if m.active && !m.startTime.IsZero() {
		elapsed := time.Since(m.startTime)
		if elapsed >= durationThreshold {
			base += " " + formatDuration(elapsed)
		}
	}

	style := lipgloss.NewStyle().Faint(true)
	rendered := style.Render(base)

	// Clamp to width using MaxWidth.
	if width < 80 {
		rendered = lipgloss.NewStyle().MaxWidth(width).Render(base)
	}

	return rendered
}

// CompletionLine returns the one-line completion summary shown after the spinner stops.
//
// Format: "✻ Worked for 5s" or "✻ Worked for 1m 30s"
func (m Model) CompletionLine(seconds float64) string {
	glyph := frames[m.frame%len(frames)]
	duration := formatSeconds(seconds)
	line := glyph + " Worked for " + duration
	return lipgloss.NewStyle().Faint(true).Render(line)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// formatDuration formats a time.Duration into a parenthesised display string.
// Examples: "(2.3s)", "(1m 30s)"
func formatDuration(d time.Duration) string {
	return "(" + formatSeconds(d.Seconds()) + ")"
}

// formatSeconds formats a duration in seconds into a human-readable string.
// Under 60s: "2.3s" (one decimal place).
// 60s+:      "1m 30s".
func formatSeconds(s float64) string {
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	mins := int(s) / 60
	secs := int(s) % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm %ds", mins, secs)
}
