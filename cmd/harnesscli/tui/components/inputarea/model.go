package inputarea

// Model is the multiline text input component.
type Model struct {
	// Value holds the current input text.
	Value string
	// Width of the input area in columns.
	Width int
	// Focused indicates if the input area has focus.
	Focused bool
}

// New creates a new input area model.
func New() Model {
	return Model{}
}

// View renders the input area. Stub for now.
func (m Model) View() string {
	return ""
}
