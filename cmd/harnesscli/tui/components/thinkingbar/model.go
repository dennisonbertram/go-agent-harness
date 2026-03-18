package thinkingbar

const defaultLabel = "Thinking"

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

// View renders nothing while inactive and a single active status line otherwise.
func (m Model) View() string {
	if !m.Active {
		return ""
	}

	label := m.Label
	if label == "" {
		label = defaultLabel
	}

	return label + "..."
}
