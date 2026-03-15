package thinkingbar

// Model is the thinking/loading indicator shown while the LLM is processing.
type Model struct {
	// Active indicates whether the thinking bar is visible.
	Active bool
	// Label is the text shown alongside the spinner.
	Label string
}

// New creates a new thinking bar model.
func New() Model {
	return Model{}
}

// View renders the thinking bar. Stub for now.
func (m Model) View() string {
	return ""
}
