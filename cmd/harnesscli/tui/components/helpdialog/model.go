package helpdialog

// Model renders a help overlay showing available key bindings.
type Model struct {
	// Visible controls whether the dialog is shown.
	Visible bool
	// Width is the available rendering width.
	Width int
	// Height is the available rendering height.
	Height int
}

// New creates a new help dialog model.
func New() Model {
	return Model{}
}

// View renders the help dialog. Stub for now.
func (m Model) View() string {
	return ""
}
