// Package interruptui implements the TUI-054 interrupt confirmation banner.
// It provides an immutable BubbleTea-style Model that displays a visual
// interrupt/stop state when the user presses Ctrl+C during an active run.
package interruptui

// State represents the current display state of the interrupt banner.
type State int

const (
	// StateHidden means the banner is not displayed.
	StateHidden State = iota
	// StateConfirm means the banner is asking the user to confirm the interrupt.
	// Displayed as: "⚠  Press Ctrl+C again to stop, or Esc to continue"
	StateConfirm
	// StateWaiting means the interrupt has been confirmed and we are waiting
	// for the current tool to finish.
	// Displayed as: "Stopping… (waiting for current tool to finish)"
	StateWaiting
	// StateDone is a transitional state shown briefly before returning to Hidden.
	StateDone
)

// Model is the immutable interrupt banner state.
// All mutation methods return a new Model value — never modify in place.
// This keeps it safe for use in BubbleTea's single-goroutine Update().
type Model struct {
	// State is the current display state of the banner.
	State State
	// Width is the terminal width used for rendering.
	Width int
}

// New creates a new Model in the Hidden state with zero width.
func New() Model {
	return Model{
		State: StateHidden,
	}
}

// Show transitions the model from Hidden to Confirm.
// If the model is not in the Hidden state, it returns an unchanged copy.
func (m Model) Show() Model {
	if m.State != StateHidden {
		return m
	}
	m.State = StateConfirm
	return m
}

// Confirm transitions the model from Confirm to Waiting.
// If the model is not in the Confirm state, it returns an unchanged copy.
func (m Model) Confirm() Model {
	if m.State != StateConfirm {
		return m
	}
	m.State = StateWaiting
	return m
}

// MarkDone transitions the model from Waiting to Done.
// If the model is not in the Waiting state, it returns an unchanged copy.
func (m Model) MarkDone() Model {
	if m.State != StateWaiting {
		return m
	}
	m.State = StateDone
	return m
}

// Hide transitions the model from any state to Hidden.
func (m Model) Hide() Model {
	m.State = StateHidden
	return m
}

// IsVisible returns true if the banner is in any visible state (not Hidden).
func (m Model) IsVisible() bool {
	return m.State != StateHidden
}

// CurrentState returns the current State value.
func (m Model) CurrentState() State {
	return m.State
}
