package statspanel

// Model renders a panel showing run statistics (cost, tokens, duration).
type Model struct {
	// CostUSD is the accumulated cost in USD.
	CostUSD float64
	// TotalTokens is the total tokens used.
	TotalTokens int64
	// StepCount is the number of completed steps.
	StepCount int
	// Width is the available rendering width.
	Width int
}

// New creates a new stats panel model.
func New() Model {
	return Model{}
}

// View renders the stats panel. Stub for now.
func (m Model) View() string {
	return ""
}
