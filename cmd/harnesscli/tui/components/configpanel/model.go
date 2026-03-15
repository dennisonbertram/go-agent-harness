package configpanel

// Model renders a configuration panel for runtime settings.
type Model struct {
	// Visible controls whether the panel is shown.
	Visible bool
	// Width is the available rendering width.
	Width int
	// Height is the available rendering height.
	Height int
}

// New creates a new config panel model.
func New() Model {
	return Model{}
}

// View renders the config panel. Stub for now.
func (m Model) View() string {
	return ""
}
