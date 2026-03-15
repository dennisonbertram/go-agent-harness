package contextgrid

// Model renders a grid showing context window usage and token counts.
type Model struct {
	// TotalTokens is the total context window size.
	TotalTokens int
	// UsedTokens is the number of tokens consumed.
	UsedTokens int
	// Width is the available rendering width.
	Width int
}

// New creates a new context grid model.
func New() Model {
	return Model{}
}

// View renders the context grid. Stub for now.
func (m Model) View() string {
	return ""
}
