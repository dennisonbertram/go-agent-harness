package statusbar

// Model is the status bar component displayed at the bottom of the TUI.
type Model struct {
	// Status text shown in the bar.
	Status string
	// Width of the status bar in columns.
	Width int
}

// New creates a new status bar model.
func New() Model {
	return Model{}
}

// View renders the status bar. Stub for now.
func (m Model) View() string {
	return ""
}
