package layout

// Constraints defines the layout boundaries for TUI components.
type Constraints struct {
	// MinWidth is the minimum terminal width supported.
	MinWidth int
	// MinHeight is the minimum terminal height supported.
	MinHeight int
	// StatusBarHeight is the height reserved for the status bar.
	StatusBarHeight int
	// InputAreaHeight is the default height of the input area.
	InputAreaHeight int
}

// DefaultConstraints returns the default layout constraints.
func DefaultConstraints() Constraints {
	return Constraints{
		MinWidth:        80,
		MinHeight:       24,
		StatusBarHeight: 1,
		InputAreaHeight: 3,
	}
}

// ViewportHeight calculates available height for the message viewport.
func (c Constraints) ViewportHeight(termHeight int) int {
	h := termHeight - c.StatusBarHeight - c.InputAreaHeight
	if h < 1 {
		return 1
	}
	return h
}

// Layout holds computed dimensions for all TUI regions.
type Layout struct {
	Width           int
	Height          int
	StatusBarHeight int // always 1
	SeparatorHeight int // always 1 (there are 2 separators: above+below viewport)
	ViewportHeight  int // remaining space after fixed elements
	InputHeight     int // min 3, grows with content up to maxInputLines
}

const (
	statusBarLines   = 1
	separatorLines   = 1 // each separator is 1 line; there are 2 total
	minInputLines    = 3
	minViewportLines = 3
	maxInputLines    = 8
	minWidth         = 20
)

// Compute calculates component dimensions for a terminal of given size.
// All returned values are >= 0. Very small terminals clamp gracefully.
func Compute(width, height int) Layout {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	l := Layout{
		Width:           width,
		Height:          height,
		StatusBarHeight: statusBarLines,
		SeparatorHeight: separatorLines,
		InputHeight:     minInputLines,
	}

	// Reserved: statusbar(1) + 2*separator(1) + input(3) = 6
	reserved := statusBarLines + 2*separatorLines + minInputLines
	remaining := height - reserved

	if remaining < 0 {
		// Tiny terminal: distribute what we have.
		// Give status bar and separators priority, then input, viewport gets 0.
		l.ViewportHeight = 0
		inputSpace := height - statusBarLines - 2*separatorLines
		if inputSpace < 0 {
			inputSpace = 0
		}
		l.InputHeight = inputSpace
		return l
	}

	// Viewport gets all remaining space.
	l.ViewportHeight = remaining
	return l
}
