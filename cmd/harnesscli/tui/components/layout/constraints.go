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
