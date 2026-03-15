package slashcomplete

// Model renders an autocomplete overlay for slash commands.
type Model struct {
	// Items is the list of completion candidates.
	Items []string
	// Selected is the index of the currently highlighted item.
	Selected int
	// Visible controls whether the overlay is shown.
	Visible bool
	// Width is the available rendering width.
	Width int
}

// New creates a new slash completion model.
func New() Model {
	return Model{}
}

// View renders the completion overlay. Stub for now.
func (m Model) View() string {
	return ""
}
