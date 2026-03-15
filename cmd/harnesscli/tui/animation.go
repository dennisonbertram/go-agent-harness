package tui

import "time"

const (
	// SpinnerInterval is the tick rate for spinner animation.
	SpinnerInterval = 120 * time.Millisecond

	// StatusMsgDuration is how long transient status messages display.
	StatusMsgDuration = 3 * time.Second

	// ScrollAnimDuration is the debounce window for scroll events.
	ScrollAnimDuration = 16 * time.Millisecond // ~60fps

	// MinRenderWidth is the minimum terminal width for TUI to render usefully.
	MinRenderWidth = 40

	// MinRenderHeight is the minimum terminal height for TUI to render usefully.
	MinRenderHeight = 10
)

// IsTooSmall reports whether the given terminal dimensions are too small
// for the TUI to render usefully.
func IsTooSmall(width, height int) bool {
	return width < MinRenderWidth || height < MinRenderHeight
}

// TooSmallView returns a minimal message to display when terminal is too small.
func TooSmallView(width, height int) string {
	msg := "Terminal too small. Please resize."
	if width < len(msg) {
		return msg[:width]
	}
	return msg
}
